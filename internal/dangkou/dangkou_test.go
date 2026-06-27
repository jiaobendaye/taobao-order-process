package dangkou

import (
	"fmt"
	"testing"

	"taobao/internal/common"
)

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
