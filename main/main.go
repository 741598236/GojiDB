package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	gojidb "GojiDB"
)

func main() {
	flags := parseFlags()
	if flags.showVersion {
		showVersionInfo()
		return
	}

	config, err := loadConfigWithFallback(flags.configFile)
	if err != nil {
		fmt.Printf("配置加载失败: %v\n", err)
		return
	}

	if err := initializeApp(config, flags); err != nil {
		fmt.Printf("应用初始化失败: %v\n", err)
		return
	}
}

// flags 存储命令行参数
type flags struct {
	configFile string
	noBanner   bool
	demoOnly   bool
	showVersion bool
}

// parseFlags 解析命令行参数
func parseFlags() *flags {
	var f flags
	flag.StringVar(&f.configFile, "config", "configs/config.yaml", "配置文件路径")
	flag.BoolVar(&f.noBanner, "no-banner", false, "不显示启动Logo")
	flag.BoolVar(&f.demoOnly, "demo", false, "仅运行演示后退出")
	flag.BoolVar(&f.showVersion, "version", false, "显示版本信息")
	flag.Parse()
	return &f
}

// showVersionInfo 显示版本信息
func showVersionInfo() {
	fmt.Printf("GojiDB v1.0.0 - 高性能键值存储数据库\n")
	fmt.Printf("Go版本: %s\n", runtime.Version())
}

// loadConfigWithFallback 加载配置，支持路径回退
func loadConfigWithFallback(configPath string) (*gojidb.Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		altPath := "../" + configPath
		if _, err := os.Stat(altPath); err == nil {
			configPath = altPath
		}
	}
	return gojidb.LoadConfig(configPath)
}

// initializeApp 初始化应用
func initializeApp(config *gojidb.Config, flags *flags) error {
	if config.CLI.ShowBanner && !flags.noBanner {
		gojidb.ShowBigLogo()
	}

	if config.CLI.ShowStats {
		gojidb.ShowSystemInfo(config)
	}

	// 同步配置
	config.GojiDB.DataPath = config.Database.DataDir
	config.GojiDB.EnableMetrics = config.Features.MetricsSupport

	db, err := gojidb.NewGojiDB(&config.GojiDB)
	if err != nil {
		return fmt.Errorf("数据库初始化失败: %w", err)
	}
	defer db.Close()

	if flags.demoOnly {
		runDemoMode(db)
		return nil
	}

	if shouldAskForDemo(config, flags) {
		if askUserForDemo() {
			gojidb.RunAutoDemo(db)
		}
	}

	db.StartInteractiveMode()
	gojidb.ShowBigExitLogo()
	return nil
}

// runDemoMode 运行演示模式
func runDemoMode(db *gojidb.GojiDB) {
	fmt.Println(gojidb.ColorYellow + "🎯 演示模式启动..." + gojidb.ColorReset)
	gojidb.RunAutoDemo(db)
	fmt.Println(gojidb.ColorGreen + "✅ 演示完成！" + gojidb.ColorReset)
}

// shouldAskForDemo 判断是否询问运行演示
func shouldAskForDemo(config *gojidb.Config, flags *flags) bool {
	return config.Features.BackupSupport && !flags.noBanner
}

// askUserForDemo 询问用户是否运行演示
func askUserForDemo() bool {
	fmt.Print(gojidb.ColorCyan + "🎬 是否运行功能演示？(y/n): " + gojidb.ColorReset)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.ToLower(strings.TrimSpace(input)) == "y"
}
