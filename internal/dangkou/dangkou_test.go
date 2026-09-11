package dangkou

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"taobao/internal/common"
)

// ---- ParseStallName 测试 ----

func TestParseStallName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantMarket  string
		wantNumber  string
		wantStall   string
	}{
		{"三段", "经济-4-国产哥GCG", "经济", "4", "国产哥GCG"},
		{"两段", "经济-4", "经济", "4", ""},
		{"一段", "经济", "经济", "", ""},
		{"空字符串", "", "", "", ""},
		{"含多个连字符取前三段", "A-B-C-D", "A", "B", "C"},
		{"带空格TrimSpace", "  经济 - 4 - 国产哥  ", "经济", "4", "国产哥"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, n, s := ParseStallName(tt.input)
			if m != tt.wantMarket || n != tt.wantNumber || s != tt.wantStall {
				t.Errorf("ParseStallName(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.input, m, n, s, tt.wantMarket, tt.wantNumber, tt.wantStall)
			}
		})
	}
}

// TestParseSpec_StripsSpaces 验证 parseSpec 对型号去空格的行为
func TestParseSpec_StripsSpaces(t *testing.T) {
	cases := []struct {
		spec         string
		wantModel    string
		wantSKU      string
	}{
		{"iPhone 15 Pro|透明壳[黑色]", "iPhone15Pro", "透明壳"},
		{"  Samsung S24  Ultra  | 硅胶壳 【蓝色】", "SamsungS24Ultra", "硅胶壳"},
		{"Pixel 8|磨砂壳", "Pixel8", "磨砂壳"},
		{"Xiaomi 14   Pro|防摔壳[红色]", "Xiaomi14Pro", "防摔壳"},
		{"  iPhone   15  Pro Max  | 硅胶壳 【蓝色】", "iPhone15ProMax", "硅胶壳"},
		// 不带 | 的情况：型号为空，整个 spec 即为 SKU
		{"JustModel NoPipe", "", "JustModel NoPipe"},
	}

	for _, c := range cases {
		model, sku := common.ParseSpec(c.spec)
		if model != c.wantModel {
			t.Errorf("common.ParseSpec(%q) model = %q, want %q", c.spec, model, c.wantModel)
		}
		if sku != c.wantSKU {
			t.Errorf("common.ParseSpec(%q) sku = %q, want %q", c.spec, sku, c.wantSKU)
		}
	}
}

// TestFindStall_ModelMatching 验证 FindStall 按型号匹配
func TestFindStall_ModelMatching(t *testing.T) {
	engine := &Engine{
		Stalls: []StallConfig{
			{
				Name:     "档口A",
				Priority: 0,
				Codes: map[string][]string{
					"a001": {"iPhone15Pro", "iPhone15ProMax"},
				},
			},
			{
				Name:     "档口B",
				Priority: 1,
				Codes: map[string][]string{
					"a001": {"SamsungS24Ultra", "Pixel8"},
				},
			},
		},
	}

	tests := []struct {
		zisheBianma string
		model       string
		wantStall   string
	}{
		{"A001", "iPhone15Pro", "档口A"},      // 型号匹配档口A
		{"A001", "SamsungS24Ultra", "档口B"},   // 型号匹配档口B，跳过档口A
		{"A001", "iPhone15ProMax", "档口A"},    // 型号匹配档口A（列表中第二个）
		{"A001", "Pixel8", "档口B"},            // 型号匹配档口B（列表中第二个）
		{"A001", "UnknownModel", ""},           // 型号不在任何档口的列表中
		{"A001", "", "档口A"},                  // model 为空时，匹配第一个有该编码的档口
		{"B999", "iPhone15Pro", ""},            // 编码不存在
		// 大小写不敏感
		{"a001", "iphone15pro", "档口A"},       // 编码和型号全小写
		{"A001", "IPHONE15PRO", "档口A"},       // 型号全大写
		{"a001", "SamsungS24Ultra", "档口B"},   // 编码小写
		{"A001", "pixel8", "档口B"},            // 型号小写
	}

	for _, tt := range tests {
		got := engine.FindStall(tt.zisheBianma, tt.model)
		if got != tt.wantStall {
			t.Errorf("FindStall(%q, %q) = %q, want %q",
				tt.zisheBianma, tt.model, got, tt.wantStall)
		}
	}
}

// TestFindStall_ModelMatching_Fixed 验证修复后型号参与档口匹配
//
// 修复前：
//
//	_, skuName := common.ParseSpec(spec)           // ← 型号被丢弃！
//	stall := engine.FindStall(zisheBianma)  // 只用编码匹配
//	→ SamsungS24Ultra 被错误分配到档口A
//
// 修复后：
//
//	model, skuName := common.ParseSpec(spec)
//	stall := engine.FindStall(zisheBianma, model)  // 编码 + 型号匹配
//	→ SamsungS24Ultra 正确分配到档口B
func TestFindStall_ModelMatching_Fixed(t *testing.T) {
	// ---- 模拟引擎配置 ----
	engine := &Engine{
		Mapping: map[string]string{
			"12345|透明壳": "A001",
		},
		Stalls: []StallConfig{
			{
				Name:     "档口A",
				Priority: 0,
				Codes: map[string][]string{
					"a001": {"iPhone15Pro", "iPhone15ProMax"},
				},
			},
			{
				Name:     "档口B",
				Priority: 1,
				Codes: map[string][]string{
					"a001": {"SamsungS24Ultra", "Pixel8"},
				},
			},
		},
	}

	orders := []struct {
		productID string
		spec      string
		wantStall string
	}{
		{productID: "12345", spec: "iPhone15Pro|透明壳[黑色]", wantStall: "档口A"},
		{productID: "12345", spec: "SamsungS24Ultra|透明壳[白色]", wantStall: "档口B"},
		{productID: "12345", spec: "Pixel8|透明壳[红色]", wantStall: "档口B"},
		// 型号带空格：parseSpec 去掉空格后正确匹配
		{productID: "12345", spec: " iPhone   15  Pro |透明壳", wantStall: "档口A"},
		{productID: "12345", spec: "Samsung S24 Ultra|透明壳[黑色]", wantStall: "档口B"},
		// 大小写不敏感
		{productID: "12345", spec: "iphone15pro|透明壳", wantStall: "档口A"},
		{productID: "12345", spec: "IPHONE15PROMAX|透明壳", wantStall: "档口A"},
		{productID: "12345", spec: "pixel8|透明壳[蓝色]", wantStall: "档口B"},
		// 混合：空格 + 大小写
		{productID: "12345", spec: " Iphone  15  Pro |透明壳", wantStall: "档口A"},
		{productID: "12345", spec: "UnknownModel|透明壳", wantStall: ""}, // 无档口匹配
	}

	fmt.Println("========== 验证修复：型号参与档口匹配 ==========")
	fmt.Println("档口A (a001): [iPhone15Pro, iPhone15ProMax]")
	fmt.Println("档口B (a001): [SamsungS24Ultra, Pixel8]")
	fmt.Println()

	for _, order := range orders {
		model, skuName := common.ParseSpec(order.spec)
		zisheBianma := engine.LookupZisheBianma(order.productID, skuName)
		stall := engine.FindStall(zisheBianma, model)

		status := "✅"
		if stall != order.wantStall {
			status = "❌"
			t.Errorf("FindStall(%q, %q) = %q, want %q",
				zisheBianma, model, stall, order.wantStall)
		}
		fmt.Printf("%s 规格=%q → 型号=%q → %q\n", status, order.spec, model, stall)
	}
}

// TestFindStall_ReversedSpecFormat 验证 spec 格式反转（sku|model）时仍能正确匹配档口
func TestFindStall_ReversedSpecFormat(t *testing.T) {
	engine := &Engine{
		Mapping: map[string]string{
			"12345|透明壳": "A001",
		},
		Stalls: []StallConfig{
			{
				Name:     "档口A",
				Priority: 0,
				Codes: map[string][]string{
					"a001": {"iPhone15Pro", "iPhone15ProMax"},
				},
			},
			{
				Name:     "档口B",
				Priority: 1,
				Codes: map[string][]string{
					"a001": {"SamsungS24Ultra", "Pixel8"},
				},
			},
		},
	}

	orders := []struct {
		productID string
		spec      string
		wantStall string
	}{
		// 旧格式 model|sku
		{productID: "12345", spec: "iPhone15Pro|透明壳[黑色]", wantStall: "档口A"},
		{productID: "12345", spec: "SamsungS24Ultra|透明壳[白色]", wantStall: "档口B"},
		// 新格式 sku|model
		{productID: "12345", spec: "透明壳[黑色]|iPhone15Pro", wantStall: "档口A"},
		{productID: "12345", spec: "透明壳[白色]|SamsungS24Ultra", wantStall: "档口B"},
		// 新格式带空格
		{productID: "12345", spec: "透明壳|Samsung S24 Ultra", wantStall: "档口B"},
		{productID: "12345", spec: "透明壳| iPhone   15  Pro Max ", wantStall: "档口A"},
		// 两侧都未命中
		{productID: "12345", spec: "透明壳|UnknownModel", wantStall: ""},
	}

	for _, order := range orders {
		part1, part2 := common.SplitSpec(order.spec)
		var zisheBianma, model string
		if code := engine.LookupZisheBianma(order.productID, part1); code != "" {
			zisheBianma = code
			model = strings.ReplaceAll(part2, " ", "")
		} else if code := engine.LookupZisheBianma(order.productID, part2); code != "" {
			zisheBianma = code
			model = strings.ReplaceAll(part1, " ", "")
		}
		stall := engine.FindStall(zisheBianma, model)

		status := "✅"
		if stall != order.wantStall {
			status = "❌"
			t.Errorf("spec=%q → stall=%q, want %q", order.spec, stall, order.wantStall)
		}
		fmt.Printf("%s 规格=%q → 编码=%q 型号=%q → %q\n", status, order.spec, zisheBianma, model, stall)
	}
}

// ---- 拿货档口.xlsx 生成测试 ----

func TestProcess_GeneratesNahuoOutput(t *testing.T) {
	// 构造自设编码配置：Sheet1(映射) + Sheet2(档口A) + Sheet3(档口B)
	configFile := createDangkouConfig(t,
		map[string]string{"12345|透明壳": "A001"},
		[]struct{ Name, Code, Model string }{
			{"经济-4-国产哥GCG", "A001", "iPhone15Pro"},
			{"康乐-4-伊点通YDT", "A001", "SamsungS24Ultra"},
		},
	)

	// 构造订单
	orderFile := createDangkouOrder(t, [][]string{
		{"12345", "iPhone15Pro|透明壳[黑色]", "1"},
		{"12345", "SamsungS24Ultra|透明壳[白色]", "2"},
	})

	result, err := Process(orderFile, configFile)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// 验证拿货档口.xlsx 已生成
	nahuoPath := filepath.Join(result.OutputDir, "拿货档口.xlsx")
	if _, err := os.Stat(nahuoPath); os.IsNotExist(err) {
		t.Fatalf("拿货档口.xlsx 未生成: %s", nahuoPath)
	}

	// 打开验证内容
	f, err := excelize.OpenFile(nahuoPath)
	if err != nil {
		t.Fatalf("打开拿货档口.xlsx 失败: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetList()[0])
	if err != nil {
		t.Fatalf("读取拿货档口 sheet 失败: %v", err)
	}
	if len(rows) < 1 {
		t.Fatalf("拿货档口 sheet 为空")
	}

	// 表头应为 产品数量 | 产品图片 | 市场 | 档口号 | 档口名称 | 支付状态 | 拿货备注
	wantHeaders := []string{"产品数量", "产品图片", "市场", "档口号", "档口名称", "支付状态", "拿货备注"}
	if len(rows[0]) < len(wantHeaders) {
		t.Fatalf("表头列数不足: %v", rows[0])
	}
	for i, h := range wantHeaders {
		if rows[0][i] != h {
			t.Errorf("表头[%d] = %q, want %q", i, rows[0][i], h)
		}
	}

	// 数据行应为 2 行，按市场排序；市场/档口号/档口名称 在第 3~5 列（索引 2~4），其余列为空
	// 经济-4-国产哥GCG → 经济, 4, 国产哥GCG
	// 康乐-4-伊点通YDT → 康乐, 4, 伊点通YDT
	// "康乐" < "经济" 按升序排序
	if len(rows) < 3 {
		t.Fatalf("数据行不足: %d 行（含表头应至少3行）", len(rows))
	}
	wantRow1 := []string{"", "", "康乐", "4", "伊点通YDT", "", ""}
	wantRow2 := []string{"", "", "经济", "4", "国产哥GCG", "", ""}
	padRow := func(r []string, n int) []string {
		out := make([]string, n)
		copy(out, r)
		return out
	}
	row1 := padRow(rows[1], len(wantRow1))
	row2 := padRow(rows[2], len(wantRow2))
	for i, w := range wantRow1 {
		if row1[i] != w {
			t.Errorf("数据行1[%d] = %q, want %q", i, row1[i], w)
		}
	}
	for i, w := range wantRow2 {
		if row2[i] != w {
			t.Errorf("数据行2[%d] = %q, want %q", i, row2[i], w)
		}
	}
}

// createDangkouConfig 构造测试用的自设编码 Excel
func createDangkouConfig(t *testing.T, mapping map[string]string, stalls []struct{ Name, Code, Model string }) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()

	// Sheet 1: 映射
	sheet1 := "自设编码"
	f.SetSheetName("Sheet1", sheet1)
	f.SetCellValue(sheet1, "A1", "商品ID")
	f.SetCellValue(sheet1, "B1", "SKU名称")
	f.SetCellValue(sheet1, "C1", "自设编码")
	row := 2
	for key, code := range mapping {
		parts := splitKey(key)
		f.SetCellValue(sheet1, fmt.Sprintf("A%d", row), parts[0])
		f.SetCellValue(sheet1, fmt.Sprintf("B%d", row), parts[1])
		f.SetCellValue(sheet1, fmt.Sprintf("C%d", row), code)
		row++
	}

	// 后续 Sheet: 档口
	for _, s := range stalls {
		f.NewSheet(s.Name)
		f.SetCellValue(s.Name, "A1", s.Code)
		f.SetCellValue(s.Name, "A2", s.Model)
	}

	path := filepath.Join(t.TempDir(), "test_dangkou_config.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func splitKey(key string) []string {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{key, ""}
}

// createDangkouOrder 构造测试用的订单 Excel
func createDangkouOrder(t *testing.T, rows [][]string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	f.SetCellValue("Sheet1", "A1", "商品id")
	f.SetCellValue("Sheet1", "B1", "商品规格")
	f.SetCellValue("Sheet1", "C1", "商品数量")
	for i, row := range rows {
		for j, val := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			f.SetCellValue("Sheet1", cell, val)
		}
	}
	path := filepath.Join(t.TempDir(), "test_dangkou_order.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}
