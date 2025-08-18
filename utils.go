package GojiDB

import (
	"fmt"
	"os"
	"runtime"
)

func getSystemDetails() map[string]string {
	details := make(map[string]string)
	details["os"] = getOSName()
	details["arch"] = getArchName()
	details["goversion"] = runtime.Version()
	details["compiler"] = runtime.Compiler
	details["cpus"] = fmt.Sprintf("%d", runtime.NumCPU())
	details["goroutines"] = fmt.Sprintf("%d", runtime.NumGoroutine())

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	details["heap_alloc"] = formatSize(int64(m.Alloc))
	details["sys_alloc"] = formatSize(int64(m.Sys))
	details["gc_count"] = fmt.Sprintf("%d", m.NumGC)

	if wd, err := os.Getwd(); err == nil {
		details["workdir"] = wd
	}
	if exe, err := os.Executable(); err == nil {
		details["executable"] = exe
	}
	return details
}

func getOSName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}

func getArchName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "ARM64"
	default:
		return runtime.GOARCH
	}
}

// formatSize 格式化文件大小显示
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
