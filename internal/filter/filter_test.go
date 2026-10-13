package filter

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"taobao/internal/common"
)

func TestExportMultiItemKeepsDoubtfulPriority(t *testing.T) {
	rows, err := common.NormalizeOrderRows([][]string{
		{"订单编号", "店铺", "销售属性", "宝贝ID", "买家留言", "数量"},
		{"123", "shop", "适用手机型号：iPhone 16;颜色分类：普通壳", "101", "", "1"},
		{"", "", "颜色分类：单独支架", "102", "", "1"},
		{"456", "shop", "颜色分类：普通壳", "103", "", "2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	result := ProcessData(parsed, &Config{AccessoryKeywords: []string{"支架"}})
	if len(result.MultiOrders) != 2 || result.Summary.NormalOrders != 1 {
		t.Fatalf("multi=%d normal=%d", len(result.MultiOrders), result.Summary.NormalOrders)
	}
	parsed[1].BuyerMsg = "需要确认"
	result = ProcessData(parsed, &Config{AccessoryKeywords: []string{"支架"}})
	if len(result.DoubtfulOrders) != 1 || len(result.MultiOrders) != 1 {
		t.Fatalf("doubtful=%d multi=%d", len(result.DoubtfulOrders), len(result.MultiOrders))
	}
}

func TestParseRowsRejectsMultipleOrderIDs(t *testing.T) {
	_, err := ParseRows([][]string{{"订单编号", "商品规格"}, {"111,222", "型号|壳"}})
	if err == nil || !strings.Contains(err.Error(), "第2行") {
		t.Fatalf("expected row 2 validation error, got %v", err)
	}
}

func TestProcessDataOrderSummary(t *testing.T) {
	rows := []RowData{
		{OrderID: "multi", SubOrderID: "sub-1", BuyerNick: "buyer-1"},
		{OrderID: "multi", SubOrderID: "sub-2", BuyerNick: "buyer-2"},
		{OrderID: "doubt", BuyerMsg: "check", BuyerNick: "buyer-3"},
		{OrderID: "multi", BuyerMsg: "check", BuyerNick: "buyer-4"},
		{OrderID: "accessory", Spec: "支架", BuyerNick: "buyer-5"},
		{OrderID: "", Spec: "支架", BuyerNick: "buyer-6"},
		{OrderID: "normal", Spec: "手机壳", BuyerNick: "buyer-7"},
	}
	result := ProcessData(rows, &Config{AccessoryKeywords: []string{"支架"}})
	want := []string{"multi", "doubt", "accessory"}
	if !reflect.DeepEqual(result.OrderSummary, want) {
		t.Fatalf("OrderSummary = %v, want %v", result.OrderSummary, want)
	}
}

func TestWriteOutputOrderSummaryFirst(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "筛选结果.xlsx")
	result := &Result{OrderSummary: []string{"multi", "doubt", "accessory"}}
	if err := writeOutput(filename, result); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	wantSheets := []string{"多疑单订单汇总", "多件订单", "疑难单", "单独配件", "正常手机壳"}
	if sheets := f.GetSheetList(); !reflect.DeepEqual(sheets, wantSheets) {
		t.Fatalf("sheets = %v, want %v", sheets, wantSheets)
	}
	rows, err := f.GetRows(wantSheets[0])
	if err != nil {
		t.Fatal(err)
	}
	wantRows := [][]string{{"订单编号"}, {"multi"}, {"doubt"}, {"accessory"}}
	if !reflect.DeepEqual(rows, wantRows) {
		t.Fatalf("summary rows = %v, want %v", rows, wantRows)
	}
}

func TestFilterSpecMerchantCode(t *testing.T) {
	rows, err := ParseRows([][]string{
		{"订单编号", "商品商家编码", "商品规格商家编码", "商品规格", "商品数量"},
		{"123", "旧商品编码", "【素材-ABC123】", "iPhone|透明壳", "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "【素材-ABC123】" {
		t.Fatalf("rows=%+v", rows)
	}
	result := ProcessData(rows, &Config{})
	output := filepath.Join(t.TempDir(), "筛选结果.xlsx")
	if err := writeOutput(output, result); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, sheet := range []string{"多件订单", "疑难单", "单独配件", "正常手机壳"} {
		got, err := f.GetCellValue(sheet, "K1")
		if err != nil || got != "商家编码" {
			t.Fatalf("sheet=%s header=%q err=%v", sheet, got, err)
		}
	}
	got, err := f.GetCellValue("正常手机壳", "K2")
	if err != nil || got != "【素材-ABC123】" {
		t.Fatalf("code=%q err=%v", got, err)
	}
}

func TestParseRowsMerchantCodeAliases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers []string
		values  []string
		want    string
	}{
		{"新列", []string{"商家编码"}, []string{"new"}, "new"},
		{"旧列", []string{"商品规格商家编码"}, []string{"old"}, "old"},
		{"新列优先", []string{"商家编码", "商品规格商家编码"}, []string{"new", "old"}, "new"},
		{"列顺序不影响优先级", []string{"商品规格商家编码", "商家编码"}, []string{"old", "new"}, "new"},
		{"新列为空", []string{"商家编码", "商品规格商家编码"}, []string{"", "old"}, "old"},
		{"均为空", []string{"商家编码", "商品规格商家编码"}, []string{"", ""}, "未知"},
		{"短行", []string{"商家编码", "商品规格商家编码"}, []string{""}, "未知"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headers := append([]string{"订单编号"}, tc.headers...)
			values := append([]string{"123"}, tc.values...)
			rows, err := ParseRows([][]string{headers, values})
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].Code != tc.want {
				t.Fatalf("rows=%+v want=%q", rows, tc.want)
			}
		})
	}
}
