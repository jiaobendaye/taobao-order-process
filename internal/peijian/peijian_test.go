package peijian

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// ---- specAfterPipe ----

func TestSpecAfterPipe(t *testing.T) {
	cases := []struct {
		spec string
		want string
	}{
		{"iPhone15|透明壳+支架[黑色]", "透明壳+支架[黑色]"},
		{"iPhone15|透明壳", "透明壳"},
		{"没有竖线", "没有竖线"},
		{"a|b|c", "c"}, // 多个|，取最后一个之后
		{"|onlyRight", "onlyRight"},
		{"onlyLeft|", ""},
	}
	for _, c := range cases {
		got := specAfterPipe(c.spec)
		if got != c.want {
			t.Errorf("specAfterPipe(%q) = %q, want %q", c.spec, got, c.want)
		}
	}
}

// ---- isFalseAccessory ----

func TestIsFalseAccessory(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"镜头", true},
		{"不含支架", true},
		{"透明壳+镜头绳", true},
		{"支架", false},
		{"吸盘", false},
		{"镜头支架", true}, // 包含"镜头"
	}
	for _, c := range cases {
		got := isFalseAccessory(c.s)
		if got != c.want {
			t.Errorf("isFalseAccessory(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

// ---- isSimpleOrder ----

func TestIsSimpleOrder(t *testing.T) {
	cases := []struct {
		order Order
		want  bool
	}{
		{Order{Spec: "iPhone15|透明壳+支架[黑色]"}, true},        // 有+且无备注
		{Order{Spec: "iPhone15|透明壳+支架", BuyerMsg: "备注"}, false}, // 有买家留言
		{Order{Spec: "iPhone15|透明壳+支架", SellerNote: "备注"}, false}, // 有卖家备注
		{Order{Spec: "iPhone15|透明壳+咨询"}, false},               // 含"咨询"
		{Order{Spec: "iPhone15|透明壳备注"}, false},                // 含"备注"
		{Order{Spec: "iPhone15|透明壳"}, false},                   // 无+
		{Order{Spec: "iPhone15|透明壳", BuyerMsg: "", SellerNote: ""}, false}, // 无+无备注
	}
	for _, c := range cases {
		got := isSimpleOrder(c.order)
		if got != c.want {
			t.Errorf("isSimpleOrder(%+v) = %v, want %v", c.order, got, c.want)
		}
	}
}

// ---- hasPotentialAccessory ----

func TestHasPotentialAccessory(t *testing.T) {
	cases := []struct {
		order Order
		want  bool
	}{
		// 有+的
		{Order{Spec: "iPhone15|壳+支架"}, true},
		// 不含壳
		{Order{Spec: "iPhone15|不含壳"}, true},
		{Order{Spec: "iPhone15|单独支架"}, true},
		{Order{Spec: "iPhone15|单独吸盘"}, true},
		{Order{Spec: "iPhone15|单独串珠"}, true},
		// 括号内有配件关键词
		{Order{Spec: "iPhone15|壳[支架]"}, true},
		{Order{Spec: "iPhone15|壳[吸盘]"}, true},
		// 括号内是镜头（false accessory）
		{Order{Spec: "iPhone15|壳[镜头]"}, false},
		// DIY 或 搭配
		{Order{Spec: "iPhone15|diy"}, true},
		{Order{Spec: "iPhone15|搭配"}, true},
		// 普通壳
		{Order{Spec: "iPhone15|透明壳"}, false},
		// "单独"但没有配件关键词
		{Order{Spec: "iPhone15|单独透明壳"}, false},
	}
	for _, c := range cases {
		got := hasPotentialAccessory(c.order)
		if got != c.want {
			t.Errorf("hasPotentialAccessory(%+v) = %v, want %v", c.order, got, c.want)
		}
	}
}

// ---- normalizeSegment ----

func TestNormalizeSegment(t *testing.T) {
	cases := []struct {
		seg  string
		want string
	}{
		{"支架[黑色]", "支架"},
		{"支架[黑色][大号]", "支架"}, // 多层 [] 括号
		{"普通支架", "普通支架"},
		{" 支架 [黑] ", "支架"},
	}
	for _, c := range cases {
		got := normalizeSegment(c.seg)
		if got != c.want {
			t.Errorf("normalizeSegment(%q) = %q, want %q", c.seg, got, c.want)
		}
	}
}

// ---- matchPart ----

func TestMatchPart(t *testing.T) {
	cfg := DefaultPartsConfig()

	cases := []struct {
		seg       string
		wantName  string
		wantColor string
		wantMatch bool
	}{
		{"旋转折叠支架[黑色]", "旋转折叠支架", "无", true}, // normalizeSegment 去掉了 [黑色]
		{"软胶吸盘", "软胶吸盘", "无", true},
		{"透明壳", "", "", false},
		{"镜头", "", "", false},
		{"撞色串珠", "撞色串珠", "无", true},
		{"编织手提绳[红色]", "编织手提绳", "无", true}, // normalizeSegment 去掉了 [红色]
		{"爱心镜", "爱心镜", "无", true},
		{" 支架  [ 深 蓝 ] ", "支架", "无", true}, // normalizeSegment 去掉了 [ 深 蓝 ]
	}
	for _, c := range cases {
		name, color, matched := matchPart(c.seg, cfg)
		if matched != c.wantMatch {
			t.Errorf("matchPart(%q) matched = %v, want %v", c.seg, matched, c.wantMatch)
		}
		if name != c.wantName {
			t.Errorf("matchPart(%q) name = %q, want %q", c.seg, name, c.wantName)
		}
		if color != c.wantColor {
			t.Errorf("matchPart(%q) color = %q, want %q", c.seg, color, c.wantColor)
		}
	}
}

// ---- ExtractParts ----

func TestExtractParts(t *testing.T) {
	cfg := DefaultPartsConfig()

	cases := []struct {
		order Order
		want  []PartDetail
	}{
		{
			Order{Spec: "iPhone15|透明壳+支架[黑色]", Quantity: 2},
			[]PartDetail{{Name: "支架", Color: "无", Quantity: 2}},
		},
		{
			Order{Spec: "iPhone15|壳+旋转折叠支架[蓝]+吸盘", Quantity: 1},
			[]PartDetail{
				{Name: "旋转折叠支架", Color: "无", Quantity: 1},
				{Name: "吸盘", Color: "无", Quantity: 1},
			},
		},
		{
			Order{Spec: "iPhone15|透明壳", Quantity: 1},
			nil, // 无+分隔
		},
		{
			Order{Spec: "iPhone15|壳+透明壳", Quantity: 3},
			nil, // 透明壳不是配件
		},
	}
	for _, c := range cases {
		got := ExtractParts(c.order, cfg)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ExtractParts(%+v) = %+v, want %+v", c.order, got, c.want)
		}
	}
}

// ---- findCol ----

func TestFindCol(t *testing.T) {
	headers := []string{"商品规格", "买家留言", " 商品数量 "}
	cases := []struct {
		name string
		want int
	}{
		{"商品规格", 0},
		{"买家留言", 1},
		{"商品数量", 2},  // TrimSpace 后匹配
		{"商品id", -1},  // 不存在
		{"shangpin", -1},
	}
	for _, c := range cases {
		got := findCol(headers, c.name)
		if got != c.want {
			t.Errorf("findCol(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}

// ---- classifyOrders ----

func TestClassifyOrders(t *testing.T) {
	orders := []Order{
		{Spec: "iPhone15|壳+支架"},               // simple
		{Spec: "iPhone15|壳+支架", BuyerMsg: "x"}, // pending (有买家留言)
		{Spec: "iPhone15|壳[支架]"},               // pending (括号内配件)
		{Spec: "iPhone15|透明壳"},                 // noParts
		{Spec: "iPhone15|diy"},                   // pending
	}
	simple, pending, noParts := classifyOrders(orders)

	if len(simple) != 1 {
		t.Errorf("simple count = %d, want 1", len(simple))
	}
	if len(pending) != 3 {
		t.Errorf("pending count = %d, want 3", len(pending))
	}
	if len(noParts) != 1 {
		t.Errorf("noParts count = %d, want 1", len(noParts))
	}
}

// ---- Merge 聚合逻辑 ----

func TestMergeAggregation(t *testing.T) {
	// 模拟 Merge 中的聚合逻辑
	agg := make(map[string]int)
	agg["支架|黑色"] = 5
	agg["支架|蓝色"] = 3
	agg["吸盘|无"] = 10
	agg["支架|黑色"] += 2 // 同 key 累加 = 7

	var entries []MergeEntry
	for k, v := range agg {
		parts := strings.SplitN(k, "|", 2)
		color := ""
		if len(parts) > 1 {
			color = parts[1]
		}
		entries = append(entries, MergeEntry{Name: parts[0], Color: color, Qty: v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Qty > entries[j].Qty })

	if len(entries) != 3 {
		t.Fatalf("entries count = %d, want 3", len(entries))
	}
	if entries[0].Name != "吸盘" || entries[0].Qty != 10 {
		t.Errorf("top entry = %+v, want 吸盘/10", entries[0])
	}
	if entries[1].Name != "支架" || entries[1].Qty != 7 {
		t.Errorf("second entry = %+v, want 支架/7", entries[1])
	}
}
