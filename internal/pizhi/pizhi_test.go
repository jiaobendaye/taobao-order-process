// Package pizhi 皮质壳档口分配测试
package pizhi

import (
	"os"
	"path/filepath"
	"testing"
)

// 逐行分配：相同 (商品ID, SKU, 型号) 的订单不合并，各自独立一行
func TestProcessData_NoAggregation(t *testing.T) {
	engine := &Engine{
		Items: map[string]ConfigItem{
			"988808938880|喜马拉雅白鳄鱼": {Stall: "鹏华"},
			"992724480793|奶芋紫鳄鱼纹":   {Stall: "森慕为"},
		},
		Stalls: []string{"鹏华", "森慕为"},
	}

	headers := []string{"店铺名称", "订单编号", "商品id", "商品规格", "商品数量"}
	rows := [][]string{
		{"店A", "ORD001", "988808938880", "iPhone15|喜马拉雅白鳄鱼", "2"},
		{"店A", "ORD002", "988808938880", "iPhone15|喜马拉雅白鳄鱼", "1"},
		{"店B", "ORD003", "992724480793", "HuaweiPura70|奶芋紫鳄鱼纹", "3"},
		{"店C", "ORD004", "999999999999", "XX|未知", "1"},
	}

	result := ProcessData(rows, headers, engine)

	if len(result.StallOrders) != 2 {
		t.Fatalf("档口数 = %d, want 2", len(result.StallOrders))
	}

	// 鹏华：2 条独立订单行（不聚合）
	penghua := result.StallOrders["鹏华"]
	if len(penghua) != 2 {
		t.Fatalf("鹏华订单行数 = %d, want 2", len(penghua))
	}
	if penghua[0].Quantity != 2 {
		t.Errorf("鹏华第1行数量 = %d, want 2", penghua[0].Quantity)
	}
	if penghua[1].Quantity != 1 {
		t.Errorf("鹏华第2行数量 = %d, want 1", penghua[1].Quantity)
	}

	// 森慕为：1 条订单行
	senmu := result.StallOrders["森慕为"]
	if len(senmu) != 1 {
		t.Fatalf("森慕为订单行数 = %d, want 1", len(senmu))
	}
	if senmu[0].Quantity != 3 {
		t.Errorf("森慕为数量 = %d, want 3", senmu[0].Quantity)
	}
}

// 不同型号下相同 (商品ID, SKU) 应该拆成多行
func TestProcessData_DifferentModelsSplit(t *testing.T) {
	engine := &Engine{
		Items: map[string]ConfigItem{
			"988808938880|喜马拉雅白鳄鱼": {Stall: "鹏华"},
		},
		Stalls: []string{"鹏华"},
	}

	headers := []string{"商品id", "商品规格", "商品数量"}
	rows := [][]string{
		{"988808938880", "iPhone15|喜马拉雅白鳄鱼", "2"},
		{"988808938880", "HuaweiPura70|喜马拉雅白鳄鱼", "1"},
	}

	result := ProcessData(rows, headers, engine)

	penghua := result.StallOrders["鹏华"]
	if len(penghua) != 2 {
		t.Fatalf("鹏华订单行数 = %d, want 2", len(penghua))
	}
}

// 同型号不同 SKU 应该拆成多行
func TestProcessData_DifferentSkusSplit(t *testing.T) {
	engine := &Engine{
		Items: map[string]ConfigItem{
			"988808938880|喜马拉雅白鳄鱼": {Stall: "鹏华"},
			"988808938880|烫金淡粉蛇纹":   {Stall: "鹏华"},
		},
		Stalls: []string{"鹏华"},
	}

	headers := []string{"商品id", "商品规格", "商品数量"}
	rows := [][]string{
		{"988808938880", "iPhone15|喜马拉雅白鳄鱼", "2"},
		{"988808938880", "iPhone15|烫金淡粉蛇纹", "1"},
	}

	result := ProcessData(rows, headers, engine)
	penghua := result.StallOrders["鹏华"]
	if len(penghua) != 2 {
		t.Fatalf("鹏华订单行数 = %d, want 2", len(penghua))
	}
}

// 空 SKU（无 | 分隔）
func TestProcessData_NoPipe(t *testing.T) {
	engine := &Engine{
		Items: map[string]ConfigItem{
			"988808938880|单品": {Stall: "鹏华"},
		},
		Stalls: []string{"鹏华"},
	}
	headers := []string{"商品id", "商品规格", "商品数量"}
	rows := [][]string{
		{"988808938880", "单品", "5"},
	}
	result := ProcessData(rows, headers, engine)
	if len(result.StallOrders["鹏华"]) != 1 {
		t.Fatalf("鹏华订单行数 = %d, want 1", len(result.StallOrders["鹏华"]))
	}
	if result.StallOrders["鹏华"][0].Quantity != 5 {
		t.Errorf("数量 = %d, want 5", result.StallOrders["鹏华"][0].Quantity)
	}
}

// 买家留言 / 卖家备注：逐行保留，不去重不合并
func TestProcessData_BuyerSellerNotes(t *testing.T) {
	engine := &Engine{
		Items: map[string]ConfigItem{
			"988808938880|喜马拉雅白鳄鱼": {Stall: "鹏华"},
		},
		Stalls: []string{"鹏华"},
	}
	headers := []string{"商品id", "商品规格", "商品数量", "买家留言", "卖家备注"}
	rows := [][]string{
		{"988808938880", "iPhone15|喜马拉雅白鳄鱼", "1", "要硬壳", "加急"},
		{"988808938880", "iPhone15|喜马拉雅白鳄鱼", "1", "要硬壳", "加急"},
		{"988808938880", "iPhone15|喜马拉雅白鳄鱼", "1", "要硅胶", "缺货换款"},
		{"988808938880", "HuaweiPura70|喜马拉雅白鳄鱼", "1", "", "补发"},
	}

	result := ProcessData(rows, headers, engine)
	penghua := result.StallOrders["鹏华"]
	if len(penghua) != 4 {
		t.Fatalf("鹏华订单行数 = %d, want 4", len(penghua))
	}

	// 每行保留自己的留言/备注，不去重不合并
	if penghua[0].BuyerNote != "要硬壳" {
		t.Errorf("第1行买家留言 = %q, want %q", penghua[0].BuyerNote, "要硬壳")
	}
	if penghua[1].BuyerNote != "要硬壳" {
		t.Errorf("第2行买家留言 = %q, want %q", penghua[1].BuyerNote, "要硬壳")
	}
	if penghua[2].BuyerNote != "要硅胶" {
		t.Errorf("第3行买家留言 = %q, want %q", penghua[2].BuyerNote, "要硅胶")
	}
	if penghua[2].SellerNote != "缺货换款" {
		t.Errorf("第3行卖家备注 = %q, want %q", penghua[2].SellerNote, "缺货换款")
	}
	if penghua[3].BuyerNote != "" {
		t.Errorf("第4行买家留言 = %q, want 空", penghua[3].BuyerNote)
	}
	if penghua[3].SellerNote != "补发" {
		t.Errorf("第4行卖家备注 = %q, want %q", penghua[3].SellerNote, "补发")
	}
}

// 表头无买家留言/卖家备注列时，不报错，输出为空字符串
func TestProcessData_NoNotesColumns(t *testing.T) {
	engine := &Engine{
		Items:  map[string]ConfigItem{"988808938880|单品": {Stall: "鹏华"}},
		Stalls: []string{"鹏华"},
	}
	headers := []string{"商品id", "商品规格", "商品数量"}
	rows := [][]string{
		{"988808938880", "单品", "2"},
	}
	result := ProcessData(rows, headers, engine)
	penghua := result.StallOrders["鹏华"]
	if len(penghua) != 1 {
		t.Fatalf("鹏华订单行数 = %d, want 1", len(penghua))
	}
	if penghua[0].BuyerNote != "" || penghua[0].SellerNote != "" {
		t.Errorf("留言/备注应为空, got buyer=%q seller=%q", penghua[0].BuyerNote, penghua[0].SellerNote)
	}
}

// spec 格式反转（sku|model）时仍能正确匹配并输出 model
func TestProcessData_ReversedSpecFormat(t *testing.T) {
	engine := &Engine{
		Items: map[string]ConfigItem{
			"988808938880|喜马拉雅白鳄鱼": {Stall: "鹏华"},
		},
		Stalls: []string{"鹏华"},
	}

	headers := []string{"商品id", "商品规格", "商品数量"}
	rows := [][]string{
		// 旧格式 model|sku
		{"988808938880", "iPhone15|喜马拉雅白鳄鱼", "2"},
		// 新格式 sku|model
		{"988808938880", "喜马拉雅白鳄鱼|iPhone15", "1"},
	}

	result := ProcessData(rows, headers, engine)
	penghua := result.StallOrders["鹏华"]
	if len(penghua) != 2 {
		t.Fatalf("鹏华订单行数 = %d, want 2 (旧格式1 + 新格式1)", len(penghua))
	}

	// 两行的 model 都应该是 iPhone15
	for i, r := range penghua {
		if r.Model != "iPhone15" {
			t.Errorf("第%d行 model = %q, want iPhone15", i+1, r.Model)
		}
		if r.SKU != "喜马拉雅白鳄鱼" {
			t.Errorf("第%d行 SKU = %q, want 喜马拉雅白鳄鱼", i+1, r.SKU)
		}
	}
}

// 端到端：用 data/ 下的真实文件加载 + 处理
func TestLoadEngineAndProcess_RealFile(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "data", "皮质壳配置表.xlsx")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Skip("配置文件不存在，跳过端到端测试")
	}

	engine, err := LoadEngine(cfgPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if len(engine.Stalls) < 2 {
		t.Errorf("档口数 = %d, want >= 2", len(engine.Stalls))
	}

	if len(engine.Items) == 0 {
		t.Fatal("配置项为空")
	}

	withImg := 0
	for _, item := range engine.Items {
		if item.ImageBytes != nil {
			withImg++
		}
	}
	if withImg == 0 {
		t.Error("没有配置项包含图片数据")
	}
	t.Logf("加载 %d 个档口, %d 个配置项, %d 个有图", len(engine.Stalls), len(engine.Items), withImg)

	orderPath := filepath.Join("..", "..", "data", "皮质打图发货单测试.xlsx")
	if _, err := os.Stat(orderPath); os.IsNotExist(err) {
		t.Skip("订单文件不存在，跳过处理测试")
	}

	result, err := Process(orderPath, cfgPath)
	if err != nil {
		t.Fatalf("处理失败: %v", err)
	}

	if result.OutputPath == "" {
		t.Error("输出路径为空")
	}
	t.Logf("输出: %s, 档口数: %d", result.OutputPath, len(result.StallOrders))

	if _, err := os.Stat(result.OutputPath); err != nil {
		t.Errorf("输出文件不存在: %v", err)
	}
}
