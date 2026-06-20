// Package peijian 手机壳配件提取与档口分配
//
// 根据配件编码 Excel 配置文件，从订单规格中提取配件名称和数量，
// 并按自设编码将配件分配到对应的档口。
//
// 配置文件格式：
//
//	Sheet 1（支架-自设编码）：商品ID | SKU名称 | 编码1 | ... | 编码5
//	Sheet 2（档口分配）：列式布局，每列一个档口，下方为该档口的自设编码
package peijian

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"

	"taobao/internal/common"
)

// ---- 类型定义 ----

// Engine 配件提取引擎
type Engine struct {
	mapping    map[string][]string // "商品ID|SKU名称" → []自设编码 (按编码1~N的顺序)
	stalls     map[string]string   // 自设编码 → 档口名
	stallOrder []string            // 档口名列表（按 Sheet 2 列顺序）
}

// Result 配件提取分配结果
type Result struct {
	StallOrders map[string][]accessoryRow `json:"-"`       // 档口名 → 配件行列表
	NoMatch     [][]string                `json:"-"`       // 无匹配自设编码的行（原始数据行）
	Unassigned  [][]string                `json:"-"`       // 有自设编码但无档口匹配的行
	Summary     map[string]int            `json:"summary"` // 档口名 → 配件数量
	OutputDir   string                    `json:"outputDir"`
	OutputPath  string                    `json:"outputPath"`
	Total       int                       `json:"total"`
}

// accessoryRow 一条配件分配记录
type accessoryRow struct {
	Row          []string // 原始订单行
	ProductID    string   // 商品ID
	Accessory    string   // 配件名称
	ZisheBianma  string   // 自设编码
	Stall        string   // 档口名
}

// ---- 引擎加载 ----

// LoadEngine 从配件编码 Excel 文件加载匹配引擎
func LoadEngine(configPath string) (*Engine, error) {
	f, err := excelize.OpenFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("打开配件编码文件失败: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) < 2 {
		return nil, fmt.Errorf("配件编码文件至少需要2个Sheet（自设编码 + 档口分配）")
	}

	engine := &Engine{}

	// Sheet 1: 自设编码映射表（多编码列）
	mapping, err := loadPeijianMapping(f, sheets[0])
	if err != nil {
		return nil, fmt.Errorf("读取自设编码映射失败: %w", err)
	}
	engine.mapping = mapping

	// Sheet 2+: 档口分配
	stalls, stallOrder, err := loadStallMapping(f, sheets[1:])
	if err != nil {
		return nil, fmt.Errorf("读取档口分配失败: %w", err)
	}
	engine.stalls = stalls
	engine.stallOrder = stallOrder

	return engine, nil
}

// loadPeijianMapping 从 Sheet 1 读取 (商品ID, SKU名称) → []自设编码 映射
// 表头：商品ID | SKU名称 | 编码1 | 编码2 | 编码3 | 编码4 | 编码5
func loadPeijianMapping(f *excelize.File, sheetName string) (map[string][]string, error) {
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("自设编码Sheet数据行不足")
	}

	headers := rows[0]
	colProductID := common.FindColumn(headers, "商品ID")
	colSKU := common.FindColumn(headers, "SKU名称")
	if colProductID < 0 {
		return nil, fmt.Errorf("未找到「商品ID」列")
	}
	if colSKU < 0 {
		return nil, fmt.Errorf("未找到「SKU名称」列")
	}

	// 找所有编码列 (编码1 ~ 编码N)
	type codeCol struct {
		name string
		idx  int
	}
	var codeCols []codeCol
	for i, h := range headers {
		h = strings.TrimSpace(h)
		if strings.HasPrefix(h, "编码") {
			codeCols = append(codeCols, codeCol{name: h, idx: i})
		}
	}
	if len(codeCols) == 0 {
		return nil, fmt.Errorf("未找到任何「编码N」列")
	}

	mapping := make(map[string]string, len(rows)-1)
	mappingCodes := make(map[string][]string, len(rows)-1)

	for i := 1; i < len(rows); i++ {
		row := rows[i]

		// 商品ID 使用 GetCellValue 读取以避免大数字精度问题
		productIDCell, _ := excelize.CoordinatesToCellName(colProductID+1, i+1)
		productID := strings.TrimSpace(common.GetCellValueSafe(f, sheetName, productIDCell))
		if productID == "" && colProductID < len(row) {
			productID = strings.TrimSpace(row[colProductID])
		}

		skuName := ""
		if colSKU < len(row) {
			skuName = strings.TrimSpace(row[colSKU])
		}

		if productID == "" || skuName == "" {
			continue
		}

		// 读取所有编码列的值
		var codes []string
		for _, cc := range codeCols {
			if cc.idx < len(row) {
				code := strings.TrimSpace(row[cc.idx])
				if code != "" {
					codes = append(codes, code)
				}
			}
		}

		key := strings.ToLower(productID + "|" + skuName)
		mappingCodes[key] = codes

		// 同时保留原始 key（用于详情输出）
		mapping[key] = skuName
	}

	if len(mappingCodes) == 0 {
		return nil, fmt.Errorf("自设编码映射表为空")
	}

	return mappingCodes, nil
}

// loadStallMapping 从 Sheet 2+ 读取编码→档口映射（列式布局）
// 格式同 dangkou 的档口 Sheet：Row 0 为档口名，Row 1+ 为该档口下的自设编码
func loadStallMapping(f *excelize.File, sheetNames []string) (map[string]string, []string, error) {
	stalls := make(map[string]string)
	var stallOrder []string

	for _, sheetName := range sheetNames {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, nil, fmt.Errorf("读取Sheet「%s」失败: %w", sheetName, err)
		}
		if len(rows) < 2 {
			continue
		}

		headerRow := rows[0]
		for col, stallName := range headerRow {
			stallName = strings.TrimSpace(stallName)
			if stallName == "" {
				continue
			}

			stallOrder = append(stallOrder, stallName)

			// 读取该列下的自设编码
			for j := 1; j < len(rows); j++ {
				if col >= len(rows[j]) {
					continue
				}
				code := strings.TrimSpace(rows[j][col])
				if code == "" {
					continue
				}
				code = strings.ToLower(common.StripInvisible(code))
				stalls[code] = stallName
			}
		}
	}

	return stalls, stallOrder, nil
}

// ---- 配件提取 ----

// extractAccessories 从 SKU 名称中提取配件列表。
// 有 +：+ 前为手机壳（忽略），+ 后的每段为配件
// 无 +：SKU 本身为配件
func extractAccessories(skuName string) []string {
	segments := strings.Split(skuName, "+")
	if len(segments) < 2 {
		// 无 +，SKU 本身就是配件
		name := strings.TrimSpace(skuName)
		if name == "" {
			return nil
		}
		return []string{name}
	}

	var accessories []string
	for i := 1; i < len(segments); i++ { // 跳过第一个（手机壳）
		seg := strings.TrimSpace(segments[i])
		if seg != "" {
			accessories = append(accessories, seg)
		}
	}
	return accessories
}

// ---- 主处理函数 ----

// Process 读取订单 Excel 并按配件编码配置提取配件、分配档口。
// configPath 为配件编码.xlsx 的路径；若为空则尝试从 peijian_config.json 读取。
func Process(filename, configPath string) (*Result, error) {
	// 加载配置路径
	if configPath == "" {
		path := common.LoadConfigPath("peijian_config.json")
		if path == "" {
			return nil, fmt.Errorf("未指定配件编码文件路径，且无法加载已保存的路径\n请在GUI中先选择配件编码文件，或通过命令行传入: phonecase-tools peijian extract <订单文件> <配件编码.xlsx>")
		}
		configPath = path
	}

	// 加载引擎
	engine, err := LoadEngine(configPath)
	if err != nil {
		return nil, err
	}

	// 打开订单文件
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, fmt.Errorf("打开订单文件失败: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetList()[0])
	if err != nil {
		return nil, fmt.Errorf("读取订单Sheet失败: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("订单数据行不足")
	}

	headers := rows[0]

	// 找列
	colProductID := common.FindColumn(headers, "商品id")
	colSpec := common.FindColumn(headers, "商品规格")
	colQty := common.FindColumn(headers, "商品数量")
	if colProductID < 0 {
		return nil, fmt.Errorf("未找到「商品id」列")
	}
	if colSpec < 0 {
		return nil, fmt.Errorf("未找到「商品规格」列")
	}

	sheetName := f.GetSheetList()[0]
	result := &Result{
		StallOrders: make(map[string][]accessoryRow),
		Summary:     make(map[string]int),
		Total:       len(rows) - 1,
	}

	// 逐行处理
	for i := 1; i < len(rows); i++ {
		row := rows[i]

		// 读取商品ID
		productIDCell, _ := excelize.CoordinatesToCellName(colProductID+1, i+1)
		productID := strings.TrimSpace(common.GetCellValueSafe(f, sheetName, productIDCell))
		if productID == "" && colProductID < len(row) {
			productID = strings.TrimSpace(row[colProductID])
		}

		// 读取商品规格并解析 SKU
		spec := ""
		if colSpec < len(row) {
			spec = strings.TrimSpace(row[colSpec])
		}
		_, skuName := common.ParseSpec(spec)

		// 查找自设编码
		key := strings.ToLower(productID + "|" + skuName)
		codes, ok := engine.mapping[key]
		if !ok {
			result.NoMatch = append(result.NoMatch, row)
			continue
		}

		// 提取配件
		accessories := extractAccessories(skuName)
		if len(accessories) == 0 {
			result.NoMatch = append(result.NoMatch, row)
			continue
		}

		// 校验
		if len(accessories) != len(codes) {
			// 不匹配：归入 Unassigned
			for _, acc := range accessories {
				result.Unassigned = append(result.Unassigned, row)
				_ = acc
			}
			continue
		}

		// 分配配件到档口
		qty := 1
		if colQty >= 0 && colQty < len(row) {
			qtyStr := strings.TrimSpace(row[colQty])
			if q, err := parseInt(qtyStr); err == nil && q > 0 {
				qty = q
			}
		}

		for j, acc := range accessories {
			code := codes[j]
			stall := engine.stalls[strings.ToLower(code)]
			if stall == "" {
				result.Unassigned = append(result.Unassigned, row)
				continue
			}

			result.StallOrders[stall] = append(result.StallOrders[stall], accessoryRow{
				Row:         row,
				ProductID:   productID,
				Accessory:   acc,
				ZisheBianma: code,
				Stall:       stall,
			})
			// 累计该档口的配件总数
			result.Summary[stall] += qty
		}
	}

	// 确保所有档口都在 summary 中（即使为 0）
	for _, stall := range engine.stallOrder {
		if _, ok := result.Summary[stall]; !ok {
			result.Summary[stall] = 0
		}
	}
	result.Summary["无匹配自设编码"] = len(result.NoMatch)
	result.Summary["未分配档口"] = len(result.Unassigned)

	// 输出
	absPath, _ := filepath.Abs(filename)
	excelDir := filepath.Dir(absPath)
	excelName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	outputDir := filepath.Join(excelDir, excelName+"_output")
	os.MkdirAll(outputDir, 0755)
	result.OutputDir = outputDir

	result.OutputPath = filepath.Join(outputDir, "配件分配.xlsx")
	if err := writeOutput(result.OutputPath, headers, engine, result); err != nil {
		return nil, fmt.Errorf("生成输出文件失败: %w", err)
	}

	return result, nil
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not an int: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// ---- 输出 ----

func writeOutput(outputPath string, headers []string, engine *Engine, result *Result) error {
	f := excelize.NewFile()
	defer f.Close()

	// 输出列：店铺名称 | 订单编号 | 商品ID | 商品规格 | 配件名称 | 商品数量
	outputHeaders := buildOutputHeaders(headers)

	type sheetData struct {
		name string
		rows [][]string
	}
	var detailSheets []sheetData

	// 按档口顺序收集数据
	for _, stallName := range engine.stallOrder {
		orders := result.StallOrders[stallName]
		if len(orders) == 0 {
			continue
		}
		rows := buildAccessoryRows(outputHeaders, headers, orders)
		detailSheets = append(detailSheets, sheetData{name: stallName, rows: rows})
	}

	// 未分配档口
	if len(result.Unassigned) > 0 {
		detailSheets = append(detailSheets, sheetData{name: "未分配档口", rows: result.Unassigned})
	}

	// 无匹配自设编码
	if len(result.NoMatch) > 0 {
		detailSheets = append(detailSheets, sheetData{name: "无匹配自设编码", rows: result.NoMatch})
	}

	// 汇总 sheet 放第一位
	f.SetSheetName("Sheet1", "汇总")
	writeSummarySheet(f, "汇总", engine.stallOrder, result)

	// 各明细 sheet
	for _, sd := range detailSheets {
		if _, err := f.NewSheet(sd.name); err != nil {
			return err
		}
		if sd.name == "未分配档口" || sd.name == "无匹配自设编码" {
			writeFullSheet(f, sd.name, headers, sd.rows)
		} else {
			writeSheet(f, sd.name, outputHeaders, sd.rows)
		}
	}

	f.SetActiveSheet(0)
	return f.SaveAs(outputPath)
}

// buildOutputHeaders 构建明细 sheet 的表头
func buildOutputHeaders(headers []string) []string {
	// 从原始表头中提取需要的列，追加 配件名称
	out := []string{}
	for _, h := range []string{"店铺名称", "订单编号", "商品id", "商品规格", "商品数量"} {
		if common.FindColumn(headers, h) >= 0 {
			out = append(out, h)
		}
	}
	out = append(out, "配件名称")
	return out
}

// buildAccessoryRows 将配件分配记录转为 sheet 行
func buildAccessoryRows(outputHeaders []string, origHeaders []string, orders []accessoryRow) [][]string {
	rows := make([][]string, len(orders))
	for i, o := range orders {
		row := make([]string, len(outputHeaders))
		for j, h := range outputHeaders {
			switch h {
			case "配件名称":
				row[j] = o.Accessory
			default:
				col := common.FindColumn(origHeaders, h)
				if col >= 0 && col < len(o.Row) {
					row[j] = o.Row[col]
				}
			}
		}
		rows[i] = row
	}
	return rows
}

// writeSummarySheet 写汇总 sheet：第一行为档口名，下方为「配件名称 x数量」
func writeSummarySheet(f *excelize.File, sheetName string, stallOrder []string, result *Result) {
	// 聚合每个档口的配件
	type partAgg struct {
		name string
		qty  int
	}

	for colIdx, stallName := range stallOrder {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(sheetName, cell, stallName)

		orders := result.StallOrders[stallName]
		if len(orders) == 0 {
			continue
		}

		// 聚合配件名称 + 数量
		agg := make(map[string]int)
		for _, o := range orders {
			qty := 1
			// 从原始行找数量
			colQty := common.FindColumn(o.Row, "商品数量")
			if colQty >= 0 && colQty < len(o.Row) {
				if q, err := parseInt(strings.TrimSpace(o.Row[colQty])); err == nil && q > 0 {
					qty = q
				}
			}
			agg[o.Accessory] += qty
		}

		// 按数量降序排列
		parts := make([]partAgg, 0, len(agg))
		for name, qty := range agg {
			parts = append(parts, partAgg{name, qty})
		}
		sort.Slice(parts, func(i, j int) bool {
			return parts[i].qty > parts[j].qty
		})

		for rowIdx, p := range parts {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheetName, cell, fmt.Sprintf("%s x%d", p.name, p.qty))
		}
	}
}

// writeSheet 写数据 sheet（含表头）
func writeSheet(f *excelize.File, sheetName string, headers []string, rows [][]string) {
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheetName, cell, val)
		}
	}
}

// writeFullSheet 写原始数据 sheet（使用原始表头 + 原始行）
func writeFullSheet(f *excelize.File, sheetName string, headers []string, rows [][]string) {
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheetName, cell, val)
		}
	}
}

// ---- 配置路径持久化 ----

const peijianConfigName = "peijian_config.json"

// SaveConfigPath 保存配件编码文件路径到 peijian_config.json
func SaveConfigPath(path string) error {
	return common.SaveConfigPath(peijianConfigName, path)
}

// LoadConfigPath 从 peijian_config.json 加载配件编码文件路径
func LoadConfigPath() string {
	return common.LoadConfigPath(peijianConfigName)
}
