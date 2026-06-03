// Package peijian 手机壳配件提取与汇总
//
// 从订单商品规格中提取配件信息（支架、吸盘、串珠等），支持提取和汇总两种模式
package peijian

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ---- 数据结构 ----

// Order 一条订单
type Order struct {
	RawRow     []string
	Spec       string
	BuyerMsg   string
	SellerNote string
	Quantity   int
}

// PartDetail 配件明细
type PartDetail struct {
	Name     string `json:"name"`
	Color    string `json:"color"`
	Quantity int    `json:"quantity"`
}

// PartsConfig 配件配置
type PartsConfig struct {
	Accessories []string `json:"accessories"`
}

// ColumnConfig 列名配置
type ColumnConfig struct {
	Spec       string `json:"spec"`
	BuyerMsg   string `json:"buyerMsg"`
	SellerNote string `json:"sellerNote"`
	Quantity   string `json:"quantity"`
}

// ExtractResult 提取结果
type ExtractResult struct {
	Orders       []Order `json:"-"`
	SimpleOrders []Order `json:"-"`
	PendingOrders []Order `json:"-"`
	NoPartsOrders []Order `json:"-"`
	OutputDir    string  `json:"outputDir"`
	Summary      ExtractSummary `json:"summary"`
}

// ExtractSummary 提取统计
type ExtractSummary struct {
	Total       int `json:"total"`
	Simple      int `json:"simple"`
	Pending     int `json:"pending"`
	NoParts     int `json:"noParts"`
}

// MergeResult 汇总结果
type MergeResult struct {
	Entries    []MergeEntry `json:"entries"`
	TotalKinds int          `json:"totalKinds"`
	TotalQty   int          `json:"totalQty"`
	OutputPath string       `json:"outputPath"`
}

// MergeEntry 汇总条目
type MergeEntry struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Qty   int    `json:"qty"`
}

// ---- 默认配件 ----

var accKeywords = []string{"支架", "吸盘", "串珠", "贴纸", "腕带", "绳", "链", "镜"}
var falseAccessoryPatterns = []string{"镜头", "不含支架"}

func isFalseAccessory(s string) bool {
	for _, p := range falseAccessoryPatterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// ---- 配置加载 ----

// DefaultPartsConfig 返回默认配件配置
func DefaultPartsConfig() *PartsConfig {
	return &PartsConfig{
		Accessories: []string{
			"旋转折叠支架", "泡泡软胶支架", "推拉支架", "糯米糍支架",
			"鲷鱼烧支架", "方形笑脸支架", "折叠支架", "苹果支架",
			"柠檬支架", "圆形支架", "双层支架", "软胶支架",
			"滴胶支架", "泡泡支架", "支架", "软胶吸盘", "吸盘",
			"撞色串珠", "串珠", "果冻贴纸", "贴纸", "编织手提绳",
			"手提绳", "腕带", "爱心镜", "镜",
		},
	}
}

// DefaultColumnConfig 返回默认列名配置
func DefaultColumnConfig() *ColumnConfig {
	return &ColumnConfig{
		Spec:       "商品规格",
		BuyerMsg:   "买家留言",
		SellerNote: "卖家备注",
		Quantity:   "商品数量",
	}
}

func loadJSONConfig(name string, v interface{}) {
	for _, p := range configSearchPaths(name) {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		json.Unmarshal(data, v)
		return
	}
}

func saveJSONConfig(name string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(name), data, 0644)
}

// ConfigPath 返回配置文件完整路径（可执行文件同目录）
func ConfigPath(name string) string {
	execPath, _ := os.Executable()
	return filepath.Join(filepath.Dir(execPath), name)
}

func configSearchPaths(name string) []string {
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	return []string{
		filepath.Join(execDir, name),
		name,
	}
}

// LoadConfigs 加载配件和列名配置
func LoadConfigs() (*PartsConfig, *ColumnConfig) {
	cfg := DefaultPartsConfig()
	loadJSONConfig("parts.json", cfg)

	colCfg := DefaultColumnConfig()
	loadJSONConfig("columns.json", colCfg)

	return cfg, colCfg
}

// SavePartsConfig 保存配件配置
func SavePartsConfig(cfg *PartsConfig) error {
	return saveJSONConfig("parts.json", cfg)
}

// SaveColumnConfig 保存列名配置
func SaveColumnConfig(cfg *ColumnConfig) error {
	return saveJSONConfig("columns.json", cfg)
}

// ---- Excel 读取 ----

func findCol(headers []string, name string) int {
	for i, h := range headers {
		if strings.TrimSpace(h) == name {
			return i
		}
	}
	return -1
}

func getCol(row []string, col int) string {
	if col >= 0 && col < len(row) {
		return strings.TrimSpace(row[col])
	}
	return ""
}

func readExcel(filename string, colCfg *ColumnConfig) (headers []string, orders []Order, err error) {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("打开Excel失败: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetList()[0])
	if err != nil {
		return nil, nil, fmt.Errorf("读取sheet失败: %w", err)
	}
	if len(rows) < 2 {
		return nil, nil, fmt.Errorf("数据行不足")
	}

	headers = rows[0]

	specCol := findCol(headers, colCfg.Spec)
	buyerCol := findCol(headers, colCfg.BuyerMsg)
	sellerCol := findCol(headers, colCfg.SellerNote)
	qtyCol := findCol(headers, colCfg.Quantity)

	if specCol < 0 {
		return nil, nil, fmt.Errorf("未找到列 '%s'，请检查 columns.json", colCfg.Spec)
	}

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		qty := 1
		if qtyCol >= 0 {
			if q, err := strconv.Atoi(getCol(row, qtyCol)); err == nil && q > 0 {
				qty = q
			}
		}
		orders = append(orders, Order{
			RawRow:     row,
			Spec:       getCol(row, specCol),
			BuyerMsg:   getCol(row, buyerCol),
			SellerNote: getCol(row, sellerCol),
			Quantity:   qty,
		})
	}
	return headers, orders, nil
}

// ---- 分类 ----

func specAfterPipe(spec string) string {
	idx := strings.LastIndex(spec, "|")
	if idx >= 0 {
		return spec[idx+1:]
	}
	return spec
}

func isSimpleOrder(o Order) bool {
	if o.BuyerMsg != "" || o.SellerNote != "" {
		return false
	}
	if strings.Contains(o.Spec, "备注") || strings.Contains(o.Spec, "咨询") {
		return false
	}
	return strings.Contains(specAfterPipe(o.Spec), "+")
}

func hasPotentialAccessory(o Order) bool {
	right := specAfterPipe(o.Spec)
	if strings.Contains(right, "+") {
		return true
	}
	if strings.Contains(right, "不含壳") {
		return true
	}
	if strings.Contains(right, "单独") {
		for _, kw := range accKeywords {
			if strings.Contains(right, kw) && !isFalseAccessory(right) {
				return true
			}
		}
	}
	if idx1 := strings.Index(right, "["); idx1 >= 0 {
		if idx2 := strings.Index(right[idx1:], "]"); idx2 >= 0 {
			inside := right[idx1+1 : idx1+idx2]
			if !isFalseAccessory(inside) {
				for _, kw := range accKeywords {
					if strings.Contains(inside, kw) {
						return true
					}
				}
			}
		}
	}
	if strings.Contains(o.Spec, "DIY") || strings.Contains(o.Spec, "搭配") {
		return true
	}
	return false
}

func classifyOrders(orders []Order) (simple, pending, noParts []Order) {
	for _, o := range orders {
		if isSimpleOrder(o) {
			simple = append(simple, o)
		} else if hasPotentialAccessory(o) {
			pending = append(pending, o)
		} else {
			noParts = append(noParts, o)
		}
	}
	return
}

// ---- 配件提取 ----

func normalizeSegment(seg string) string {
	seg = strings.TrimSpace(seg)
	for {
		start := strings.LastIndex(seg, "[")
		end := strings.LastIndex(seg, "]")
		if start >= 0 && end > start && end == len(seg)-1 {
			seg = strings.TrimSpace(seg[:start])
		} else {
			break
		}
	}
	for {
		start := strings.LastIndex(seg, "【")
		end := strings.LastIndex(seg, "】")
		if start >= 0 && end > start && end == len(seg)-1 {
			seg = strings.TrimSpace(seg[:start])
		} else {
			break
		}
	}
	return seg
}

func matchPart(seg string, cfg *PartsConfig) (name, color string, matched bool) {
	seg = normalizeSegment(seg)
	if seg == "" || isFalseAccessory(seg) {
		return "", "", false
	}
	for _, partName := range cfg.Accessories {
		if strings.HasSuffix(seg, partName) {
			color := strings.TrimSpace(strings.TrimSuffix(seg, partName))
			if color == "" {
				color = "无"
			}
			return partName, color, true
		}
	}
	return "", "", false
}

// ExtractParts 从订单中提取配件
func ExtractParts(o Order, cfg *PartsConfig) []PartDetail {
	right := specAfterPipe(o.Spec)
	segments := strings.Split(right, "+")
	if len(segments) < 2 {
		return nil
	}

	var parts []PartDetail
	for i := 1; i < len(segments); i++ {
		seg := strings.TrimSpace(segments[i])
		if seg == "" {
			continue
		}
		name, color, matched := matchPart(seg, cfg)
		if matched {
			qty := o.Quantity
			if qty <= 0 {
				qty = 1
			}
			parts = append(parts, PartDetail{Name: name, Color: color, Quantity: qty})
		}
	}
	return parts
}

// ---- 主处理函数 ----

// Extract 从 Excel 提取配件信息
func Extract(filename string) (*ExtractResult, error) {
	return ExtractWithConfig(filename, nil, nil)
}

// ExtractWithConfig 使用指定配置提取
func ExtractWithConfig(filename string, partsCfg *PartsConfig, colCfg *ColumnConfig) (*ExtractResult, error) {
	if partsCfg == nil || colCfg == nil {
		p, c := LoadConfigs()
		if partsCfg == nil {
			partsCfg = p
		}
		if colCfg == nil {
			colCfg = c
		}
	}

	headers, orders, err := readExcel(filename, colCfg)
	if err != nil {
		return nil, err
	}

	simpleOrders, pendingOrders, noPartsOrders := classifyOrders(orders)

	absPath, _ := filepath.Abs(filename)
	excelDir := filepath.Dir(absPath)
	excelName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	outputDir := filepath.Join(excelDir, excelName+"_output")
	os.MkdirAll(outputDir, 0755)

	result := &ExtractResult{
		Orders:        orders,
		SimpleOrders:  simpleOrders,
		PendingOrders: pendingOrders,
		NoPartsOrders: noPartsOrders,
		OutputDir:     outputDir,
		Summary: ExtractSummary{
			Total:   len(orders),
			Simple:  len(simpleOrders),
			Pending: len(pendingOrders),
			NoParts: len(noPartsOrders),
		},
	}

	pendingPath := filepath.Join(outputDir, "pending.xlsx")
	if err := writePendingXlsx(pendingPath, headers, simpleOrders, pendingOrders, noPartsOrders, partsCfg); err != nil {
		return nil, fmt.Errorf("生成 pending.xlsx 失败: %w", err)
	}

	return result, nil
}

// Merge 汇总 pending.xlsx 中的配件统计
func Merge(pendingPath string) (*MergeResult, error) {
	f, err := excelize.OpenFile(pendingPath)
	if err != nil {
		return nil, fmt.Errorf("打开 pending.xlsx 失败: %w", err)
	}
	defer f.Close()

	agg := make(map[string]int)

	for _, sheetName := range f.GetSheetList() {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			continue
		}
		if len(rows) < 2 {
			continue
		}

		headers := rows[0]
		nameCol, colorCol, qtyCol := -1, -1, -1
		for i, h := range headers {
			switch h {
			case "配件名称":
				nameCol = i
			case "颜色":
				colorCol = i
			case "配件数量":
				qtyCol = i
			}
		}
		if nameCol < 0 || colorCol < 0 || qtyCol < 0 {
			continue
		}

		for i := 1; i < len(rows); i++ {
			row := rows[i]
			if nameCol >= len(row) {
				continue
			}
			name := strings.TrimSpace(row[nameCol])
			if name == "" {
				continue
			}
			color := ""
			if colorCol < len(row) {
				color = strings.TrimSpace(row[colorCol])
			}
			qty := 1
			if qtyCol < len(row) {
				if q, err := strconv.Atoi(strings.TrimSpace(row[qtyCol])); err == nil && q > 0 {
					qty = q
				}
			}
			key := name + "|" + color
			agg[key] += qty
		}
	}

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

	resultPath := filepath.Join(filepath.Dir(pendingPath), "result.xlsx")
	if err := writeResultXlsx(resultPath, entries); err != nil {
		return nil, fmt.Errorf("生成 result.xlsx 失败: %w", err)
	}

	totalQty := 0
	for _, e := range entries {
		totalQty += e.Qty
	}

	return &MergeResult{
		Entries:    entries,
		TotalKinds: len(entries),
		TotalQty:   totalQty,
		OutputPath: resultPath,
	}, nil
}

// ---- Excel 输出 ----

func writePendingXlsx(outputPath string, headers []string, simpleOrders, pendingOrders, noPartsOrders []Order, cfg *PartsConfig) error {
	f := excelize.NewFile()
	defer f.Close()

	numCols := len(headers)
	lastDataCol := colToLetters(numCols + 3)

	multiStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D6EAF8"}, Pattern: 1},
	})

	writeSheet := func(sheetName string, orders []Order, extract bool) {
		// 原始数据表头
		for colIdx, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
			f.SetCellValue(sheetName, cell, h)
		}
		// 配件列表头
		partNameCol := numCols + 1
		f.SetCellValue(sheetName, colToLetters(partNameCol)+"1", "配件名称")
		f.SetCellValue(sheetName, colToLetters(partNameCol+1)+"1", "颜色")
		f.SetCellValue(sheetName, colToLetters(partNameCol+2)+"1", "配件数量")

		rowIdx := 2
		for _, o := range orders {
			parts := []PartDetail{}
			if extract {
				parts = ExtractParts(o, cfg)
			}

			if len(parts) == 0 {
				writeOrderRow(f, sheetName, &rowIdx, o.RawRow, numCols, "", "", "")
			} else {
				startRow := rowIdx
				for _, p := range parts {
					writeOrderRow(f, sheetName, &rowIdx, o.RawRow, numCols,
						p.Name, p.Color, strconv.Itoa(p.Quantity))
				}
				if len(parts) > 1 {
					endRow := rowIdx - 1
					f.SetCellStyle(sheetName,
						fmt.Sprintf("A%d", startRow),
						fmt.Sprintf("%s%d", lastDataCol, endRow),
						multiStyle)
				}
			}
		}
	}

	simpleName := "简单订单"
	f.SetSheetName("Sheet1", simpleName)
	writeSheet(simpleName, simpleOrders, true)

	pendingName := "待处理订单"
	f.NewSheet(pendingName)
	writeSheet(pendingName, pendingOrders, false)

	noPartsName := "无配件订单"
	f.NewSheet(noPartsName)
	writeSheet(noPartsName, noPartsOrders, false)

	f.SetActiveSheet(0)
	return f.SaveAs(outputPath)
}

func writeOrderRow(f *excelize.File, sheetName string, rowIdx *int, rawRow []string, numCols int, partName, partColor, partQty string) {
	for colIdx, val := range rawRow {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, *rowIdx)
		f.SetCellValue(sheetName, cell, val)
	}
	f.SetCellValue(sheetName, colToLetters(numCols+1)+strconv.Itoa(*rowIdx), partName)
	f.SetCellValue(sheetName, colToLetters(numCols+2)+strconv.Itoa(*rowIdx), partColor)
	f.SetCellValue(sheetName, colToLetters(numCols+3)+strconv.Itoa(*rowIdx), partQty)
	*rowIdx++
}

func writeResultXlsx(outputPath string, entries []MergeEntry) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "配件汇总"
	f.SetSheetName("Sheet1", sheetName)

	f.SetCellValue(sheetName, "A1", "配件名称")
	f.SetCellValue(sheetName, "B1", "颜色")
	f.SetCellValue(sheetName, "C1", "数量")

	for i, e := range entries {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), e.Name)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), e.Color)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), e.Qty)
	}

	return f.SaveAs(outputPath)
}

func colToLetters(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}
