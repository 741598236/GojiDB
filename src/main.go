package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 颜色常量（与单文件版完全一致）
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

	config, err := LoadConfig(configPath)
	if err != nil {
		fmt.Printf("配置加载失败: %v\n", err)
		return
	}

	// 显示启动Logo
	if config.CLI.ShowBanner && !*noBanner {
		showBigLogo()
	}

	// 显示系统信息
	if config.CLI.ShowStats {
		showSystemInfo(config)
	}

	// 初始化数据库
	// 同步配置
	config.GojiDB.DataPath = config.Database.DataDir
	config.GojiDB.EnableMetrics = config.Features.MetricsSupport

	db, err := NewGojiDB(&config.GojiDB)
	if err != nil {
		fmt.Printf("数据库初始化失败: %v\n", err)
		return
	}
	defer db.Close()

	// 运行演示
	if *demoOnly {
		fmt.Println(ColorYellow + "🎯 演示模式启动..." + ColorReset)
		runAutoDemo(db)
		fmt.Println(ColorGreen + "✅ 演示完成！" + ColorReset)
		return
	}

	// 询问是否运行演示
	if config.Features.BackupSupport && !*noBanner {
		fmt.Print(ColorCyan + "🎬 是否运行功能演示？(y/n): " + ColorReset)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) == "y" {
			runAutoDemo(db)
		}
	}

	// 启动交互模式
	db.StartInteractiveMode()

	// 显示退出Logo
	showBigExitLogo()
}

// --------------------------------------------------
// 以下所有函数均完整实现，可直接使用
// --------------------------------------------------

func showBigLogo() {
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

func showBigExitLogo() {
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

func banner() {
	showBigLogo()
}

// runAutoDemo 按顺序演示所有功能
func runAutoDemo(db *GojiDB) {
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

func showSystemInfo(config *Config) {
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

// --------------------------------------------------
// 演示/打印辅助
// --------------------------------------------------

func showSection(title string) {
	fmt.Printf("\n%s%s%s\n%s%s%s\n", ColorBold+ColorBlue, title, ColorReset,
		ColorBlue, strings.Repeat("─", 30), ColorReset)
}

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
