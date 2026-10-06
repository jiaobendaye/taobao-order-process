package filter

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

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
