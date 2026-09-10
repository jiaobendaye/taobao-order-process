// Package datu 打图工厂分配测试
package datu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// ---- ParseDatuCode ----

func TestParseDatuCode(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantMaterial string
		wantCode     string
	}{
		{"标准格式带连字符", "【DYT彩银白色-DTY7958】", "DYT彩银白色", "DTY7958"},
		{"无连字符", "【DTY彩银白色DTY7958】", "", ""},
		{"缺少右括号", "【DYT彩银白色-DTY7958", "", ""},
		{"空字符串", "", "", ""},
		{"多个连字符 SplitN 限 2", "【AB-CD-EF】", "AB", "CD-EF"},
		{"带前后空格", "  【PH仓-皮质硬壳】  ", "PH仓", "皮质硬壳"},
		{"只有方括号无内容", "【】", "", ""},
		{"方括号中间单字段", "【PH皮质】", "", ""},
		{"多个括号取最后一个", "【PH仓】【DYT彩银白色-DTY7958】", "DYT彩银白色", "DTY7958"},
		{"多个括号取最后一个2", "【A-B】【C-D】", "C", "D"},
		{"三个括号取最后一个", "【X-Y】【A-B】【C-D】", "C", "D"},
		{"多个括号最后一个无连字符", "【A-B】【PH皮质】", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, c := ParseDatuCode(tt.input)
			if m != tt.wantMaterial {
				t.Errorf("material = %q, want %q", m, tt.wantMaterial)
			}
			if c != tt.wantCode {
				t.Errorf("code = %q, want %q", c, tt.wantCode)
			}
		})
	}
}

// ---- ProcessData 明细行（一行一订单, 不聚合） ----

func newEngine() *Engine {
	return &Engine{
		FactoryByProductID: map[string]string{
			"1050879735957": "打图工厂1",
			"1053838905482": "打图工厂1",
			"1058553670530": "打图工厂1",
			"1058691249562": "打图工厂1",
		},
		Factories: []string{"打图工厂1"},
	}
}

func TestProcessData_OneRowPerOrder_NoAggregation(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量", "付款时间"}
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【DYT彩银白色-DTY7958】", "1", "2026-06-28 10:56:42"},
		{"1050879735957", "小米 14|薄荷海", "【DYT彩银白色-DTY7958】", "2", "2026-06-28 11:00:00"},
		{"1053838905482", "华为 Mate 60 Pro|薄荷海", "【DYT彩银白色-DTY7958】", "3", "2026-06-28 11:05:00"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	// 不聚合: 3 订单 → 3 行
	if len(got) != 3 {
		t.Fatalf("rows = %d, want 3 (一行一订单, 不聚合)", len(got))
	}
	// Total 等于输出行数
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	// 验证每行的 Quantity 没有累加
	for _, r := range got {
		if r.Quantity == 6 || r.Quantity == 5 {
			t.Errorf("Quantity 不应累加: %d", r.Quantity)
		}
	}
}

func TestProcessData_DifferentCodeMultipleRows(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量", "付款时间"}
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【DYT彩银白色-DTY7958】", "1", "2026-06-28 10:00:00"},
		{"1050879735957", "小米 14|薄荷海", "【DYT冰雾魔方透白-DTY7959】", "1", "2026-06-28 10:30:00"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2 (不同编码 → 不同行)", len(got))
	}
	if got[0].Code != "DTY7958" || got[1].Code != "DTY7959" {
		t.Errorf("编码顺序错乱")
	}
}

func TestProcessData_DifferentMaterialMultipleRows(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量", "付款时间"}
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【DYT彩银白色-DTY7958】", "1", "2026-06-28 10:00:00"},
		{"1050879735957", "小米 14|薄荷海", "【DTY彩银白色-DTY7958】", "1", "2026-06-28 11:00:00"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2 (不同素材 → 不同行)", len(got))
	}
}

func TestProcessData_UnmatchedCodeProducesEmptyFields(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量", "付款时间"}
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【PH皮质】", "1", "2026-06-28 10:56:42"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	if got[0].Code != "" || got[0].Material != "" {
		t.Errorf("编码/素材应为空, got (%q,%q)", got[0].Code, got[0].Material)
	}
	if got[0].Model != "小米14" {
		t.Errorf("手机型号 = %q, want 小米14", got[0].Model)
	}
	if got[0].PaymentTime != "2026-06-28 10:56:42" {
		t.Errorf("PaymentTime = %q, want %q", got[0].PaymentTime, "2026-06-28 10:56:42")
	}
}

func TestProcessData_SkipNonDatuOrders(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量", "付款时间"}
	rows := [][]string{
		{"999999999999", "iPhone 15|某SKU", "【PH仓-皮质硬壳】", "1", "2026-06-28 12:00:00"}, // 不在 Engine
		{"1050879735957", "小米 14|薄荷海", "【DYT彩银白色-DTY7958】", "1", "2026-06-28 12:30:00"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1 (非打图订单应跳过)", len(got))
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1 (只算匹配订单)", result.Total)
	}
}

func TestProcessData_MultipleFactories(t *testing.T) {
	engine := &Engine{
		FactoryByProductID: map[string]string{
			"1111111111111": "打图工厂1",
			"2222222222222": "打图工厂2",
		},
		Factories: []string{"打图工厂1", "打图工厂2"},
	}
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量", "付款时间"}
	rows := [][]string{
		{"1111111111111", "iPhone15|A", "【X-001】", "2", "2026-06-28 10:00:00"},
		{"2222222222222", "华为Pura70|B", "【Y-002】", "3", "2026-06-28 11:00:00"},
	}
	result := ProcessData(rows, headers, engine)
	if len(result.FactoryOrders["打图工厂1"]) != 1 {
		t.Errorf("打图工厂1 rows = %d, want 1", len(result.FactoryOrders["打图工厂1"]))
	}
	if len(result.FactoryOrders["打图工厂2"]) != 1 {
		t.Errorf("打图工厂2 rows = %d, want 1", len(result.FactoryOrders["打图工厂2"]))
	}
	if result.FactoryOrders["打图工厂1"][0].Quantity != 2 {
		t.Errorf("打图工厂1 数量 = %d, want 2", result.FactoryOrders["打图工厂1"][0].Quantity)
	}
}

func TestProcessData_QuantityFallback(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "付款时间"} // 无商品数量列
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【X-001】", "2026-06-28 10:00:00"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	if got[0].Quantity != 1 {
		t.Errorf("数量缺省应为 1, got %d", got[0].Quantity)
	}
}

// ---- 付款时间列 ----

func TestProcessData_PaymentTimePassThrough(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量", "付款时间"}
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【DYT彩银白色-DTY7958】", "1", "2026-06-28 10:56:42"},
		{"1053838905482", "华为 Mate 60 Pro|薄荷海", "【DYT彩银白色-DTY7958】", "1", "2026-06-28 11:00:00"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2", len(got))
	}
	if got[0].PaymentTime != "2026-06-28 10:56:42" {
		t.Errorf("PaymentTime[0] = %q, want %q", got[0].PaymentTime, "2026-06-28 10:56:42")
	}
	if got[1].PaymentTime != "2026-06-28 11:00:00" {
		t.Errorf("PaymentTime[1] = %q, want %q", got[1].PaymentTime, "2026-06-28 11:00:00")
	}
}

func TestProcessData_MissingPayTimeColumn(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量"} // 无 付款时间 列
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【DYT彩银白色-DTY7958】", "1"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	if got[0].PaymentTime != "" {
		t.Errorf("无 付款时间 列时 PaymentTime 应为空, got %q", got[0].PaymentTime)
	}
}

func TestProcessData_NameIsFixed(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码"}
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【X-001】"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if got[0].Name != DefaultName {
		t.Errorf("Name = %q, want %q", got[0].Name, DefaultName)
	}
}

// ---- 买家留言 / 卖家备注 ----

func TestProcessData_BuyerAndSellerNotesPassThrough(t *testing.T) {
	engine := newEngine()
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量", "付款时间", "买家留言", "卖家备注"}
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【DYT彩银白色-DTY7958】", "1", "2026-06-28 10:00:00", "请尽快发货", "已通知仓库"},
		{"1053838905482", "华为 Mate 60 Pro|薄荷海", "【DYT彩银白色-DTY7958】", "2", "2026-06-28 11:00:00", "", ""},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2", len(got))
	}
	if got[0].BuyerNote != "请尽快发货" {
		t.Errorf("BuyerNote[0] = %q, want %q", got[0].BuyerNote, "请尽快发货")
	}
	if got[0].SellerNote != "已通知仓库" {
		t.Errorf("SellerNote[0] = %q, want %q", got[0].SellerNote, "已通知仓库")
	}
	if got[1].BuyerNote != "" || got[1].SellerNote != "" {
		t.Errorf("第二行备注应为空, got (%q,%q)", got[1].BuyerNote, got[1].SellerNote)
	}
}

func TestProcessData_MissingNoteColumns(t *testing.T) {
	engine := newEngine()
	// 源文件无 买家留言 / 卖家备注 列
	headers := []string{"商品id", "商品规格", "商品规格商家编码", "商品数量", "付款时间"}
	rows := [][]string{
		{"1050879735957", "小米 14|薄荷海", "【DYT彩银白色-DTY7958】", "1", "2026-06-28 10:00:00"},
	}
	result := ProcessData(rows, headers, engine)
	got := result.FactoryOrders["打图工厂1"]
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	if got[0].BuyerNote != "" {
		t.Errorf("无列时 BuyerNote 应为空, got %q", got[0].BuyerNote)
	}
	if got[0].SellerNote != "" {
		t.Errorf("无列时 SellerNote 应为空, got %q", got[0].SellerNote)
	}
}

func TestWriteFactorySheet_HeadersIncludeNotes(t *testing.T) {
	// 端到端：写 Excel 后读回来，验证表头包含 买家留言、卖家备注 两列
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "test.xlsx")

	engine := newEngine()
	result := &Result{
		FactoryOrders: map[string][]OutputRow{
			"打图工厂1": {
				{Code: "DTY7958", Model: "小米14", Material: "DYT彩银白色", Quantity: 1, Name: DefaultName, PaymentTime: "2026-06-28 10:00:00", BuyerNote: "留言A", SellerNote: "备注B"},
			},
		},
	}
	if err := writeOutput(outPath, engine, result); err != nil {
		t.Fatalf("writeOutput 失败: %v", err)
	}

	f, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("打开输出文件失败: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("打图工厂1")
	if err != nil {
		t.Fatalf("读 sheet 失败: %v", err)
	}
	wantHeaders := []string{"序号", "编码", "手机型号", "素材", "数量", "姓名", "付款时间", "买家留言", "卖家备注"}
	if len(rows) < 1 {
		t.Fatal("输出 sheet 为空")
	}
	gotHeaders := rows[0]
	if len(gotHeaders) != len(wantHeaders) {
		t.Fatalf("表头列数 = %d, want %d (got=%v)", len(gotHeaders), len(wantHeaders), gotHeaders)
	}
	for i, h := range wantHeaders {
		if gotHeaders[i] != h {
			t.Errorf("表头[%d] = %q, want %q", i, gotHeaders[i], h)
		}
	}
	// 第二行的 H、I 列应该是买家留言、卖家备注
	if len(rows) < 2 {
		t.Fatal("无数据行")
	}
	if rows[1][7] != "留言A" {
		t.Errorf("买家留言列 = %q, want %q", rows[1][7], "留言A")
	}
	if rows[1][8] != "备注B" {
		t.Errorf("卖家备注列 = %q, want %q", rows[1][8], "备注B")
	}
}

// ---- Engine.LookupFactory ----

func TestLookupFactory(t *testing.T) {
	engine := newEngine()
	if got := engine.LookupFactory("1050879735957"); got != "打图工厂1" {
		t.Errorf("已知 ID = %q, want 打图工厂1", got)
	}
	if got := engine.LookupFactory("999999999999"); got != "" {
		t.Errorf("未知 ID 应返回空, got %q", got)
	}
}

// ---- 端到端：data/ 下的真实文件 ----

func TestLoadEngineAndProcess_RealFile(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "data", "打图工厂配置表.xlsx")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Skip("打图工厂配置文件不存在，跳过端到端测试")
	}

	engine, err := LoadEngine(cfgPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if len(engine.Factories) == 0 {
		t.Fatal("未识别到任何工厂")
	}
	if len(engine.FactoryByProductID) == 0 {
		t.Fatal("未识别到任何商品ID")
	}
	t.Logf("加载 %d 个工厂, %d 个商品ID", len(engine.Factories), len(engine.FactoryByProductID))

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
	if _, err := os.Stat(result.OutputPath); err != nil {
		t.Errorf("输出文件不存在: %v", err)
	}
	t.Logf("输出: %s, 工厂数: %d, 行数: %d",
		result.OutputPath, len(result.FactoryOrders), totalOrders(result))
}

func totalOrders(r *Result) int {
	n := 0
	for _, rows := range r.FactoryOrders {
		n += len(rows)
	}
	return n
}
