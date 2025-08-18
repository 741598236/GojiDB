package GojiDB

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CLI颜色定义
const (
	CliReset     = "\033[0m"
	CliBold      = "\033[1m"
	CliUnderline = "\033[4m"
	CliRed       = "\033[31m"
	CliGreen     = "\033[32m"
	CliYellow    = "\033[33m"
	CliBlue      = "\033[34m"
	CliPurple    = "\033[35m"
	CliCyan      = "\033[36m"
	CliGray      = "\033[37m"
)

// 主程序颜色常量（从main.go迁移）
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorPurple  = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
	ColorMagenta = "\033[35m"
	ColorBold    = "\033[1m"
	ColorDim     = "\033[2m"
)

// 表格边框字符
const (
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
	BoxTeeDown     = "┬"
	BoxTeeUp       = "┴"
	BoxTeeLeft     = "┤"
	BoxTeeRight    = "├"
	BoxCross       = "┼"
)

// 命令处理器类型定义
type commandHandler func(*GojiDB, []string) error

// commandRegistry 全局命令注册表
var commandRegistry = map[string]commandHandler{
	"help":        handleHelp,
	"h":           handleHelp,
	"?":           handleHelp,
	"put":         handlePut,
	"set":         handlePut,
	"get":         handleGet,
	"delete":      handleDelete,
	"del":         handleDelete,
	"rm":          handleDelete,
	"list":        handleList,
	"ls":          handleList,
	"ttl":         handleTTL,
	"snapshot":    handleSnapshot,
	"snap":        handleSnapshot,
	"stats":       handleStats,
	"status":      handleStats,
	"compact":     handleCompact,
	"clear":       handleClear,
	"info":        handleInfo,
	"history":     handleHistory,
	"batch":       handleBatch,
	"export":      handleExport,
	"transaction": handleTransaction,
	"tx":          handleTransaction,
	"snapshots":   handleSnapshots,
	"restore":     handleRestore,
	"health":      handleHealth,
	"search":      handleSearch,
	"import":      handleImport,
	"benchmark":   handleBenchmark,
	"monitor":     handleMonitor,
	"wal":         handleWAL,
	"exit":        handleExit,
	"quit":        handleExit,
	"q":           handleExit,
}

// StartInteractiveMode 简化的交互模式启动器
func (db *GojiDB) StartInteractiveMode() {
	printWelcome()
	
	cli := newInteractiveCLI(db)
	cli.run()
}

// interactiveCLI 封装交互式CLI的状态和行为
type interactiveCLI struct {
	db       *GojiDB
	scanner  *bufio.Scanner
	history  *commandHistory
}

// newInteractiveCLI 创建新的交互式CLI实例
func newInteractiveCLI(db *GojiDB) *interactiveCLI {
	return &interactiveCLI{
		db:      db,
		scanner: bufio.NewScanner(os.Stdin),
		history: newCommandHistory(),
	}
}

// run 运行主交互循环
func (cli *interactiveCLI) run() {
	defer cli.history.save()
	
	for {
		fmt.Printf("%s%s🍒 %s%s %s[%s]%s %s❯%s ", 
			CliBold, CliCyan, DBName, CliReset, 
			CliGray, time.Now().Format("15:04:05"), CliReset, 
			CliPurple, CliReset)

		if !cli.scanner.Scan() {
			break
		}

		line := strings.TrimSpace(cli.scanner.Text())
		if line == "" {
			continue
		}

		if err := cli.executeCommand(line); err != nil {
			if err == errExitCLI {
				return
			}
			fmt.Printf("%s❌ 错误: %v%s\n", CliRed, err, CliReset)
		}
	}
}

// executeCommand 执行单个命令
func (cli *interactiveCLI) executeCommand(line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	cmd := strings.ToLower(parts[0])
	
	// 保存到历史记录（排除退出命令）
	if cmd != "exit" && cmd != "quit" && cmd != "q" {
		cli.history.add(line)
	}

	// 查找并执行命令处理器
	if handler, exists := commandRegistry[cmd]; exists {
		return handler(cli.db, parts)
	}

	return fmt.Errorf("未知命令: %s (输入 'help' 获取帮助)", cmd)
}

// commandHistory 命令历史管理器
type commandHistory struct {
	entries []string
	file    string
}

func newCommandHistory() *commandHistory {
	h := &commandHistory{
		entries: make([]string, 0, 100),
		file:    "../logs/gojidb_history.json",
	}
	os.MkdirAll("../logs", 0755)
	h.load()
	return h
}

func (h *commandHistory) add(cmd string) {
	h.entries = append(h.entries, cmd)
	if len(h.entries) > 100 {
		h.entries = h.entries[len(h.entries)-100:]
	}
}

func (h *commandHistory) load() {
	if data, err := os.ReadFile(h.file); err == nil {
		json.Unmarshal(data, &h.entries)
	}
}

func (h *commandHistory) save() {
	if data, err := json.MarshalIndent(h.entries, "", "  "); err == nil {
		os.WriteFile(h.file, data, 0644)
	}
}

func (db *GojiDB) handleWAL(parts []string) {
	if len(parts) < 2 {
		db.showWALHelp()
		return
	}

	cmd := strings.ToLower(parts[1])
	subParts := parts[2:]

	switch cmd {
	case "status", "info":
		db.handleWALStatus()
	case "checkpoint", "check":
		db.handleWALCheckpoint()
	case "stats", "statistics":
		db.handleWALStats()
	case "toggle":
		db.handleWALToggle(subParts)
	case "config":
		db.handleWALConfig(subParts)
	default:
		fmt.Printf("%s❌ 未知WAL命令 '%s'%s\n", CliRed, cmd, CliReset)
		db.showWALHelp()
	}
}

func (db *GojiDB) showWALHelp() {
	help := fmt.Sprintf(`
%s%s🔍 WAL审计链管理命令:%s

`, CliBold+CliUnderline, CliCyan, CliReset) +
		fmt.Sprintf("%s📊 状态查看:%s\n", CliBold, CliReset) +
		fmt.Sprintf("  %swal status%s              - 查看WAL状态信息\n", CliGreen, CliReset) +
		fmt.Sprintf("  %swal stats%s               - 查看WAL统计信息\n", CliGreen, CliReset) +
		fmt.Sprintf("  %swal config%s              - 查看WAL配置\n", CliBlue, CliReset) +
		fmt.Sprintf("%s🔄 管理操作:%s\n", CliBold, CliReset) +
		fmt.Sprintf("  %swal checkpoint%s          - 手动创建WAL检查点\n", CliYellow, CliReset) +
		fmt.Sprintf("  %swal toggle [on|off]%s     - 启用/禁用WAL功能\n", CliPurple, CliReset) +
		fmt.Sprintf("\n%s💡 示例:%s\n", CliBold, CliReset) +
		fmt.Sprintf("  %swal status%s              # 查看当前WAL状态\n", CliGray, CliReset) +
		fmt.Sprintf("  %swal checkpoint%s          # 创建检查点清理WAL\n", CliGray, CliReset) +
		fmt.Sprintf("  %swal toggle off%s          # 临时禁用WAL功能\n", CliGray, CliReset)

	fmt.Println(help)
}

func (db *GojiDB) handleWALStatus() {
	fmt.Printf("%s%s📊 WAL审计链状态%s\n", CliBold+CliUnderline, CliCyan, CliReset)

	// WAL启用状态
	walEnabled := db.config.EnableWAL && db.walManager != nil
	status := "❌ 禁用"
	statusColor := CliRed
	if walEnabled {
		status = "✅ 启用"
		statusColor = CliGreen
	}

	// WAL文件大小
	walSize := int64(0)
	walPath := ""
	if walEnabled {
		walPath = filepath.Join(db.config.DataPath, "wal.log")
		if size, err := db.walManager.GetWALSize(); err == nil {
			walSize = size
		}
	}

	// 系统信息
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("%s┌─ 🔍 WAL状态 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 状态: %s%-45s%s│%s\n",
		CliGray, CliReset, statusColor, status, CliReset, CliGray)
	fmt.Printf("%s│%s 路径: %s%-45s%s│%s\n",
		CliGray, CliReset, CliBlue, walPath, CliReset, CliGray)
	fmt.Printf("%s│%s 大小: %s%-45s%s│%s\n",
		CliGray, CliReset, CliYellow, formatSize(walSize), CliReset, CliGray)
	fmt.Printf("%s│%s 同步写入: %s%-45s%s│%s\n",
		CliGray, CliReset, CliCyan, fmt.Sprintf("%t", db.config.WALSyncWrites), CliReset, CliGray)
	fmt.Printf("%s│%s 刷新间隔: %s%-45s%s│%s\n",
		CliGray, CliReset, CliPurple, db.config.WALFlushInterval, CliReset, CliGray)
	fmt.Printf("%s│%s 最大大小: %s%-45s%s│%s\n",
		CliGray, CliReset, CliGreen, formatSize(db.config.WALMaxSize), CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)
}

func (db *GojiDB) handleWALStats() {
	fmt.Printf("%s%s📈 WAL统计信息%s\n", CliBold+CliUnderline, CliCyan, CliReset)

	if !db.config.EnableWAL || db.walManager == nil {
		fmt.Printf("%s❌ WAL未启用，无统计信息%s\n", CliRed, CliReset)
		return
	}

	stats := db.GetMetrics()
	walSize, _ := db.walManager.GetWALSize()

	// 计算WAL相关指标
	uptime := time.Since(stats.StartTime)
	walWritesPerSecond := float64(stats.TotalWrites) / uptime.Seconds()

	fmt.Printf("%s┌─ 📊 WAL统计 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s WAL写入次数: %s%-40d%s│%s\n",
		CliGray, CliReset, CliGreen, stats.TotalWrites, CliReset, CliGray)
	fmt.Printf("%s│%s WAL写入速率: %s%-40.2f%s│%s\n",
		CliGray, CliReset, CliCyan, walWritesPerSecond, CliReset, CliGray)
	fmt.Printf("%s│%s WAL文件大小: %s%-40s%s│%s\n",
		CliGray, CliReset, CliYellow, formatSize(walSize), CliReset, CliGray)
	fmt.Printf("%s│%s 运行时间: %s%-40s%s│%s\n",
		CliGray, CliReset, CliBlue, uptime.Round(time.Second), CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)
}

func (db *GojiDB) handleWALCheckpoint() {
	fmt.Printf("%s%s🔄 创建WAL检查点%s\n", CliBold+CliUnderline, CliCyan, CliReset)

	if !db.config.EnableWAL || db.walManager == nil {
		fmt.Printf("%s❌ WAL未启用，无法创建检查点%s\n", CliRed, CliReset)
		return
	}

	// 获取当前WAL大小
	oldSize, _ := db.walManager.GetWALSize()

	fmt.Printf("%s🔄 正在创建检查点...%s\n", CliYellow, CliReset)
	start := time.Now()

	if err := db.walManager.Checkpoint(); err != nil {
		fmt.Printf("%s❌ 检查点创建失败: %v%s\n", CliRed, err, CliReset)
		return
	}

	// 获取新的大小
	newSize, _ := db.walManager.GetWALSize()
	elapsed := time.Since(start)

	fmt.Printf("%s✅ 检查点创建完成！%s\n", CliGreen, CliReset)
	fmt.Printf("%s📊 清理大小: %s%s%s\n", CliBlue, CliYellow, formatSize(oldSize-newSize), CliReset)
	fmt.Printf("%s⏱️  耗时: %s%s%s\n", CliBlue, CliCyan, elapsed, CliReset)
}

func (db *GojiDB) handleWALToggle(parts []string) {
	if len(parts) < 1 {
		fmt.Printf("%s❌ 用法: wal toggle [on|off]%s\n", CliRed, CliReset)
		return
	}

	action := strings.ToLower(parts[0])
	if action != "on" && action != "off" {
		fmt.Printf("%s❌ 参数必须是 'on' 或 'off'%s\n", CliRed, CliReset)
		return
	}

	currentStatus := db.config.EnableWAL
	newStatus := action == "on"

	if currentStatus == newStatus {
		fmt.Printf("%s💡 WAL已经是%s状态%s\n", CliYellow, action, CliReset)
		return
	}

	// 提供具体的配置修改指导
	fmt.Printf("%s⚠️  注意: 切换WAL状态需要重启数据库%s\n", CliYellow, CliReset)
	fmt.Printf("%s📋 当前配置: EnableWAL=%t%s\n", CliGray, currentStatus, CliReset)
	fmt.Printf("%s🔄 新配置: EnableWAL=%t%s\n", CliGray, newStatus, CliReset)
	fmt.Printf("%s💡 操作步骤：%s\n", CliGray, CliReset)
	fmt.Printf("%s   1. 修改 configs/config.yaml%s\n", CliGray, CliReset)
	fmt.Printf("%s   2. 在 GojiDB 配置部分设置: enable_wal: %t%s\n", CliGray, newStatus, CliReset)
	fmt.Printf("%s   3. 重启数据库%s\n", CliGray, CliReset)
}

func (db *GojiDB) handleWALConfig(parts []string) {
	fmt.Printf("%s%s⚙️ WAL配置详情%s\n", CliBold+CliUnderline, CliCyan, CliReset)

	fmt.Printf("%s┌─ ⚙️ WAL配置 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 启用状态: %s%-40t%s│%s\n",
		CliGray, CliReset, CliGreen, db.config.EnableWAL, CliReset, CliGray)
	fmt.Printf("%s│%s 数据路径: %s%-40s%s│%s\n",
		CliGray, CliReset, CliBlue, db.config.DataPath, CliReset, CliGray)
	fmt.Printf("%s│%s WAL路径: %s%-40s%s│%s\n",
		CliGray, CliReset, CliYellow, filepath.Join(db.config.DataPath, "wal.log"), CliReset, CliGray)
	fmt.Printf("%s│%s 同步写入: %s%-40t%s│%s\n",
		CliGray, CliReset, CliCyan, db.config.WALSyncWrites, CliReset, CliGray)
	fmt.Printf("%s│%s 刷新间隔: %s%-40s%s│%s\n",
		CliGray, CliReset, CliPurple, db.config.WALFlushInterval, CliReset, CliGray)
	fmt.Printf("%s│%s 最大大小: %s%-40s%s│%s\n",
		CliGray, CliReset, CliGreen, formatSize(db.config.WALMaxSize), CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)
}

func printWelcome() {
	fmt.Printf(`
%s%s╔══════════════════════════════════════════════════════╗%s
`, CliBold, CliCyan, CliReset)
	fmt.Printf(`%s%s║           %s🍒 GojiDB v2.1.0 智能交互终端%s               ║%s
`, CliBold, CliCyan, CliBold+CliWhite, CliCyan, CliReset)
	fmt.Printf(`%s%s║           %s✨ 高性能键值存储数据库%s                      ║%s
`, CliBold, CliCyan, CliBold+CliGreen, CliCyan, CliReset)
	fmt.Printf(`%s%s╚══════════════════════════════════════════════════════╝%s

`, CliBold, CliCyan, CliReset)
	fmt.Printf("%s💡 输入 %shelp%s 获取命令帮助，%sexit%s 退出程序%s\n\n", CliGray, CliYellow, CliGray, CliRed, CliGray, CliReset)
}

func (db *GojiDB) showHelp() {
	help := fmt.Sprintf(`
%s%s🎯 可用命令:%s

`, CliBold, CliCyan, CliReset) +
		fmt.Sprintf("%s📊 数据操作:%s\n", CliBold, CliReset) +
		fmt.Sprintf("  %sput/set <key> <value> [ttl]%s  - 存储键值对 (TTL可选)\n", CliGreen, CliReset) +
		fmt.Sprintf("  %sget <key>%s                - 获取键值\n", CliGreen, CliReset) +
		fmt.Sprintf("  %sdelete/del/rm <key>%s      - 删除键\n", CliGreen, CliReset) +
		fmt.Sprintf("  %slist/ls [pattern]%s        - 列出所有键或匹配模式的键\n", CliGreen, CliReset) +
		fmt.Sprintf("  %ssearch <pattern>%s         - 高级搜索键值\n\n", CliBlue, CliReset) +

		fmt.Sprintf("%s⏰ TTL管理:%s\n", CliBold, CliReset) +
		fmt.Sprintf("  %sttl <key>%s                - 查看键的剩余TTL\n\n", CliYellow, CliReset) +

		fmt.Sprintf("%s🔄 事务操作:%s\n", CliBold, CliReset) +
		fmt.Sprintf("  %stransaction/tx%s           - 进入事务模式\n\n", CliPurple, CliReset) +

		fmt.Sprintf("%s💾 快照管理:%s\n", CliBold, CliReset) +
		fmt.Sprintf("  %ssnapshot/snap%s            - 创建数据库快照\n", CliBlue, CliReset) +
		fmt.Sprintf("  %ssnapshots%s                - 列出所有快照\n", CliBlue, CliReset) +
		fmt.Sprintf("  %srestore <snapshot_id>%s    - 从快照恢复\n\n", CliBlue, CliReset) +

		fmt.Sprintf("%s🔧 系统管理:%s\n", CliBold, CliReset) +
		fmt.Sprintf("  %scompact%s                  - 手动触发数据合并\n", CliBlue, CliReset) +
		fmt.Sprintf("  %sstats/status%s             - 查看数据库统计信息\n", CliBlue, CliReset) +
		fmt.Sprintf("  %shealth%s                   - 系统健康检查\n", CliGreen, CliReset) +
		fmt.Sprintf("  %smonitor%s                  - 实时监控模式\n", CliCyan, CliReset) +
		fmt.Sprintf("  %swal <command>%s           - WAL审计链管理\n", CliCyan, CliReset) +
		fmt.Sprintf("  %sbenchmark%s                - 性能基准测试\n", CliYellow, CliReset) +
		fmt.Sprintf("  %simport <file>%s           - 导入数据\n", CliMagenta, CliReset) +
		fmt.Sprintf("  %sexport%s                   - 导出监控数据\n", CliCyan, CliReset) +
		fmt.Sprintf("  %sclear%s                    - 清屏\n", CliBlue, CliReset) +
		fmt.Sprintf("  %sinfo%s                     - 查看系统信息\n\n", CliBlue, CliReset) +

		fmt.Sprintf("%s⚙️  通用:%s\n", CliBold, CliReset) +
		fmt.Sprintf("  %shelp/h/?%s                 - 显示此帮助信息\n", CliPurple, CliReset) +
		fmt.Sprintf("  %shistory%s                  - 查看命令历史\n", CliGray, CliReset) +
		fmt.Sprintf("  %sbatch%s                    - 批量操作模式\n", CliYellow, CliReset) +
		fmt.Sprintf("  %sexit/quit/q%s              - 退出程序\n", CliRed, CliReset)

	fmt.Println(help)
}

func (db *GojiDB) handlePut(parts []string) {
	if len(parts) < 3 {
		fmt.Printf("%s❌ 用法: put <key> <value> [ttl]%s\n", CliRed, CliReset)
		return
	}

	key := parts[1]
	rest := parts[2:]

	// 处理带引号的值
	value := strings.Join(rest, " ")
	var ttl time.Duration

	// 检查最后一个参数是否为数字(TTL)
	if len(rest) > 1 {
		if sec, err := strconv.Atoi(rest[len(rest)-1]); err == nil {
			ttl = time.Duration(sec) * time.Second
			value = strings.Join(rest[:len(rest)-1], " ")
		}
	}

	// 移除引号
	value = strings.Trim(value, `"'`)

	if err := db.PutWithTTL(key, []byte(value), ttl); err != nil {
		fmt.Printf("%s❌ 存储失败: %v%s\n", CliRed, err, CliReset)
	} else {
		fmt.Printf("%s✅ 存储成功: %s%s%s\n", CliGreen, CliCyan, key, CliReset)
		if ttl > 0 {
			fmt.Printf("   %s⏰ TTL: %v%s\n", CliGray, ttl, CliReset)
		}
	}
}

func (db *GojiDB) handleGet(parts []string) {
	if len(parts) < 2 {
		fmt.Printf("%s❌ 用法: get <key>%s\n", CliRed, CliReset)
		return
	}

	key := parts[1]
	value, err := db.Get(key)
	if err != nil {
		fmt.Printf("%s❌ 读取失败: %v%s\n", CliRed, err, CliReset)
	} else {
		fmt.Printf("%s%s%s = %s\"%s\"%s\n", CliCyan, key, CliReset, CliGreen, string(value), CliReset)

		// 显示TTL信息
		if ttl, err := db.GetTTL(key); err == nil && ttl > 0 {
			fmt.Printf("   %s⏰ TTL: %v%s\n", CliGray, ttl, CliReset)
		}
	}
}

func (db *GojiDB) handleDelete(parts []string) {
	if len(parts) < 2 {
		fmt.Printf("%s❌ 用法: delete <key>%s\n", CliRed, CliReset)
		return
	}

	key := parts[1]
	if err := db.Delete(key); err != nil {
		fmt.Printf("%s❌ 删除失败: %v%s\n", CliRed, err, CliReset)
	} else {
		fmt.Printf("%s✅ 删除成功: %s%s%s\n", CliGreen, CliCyan, key, CliReset)
	}
}

func (db *GojiDB) handleList(parts []string) {
	keys := db.ListKeys()
	if len(keys) == 0 {
		fmt.Printf("%s📭 数据库为空%s\n", CliGray, CliReset)
		return
	}

	// 过滤模式
	pattern := ""
	if len(parts) > 1 {
		pattern = parts[1]
	}

	filtered := make([]string, 0)
	for _, k := range keys {
		if pattern == "" || strings.Contains(k, pattern) {
			filtered = append(filtered, k)
		}
	}

	if len(filtered) == 0 {
		if pattern != "" {
			fmt.Printf("%s🔍 没有找到匹配 '%s' 的键%s\n", CliGray, pattern, CliReset)
		} else {
			fmt.Printf("%s📭 数据库为空%s\n", CliGray, CliReset)
		}
		return
	}

	fmt.Printf("%s%s📋 键列表 (%d 个键)%s\n", CliBold+CliUnderline, CliCyan, len(filtered), CliReset)

	// 创建表格格式
	border := CliGray + "┌" + strings.Repeat("─", 27) + "┬" + strings.Repeat("─", 15) + "┬" + strings.Repeat("─", 22) + "┐" + CliReset
	header := CliGray + "│" + CliReset + " %-25s │ %-13s │ %-20s " + CliGray + "│" + CliReset
	sep := CliGray + "├" + strings.Repeat("─", 27) + "┼" + strings.Repeat("─", 15) + "┼" + strings.Repeat("─", 22) + "┤" + CliReset
	fmt.Println(border)
	fmt.Printf(header+"\n", "键名", "TTL/状态", "预览值")
	fmt.Println(sep)

	permanent := 0
	expiring := 0

	for _, k := range filtered {
		value, err := db.Get(k)
		if err != nil {
			continue
		}
		preview := string(value)
		if len(preview) > 20 {
			preview = preview[:17] + "..."
		}

		ttl, _ := db.GetTTL(k)
		var ttlStr string
		var ttlColor string
		if ttl > 0 {
			ttlStr = fmt.Sprintf("⏰ %s", ttl.Round(time.Second))
			ttlColor = CliYellow
			expiring++
		} else {
			ttlStr = "♾️ 永久有效"
			ttlColor = CliGreen
			permanent++
		}

		fmt.Printf(CliGray+"│"+CliReset+" %-25s "+CliGray+"│"+CliReset+" %s%-13s "+CliGray+"│"+CliReset+" \"%s\" "+CliGray+"│"+CliReset+"\n",
			k, ttlColor, ttlStr, preview)
	}

	fmt.Println(CliGray + "└" + strings.Repeat("─", 27) + "┴" + strings.Repeat("─", 15) + "┴" + strings.Repeat("─", 22) + "┘" + CliReset)

	fmt.Printf("\n%s📊 统计: %s永久有效: %s%d%s, 有过期时间: %s%d%s\n",
		CliBold, CliReset, CliGreen, permanent, CliReset, CliYellow, expiring, CliReset)
}

func (db *GojiDB) handleTTL(parts []string) {
	if len(parts) < 2 {
		fmt.Printf("%s❌ 用法: ttl <key>%s\n", CliRed, CliReset)
		return
	}

	key := parts[1]
	duration, err := db.GetTTL(key)
	if err != nil {
		fmt.Printf("%s❌ 获取TTL失败: %v%s\n", CliRed, err, CliReset)
	} else if duration == -1 {
		fmt.Printf("%s⏰ 键 '%s' 永不过期%s\n", CliBlue, key, CliReset)
	} else {
		fmt.Printf("%s⏰ 键 '%s' 剩余TTL: %s%s%s\n", CliCyan, key, CliGreen, duration, CliReset)
	}
}

func (db *GojiDB) handleSnapshot(parts []string) {
	fmt.Printf("%s%s📸 正在创建快照...%s\n", CliBold, CliCyan, CliReset)

	start := time.Now()
	snap, err := db.CreateSnapshot()
	if err != nil {
		fmt.Printf("%s%s❌ 快照失败: %v%s\n", CliBold, CliRed, err, CliReset)
		return
	}
	elapsed := time.Since(start)

	fmt.Printf("%s%s✅ 快照创建成功%s\n", CliBold+CliUnderline, CliGreen, CliReset)
	fmt.Printf("%s┌─ 📸 快照详情 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 🆔 快照ID: %s%-36s%s│%s\n", CliGray, CliReset, CliYellow, snap.ID, CliReset, CliGray)
	fmt.Printf("%s│%s ⏰ 创建时间: %s%-34s%s│%s\n", CliGray, CliReset, CliCyan, snap.Timestamp.Format("2006-01-02 15:04:05"), CliReset, CliGray)
	fmt.Printf("%s│%s 📊 键数: %s%-38d%s│%s\n", CliGray, CliReset, CliGreen, snap.KeyCount, CliReset, CliGray)
	fmt.Printf("%s│%s 💾 大小: %s%-38s%s│%s\n", CliGray, CliReset, CliPurple, formatSize(int64(snap.DataSize)), CliReset, CliGray)
	fmt.Printf("%s│%s ⚡ 耗时: %s%-38s%s│%s\n", CliGray, CliReset, CliBlue, elapsed.Round(time.Millisecond), CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)
}

func (db *GojiDB) handleStats() {
	stats := db.GetMetrics()
	keys := db.ListKeys()

	// 初始化默认值
	var hitRate float64
	var qps float64
	var uptime time.Duration = time.Since(time.Now())

	if stats != nil {
		// 计算缓存命中率
		totalRequests := stats.CacheHits + stats.CacheMisses
		if totalRequests > 0 {
			hitRate = float64(stats.CacheHits) / float64(totalRequests) * 100
		}

		// 计算QPS
		uptime = time.Since(stats.StartTime)
		if uptime.Seconds() > 0 {
			totalOps := stats.TotalReads + stats.TotalWrites + stats.TotalDeletes
			qps = float64(totalOps) / uptime.Seconds()
		}
	} else {
		// 使用默认空值
		stats = &Metrics{
			CacheHits:        0,
			CacheMisses:      0,
			TotalReads:       0,
			TotalWrites:      0,
			TotalDeletes:     0,
			CompactionCount:  0,
			ExpiredKeyCount:  0,
			CompressionRatio: 0,
			StartTime:        time.Now(),
		}
	}

	fmt.Printf("%s%s📊 GojiDB 智能统计面板%s\n", CliBold+CliUnderline, CliCyan, CliReset)

	// 基本信息卡片
	fmt.Printf("%s┌─ 🗄️  数据概览 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 💾 总键数: %s%-12d%s 🕐 运行时间: %s%-12s%s│%s\n",
		CliGray, CliReset, CliYellow, len(keys), CliReset, CliBlue, uptime.Round(time.Second), CliReset, CliGray)
	fmt.Printf("%s│%s 📖 总读取: %s%-12d%s ✏️  总写入: %s%-12d%s│%s\n",
		CliGray, CliReset, CliGreen, stats.TotalReads, CliReset, CliGreen, stats.TotalWrites, CliReset, CliGray)
	fmt.Printf("%s│%s 🗑️  总删除: %s%-12d%s 🔄 合并次数: %s%-12d%s│%s\n",
		CliGray, CliReset, CliRed, stats.TotalDeletes, CliReset, CliPurple, stats.CompactionCount, CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n\n", CliGray, CliReset)

	// 性能指标卡片
	fmt.Printf("%s┌─ ⚡ 性能指标 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 🎯 缓存命中: %s%-10d%s ❌ 未命中: %s%-10d%s│%s\n",
		CliGray, CliReset, CliGreen, stats.CacheHits, CliReset, CliRed, stats.CacheMisses, CliReset, CliGray)
	fmt.Printf("%s│%s 📈 命中率: %s%-10.2f%%%s ⚡ QPS: %s%-10.2f%s│%s\n",
		CliGray, CliReset, CliYellow, hitRate, CliReset, CliCyan, qps, CliReset, CliGray)
	fmt.Printf("%s│%s ⏰ 过期键: %s%-10d%s 📊 压缩比: %s%-10.2f%s│%s\n",
		CliGray, CliReset, CliRed, stats.ExpiredKeyCount, CliReset, CliPurple, float64(stats.CompressionRatio)/100.0, CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n\n", CliGray, CliReset)

	// 存储信息卡片
	fmt.Printf("%s┌─ 💿 存储信息 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	if db.activeFile != nil {
		stat, _ := db.activeFile.Stat()
		fmt.Printf("%s│%s 📄 活跃文件: %s%-20s%s 💾 文件大小: %s%-8s%s│%s\n",
			CliGray, CliReset, CliPurple, stat.Name(), CliReset, CliYellow, formatSize(stat.Size()), CliReset, CliGray)
	}
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)

	// 性能评级
	var rating string
	var ratingColor string
	if qps > 10000 {
		rating = "🚀 极致性能"
		ratingColor = CliGreen + CliBold
	} else if qps > 1000 {
		rating = "⚡ 优秀性能"
		ratingColor = CliGreen
	} else if qps > 100 {
		rating = "✅ 良好性能"
		ratingColor = CliYellow
	} else {
		rating = "⚠️  正常性能"
		ratingColor = CliBlue
	}

	fmt.Printf("\n%s📊 性能评级: %s%s%s\n", CliBold, ratingColor, rating, CliReset)
}

func (db *GojiDB) handleCompact() {
	fmt.Printf("%s%s🧹 正在执行数据合并...%s\n", CliBold, CliYellow, CliReset)
	fmt.Printf("%s⚡ 此操作将优化存储空间并提升性能%s\n", CliGray, CliReset)

	start := time.Now()

	// 获取合并前的统计
	_ = db.GetMetrics()
	_ = len(db.ListKeys())

	err := db.Compact()
	if err != nil {
		fmt.Printf("%s%s❌ 数据合并失败: %v%s\n", CliBold, CliRed, err, CliReset)
		return
	}

	elapsed := time.Since(start)

	// 获取合并后的统计
	afterStats := db.GetMetrics()
	afterKeys := len(db.ListKeys())

	fmt.Printf("%s%s✅ 数据合并完成%s\n", CliBold+CliUnderline, CliGreen, CliReset)
	fmt.Printf("%s┌─ 🧹 合并结果 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s ⏱️  耗时: %s%-35s%s│%s\n",
		CliGray, CliReset, CliCyan, elapsed.Round(time.Millisecond), CliReset, CliGray)
	fmt.Printf("%s│%s 📊 键数: %s%-35d%s│%s\n",
		CliGray, CliReset, CliGreen, afterKeys, CliReset, CliGray)
	fmt.Printf("%s│%s 🔄 合并次数: %s%-35d%s│%s\n",
		CliGray, CliReset, CliPurple, afterStats.CompactionCount, CliReset, CliGray)
	fmt.Printf("%s│%s 🎯 性能提升: %s%-35s%s│%s\n",
		CliGray, CliReset, CliYellow, "存储空间已优化", CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)
}

func (db *GojiDB) handleClear() {
	fmt.Print("\033[2J\033[H")
	printWelcome()
}

func (db *GojiDB) handleInfo() {
	details := getSystemDetails()

	fmt.Printf("%s%sℹ️  系统信息%s\n", CliBold, CliCyan, CliReset)
	fmt.Printf("%s┌─ 系统 ─────────────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 操作系统: %s%-15s%s 架构: %s%-15s%s│%s\n",
		CliGray, CliReset, CliYellow, details["os"], CliReset, CliYellow, details["arch"], CliReset, CliGray)
	fmt.Printf("%s│%s Go版本: %s%-15s%s 编译器: %s%-15s%s│%s\n",
		CliGray, CliReset, CliGreen, details["goversion"], CliReset, CliGreen, details["compiler"], CliReset, CliGray)
	fmt.Printf("%s│%s CPU核心: %s%-15s%s Goroutines: %s%-15s%s│%s\n",
		CliGray, CliReset, CliBlue, details["cpus"], CliReset, CliBlue, details["goroutines"], CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)
}

func (db *GojiDB) handleHistory(history []string) {
	fmt.Printf("%s%s命令历史:%s\n", CliBold, CliCyan, CliReset)
	if len(history) == 0 {
		fmt.Printf("  %s暂无历史记录%s\n", CliGray, CliReset)
		return
	}

	start := 0
	if len(history) > 10 {
		start = len(history) - 10
	}

	for i, cmd := range history[start:] {
		fmt.Printf("  %s%d:%s %s%s%s\n", CliGray, start+i+1, CliReset, CliYellow, cmd, CliReset)
	}
	fmt.Printf("  %s共 %d 条记录%s\n", CliGray, len(history), CliReset)
}

func (db *GojiDB) handleBatch() {
	fmt.Printf("%s%s🔄 进入批量操作模式%s\n", CliBold, CliYellow, CliReset)
	fmt.Printf("%s可用命令:%s\n", CliCyan, CliReset)
	fmt.Printf("  %sput <key> <value>%s - 添加批量写入项\n", CliGreen, CliReset)
	fmt.Printf("  %sget <key1,key2,...>%s - 批量读取键\n", CliBlue, CliReset)
	fmt.Printf("  %sexecute%s - 执行批量操作\n", CliYellow, CliReset)
	fmt.Printf("  %scancel%s - 取消批量操作\n", CliRed, CliReset)

	batchItems := make(map[string][]byte)
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("%s[batch] %s❯%s ", CliBold, CliPurple, CliReset)

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "put":
			if len(parts) < 3 {
				fmt.Printf("%s❌ 用法: put <key> <value>%s\n", CliRed, CliReset)
				continue
			}
			key := parts[1]
			value := strings.Join(parts[2:], " ")
			// 移除引号
			value = strings.Trim(value, `"'`)
			batchItems[key] = []byte(value)
			fmt.Printf("%s✅ 添加: %s%s%s = %s\"%s\"%s\n", CliGreen, CliCyan, key, CliReset, CliYellow, value, CliReset)

		case "get":
			if len(parts) < 2 {
				fmt.Printf("%s❌ 用法: get <key1,key2,...>%s\n", CliRed, CliReset)
				continue
			}
			keys := strings.Split(parts[1], ",")
			results, err := db.BatchGet(keys)
			if err != nil {
				fmt.Printf("%s❌ 批量读取失败: %v%s\n", CliRed, err, CliReset)
				continue
			}

			fmt.Printf("%s📋 批量读取结果:%s\n", CliCyan, CliReset)
			for key, value := range results {
				preview := string(value)
				if len(preview) > 50 {
					preview = preview[:47] + "..."
				}
				fmt.Printf("  %s%s%s = %s\"%s\"%s\n", CliGreen, key, CliReset, CliYellow, preview, CliReset)
			}

		case "execute":
			if len(batchItems) == 0 {
				fmt.Printf("%s⚠️  没有待执行的批量操作%s\n", CliYellow, CliReset)
				continue
			}

			fmt.Printf("%s🔄 执行批量写入 %d 项...%s\n", CliYellow, len(batchItems), CliReset)
			start := time.Now()

			err := db.BatchPut(batchItems)
			elapsed := time.Since(start)

			if err != nil {
				fmt.Printf("%s❌ 批量写入失败: %v%s\n", CliRed, err, CliReset)
			} else {
				fmt.Printf("%s✅ 批量写入完成! 耗时: %v%s\n", CliGreen, elapsed, CliReset)
				batchItems = make(map[string][]byte)
			}

		case "cancel":
			fmt.Printf("%s🚫 取消批量操作%s\n", CliRed, CliReset)
			return

		case "help":
			fmt.Printf("%s批量操作帮助:%s\n", CliCyan, CliReset)
			fmt.Printf("  put <key> <value> - 添加键值对到批量\n")
			fmt.Printf("  get <k1,k2,k3> - 批量读取指定键\n")
			fmt.Printf("  execute - 执行所有批量写入\n")
			fmt.Printf("  cancel - 取消并退出批量模式\n")

		default:
			fmt.Printf("%s❌ 未知命令 '%s'，输入 'help' 查看帮助%s\n", CliRed, cmd, CliReset)
		}
	}
}

const CliWhite = "\033[97m"
const CliMagenta = "\033[95m"

func (db *GojiDB) handleExport() {
	metrics := db.GetMetrics()
	keys := db.ListKeys()

	// 获取文件统计信息
	var totalSize int64
	if db.activeFile != nil {
		if stat, err := db.activeFile.Stat(); err == nil {
			totalSize = stat.Size()
		}
	}

	// 构建导出数据
	exportData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"exported_at":   time.Now().Format("2006-01-02 15:04:05"),
			"uptime":        time.Since(metrics.StartTime).String(),
			"database_name": DBName,
		},
		"statistics": map[string]interface{}{
			"total_keys":       len(keys),
			"total_reads":      metrics.TotalReads,
			"total_writes":     metrics.TotalWrites,
			"total_deletes":    metrics.TotalDeletes,
			"cache_hits":       metrics.CacheHits,
			"cache_misses":     metrics.CacheMisses,
			"expired_keys":     metrics.ExpiredKeyCount,
			"compaction_count": metrics.CompactionCount,
			"data_size":        totalSize,
		},
		"system_info": map[string]interface{}{
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
		},
		"features": map[string]interface{}{
			"compression": true,
			"ttl_support": db.config.TTLCheckInterval > 0,
		},
	}

	// 生成文件名
	filename := fmt.Sprintf("gojidb_metrics_%s.json", time.Now().Format("20060102_150405"))

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		fmt.Printf("%s%s❌ 生成JSON数据失败: %v%s\n", CliBold, CliRed, err, CliReset)
		return
	}

	// 写入文件
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		fmt.Printf("%s%s❌ 写入文件失败: %v%s\n", CliBold, CliRed, err, CliReset)
		return
	}

	// 获取文件信息
	fileInfo, _ := os.Stat(filename)

	fmt.Printf("%s%s✅ 监控数据导出完成%s\n", CliBold+CliUnderline, CliGreen, CliReset)
	fmt.Printf("%s┌─ 📊 导出详情 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 📁 文件名: %s%-35s%s│%s\n",
		CliGray, CliReset, CliYellow, filename, CliReset, CliGray)
	fmt.Printf("%s│%s 💾 文件大小: %s%-35s%s│%s\n",
		CliGray, CliReset, CliCyan, formatSize(fileInfo.Size()), CliReset, CliGray)
	fmt.Printf("%s│%s 📊 键数: %s%-35d%s│%s\n",
		CliGray, CliReset, CliGreen, len(keys), CliReset, CliGray)
	fmt.Printf("%s│%s ⏰ 导出时间: %s%-35s%s│%s\n",
		CliGray, CliReset, CliBlue, time.Now().Format("2006-01-02 15:04:05"), CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)

	fmt.Printf("%s💡 提示: 使用 'import %s' 可导入此数据%s\n", CliGray, filename, CliReset)
}

func (db *GojiDB) handleTransaction() {
	fmt.Printf("%s%s🔄 事务模式%s\n", CliBold, CliPurple, CliReset)
	fmt.Printf("%s开始新事务...%s\n", CliCyan, CliReset)

	tx, err := db.BeginTransaction()
	if err != nil {
		fmt.Printf("%s❌ 创建事务失败: %v%s\n", CliRed, err, CliReset)
		return
	}

	fmt.Printf("%s✅ 事务已创建: %s%s%s\n", CliGreen, CliYellow, tx.id, CliReset)
	fmt.Printf("%s可用命令:%s\n", CliCyan, CliReset)
	fmt.Printf("  %sput <key> <value>%s - 添加键值对到事务\n", CliGreen, CliReset)
	fmt.Printf("  %sdel <key>%s - 从事务中删除键\n", CliRed, CliReset)
	fmt.Printf("  %scommit%s - 提交事务\n", CliYellow, CliReset)
	fmt.Printf("  %srollback%s - 回滚事务\n", CliRed, CliReset)
	fmt.Printf("  %scancel%s - 退出事务模式\n", CliGray, CliReset)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s[tx:%s] %s❯%s ", CliBold, CliYellow+tx.id[:8]+CliReset, CliPurple, CliReset)

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "put":
			if len(parts) < 3 {
				fmt.Printf("%s❌ 用法: put <key> <value>%s\n", CliRed, CliReset)
				continue
			}
			key := parts[1]
			value := strings.Join(parts[2:], " ")
			value = strings.Trim(value, `"'`)
			if err := tx.Put(key, []byte(value)); err != nil {
				fmt.Printf("%s❌ 事务PUT失败: %v%s\n", CliRed, err, CliReset)
			} else {
				fmt.Printf("%s✅ 已添加到事务: %s%s%s = %s\"%s\"%s\n", CliGreen, CliCyan, key, CliReset, CliYellow, value, CliReset)
			}

		case "del", "delete":
			if len(parts) < 2 {
				fmt.Printf("%s❌ 用法: del <key>%s\n", CliRed, CliReset)
				continue
			}
			key := parts[1]
			if err := tx.Delete(key); err != nil {
				fmt.Printf("%s❌ 事务DELETE失败: %v%s\n", CliRed, err, CliReset)
			} else {
				fmt.Printf("%s🗑️  已标记删除: %s%s%s\n", CliYellow, CliCyan, key, CliReset)
			}

		case "commit":
			if err := tx.Commit(); err != nil {
				fmt.Printf("%s❌ 事务提交失败: %v%s\n", CliRed, err, CliReset)
			} else {
				fmt.Printf("%s✅ 事务提交成功！%s\n", CliGreen, CliReset)
			}
			return

		case "rollback":
			if err := tx.Rollback(); err != nil {
				fmt.Printf("%s❌ 事务回滚失败: %v%s\n", CliRed, err, CliReset)
			} else {
				fmt.Printf("%s🔄 事务已回滚%s\n", CliYellow, CliReset)
			}
			return

		case "cancel", "exit":
			_ = tx.Rollback()
			fmt.Printf("%s🚫 已退出事务模式%s\n", CliRed, CliReset)
			return

		case "help":
			fmt.Printf("%s事务模式帮助:%s\n", CliCyan, CliReset)
			fmt.Printf("  put <key> <value> - 添加键值对\n")
			fmt.Printf("  del <key> - 删除键\n")
			fmt.Printf("  commit - 提交所有操作\n")
			fmt.Printf("  rollback - 取消所有操作\n")
			fmt.Printf("  cancel - 退出事务模式\n")

		default:
			fmt.Printf("%s❌ 未知命令 '%s'，输入 'help' 查看帮助%s\n", CliRed, cmd, CliReset)
		}
	}
}

func (db *GojiDB) handleSnapshots() {
	fmt.Printf("%s%s📸 快照管理%s\n", CliBold, CliCyan, CliReset)

	snapshots, err := db.ListSnapshots()
	if err != nil {
		fmt.Printf("%s❌ 获取快照列表失败: %v%s\n", CliRed, err, CliReset)
		return
	}

	if len(snapshots) == 0 {
		fmt.Printf("%s📭 暂无快照%s\n", CliGray, CliReset)
		return
	}

	fmt.Printf("%s📋 找到 %d 个快照:%s\n", CliBold, len(snapshots), CliReset)
	fmt.Println(CliGray + "┌" + strings.Repeat("─", 25) + "┬" + strings.Repeat("─", 19) + "┬" + strings.Repeat("─", 8) + "┬" + strings.Repeat("─", 12) + "┐" + CliReset)
	fmt.Printf(CliGray+"│"+CliReset+" %s%-23s%s "+CliGray+"│"+CliReset+" %s%-17s%s "+CliGray+"│"+CliReset+" %s%-6s%s "+CliGray+"│"+CliReset+" %s%-10s%s "+CliGray+"│"+CliReset+"\n",
		CliBold+CliCyan, "快照ID", CliReset+CliGray, CliBold+CliCyan, "创建时间", CliReset+CliGray, CliBold+CliCyan, "键数", CliReset+CliGray, CliBold+CliCyan, "大小", CliReset+CliGray)

	for _, snap := range snapshots {
		shortID := snap.ID
		if len(shortID) > 20 {
			shortID = shortID[:20] + "..."
		}
		fmt.Println(CliGray + "├" + strings.Repeat("─", 25) + "┼" + strings.Repeat("─", 19) + "┼" + strings.Repeat("─", 8) + "┼" + strings.Repeat("─", 12) + "┤" + CliReset)
		fmt.Printf(CliGray+"│"+CliReset+" %s%-23s%s "+CliGray+"│"+CliReset+" %s%-17s%s "+CliGray+"│"+CliReset+" %s%-6d%s "+CliGray+"│"+CliReset+" %s%-10s%s "+CliGray+"│"+CliReset+"\n",
			CliYellow, shortID, CliReset+CliGray, CliGreen, snap.Timestamp.Format("01-02 15:04"), CliReset+CliGray, CliBlue, snap.KeyCount, CliReset+CliGray, CliPurple, formatSize(snap.DataSize), CliReset+CliGray)
	}
	fmt.Println(CliGray + "└" + strings.Repeat("─", 25) + "┴" + strings.Repeat("─", 19) + "┴" + strings.Repeat("─", 8) + "┴" + strings.Repeat("─", 12) + "┘" + CliReset)
}

func (db *GojiDB) handleRestore(parts []string) {
	if len(parts) < 2 {
		fmt.Printf("%s❌ 用法: restore <snapshot_id>%s\n", CliRed, CliReset)
		return
	}

	snapshotID := parts[1]
	fmt.Printf("%s⚠️  即将从快照 %s%s%s 恢复数据库...%s\n", CliYellow, CliCyan, snapshotID, CliYellow, CliReset)
	fmt.Printf("%s⚠️  此操作将覆盖当前所有数据！%s\n", CliRed, CliReset)
	fmt.Printf("%s是否继续? (y/N): %s", CliYellow, CliReset)

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return
	}

	response := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if response != "y" && response != "yes" {
		fmt.Printf("%s❌ 已取消恢复操作%s\n", CliRed, CliReset)
		return
	}

	fmt.Printf("%s🔄 正在恢复数据库...%s\n", CliYellow, CliReset)
	start := time.Now()

	if err := db.RestoreFromSnapshot(snapshotID); err != nil {
		fmt.Printf("%s❌ 恢复失败: %v%s\n", CliRed, err, CliReset)
	} else {
		elapsed := time.Since(start)
		fmt.Printf("%s✅ 恢复完成！耗时: %s%s%s\n", CliGreen, CliCyan, elapsed, CliReset)
	}
}

func (db *GojiDB) handleHealth() {
	fmt.Printf("%s%s🏥 GojiDB 健康检查%s\n", CliBold+CliUnderline, CliCyan, CliReset)

	start := time.Now()

	// 检查文件系统
	var activeFileSize int64
	if db.activeFile != nil {
		if stat, err := db.activeFile.Stat(); err == nil {
			activeFileSize = stat.Size()
		}
	}

	// 内存使用
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memUsage := formatSize(int64(m.Alloc))
	memTotal := formatSize(int64(m.Sys))

	// 获取统计信息
	stats := db.GetMetrics()
	keys := db.ListKeys()

	// 计算QPS
	uptime := time.Since(stats.StartTime)
	qps := float64(stats.TotalReads+stats.TotalWrites+stats.TotalDeletes) / uptime.Seconds()

	// 计算缓存命中率
	cacheHitRate := 0.0
	if stats.CacheHits+stats.CacheMisses > 0 {
		cacheHitRate = float64(stats.CacheHits) / float64(stats.CacheHits+stats.CacheMisses) * 100
	}

	// 健康评分计算
	healthScore := 100
	var issues []string

	if qps < 50 {
		healthScore -= 15
		issues = append(issues, "QPS较低，可能需要优化")
	}

	if len(keys) == 0 {
		healthScore -= 5
		issues = append(issues, "数据库为空")
	}

	if int(stats.ExpiredKeyCount) > len(keys)/3 {
		healthScore -= 10
		issues = append(issues, "过期键比例较高")
	}

	if cacheHitRate < 80 {
		healthScore -= 10
		issues = append(issues, "缓存命中率偏低")
	}

	if m.Alloc > m.Sys/2 {
		healthScore -= 5
		issues = append(issues, "内存使用较高")
	}

	// 健康状态评级
	var status string
	var statusColor string
	if healthScore >= 90 {
		status = "🟢 优秀"
		statusColor = CliGreen + CliBold
	} else if healthScore >= 75 {
		status = "🟡 良好"
		statusColor = CliYellow + CliBold
	} else if healthScore >= 60 {
		status = "🟠 一般"
		statusColor = CliYellow
	} else {
		status = "🔴 需要关注"
		statusColor = CliRed + CliBold
	}

	fmt.Printf("%s┌─ 🏥 健康概览 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 系统状态: %s%-45s%s│%s\n",
		CliGray, CliReset, statusColor, status, CliReset, CliGray)
	fmt.Printf("%s│%s 健康评分: %s%-45d/100%s│%s\n",
		CliGray, CliReset, CliYellow, healthScore, CliReset, CliGray)
	fmt.Printf("%s│%s 检查耗时: %s%-45s%s│%s\n",
		CliGray, CliReset, CliBlue, time.Since(start).Round(time.Millisecond), CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n\n", CliGray, CliReset)

	fmt.Printf("%s┌─ 📊 系统指标 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 💾 总键数: %s%-35d%s│%s\n",
		CliGray, CliReset, CliGreen, len(keys), CliReset, CliGray)
	fmt.Printf("%s│%s ⚡ QPS: %s%-35.2f%s│%s\n",
		CliGray, CliReset, CliCyan, qps, CliReset, CliGray)
	fmt.Printf("%s│%s 🎯 缓存命中率: %s%-35.1f%%%s│%s\n",
		CliGray, CliReset, CliYellow, cacheHitRate, CliReset, CliGray)
	fmt.Printf("%s│%s 🕐 运行时间: %s%-35s%s│%s\n",
		CliGray, CliReset, CliBlue, uptime.Round(time.Second), CliReset, CliGray)
	fmt.Printf("%s│%s 📄 活跃文件: %s%-35s%s│%s\n",
		CliGray, CliReset, CliPurple, formatSize(activeFileSize), CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n\n", CliGray, CliReset)

	fmt.Printf("%s┌─ 🧠 内存信息 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 📊 已使用: %s%-35s%s│%s\n",
		CliGray, CliReset, CliRed, memUsage, CliReset, CliGray)
	fmt.Printf("%s│%s 💾 总内存: %s%-35s%s│%s\n",
		CliGray, CliReset, CliPurple, memTotal, CliReset, CliGray)
	fmt.Printf("%s│%s 🔄 GC次数: %s%-35d%s│%s\n",
		CliGray, CliReset, CliBlue, m.NumGC, CliReset, CliGray)
	fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)

	if len(issues) > 0 {
		fmt.Printf("\n%s%s⚠️  发现的问题:%s\n", CliBold, CliRed, CliReset)
		fmt.Printf("%s┌─ 建议改进 ──────────────────────────────────────┐%s\n", CliGray, CliReset)
		for i, issue := range issues {
			fmt.Printf("%s│%s %d. %s%-45s%s│%s\n",
				CliGray, CliReset, i+1, CliYellow, issue, CliReset, CliGray)
		}
		fmt.Printf("%s└────────────────────────────────────────────────────┘%s\n", CliGray, CliReset)
	} else {
		fmt.Printf("\n%s%s✅ 系统运行良好，无需关注%s\n", CliBold, CliGreen, CliReset)
	}
}

func (db *GojiDB) handleSearch(parts []string) {
	if len(parts) < 2 {
		fmt.Printf("%s❌ 用法: search <pattern> [value_pattern]%s\n", CliRed, CliReset)
		return
	}

	pattern := parts[1]
	valuePattern := ""
	if len(parts) > 2 {
		valuePattern = strings.Join(parts[2:], " ")
	}

	keys := db.ListKeys()
	if len(keys) == 0 {
		fmt.Printf("%s📭 数据库为空%s\n", CliGray, CliReset)
		return
	}

	var matches []struct {
		Key   string
		Value string
		TTL   time.Duration
	}

	for _, key := range keys {
		if !strings.Contains(strings.ToLower(key), strings.ToLower(pattern)) {
			continue
		}

		value, err := db.Get(key)
		if err != nil {
			continue
		}

		valueStr := string(value)
		if valuePattern != "" && !strings.Contains(strings.ToLower(valueStr), strings.ToLower(valuePattern)) {
			continue
		}

		ttl, _ := db.GetTTL(key)
		matches = append(matches, struct {
			Key   string
			Value string
			TTL   time.Duration
		}{
			Key:   key,
			Value: valueStr,
			TTL:   ttl,
		})
	}

	if len(matches) == 0 {
		fmt.Printf("%s🔍 没有找到匹配的键值%s\n", CliGray, CliReset)
		return
	}

	fmt.Printf("%s📋 找到 %d 个匹配项:%s\n", CliBold, len(matches), CliReset)
	for i, match := range matches {
		preview := match.Value
		if len(preview) > 50 {
			preview = preview[:47] + "..."
		}
		fmt.Printf("  %s%2d.%s %s%-20s%s = %s\"%s\"%s", CliGray, i+1, CliReset, CliCyan, match.Key, CliReset, CliGreen, preview, CliReset)
		if match.TTL > 0 {
			fmt.Printf(" %s⏰%s", CliYellow, match.TTL)
		}
		fmt.Println()
	}
}

func (db *GojiDB) handleImport(parts []string) {
	if len(parts) < 2 {
		fmt.Printf("%s❌ 用法: import <filename>%s\n", CliRed, CliReset)
		return
	}

	filename := parts[1]
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("%s❌ 读取文件失败: %v%s\n", CliRed, err, CliReset)
		return
	}

	var importData map[string]string
	if err := json.Unmarshal(data, &importData); err != nil {
		fmt.Printf("%s❌ 解析JSON失败: %v%s\n", CliRed, err, CliReset)
		return
	}

	fmt.Printf("%s📥 准备导入 %d 条记录...%s\n", CliYellow, len(importData), CliReset)

	start := time.Now()
	count := 0
	for key, value := range importData {
		if err := db.Put(key, []byte(value)); err != nil {
			fmt.Printf("%s❌ 导入键 %s 失败: %v%s\n", CliRed, key, err, CliReset)
			continue
		}
		count++
	}

	elapsed := time.Since(start)
	fmt.Printf("%s✅ 导入完成！成功导入 %d/%d 条记录，耗时: %s%s%s\n", CliGreen, count, len(importData), CliCyan, elapsed, CliReset)
}

func (db *GojiDB) handleBenchmark(parts []string) {
	iterations := 1000
	if len(parts) > 1 {
		if val, err := strconv.Atoi(parts[1]); err == nil && val > 0 {
			iterations = val
		}
	}

	fmt.Printf("%s%s⚡ 性能基准测试%s\n", CliBold, CliYellow, CliReset)
	fmt.Printf("%s测试项目: %d 次读写操作%s\n", CliCyan, iterations, CliReset)

	// 写入测试
	fmt.Printf("%s📝 写入测试中...%s", CliBlue, CliReset)
	start := time.Now()
	for i := 0; i < iterations; i++ {
		key := fmt.Sprintf("bench:write:%d", i)
		value := fmt.Sprintf("benchmark data %d", i)
		db.Put(key, []byte(value))
	}
	writeTime := time.Since(start)
	writeQPS := float64(iterations) / writeTime.Seconds()

	// 读取测试
	fmt.Printf("%s📖 读取测试中...%s", CliGreen, CliReset)
	start = time.Now()
	for i := 0; i < iterations; i++ {
		key := fmt.Sprintf("bench:write:%d", i)
		db.Get(key)
	}
	readTime := time.Since(start)
	readQPS := float64(iterations) / readTime.Seconds()

	// 清理测试数据
	for i := 0; i < iterations; i++ {
		key := fmt.Sprintf("bench:write:%d", i)
		db.Delete(key)
	}

	fmt.Printf("%s📊 测试结果:%s\n", CliBold, CliReset)
	fmt.Printf("%s┌─ 写入性能 ─────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 总时间: %s%-12s%s│%s\n", CliGray, CliReset, CliCyan, writeTime.Round(time.Millisecond), CliReset+CliGray, CliReset)
	fmt.Printf("%s│%s QPS: %s%-15.0f%s│%s\n", CliGray, CliReset, CliGreen, writeQPS, CliReset+CliGray, CliReset)
	fmt.Printf("%s└─────────────────────────────────┘%s\n", CliGray, CliReset)

	fmt.Printf("%s┌─ 读取性能 ─────────────────────┐%s\n", CliGray, CliReset)
	fmt.Printf("%s│%s 总时间: %s%-12s%s│%s\n", CliGray, CliReset, CliCyan, readTime.Round(time.Millisecond), CliReset+CliGray, CliReset)
	fmt.Printf("%s│%s QPS: %s%-15.0f%s│%s\n", CliGray, CliReset, CliGreen, readQPS, CliReset+CliGray, CliReset)
	fmt.Printf("%s└─────────────────────────────────┘%s\n", CliGray, CliReset)
}

func (db *GojiDB) handleMonitor() {
	fmt.Printf("%s%s📊 实时监控模式 (按 Ctrl+C 退出)%s\n", CliBold, CliCyan, CliReset)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 清屏并重置光标
			fmt.Print("\033[2J\033[H")

			fmt.Printf("%s%s📊 GojiDB 实时监控%s\n", CliBold, CliCyan, CliReset)
			fmt.Printf("%s%s%s\n", CliGray, time.Now().Format("2006-01-02 15:04:05"), CliReset)

			stats := db.GetMetrics()
			keys := db.ListKeys()

			// 内存信息
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			// WAL信息
			walStatus := "❌ 禁用"
			walSize := "0 B"
			if db.config.EnableWAL && db.walManager != nil {
				walStatus = "✅ 启用"
				if size, err := db.walManager.GetWALSize(); err == nil {
					walSize = formatSize(size)
				}
			}

			fmt.Printf("%s┌─ 实时指标 ──────────────────────┐%s\n", CliGray, CliReset)
			fmt.Printf("%s│%s 总键数: %s%-15d%s│%s\n", CliGray, CliReset, CliYellow, len(keys), CliReset+CliGray, CliReset)
			fmt.Printf("%s│%s 内存使用: %s%-13s%s│%s\n", CliGray, CliReset, CliPurple, formatSize(int64(m.Alloc)), CliReset+CliGray, CliReset)
			fmt.Printf("%s│%s WAL状态: %s%-12s%s│%s\n", CliGray, CliReset, CliCyan, walStatus, CliReset+CliGray, CliReset)
			fmt.Printf("%s│%s WAL大小: %s%-12s%s│%s\n", CliGray, CliReset, CliBlue, walSize, CliReset+CliGray, CliReset)
			fmt.Printf("%s│%s 总读取: %s%-15d%s│%s\n", CliGray, CliReset, CliGreen, stats.TotalReads, CliReset+CliGray, CliReset)
			fmt.Printf("%s│%s 总写入: %s%-15d%s│%s\n", CliGray, CliReset, CliGreen, stats.TotalWrites, CliReset+CliGray, CliReset)
			fmt.Printf("%s│%s 缓存命中: %s%-15d%s│%s\n", CliGray, CliReset, CliCyan, stats.CacheHits, CliReset+CliGray, CliReset)
			fmt.Printf("%s└─────────────────────────────────┘%s\n", CliGray, CliReset)
		}
	}
}

// --------------------------------------------------
// 从main.go迁移过来的启动和演示相关函数
// --------------------------------------------------

// ShowBigLogo 显示启动Logo
func ShowBigLogo() {
	bigLogo := ColorBold + ColorCyan + `
             ██████╗  ██████╗      ██╗██╗██████╗ ██████╗
            ██╔════╝ ██╔═══██╗     ██║██║██╔══██╗██╔══██╗
            ██║  ███╗██║   ██║     ██║██║██║  ██║██████╔╝
            ██║   ██║██║   ██║██   ██║██║██║  ██║██╔══██╗
            ╚██████╔╝╚██████╔╝╚█████╔╝██║██████╔╝██████╔╝
             ╚═════╝  ╚═════╝  ╚════╝ ╚═╝╚═════╝ ╚═════╝

` + ColorReset

	fmt.Print(bigLogo)

	// 显示启动信息框
	startInfo := []string{
		fmt.Sprintf("🚀 %sGojiDB v2.1.0%s 智能启动中...", ColorBold+ColorYellow, ColorReset),
		fmt.Sprintf("📅 启动时间: %s%s%s", ColorCyan, time.Now().Format("2006-01-02 15:04:05"), ColorReset),
		fmt.Sprintf("💻 系统: %s%s/%s%s", ColorGreen, runtime.GOOS, runtime.GOARCH, ColorReset),
		fmt.Sprintf("🐹 Go版本: %s%s%s", ColorPurple, runtime.Version(), ColorReset),
		fmt.Sprintf("🔥 CPU核心: %s%d%s", ColorRed, runtime.NumCPU(), ColorReset),
		fmt.Sprintf("🎯 模式: %s高性能模式%s", ColorBold+ColorGreen, ColorReset),
	}
	printFancyBox("🎉 GojiDB 智能启动", startInfo, ColorGreen, 70)
}

// ShowBigExitLogo 显示退出Logo
func ShowBigExitLogo() {
	bigExitLogo := ColorBold + ColorPurple + `
             ██████╗  ██████╗      ██╗██╗██████╗ ██████╗
            ██╔════╝ ██╔═══██╗     ██║██║██╔══██╗██╔══██╗
            ██║  ███╗██║   ██║     ██║██║██║  ██║██████╔╝
            ██║   ██║██║   ██║██   ██║██║██║  ██║██╔══██╗
            ╚██████╔╝╚██████╔╝╚█████╔╝██║██████╔╝██████╔╝
             ╚═════╝  ╚═════╝  ╚════╝ ╚═╝╚═════╝ ╚═════╝

` + ColorReset

	fmt.Print(bigExitLogo)

	// 显示退出信息框
	exitInfo := []string{
		fmt.Sprintf("%s🎬 感谢使用 GojiDB v2.1.0%s", ColorBold+ColorYellow, ColorReset),
		fmt.Sprintf("%s💾 数据已安全保存%s", ColorGreen, ColorReset),
		fmt.Sprintf("%s🔒 系统正常退出%s", ColorPurple, ColorReset),
		fmt.Sprintf("%s🚀 期待下次再见！%s", ColorRed+ColorBold, ColorReset),
	}
	printFancyBox("👋 GojiDB 优雅退出", exitInfo, ColorPurple, 70)
}

// banner 显示Logo的简化版本
func banner() {
	ShowBigLogo()
}

// RunAutoDemo 按顺序演示所有功能
func RunAutoDemo(db *GojiDB) {
	showLoadingAnimation("开始高级功能演示", 1*time.Second)

	// 1. 基础CRUD增强版
	showSection("1️⃣  基础CRUD - 批量操作")
	fmt.Printf("   📊 写入1000条测试数据...\n")
	start := time.Now()
	for i := 0; i < 1000; i++ {
		db.Put(fmt.Sprintf("demo:user:%d", i), []byte(fmt.Sprintf("User %d - %s", i, time.Now().Format("15:04:05"))))
	}
	fmt.Printf("   ✅ 批量写入完成，耗时: %v\n", time.Since(start))

	// 批量读取测试
	start = time.Now()
	count := 0
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("demo:user:%d", i)
		if _, err := db.Get(key); err == nil {
			count++
		} else {
			fmt.Printf("   ❌ 读取失败 %s: %v\n", key, err)
		}
	}
	fmt.Printf("   ✅ 批量读取100条，成功: %d，耗时: %v\n", count, time.Since(start))

	// 2. TTL高级演示
	showSection("2️⃣  TTL - 智能过期策略")
	fmt.Printf("   ⏰ 设置不同TTL值的键...\n")
	db.PutWithTTL("demo:ttl:short", []byte("3秒后过期"), 3*time.Second)
	db.PutWithTTL("demo:ttl:medium", []byte("5秒后过期"), 5*time.Second)
	db.PutWithTTL("demo:ttl:long", []byte("8秒后过期"), 8*time.Second)

	for i := 0; i < 3; i++ {
		if ttl, _ := db.GetTTL(fmt.Sprintf("demo:ttl:%s", []string{"short", "medium", "long"}[i])); ttl > 0 {
			fmt.Printf("   📊 %s TTL: %v\n", []string{"short", "medium", "long"}[i], ttl.Round(time.Second))
		}
	}

	showLoadingAnimation("观察TTL过期过程", 3*time.Second)
	fmt.Printf("   ✅ TTL演示完成\n")

	// 3. 事务高级演示
	showSection("3️⃣  事务 - 原子性操作")
	fmt.Printf("   🔒 执行银行转账事务...\n")
	db.Put("demo:account:alice", []byte("1000"))
	db.Put("demo:account:bob", []byte("500"))

	// 获取当前余额
	aliceBal, _ := db.Get("demo:account:alice")
	bobBal, _ := db.Get("demo:account:bob")

	aliceNum, _ := strconv.Atoi(string(aliceBal))
	bobNum, _ := strconv.Atoi(string(bobBal))

	aliceNum -= 200
	bobNum += 200

	tx, _ := db.BeginTransaction()
	tx.Put("demo:account:alice", []byte(strconv.Itoa(aliceNum)))
	tx.Put("demo:account:bob", []byte(strconv.Itoa(bobNum)))

	if err := tx.Commit(); err != nil {
		fmt.Printf("   ❌ 事务失败: %v\n", err)
	} else {
		aliceNew, _ := db.Get("demo:account:alice")
		bobNew, _ := db.Get("demo:account:bob")
		fmt.Printf("   ✅ 转账完成: Alice %s → Bob %s\n", string(aliceNew), string(bobNew))
	}

	// 4. 并发性能测试
	showSection("4️⃣  并发性能测试")
	fmt.Printf("   🚀 启动10个并发goroutine...\n")
	var wg sync.WaitGroup
	start = time.Now()
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				db.Put(fmt.Sprintf("demo:concurrent:%d:%d", id, i), []byte(fmt.Sprintf("data-%d-%d", id, i)))
			}
		}(g)
	}
	wg.Wait()
	fmt.Printf("   ✅ 并发写入500条，耗时: %v\n", time.Since(start))

	// 5. 快照与恢复
	showSection("5️⃣  快照 - 数据备份")
	fmt.Printf("   📸 创建完整数据快照...\n")
	snap, _ := db.CreateSnapshot()
	fmt.Printf("   ✅ 快照ID: %s%s…%s\n", ColorCyan, snap.ID[:16], ColorReset)
	fmt.Printf("   📊 包含键数: %d\n", snap.KeyCount)
	fmt.Printf("   💾 快照大小: %.2f KB\n", float64(snap.DataSize)/1024)

	// 6. 数据压缩演示
	showSection("6️⃣  数据压缩 - 空间优化")
	fmt.Printf("   📦 写入重复数据测试压缩...\n")
	bigData := strings.Repeat("GojiDB-", 1000) // 7KB数据
	start = time.Now()
	db.Put("demo:bigdata", []byte(bigData))
	fmt.Printf("   ✅ 大数据写入，压缩后大小: %.2f KB\n", float64(len(bigData))/1024)

	// 7. 合并与优化
	showSection("7️⃣  数据合并 - 性能优化")
	fmt.Printf("   🧹 准备数据用于合并测试...\n")
	for i := 0; i < 200; i++ {
		db.Put(fmt.Sprintf("demo:merge:%d", i), []byte(strings.Repeat("x", 100)))
	}
	for i := 0; i < 150; i++ {
		db.Delete(fmt.Sprintf("demo:merge:%d", i))
	}

	showLoadingAnimation("执行数据合并优化", 2*time.Second)
	start = time.Now()
	_ = db.Compact()
	fmt.Printf("   ✅ 合并完成，耗时: %v\n", time.Since(start))

	// 8. 高级查询与统计
	showSection("8️⃣  高级查询与性能统计")
	m := db.GetMetrics()
	fmt.Printf("   📊 总操作统计:\n")
	fmt.Printf("      📖 读取: %d 次\n", m.TotalReads)
	fmt.Printf("      ✏️  写入: %d 次\n", m.TotalWrites)
	fmt.Printf("      🗑️  删除: %d 次\n", m.TotalDeletes)
	fmt.Printf("      📏 数据大小: %.2f MB\n", float64(m.DataSize)/1024/1024)
	fmt.Printf("      🗂️  键总数: %d\n", len(db.ListKeys()))

	// 9. 数据可视化
	showSection("9️⃣  数据可视化")
	db.PrintDataVisualization()

	// 10. 清理演示数据
	showSection("🔟  清理演示数据")
	fmt.Printf("   🧹 清理演示数据（删除所有demo前缀的键）...\n")

	// 获取所有键并删除以"demo:"开头的
	allKeys := db.ListKeys()
	cleanCount := 0
	for _, key := range allKeys {
		if strings.HasPrefix(key, "demo:") {
			if err := db.Delete(key); err == nil {
				cleanCount++
			}
		}
	}

	fmt.Printf("   ✅ 清理完成，删除 %d 条演示数据\n", cleanCount)

	// 11. 最终状态
	showSection("🎊  演示总结")
	db.PrintFullDatabaseStatus()

	fmt.Printf("\n%s🎉 高级功能演示完成！现在进入交互模式...\n", ColorBold+ColorGreen)
}

// ShowSystemInfo 显示系统信息
func ShowSystemInfo(config *Config) {
	info := GetSystemInfo()
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)

	sysInfo := []string{
		fmt.Sprintf("%s🖥️  主机: %s%s", ColorCyan, info["hostname"], ColorReset),
		fmt.Sprintf("%s👤 用户: %s%s", ColorBlue, info["username"], ColorReset),
		fmt.Sprintf("%s⚙️  系统: %s/%s%s", ColorGreen, info["os"], info["arch"], ColorReset),
		fmt.Sprintf("%s🐹 Go版本: %s%s", ColorPurple, info["go_version"], ColorReset),
		fmt.Sprintf("%s🔥 CPU核心: %s%s", ColorRed, info["cpu_count"], ColorReset),
		fmt.Sprintf("%s📁 数据目录: %s%s", ColorYellow, config.Database.DataDir, ColorReset),
		fmt.Sprintf("%s💾 内存限制: %d MB%s", ColorMagenta, config.Database.MaxMemoryMB, ColorReset),
		fmt.Sprintf("%s🧠 内存使用: %.2f MB%s", ColorYellow, float64(mem.Alloc)/1024/1024, ColorReset),
		fmt.Sprintf("%s🔄 GC次数: %d%s", ColorPurple, mem.NumGC, ColorReset),
	}
	printFancyBox("📊 系统信息", sysInfo, ColorBlue, 70)
}

// showSection 显示章节标题
func showSection(title string) {
	fmt.Printf("\n%s%s%s\n%s%s%s\n", ColorBold+ColorBlue, title, ColorReset,
		ColorBlue, strings.Repeat("─", 30), ColorReset)
}

// showLoadingAnimation 显示加载动画
func showLoadingAnimation(msg string, d time.Duration) {
	spinner := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
	start := time.Now()
	for time.Since(start) < d {
		for _, s := range spinner {
			fmt.Printf("\r%s%c %s%s", ColorCyan, s, msg, ColorReset)
			time.Sleep(100 * time.Millisecond)
		}
	}
	fmt.Printf("\r%s✅ %s 完成%s\n", ColorGreen, msg, ColorReset)
}

// 独立命令处理器函数
func handleHelp(db *GojiDB, parts []string) error {
	db.showHelp()
	return nil
}

func handlePut(db *GojiDB, parts []string) error {
	if len(parts) < 3 {
		return fmt.Errorf("用法: put <key> <value> [ttl]")
	}
	
	key, value := parts[1], strings.Join(parts[2:], " ")
	value = strings.Trim(value, `"'`)
	
	if len(parts) > 3 {
		if ttl, err := time.ParseDuration(parts[3]); err == nil {
			return db.PutWithTTL(key, []byte(value), ttl)
		}
	}
	
	return db.Put(key, []byte(value))
}

func handleGet(db *GojiDB, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("用法: get <key>")
	}
	
	value, err := db.Get(parts[1])
	if err != nil {
		return err
	}
	
	fmt.Printf("%s%s%s = %s%s%s\n", 
		CliGreen, parts[1], CliReset, 
		CliYellow, string(value), CliReset)
	return nil
}

func handleDelete(db *GojiDB, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("用法: delete <key>")
	}
	return db.Delete(parts[1])
}

func handleList(db *GojiDB, parts []string) error {
	pattern := ""
	if len(parts) > 1 {
		pattern = parts[1]
	}
	
	keys := db.ListKeys()
	filtered := filterKeys(keys, pattern)
	
	fmt.Printf("%s找到 %d 个键:%s\n", CliCyan, len(filtered), CliReset)
	for i, key := range filtered {
		fmt.Printf("  %s%d.%s %s%s%s\n", CliGray, i+1, CliReset, CliGreen, key, CliReset)
	}
	return nil
}

func handleTTL(db *GojiDB, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("用法: ttl <key>")
	}
	
	ttl, err := db.GetTTL(parts[1])
	if err != nil {
		return err
	}
	
	if ttl > 0 {
		fmt.Printf("%s%s%s TTL: %s%s%s\n", 
			CliYellow, parts[1], CliReset, 
			CliGreen, ttl.Round(time.Second), CliReset)
	} else {
		fmt.Printf("%s%s%s 没有TTL或已过期%s\n", 
			CliRed, parts[1], CliReset, CliReset)
	}
	return nil
}

func handleCompact(db *GojiDB, parts []string) error {
	fmt.Printf("%s🧹 开始数据合并...%s\n", CliCyan, CliReset)
	if err := db.Compact(); err != nil {
		return err
	}
	fmt.Printf("%s✅ 数据合并完成%s\n", CliGreen, CliReset)
	return nil
}

func handleClear(db *GojiDB, parts []string) error {
	fmt.Printf("%s🗑️  清空屏幕...%s\n", CliYellow, CliReset)
	db.handleClear()
	return nil
}

func handleStats(db *GojiDB, parts []string) error {
	db.handleStats()
	return nil
}

func handleInfo(db *GojiDB, parts []string) error {
	db.handleInfo()
	return nil
}

func handleHistory(db *GojiDB, parts []string) error {
	return fmt.Errorf("history命令已集成到CLI中，无需单独调用")
}

func handleBatch(db *GojiDB, parts []string) error {
	db.handleBatch()
	return nil
}

func handleExport(db *GojiDB, parts []string) error {
	db.handleExport()
	return nil
}

func handleSnapshots(db *GojiDB, parts []string) error {
	db.handleSnapshots()
	return nil
}

func handleRestore(db *GojiDB, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("用法: restore <backup_path>")
	}
	db.handleRestore(parts)
	return nil
}

func handleHealth(db *GojiDB, parts []string) error {
	db.handleHealth()
	return nil
}

func handleSearch(db *GojiDB, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("用法: search <pattern>")
	}
	
	pattern := parts[1]
	keys := db.ListKeys()
	var results []string
	
	for _, key := range keys {
		if strings.Contains(key, pattern) {
			results = append(results, key)
		}
	}
	
	fmt.Printf("%s找到 %d 个匹配结果:%s\n", CliCyan, len(results), CliReset)
	for i, key := range results {
		if value, err := db.Get(key); err == nil {
			fmt.Printf("  %s%d.%s %s%s%s = %s%s...%s\n", 
				CliGray, i+1, CliReset, 
				CliGreen, key, CliReset, 
				CliYellow, truncateString(string(value), 50), CliReset)
		}
	}
	return nil
}

func handleImport(db *GojiDB, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("用法: import <file_path>")
	}
	db.handleImport(parts)
	return nil
}

func handleBenchmark(db *GojiDB, parts []string) error {
	fmt.Printf("%s🏃 运行快速基准测试...%s\n", CliCyan, CliReset)
	
	start := time.Now()
	count := 1000
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("bench:%d", i)
		value := fmt.Sprintf("value-%d", i)
		if err := db.Put(key, []byte(value)); err != nil {
			return err
		}
	}
	
	duration := time.Since(start)
	qps := float64(count) / duration.Seconds()
	
	fmt.Printf("%s✅ 基准测试完成: %d 次操作, %.2f ops/sec, 耗时 %v%s\n", 
		CliGreen, count, qps, duration, CliReset)
	return nil
}

func handleMonitor(db *GojiDB, parts []string) error {
	db.handleMonitor()
	return nil
}

func handleTransaction(db *GojiDB, parts []string) error {
	db.handleTransaction()
	return nil
}

func handleSnapshot(db *GojiDB, parts []string) error {
	if len(parts) > 1 {
		cmd := strings.ToLower(parts[1])
		switch cmd {
		case "create":
			id, err := db.CreateSnapshot()
			if err != nil {
				return err
			}
			fmt.Printf("%s✅ 快照已创建: %s%s\n", CliGreen, id.ID, CliReset)
		case "list":
			snapshots, err := db.ListSnapshots()
			if err != nil {
				return err
			}
			fmt.Printf("%s找到 %d 个快照:%s\n", CliCyan, len(snapshots), CliReset)
			for _, snapshot := range snapshots {
				fmt.Printf("  %s%s%s\n", CliBlue, snapshot.ID, CliReset)
			}
		case "restore":
			if len(parts) < 3 {
				return fmt.Errorf("用法: snapshot restore <id>")
			}
			return db.RestoreFromSnapshot(parts[2])
		default:
			return fmt.Errorf("未知快照命令: %s", cmd)
		}
	} else {
		id, err := db.CreateSnapshot()
		if err != nil {
			return err
		}
		fmt.Printf("%s✅ 快照已创建: %s%s\n", CliGreen, id.ID, CliReset)
	}
	return nil
}

func handleWAL(db *GojiDB, parts []string) error {
	db.handleWAL(parts)
	return nil
}

func handleExit(db *GojiDB, parts []string) error {
	fmt.Printf("%s👋 再见！感谢使用 GojiDB%s\n", CliGreen, CliReset)
	return errExitCLI
}

// 辅助函数
func filterKeys(keys []string, pattern string) []string {
	if pattern == "" {
		return keys
	}
	
	var filtered []string
	for _, key := range keys {
		if strings.Contains(key, pattern) {
			filtered = append(filtered, key)
		}
	}
	return filtered
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

var errExitCLI = fmt.Errorf("exit CLI")
