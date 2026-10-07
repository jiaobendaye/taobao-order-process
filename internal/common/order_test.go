package common

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestNormalizeOrderRowsExport(t *testing.T) {
	rows := [][]string{
		{"序号", "店铺", "收货人详细地址", "联系人", "联系电话", "订单编号", "买家留言", "销售属性", "数量", "宝贝ID"},
		{"1", "shop", "address", "name", "phone", "1234567890123456789", "note", "适用手机型号：iPhone 16 pro;颜色分类：红色+支架[新款]", "1", "991729207193"},
		{"", "", "", "", "", "", "", "颜色分类：单独支架", "2", "857197208121"},
		{"2", "shop2", "other", "other", "other", "222", "", "颜色分类：纯壳", "1", "3"},
	}
	got, err := NormalizeOrderRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		row          int
		header, want string
	}{
		{1, "商品规格", "iPhone 16 pro|红色+支架[新款]"},
		{2, "商品规格", "单独支架"},
		{2, "订单编号", "1234567890123456789"},
		{2, "买家留言", "note"},
		{2, "商品id", "857197208121"},
		{1, "订单组多件", "1"}, {2, "订单组多件", "1"},
		{3, "订单组多件", ""},
	} {
		col := FindColumn(got[0], tc.header)
		if col < 0 || col >= len(got[tc.row]) || got[tc.row][col] != tc.want {
			t.Errorf("row %d %s = %v, want %q", tc.row, tc.header, got[tc.row], tc.want)
		}
	}
	if rows[2][5] != "" {
		t.Fatal("input rows mutated")
	}
}

func TestNormalizeOrderRowsLegacyAndAliases(t *testing.T) {
	rows := [][]string{{"订单号", "商品规格", "销售属性", "商品数量", "商品id"}, {"123", "model|original", "颜色分类：replacement", "2", "999"}}
	got, err := NormalizeOrderRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	if got[1][FindColumn(got[0], "商品规格")] != "model|original" {
		t.Fatal("original spec overwritten")
	}
	if got[1][FindColumn(got[0], "订单编号")] != "123" {
		t.Fatal("order alias missing")
	}
	if FindColumn(got[0], "订单组多件") >= 0 {
		t.Fatal("legacy workbook marked as export")
	}
}

func TestNormalizeOrderRowsRejectsInvalidIDs(t *testing.T) {
	_, err := NormalizeOrderRows([][]string{{"订单号", "销售属性", "宝贝ID"}, {"111,222", "颜色分类：壳", "123"}})
	if err == nil || !strings.Contains(err.Error(), "第2行") {
		t.Fatalf("error = %v", err)
	}
}

func TestNormalizeOrderRowsRejectsOrphanItem(t *testing.T) {
	_, err := NormalizeOrderRows([][]string{{"订单编号", "销售属性", "宝贝ID"}, {"", "颜色分类：壳", "123"}})
	if err == nil || !strings.Contains(err.Error(), "第2行") {
		t.Fatalf("orphan item error = %v", err)
	}
}

func TestNormalizeOrderRowsModelOnly(t *testing.T) {
	rows, err := NormalizeOrderRows([][]string{{"订单编号", "销售属性", "宝贝ID"}, {"123", "适用手机型号：iPhone 16", "456"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := rows[1][FindColumn(rows[0], "商品规格")]; got != "iPhone 16|" {
		t.Fatalf("model-only spec = %q", got)
	}
}

func TestReadOrderRowsRejectsXLS(t *testing.T) {
	_, err := ReadOrderRows("订单.xls")
	if err == nil || !strings.Contains(err.Error(), "另存为 .xlsx") {
		t.Fatalf("xls error = %v", err)
	}
}

// TestReadOrderRowsXLSX 通过临时 xlsx 验证读取→标准化全链路：
// 别名列、销售属性拆分、续行继承、多件标记。
func TestReadOrderRowsXLSX(t *testing.T) {
	path := filepath.Join(t.TempDir(), "订单.xlsx")
	f := excelize.NewFile()
	sheet := f.GetSheetList()[0]
	headers := []string{"序号", "店铺", "联系人", "订单编号", "销售属性", "数量", "宝贝ID"}
	for c, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(c+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	data := [][]interface{}{
		{"1", "shop", "张三", "1234567890123456789", "适用手机型号：iPhone 16;颜色分类：红色+支架[新款]", 1, "991729207193"},
		{"", "", "", "", "颜色分类：单独支架", 2, "857197208121"},
		{"2", "shop", "李四", "222", "颜色分类：纯壳", 1, "3"},
	}
	for r, row := range data {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue(sheet, cell, v)
		}
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	rows, err := ReadOrderRows(path)
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		row          int
		header, want string
	}{
		{1, "商品规格", "iPhone 16|红色+支架[新款]"},
		{2, "商品规格", "单独支架"},
		{2, "订单编号", "1234567890123456789"},
		{2, "商品id", "857197208121"},
		{1, "订单组多件", "1"}, {2, "订单组多件", "1"}, {3, "订单组多件", ""},
	}
	for _, tc := range checks {
		col := FindColumn(rows[0], tc.header)
		if col < 0 {
			t.Fatalf("缺少列 %s", tc.header)
		}
		if got := rows[tc.row][col]; got != tc.want {
			t.Errorf("row %d %s = %q, want %q", tc.row, tc.header, got, tc.want)
		}
	}
}

// TestReadOrderRowsRealSample 用真实导出样本验证大整数文本不被改写；
// 样本不在仓库（.gitignore），缺失时跳过。
func TestReadOrderRowsRealSample(t *testing.T) {
	path := filepath.Join("..", "..", "build", "data", "导出文件.xlsx")
	rows, err := ReadOrderRows(path)
	if err != nil {
		t.Skipf("样本不可用: %v", err)
	}
	if len(rows) != 63 {
		t.Fatalf("rows = %d, want 63", len(rows))
	}
	if got := rows[1][FindColumn(rows[0], "订单编号")]; got != "5127302883641014316" {
		t.Fatalf("订单编号 = %q", got)
	}
	if got := rows[1][FindColumn(rows[0], "商品规格")]; got != "荣耀500 Pro|甜豆灰粉[升级版液态硅胶]" {
		t.Fatalf("商品规格 = %q", got)
	}
}
