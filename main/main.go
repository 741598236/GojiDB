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
	// 命令行参数解析
	var (
		configFile = flag.String("config", "configs/config.yaml", "配置文件路径")
		noBanner   = flag.Bool("no-banner", false, "不显示启动Logo")
		demoOnly   = flag.Bool("demo", false, "仅运行演示后退出")
		version    = flag.Bool("version", false, "显示版本信息")
	)
	flag.Parse()

	// 显示版本信息
	if *version {
		fmt.Printf("GojiDB v2.1.0 - 高性能键值存储数据库\n")
		fmt.Printf("Go版本: %s\n", runtime.Version())
		return
	}

	// 加载配置
	configPath := *configFile
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 如果配置文件不存在，尝试从上级目录查找
		altPath := "../" + configPath
		if _, err := os.Stat(altPath); err == nil {
			configPath = altPath
		}
	}

	config, err := gojidb.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("配置加载失败: %v\n", err)
		return
	}

	// 显示启动Logo
	if config.CLI.ShowBanner && !*noBanner {
		gojidb.ShowBigLogo()
	}

	// 显示系统信息
	if config.CLI.ShowStats {
		gojidb.ShowSystemInfo(config)
	}

	// 初始化数据库
	// 同步配置
	config.GojiDB.DataPath = config.Database.DataDir
	config.GojiDB.EnableMetrics = config.Features.MetricsSupport

	db, err := gojidb.NewGojiDB(&config.GojiDB)
	if err != nil {
		fmt.Printf("数据库初始化失败: %v\n", err)
		return
	}
	defer db.Close()

	// 运行演示
	if *demoOnly {
		fmt.Println(gojidb.ColorYellow + "🎯 演示模式启动..." + gojidb.ColorReset)
		gojidb.RunAutoDemo(db)
		fmt.Println(gojidb.ColorGreen + "✅ 演示完成！" + gojidb.ColorReset)
		return
	}

	// 询问是否运行演示
	if config.Features.BackupSupport && !*noBanner {
		fmt.Print(gojidb.ColorCyan + "🎬 是否运行功能演示？(y/n): " + gojidb.ColorReset)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) == "y" {
			gojidb.RunAutoDemo(db)
		}
	}

	// 启动交互模式
	db.StartInteractiveMode()

	// 显示退出Logo
	gojidb.ShowBigExitLogo()
}
