package peijian

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"

	"taobao/internal/common"
)

// ---- extractAccessories 测试 ----

func TestExtractAccessories_WithPlus(t *testing.T) {
	tests := []struct {
		sku  string
		want []string
	}{
		{"奶油蓝+红苹果支架+红色手提绳", []string{"红苹果支架", "红色手提绳"}},
		{"芝士奶黄+小黄花串珠挂件+圆形挂钩", []string{"小黄花串珠挂件", "圆形挂钩"}},
		{"薄荷曼波-大孔镜头+黄油美式磁吸支架", []string{"黄油美式磁吸支架"}},
		{"壳+A配件+B配件+C配件", []string{"A配件", "B配件", "C配件"}},
	}
	for _, tt := range tests {
		t.Run(tt.sku, func(t *testing.T) {
			got := extractAccessories(tt.sku)
			if !equalStrings(got, tt.want) {
				t.Errorf("extractAccessories(%q) = %v, want %v", tt.sku, got, tt.want)
			}
		})
	}
}

func TestExtractAccessories_NoPlus(t *testing.T) {
	tests := []struct {
		sku  string
		want []string
	}{
		{"蓝色蝴蝶结", []string{"蓝色蝴蝶结"}},
		{"透明壳", []string{"透明壳"}},
		{"", nil},
	}
	for _, tt := range tests {
		t.Run(tt.sku, func(t *testing.T) {
			got := extractAccessories(tt.sku)
			if !equalStrings(got, tt.want) {
				t.Errorf("extractAccessories(%q) = %v, want %v", tt.sku, got, tt.want)
			}
		})
	}
}

func TestExtractAccessories_Spaces(t *testing.T) {
	got := extractAccessories(" 奶油蓝 + 红苹果支架 + 红色手提绳 ")
	want := []string{"红苹果支架", "红色手提绳"}
	if !equalStrings(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// ---- LoadEngine / loadPeijianMapping 测试 ----

func TestLoadEngine_WithTestData(t *testing.T) {
	configPath := filepath.Join("..", "..", "data", "配件编码测试.xlsx")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("测试数据文件不存在: " + configPath)
	}

	engine, err := LoadEngine(configPath)
	if err != nil {
		t.Fatalf("LoadEngine failed: %v", err)
	}

	// 验证 mapping
	key := toLower("990696037645" + "|" + "奶油蓝+红苹果支架+红色手提绳")
	codes, ok := engine.Mapping[key]
	if !ok {
		t.Fatal("mapping should contain 990696037645")
	}
	if len(codes) != 2 || codes[0] != "HT粘贴支架" || codes[1] != "SLL配件" {
		t.Errorf("wrong codes for 990696037645: %v", codes)
	}

	// 验证无 + 的 SKU
	key2 := toLower("904467045024" + "|" + "蓝色蝴蝶结")
	codes2, ok := engine.Mapping[key2]
	if !ok {
		t.Fatal("mapping should contain 904467045024")
	}
	if len(codes2) != 1 || codes2[0] != "DTY推拉支架" {
		t.Errorf("wrong codes for 904467045024: %v", codes2)
	}

	// 验证 stalls
	if stall := engine.Stalls[toLower("HT粘贴支架")]; stall != "鸿腾" {
		t.Errorf("HT粘贴支架 should be in 鸿腾, got %q", stall)
	}
	if stall := engine.Stalls[toLower("DTY推拉支架")]; stall != "大头鸭" {
		t.Errorf("DTY推拉支架 should be in 大头鸭, got %q", stall)
	}

	// 验证 stallOrder
	wantOrder := []string{"大头鸭", "有米", "鸿腾", "水玲珑", "1688"}
	if !equalStrings(engine.StallOrder, wantOrder) {
		t.Errorf("stallOrder = %v, want %v", engine.StallOrder, wantOrder)
	}
}

func TestLoadPeijianMapping_MultipleCodes(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "支架-自设编码"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"商品ID", "SKU名称", "编码1", "编码2", "编码3"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	data := [][]string{
		{"123", "壳A+配件1+配件2", "CODE1", "CODE2", ""},
		{"456", "壳B+配件3", "CODE3", "", ""},
		{"789", "单品配件", "CODE4", "", ""},
	}
	for r, row := range data {
		for c, val := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue(sheet, cell, val)
		}
	}

	mapping, err := loadPeijianMapping(f, sheet)
	if err != nil {
		t.Fatalf("loadPeijianMapping failed: %v", err)
	}

	key1 := toLower("123" + "|" + "壳A+配件1+配件2")
	if codes := mapping[key1]; len(codes) != 2 || codes[0] != "CODE1" || codes[1] != "CODE2" {
		t.Errorf("key1: got %v", codes)
	}

	key2 := toLower("456" + "|" + "壳B+配件3")
	if codes := mapping[key2]; len(codes) != 1 || codes[0] != "CODE3" {
		t.Errorf("key2: got %v", codes)
	}

	key3 := toLower("789" + "|" + "单品配件")
	if codes := mapping[key3]; len(codes) != 1 || codes[0] != "CODE4" {
		t.Errorf("key3: got %v", codes)
	}
}

func TestLoadStallMapping_ColumnLayout(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "档口分配"
	f.SetSheetName("Sheet1", sheet)
	f.SetCellValue(sheet, "A1", "档口A")
	f.SetCellValue(sheet, "B1", "档口B")
	f.SetCellValue(sheet, "C1", "档口C")

	f.SetCellValue(sheet, "A2", "CODE1")
	f.SetCellValue(sheet, "A3", "CODE2")
	f.SetCellValue(sheet, "B2", "CODE3")
	f.SetCellValue(sheet, "C2", "CODE4")
	f.SetCellValue(sheet, "C3", "CODE5")

	stalls, order, err := loadStallMapping(f, []string{sheet})
	if err != nil {
		t.Fatalf("loadStallMapping failed: %v", err)
	}

	if stalls[toLower("CODE1")] != "档口A" {
		t.Errorf("CODE1 should be in 档口A")
	}
	if stalls[toLower("CODE2")] != "档口A" {
		t.Errorf("CODE2 should be in 档口A")
	}
	if stalls[toLower("CODE3")] != "档口B" {
		t.Errorf("CODE3 should be in 档口B")
	}
	if stalls[toLower("CODE5")] != "档口C" {
		t.Errorf("CODE5 should be in 档口C")
	}

	wantOrder := []string{"档口A", "档口B", "档口C"}
	if !equalStrings(order, wantOrder) {
		t.Errorf("order = %v, want %v", order, wantOrder)
	}
}

func TestLoadStallMapping_MultipleSheets(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()

	sheet1 := "档口Sheet1"
	_, err := f.NewSheet(sheet1)
	if err != nil {
		t.Fatal(err)
	}
	f.SetCellValue(sheet1, "A1", "档口X")
	f.SetCellValue(sheet1, "A2", "CODEX")

	sheet2 := "档口Sheet2"
	_, err = f.NewSheet(sheet2)
	if err != nil {
		t.Fatal(err)
	}
	f.SetCellValue(sheet2, "A1", "档口Y")
	f.SetCellValue(sheet2, "A2", "CODEY")

	stalls, order, err := loadStallMapping(f, []string{sheet1, sheet2})
	if err != nil {
		t.Fatalf("loadStallMapping failed: %v", err)
	}

	if stalls[toLower("CODEX")] != "档口X" {
		t.Errorf("CODEX should be in 档口X")
	}
	if stalls[toLower("CODEY")] != "档口Y" {
		t.Errorf("CODEY should be in 档口Y")
	}
	if len(order) != 2 || order[0] != "档口X" || order[1] != "档口Y" {
		t.Errorf("order = %v", order)
	}
}

// ---- Process 集成测试 ----

func TestProcess_EndToEnd(t *testing.T) {
	configPath := filepath.Join("..", "..", "data", "配件编码测试.xlsx")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("测试数据文件不存在: " + configPath)
	}

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	headers := []string{"店铺名称", "订单编号", "子订单编号", "买家留言", "卖家备注", "商品id", "商品规格", "商品数量", "付款时间"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// 匹配的订单（有+）：990696037645 → HT粘贴支架(鸿腾), SLL配件(水玲珑)
	f.SetCellValue(sheet, "A2", "测试店")
	f.SetCellValue(sheet, "B2", "ORDER001")
	f.SetCellValue(sheet, "F2", "990696037645")
	f.SetCellValue(sheet, "G2", "iPhone15Pro|奶油蓝+红苹果支架+红色手提绳")
	f.SetCellValue(sheet, "H2", "3")

	// 匹配的订单（无+）：904467045024 → DTY推拉支架(大头鸭)
	f.SetCellValue(sheet, "A3", "测试店")
	f.SetCellValue(sheet, "B3", "ORDER002")
	f.SetCellValue(sheet, "F3", "904467045024")
	f.SetCellValue(sheet, "G3", "iPhone15Pro|蓝色蝴蝶结")
	f.SetCellValue(sheet, "H3", "1")

	// 匹配的订单（1个+）：899164787874 → HT磁吸支架(鸿腾)
	f.SetCellValue(sheet, "A4", "测试店")
	f.SetCellValue(sheet, "B4", "ORDER003")
	f.SetCellValue(sheet, "F4", "899164787874")
	f.SetCellValue(sheet, "G4", "iPhone15Pro|薄荷曼波-大孔镜头+黄油美式磁吸支架")
	f.SetCellValue(sheet, "H4", "2")

	// 匹配的订单（2个+）：1051641558158 → YM配件(有米), 16配件(1688)
	f.SetCellValue(sheet, "A5", "测试店")
	f.SetCellValue(sheet, "B5", "ORDER004")
	f.SetCellValue(sheet, "F5", "1051641558158")
	f.SetCellValue(sheet, "G5", "iPhone15Pro|芝士奶黄+小黄花串珠挂件+圆形挂钩")
	f.SetCellValue(sheet, "H5", "5")

	// 不匹配的订单
	f.SetCellValue(sheet, "A6", "测试店")
	f.SetCellValue(sheet, "B6", "ORDER009")
	f.SetCellValue(sheet, "F6", "999999999")
	f.SetCellValue(sheet, "G6", "iPhone15Pro|不存在的SKU")
	f.SetCellValue(sheet, "H6", "1")

	tmpDir := t.TempDir()
	orderFile := filepath.Join(tmpDir, "test_orders.xlsx")
	if err := f.SaveAs(orderFile); err != nil {
		t.Fatal(err)
	}

	result, err := Process(orderFile, configPath)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("total = %d, want 5", result.Total)
	}

	// 验证分配
	if result.Summary["大头鸭"] < 1 {
		t.Errorf("大头鸭 should have orders: summary=%v", result.Summary)
	}
	if result.Summary["鸿腾"] < 1 {
		t.Errorf("鸿腾 should have orders: summary=%v", result.Summary)
	}
	if result.Summary["水玲珑"] < 1 {
		t.Errorf("水玲珑 should have orders: summary=%v", result.Summary)
	}
	if result.Summary["有米"] < 1 {
		t.Errorf("有米 should have orders: summary=%v", result.Summary)
	}
	if result.Summary["1688"] < 1 {
		t.Errorf("1688 should have orders: summary=%v", result.Summary)
	}

	// NoMatch
	if len(result.NoMatch) != 1 {
		t.Errorf("noMatch = %d, want 1", len(result.NoMatch))
	}

	// Output file
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Errorf("output file not created: %s", result.OutputPath)
	}

	t.Logf("Summary: %v", result.Summary)
	t.Logf("Output: %s", result.OutputPath)
}

func TestLoadEngine_CodeCountValidation(t *testing.T) {
	// 创建编码数 ≠ 配件数的配置，验证 LoadEngine 加载时报错
	configFile := createTestConfig(t,
		[]string{"商品ID", "SKU名称", "编码1", "编码2"},
		[][]string{
			{"999", "壳+配件A", "CODE_A", "CODE_B"}, // 1个配件但2个编码
		},
		[]string{"测试档口"},
		[]string{"CODE_A"},
	)

	_, err := LoadEngine(configFile)
	if err == nil {
		t.Fatal("expected LoadEngine to reject config with mismatched code/accessory count")
	}
	t.Logf("Correctly rejected: %v", err)
}

// createTestConfig 创建测试用的配件编码 Excel（mapping + 档口分配）
func createTestConfig(t *testing.T, mapHeaders []string, mapData [][]string, stallNames []string, stallCodes []string) string {
	t.Helper()

	f := excelize.NewFile()
	defer f.Close()

	// Sheet 1: mapping
	sheet1 := "支架-自设编码"
	f.SetSheetName("Sheet1", sheet1)

	for i, h := range mapHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet1, cell, h)
	}
	for r, row := range mapData {
		for c, val := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue(sheet1, cell, val)
		}
	}

	// Sheet 2: 档口分配
	sheet2 := "档口分配"
	_, err := f.NewSheet(sheet2)
	if err != nil {
		t.Fatal(err)
	}

	for i, name := range stallNames {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet2, cell, name)
	}
	// 每个档口下的编码（简单处理：每列一个编码）
	for i, code := range stallCodes {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet2, cell, code)
	}

	tmpFile := filepath.Join(t.TempDir(), "test_config.xlsx")
	if err := f.SaveAs(tmpFile); err != nil {
		t.Fatal(err)
	}
	return tmpFile
}

// createTestOrder 创建测试用的订单 Excel
func createTestOrder(t *testing.T, headers []string, data [][]string) string {
	t.Helper()

	f := excelize.NewFile()
	defer f.Close()

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", cell, h)
	}
	for r, row := range data {
		for c, val := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue("Sheet1", cell, val)
		}
	}

	tmpFile := filepath.Join(t.TempDir(), "test_order.xlsx")
	if err := f.SaveAs(tmpFile); err != nil {
		t.Fatal(err)
	}
	return tmpFile
}

func TestProcess_UnassignedStall(t *testing.T) {
	// 创建不带档口分配的配置（只有mapping，没有stall）
	f := excelize.NewFile()
	defer f.Close()

	// Sheet 1: mapping
	sheet1 := "Sheet1"
	f.SetSheetName("Sheet1", sheet1)
	f.SetCellValue(sheet1, "A1", "商品ID")
	f.SetCellValue(sheet1, "B1", "SKU名称")
	f.SetCellValue(sheet1, "C1", "编码1")
	f.SetCellValue(sheet1, "A2", "123")
	f.SetCellValue(sheet1, "B2", "测试配件")
	f.SetCellValue(sheet1, "C2", "NO_SUCH_CODE")

	// Sheet 2: 空档口分配（编码不在任何档口）
	sheet2 := "档口分配"
	_, err := f.NewSheet(sheet2)
	if err != nil {
		t.Fatal(err)
	}
	f.SetCellValue(sheet2, "A1", "空档口")
	f.SetCellValue(sheet2, "A2", "SOME_OTHER_CODE")

	configFile := filepath.Join(t.TempDir(), "test_config.xlsx")
	if err := f.SaveAs(configFile); err != nil {
		t.Fatal(err)
	}

	// 创建订单
	f2 := excelize.NewFile()
	defer f2.Close()
	f2.SetCellValue("Sheet1", "A1", "商品id")
	f2.SetCellValue("Sheet1", "B1", "商品规格")
	f2.SetCellValue("Sheet1", "C1", "商品数量")
	f2.SetCellValue("Sheet1", "A2", "123")
	f2.SetCellValue("Sheet1", "B2", "Phone|测试配件")
	f2.SetCellValue("Sheet1", "C2", "1")

	orderFile := filepath.Join(t.TempDir(), "test_order.xlsx")
	if err := f2.SaveAs(orderFile); err != nil {
		t.Fatal(err)
	}

	result, err := Process(orderFile, configFile)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(result.Unassigned) != 1 {
		t.Errorf("expected 1 unassigned, got %d", len(result.Unassigned))
	}
}

// ---- 公共函数测试 ----

func TestParseSpec_WithCommon(t *testing.T) {
	tests := []struct {
		spec      string
		wantModel string
		wantSKU   string
	}{
		{"iPhone15Pro|透明壳[黑色]", "iPhone15Pro", "透明壳"},
		{"华为Pura 70 Pro+|薄荷波点+薄荷糖支架", "华为Pura70Pro+", "薄荷波点+薄荷糖支架"},
		{"iPhone 15 Pro Max|奶油蓝+红苹果支架+红色手提绳", "iPhone15ProMax", "奶油蓝+红苹果支架+红色手提绳"},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			model, sku := common.ParseSpec(tt.spec)
			if model != tt.wantModel {
				t.Errorf("model = %q, want %q", model, tt.wantModel)
			}
			if sku != tt.wantSKU {
				t.Errorf("sku = %q, want %q", sku, tt.wantSKU)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
		err   bool
	}{
		{"5", 5, false},
		{"100", 100, false},
		{"0", 0, false},
		{"abc", 0, true},
		{"12.5", 0, true},
		{"-1", 0, true},
	}
	for _, tt := range tests {
		got, err := parseInt(tt.input)
		if tt.err && err == nil {
			t.Errorf("parseInt(%q) should error", tt.input)
		}
		if !tt.err && err != nil {
			t.Errorf("parseInt(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("parseInt(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// ---- 辅助函数 ----

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func toLower(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result = append(result, c)
	}
	return string(result)
}
