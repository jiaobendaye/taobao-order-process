package common

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// NormalizeOrderRows 保留原始列并在末尾补充标准列，供各处理模块按标准列名读值。
// 原始数据不做修改；导出表布局下的空订单号续行继承上一条订单信息，
// 并用「订单组多件」列标记同组多件订单。
func NormalizeOrderRows(rows [][]string) ([][]string, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("订单数据行不足")
	}
	if err := ValidateOrderIDs(rows); err != nil {
		return nil, err
	}
	headers := append([]string(nil), rows[0]...)
	// 导出表布局：有「销售属性」且无「商品规格」列；此布局才做续行继承与多件标记
	export := FindColumn(headers, "销售属性") >= 0 && FindColumn(headers, "商品规格") < 0
	aliases := []struct {
		canonical string
		names     []string
	}{
		{"店铺名称", []string{"店铺"}},
		{"订单编号", []string{"订单号"}},
		{"收件人姓名", []string{"联系人"}},
		{"收件人手机号", []string{"联系电话"}},
		{"收件人详细地址", []string{"收货人详细地址"}},
		{"商品规格", []string{"销售属性"}},
		{"商品数量", []string{"数量"}},
		{"商品id", []string{"宝贝ID", "商品ID"}},
	}
	type mapping struct {
		src, dst int
		spec     bool
	}
	var mapped []mapping
	for _, alias := range aliases {
		if FindColumn(headers, alias.canonical) >= 0 {
			continue
		}
		for _, name := range alias.names {
			if src := FindColumn(rows[0], name); src >= 0 {
				mapped = append(mapped, mapping{src, len(headers), name == "销售属性"})
				headers = append(headers, alias.canonical)
				break
			}
		}
	}
	marker := -1
	if export {
		marker = len(headers)
		headers = append(headers, "订单组多件")
	}
	result := make([][]string, len(rows))
	result[0] = headers
	for i, row := range rows[1:] {
		out := make([]string, len(headers))
		copy(out, row)
		for _, m := range mapped {
			if m.src >= len(row) {
				continue
			}
			out[m.dst] = row[m.src]
			if m.spec {
				out[m.dst] = parseSalesAttributes(row[m.src])
			}
		}
		result[i+1] = out
	}
	if !export {
		return result, nil
	}

	orderCol := FindColumn(headers, "订单编号")
	if orderCol < 0 {
		return nil, fmt.Errorf("未找到「订单编号」列")
	}
	productCol := FindColumn(headers, "商品id")
	specCol := FindColumn(headers, "商品规格")
	if productCol < 0 || specCol < 0 {
		return nil, fmt.Errorf("订单缺少「宝贝ID」或「销售属性」列")
	}
	inherit := []string{"店铺名称", "订单编号", "收件人姓名", "收件人手机号", "收件人详细地址", "买家留言", "卖家备注"}
	var group []int
	flush := func() {
		if len(group) > 1 {
			for _, index := range group {
				result[index][marker] = "1"
			}
		}
		group = nil
	}
	for i := 1; i < len(result); i++ {
		row := result[i]
		if strings.TrimSpace(row[orderCol]) != "" {
			flush()
			group = append(group, i)
			continue
		}
		if strings.TrimSpace(row[productCol]) == "" && strings.TrimSpace(row[specCol]) == "" {
			flush()
			continue
		}
		if len(group) == 0 {
			return nil, fmt.Errorf("第%d行商品没有订单编号，请修正后重试", i+1)
		}
		if hasOrderIdentity(row, headers) {
			return nil, fmt.Errorf("第%d行商品没有订单编号，请修正后重试", i+1)
		}
		for _, name := range inherit {
			col := FindColumn(headers, name)
			if col >= 0 && row[col] == "" {
				row[col] = result[group[0]][col]
			}
		}
		group = append(group, i)
	}
	flush()
	return result, nil
}

func hasOrderIdentity(row, headers []string) bool {
	// 订单级字段非空说明这是一笔新订单但漏填了订单编号，直接报错而不是误并入上一组。
	for _, name := range []string{"店铺", "店铺名称", "联系人", "收件人姓名", "联系电话", "收件人手机号", "收货人详细地址", "收件人详细地址"} {
		if col := FindColumn(headers, name); col >= 0 && strings.TrimSpace(row[col]) != "" {
			return true
		}
	}
	return false
}

func parseSalesAttributes(s string) string {
	// 解析「适用手机型号：…;颜色分类：…」为「型号|SKU」；
	// 无法识别的非空属性原样保留，避免无声丢失信息。
	var model, sku string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == '；' }) {
		part = strings.TrimSpace(part)
		sep := strings.IndexAny(part, ":：")
		if sep < 0 {
			continue
		}
		_, width := utf8.DecodeRuneInString(part[sep:])
		key, value := strings.TrimSpace(part[:sep]), strings.TrimSpace(part[sep+width:])
		switch key {
		case "适用手机型号":
			model = value
		case "颜色分类":
			sku = value
		}
	}
	if sku == "" && model != "" {
		return model + "|"
	}
	if sku == "" {
		return s
	}
	if model == "" {
		return sku
	}
	return model + "|" + sku
}

// ReadOrderRows 读取订单工作簿的第一个 Sheet 并标准化。
// 只用于订单文件；配置工作簿由各模块自己的加载器处理。
// 旧版二进制 .xls 由第三方纯 Go 库读取时存在共享字符串表解析缺陷
// （WPS 导出的文件会丢失部分字符串），因此统一要求先另存为 .xlsx。
func ReadOrderRows(filename string) ([][]string, error) {
	if strings.EqualFold(filepath.Ext(filename), ".xls") {
		return nil, fmt.Errorf("订单文件为旧版 .xls 格式，请先用 WPS/Excel 另存为 .xlsx 再处理")
	}
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, fmt.Errorf("打开订单文件失败: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("订单文件没有工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("读取订单Sheet失败: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("订单数据行不足")
	}
	for _, name := range []string{"商品id", "宝贝ID", "订单编号", "订单号"} {
		col := FindColumn(rows[0], name)
		if col < 0 {
			continue
		}
		for i := 1; i < len(rows); i++ {
			cell, _ := excelize.CoordinatesToCellName(col+1, i+1)
			value := strings.TrimSpace(GetCellValueSafe(f, sheets[0], cell))
			if value != "" {
				for len(rows[i]) <= col {
					rows[i] = append(rows[i], "")
				}
				rows[i][col] = value
			}
		}
	}
	return NormalizeOrderRows(rows)
}
