package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"taobao/internal/dangkou"
	"taobao/internal/filter"
	"taobao/internal/logger"
	"taobao/internal/peijian"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	logger.Init()
	defer logger.Close()

	// 有命令行参数 → CLI 模式
	if len(os.Args) > 1 {
		runCLI()
		return
	}

	// 无参数 → GUI 模式
	app := NewApp()
	err := wails.Run(&options.App{
		Title:  "手机壳订单处理工具",
		Width:  900,
		Height: 680,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			WebviewGpuPolicy: linux.WebviewGpuPolicyAlways,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

func runCLI() {
	cmd := os.Args[1]
	switch cmd {
	case "filter":
		if len(os.Args) < 3 {
			fmt.Println("用法: phonecase-tools filter <Excel文件>")
			os.Exit(1)
		}
		result, err := filter.Process(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已生成 %s/筛选结果.xlsx\n", result.OutputDir)
		fmt.Printf("  多件订单: %d条\n", result.Summary.MultiOrders)
		fmt.Printf("  疑难单: %d条\n", result.Summary.DoubtfulOrders)
		fmt.Printf("  正常手机壳: %d条\n", result.Summary.NormalOrders)
		fmt.Printf("  单独配件: %d条\n", result.Summary.AccessoryRows)
		fmt.Printf("  总计: %d条\n", result.Summary.Total)

	case "dangkou":
		if len(os.Args) < 3 {
			fmt.Println("用法: phonecase-tools dangkou <订单Excel文件> [自设编码.xlsx]")
			os.Exit(1)
		}
		configPath := ""
		if len(os.Args) >= 4 {
			configPath = os.Args[3]
		}
		result, err := dangkou.Process(os.Args[2], configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已生成 %s/档口分配.xlsx\n", result.OutputDir)
		fmt.Println()
		for name, count := range result.Summary {
			fmt.Printf("  %s: %d条\n", name, count)
		}
		fmt.Printf("  总计: %d条\n", result.Total)

	case "peijian":
		if len(os.Args) < 4 {
			fmt.Println("用法: phonecase-tools peijian extract <Excel文件>")
			fmt.Println("      phonecase-tools peijian merge <pending.xlsx>")
			os.Exit(1)
		}
		sub := os.Args[2]
		switch sub {
		case "extract":
			result, err := peijian.Extract(os.Args[3])
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("已生成 %s/pending.xlsx\n", result.OutputDir)
			fmt.Printf("共 %d 条订单\n", result.Summary.Total)
			fmt.Printf("  简单订单:   %d 条（已提取配件）\n", result.Summary.Simple)
			fmt.Printf("  待处理订单: %d 条（需人工处理）\n", result.Summary.Pending)
			fmt.Printf("  无配件订单: %d 条\n", result.Summary.NoParts)
		case "merge":
			result, err := peijian.Merge(os.Args[3])
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("\n配件统计汇总")
			fmt.Println("──────────────────────────────────────────────────")
			fmt.Printf("%-24s %-16s %s\n", "配件名称", "颜色", "数量")
			fmt.Println("──────────────────────────────────────────────────")
			for _, e := range result.Entries {
				fmt.Printf("%-24s %-16s %d\n", e.Name, e.Color, e.Qty)
			}
			fmt.Println("──────────────────────────────────────────────────")
			fmt.Printf("共 %d 种配件，总计 %d 件\n", result.TotalKinds, result.TotalQty)
			fmt.Printf("\n已生成 %s\n", result.OutputPath)
		default:
			fmt.Fprintf(os.Stderr, "未知命令: %s\n", sub)
			os.Exit(1)
		}

	default:
		fmt.Println("用法:")
		fmt.Println("  phonecase-tools                       启动桌面应用")
		fmt.Println("  phonecase-tools filter <Excel文件>    订单筛选")
		fmt.Println("  phonecase-tools dangkou <订单Excel文件> [自设编码.xlsx]   档口分配")
		fmt.Println("  phonecase-tools peijian extract <Excel文件>  配件提取")
		fmt.Println("  phonecase-tools peijian merge <pending.xlsx> 配件汇总")
		os.Exit(1)
	}
}
