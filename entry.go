package GojiDB

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"sync/atomic"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
)

type CompressionType byte

const (
	NoCompression CompressionType = iota
	SnappyCompression
	ZSTDCompression
	GzipCompression
	MixedCompression    // 混合压缩模式
	ParallelCompression // 并行压缩模式
	StreamCompression   // 流式压缩模式
)

type Entry struct {
	CRC         uint32
	Timestamp   uint32
	TTL         uint32
	KeySize     uint16
	ValueSize   uint32
	Compression CompressionType
	Key         []byte
	Value       []byte
}

type KeyDir struct {
	FileID      uint32
	ValueSize   uint32
	ValuePos    uint64
	Timestamp   uint32
	TTL         uint32
	Compression CompressionType
	Deleted     bool
}

const (
	HeaderSize      = 19 // CRC(4)+Timestamp(4)+TTL(4)+KeySize(2)+ValueSize(4)+Compression(1)
	DefaultFilePerm = 0644
	TombstoneValue  = "__DELETED__"
	SnapshotDir     = "snapshots"
	DBVersion       = "2.1.0"
	DBName          = "GojiDB"
)

// ======== 以下为单文件里 Entry 相关的工具函数，均搬过来 ========

func (db *GojiDB) compress(data []byte) ([]byte, CompressionType, error) {
	originalSize := len(data)
	if originalSize == 0 || db.config.CompressionType == NoCompression {
		return data, NoCompression, nil
	}

	// 使用智能压缩器进行压缩
	if db.smartCompressor != nil {
		compressed, algorithm, err := db.smartCompressor.CompressWithSmartChoice(data)
		if err == nil && len(compressed) > 0 {
			// 确保使用标准压缩类型，避免使用智能压缩器的特殊类型
			// 这样可以确保解压时的一致性
			return compressed, algorithm, err
		}
		// 如果智能压缩失败，回退到传统压缩
	}

	var compressed []byte
	var err error
	algorithm := db.config.CompressionType

	switch db.config.CompressionType {
	case SnappyCompression:
		compressed = snappy.Encode(nil, data)
	case ZSTDCompression:
		enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
		defer enc.Close()
		compressed = enc.EncodeAll(data, make([]byte, 0, len(data)))
	case GzipCompression:
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err = w.Write(data); err != nil {
			return nil, NoCompression, err
		}
		w.Close()
		compressed = buf.Bytes()
	default:
		return data, NoCompression, nil
	}

	// 计算并更新压缩比
	if db.config.EnableMetrics && originalSize > 0 {
		compressedSize := len(compressed)
		if compressedSize > 0 {
			ratio := float64(originalSize-compressedSize) / float64(originalSize)
			// 使用原子操作更新压缩比
			newRatio := int64(ratio * 10000) // 转换为百分比*100
			atomic.StoreInt64(&db.metrics.CompressionRatio, newRatio)
		}
	}

	return compressed, algorithm, err
}

func (db *GojiDB) decompress(data []byte, ct CompressionType) ([]byte, error) {
	// 使用智能压缩器进行解压
	if db.smartCompressor != nil {
		return db.smartCompressor.Decompress(data)
	}

	// 回退到传统解压方式
	switch ct {
	case NoCompression:
		return data, nil
	case SnappyCompression:
		return snappy.Decode(nil, data)
	case ZSTDCompression:
		dec, _ := zstd.NewReader(nil)
		defer dec.Close()
		return dec.DecodeAll(data, nil)
	case GzipCompression:
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	default:
		return data, nil
	}
}

func (db *GojiDB) serializeEntry(e *Entry) ([]byte, error) {
	// 计算实际需要的长度
	totalSize := HeaderSize + int(e.KeySize) + int(e.ValueSize)
	
	// 创建正确大小的切片
	buf := make([]byte, totalSize)
	
	binary.BigEndian.PutUint32(buf[0:4], e.CRC)
	binary.BigEndian.PutUint32(buf[4:8], e.Timestamp)
	binary.BigEndian.PutUint32(buf[8:12], e.TTL)
	binary.BigEndian.PutUint16(buf[12:14], e.KeySize)
	binary.BigEndian.PutUint32(buf[14:18], e.ValueSize)
	buf[18] = byte(e.Compression)
	copy(buf[HeaderSize:HeaderSize+int(e.KeySize)], e.Key)
	copy(buf[HeaderSize+int(e.KeySize):totalSize], e.Value)
	return buf, nil
}

func (db *GojiDB) readEntry(r *bufio.Reader) (*Entry, int, error) {
	// 使用Entry对象池
	e := GlobalEntryPool.GetEntry()
	
	// 读取头部
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		GlobalEntryPool.PutEntry(e)
		return nil, 0, err
	}
	
	e.CRC = binary.BigEndian.Uint32(header[0:4])
	e.Timestamp = binary.BigEndian.Uint32(header[4:8])
	e.TTL = binary.BigEndian.Uint32(header[8:12])
	e.KeySize = binary.BigEndian.Uint16(header[12:14])
	e.ValueSize = binary.BigEndian.Uint32(header[14:18])
	e.Compression = CompressionType(header[18])
	
	// 读取键
	if e.KeySize > 0 {
		e.Key = make([]byte, e.KeySize)
		if _, err := io.ReadFull(r, e.Key); err != nil {
			GlobalEntryPool.PutEntry(e)
			return nil, 0, err
		}
	}

	// 读取值
	if e.ValueSize > 0 {
		e.Value = make([]byte, e.ValueSize)
		if _, err := io.ReadFull(r, e.Value); err != nil {
			GlobalEntryPool.PutEntry(e)
			return nil, 0, err
		}
	}
	
	totalSize := HeaderSize + int(e.KeySize) + int(e.ValueSize)
	return e, totalSize, nil
}

func (db *GojiDB) calculateCRC(e *Entry) uint32 {
	// 使用CRC计算器对象池
	crc := GlobalCRC32Pool.GetCalculator()
	defer GlobalCRC32Pool.PutCalculator(crc)
	
	// 使用序列化缓冲区池
	buf := *GlobalSerBufPool.GetBuffer()
	defer GlobalSerBufPool.PutBuffer(&buf)
	
	// 计算CRC
	crc.Reset()
	
	// 写入时间戳
	buf = buf[:0]
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, e.Timestamp)
	buf = append(buf, tmp...)
	
	binary.BigEndian.PutUint32(tmp, e.TTL)
	buf = append(buf, tmp...)
	
	tmp2 := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp2, e.KeySize)
	buf = append(buf, tmp2...)
	
	binary.BigEndian.PutUint32(tmp, e.ValueSize)
	buf = append(buf, tmp...)
	
	buf = append(buf, byte(e.Compression))
	buf = append(buf, e.Key...)
	buf = append(buf, e.Value...)
	
	return crc.Sum32(buf)
}
