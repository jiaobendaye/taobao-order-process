// Package pizhi 皮质壳档口分配
//
// 根据皮质壳配置表将订单按 (商品ID, SKU名称, 手机型号) 聚合后分配到对应档口。
// 配置文件格式：每个 Sheet 一个档口（Sheet 名 = 档口名），
// 每行 3 列：(商品ID, SKU名称, 图片)。图片嵌入到 C 列单元格。
package pizhi

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"taobao/internal/common"
)

// ---- 类型定义 ----

// ConfigItem 单个 (商品ID, SKU) 配置项
type ConfigItem struct {
	Stall      string // 档口名（Sheet 名）
	ImageBytes []byte // 嵌入图片二进制
	ImageExt   string // 图片扩展名（png/jpg）
}

// Engine 档口匹配引擎
type Engine struct {
	Items  map[string]ConfigItem `json:"items"`  // "商品ID|SKU" → 配置项
	Stalls []string              `json:"stalls"` // 按 Sheet 顺序排列的档口名
}

// AggregateRow 档口内的聚合行
type AggregateRow struct {
	ProductID string // 商品ID
	SKU       string // SKU名称
	Model     string // 手机型号
	Quantity  int    // 数量合计
	ImageKey  string // 用于在 Engine.Items 中查找图片的 key
}

// Result 处理结果
type Result struct {
	StallAggregates map[string][]AggregateRow `json:"stallAggregates"` // 档口名 → 聚合行
	Unmatched       [][]string                `json:"unmatched"`       // 未匹配的订单行
	OutputPath      string                    `json:"outputPath"`
	Total           int                       `json:"total"`
}

// ---- 引擎加载 ----

// LoadEngine 从皮质壳配置 Excel 加载匹配引擎
func LoadEngine(configPath string) (*Engine, error) {
	f, err := excelize.OpenFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	engine := &Engine{
		Items: make(map[string]ConfigItem),
	}

	for _, sheetName := range sheets {
		// 跳过 WPS 内部保留 sheet
		if strings.HasPrefix(sheetName, "WpsReserved") {
			continue
		}

		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("读取Sheet「%s」失败: %w", sheetName, err)
		}
		if len(rows) < 2 {
			continue
		}

		headers := rows[0]
		colProductID := common.FindColumn(headers, "商品ID")
		colSKU := common.FindColumn(headers, "sku名称")
		if colProductID < 0 || colSKU < 0 {
			return nil, fmt.Errorf("Sheet「%s」缺少必要列（商品ID/sku名称）", sheetName)
		}

		// 提取该 sheet 的所有嵌入图片，按行号索引
		imgByRow := mapImagesByRow(f, sheetName)

		stallName := sheetName
		engine.Stalls = append(engine.Stalls, stallName)

		for i := 1; i < len(rows); i++ {
			row := rows[i]

			// 商品ID 用 GetCellValue 读避免科学计数法
			pidCell, _ := excelize.CoordinatesToCellName(colProductID+1, i+1)
			productID := strings.TrimSpace(common.GetCellValueSafe(f, sheetName, pidCell))
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

			key := strings.ToLower(productID + "|" + skuName)
			item := ConfigItem{Stall: stallName}

			// 提取该行对应的图片
			if img, ok := imgByRow[i+1]; ok { // excelize 行号是 1-based
				item.ImageBytes = img.Data
				item.ImageExt = img.Ext
			}

			engine.Items[key] = item
		}
	}

	if len(engine.Items) == 0 {
		return nil, fmt.Errorf("配置表为空，未找到任何 (商品ID, SKU) 项")
	}

	return engine, nil
}

// imageRef 提取的图片引用
type imageRef struct {
	Data []byte
	Ext  string
}

// mapImagesByRow 提取 sheet 中所有嵌入图片，按行号索引
func mapImagesByRow(f *excelize.File, sheetName string) map[int]imageRef {
	result := make(map[int]imageRef)

	// GetSheetDimension 返回类似 "A1:D33" 的范围，解析出最大行号
	dim, _ := f.GetSheetDimension(sheetName)
	maxRow := parseMaxRow(dim)
	if maxRow == 0 {
		maxRow = 200 // 兜底
	}

	for row := 1; row <= maxRow; row++ {
		// 尝试 C 列（第 3 列）及其他可能列
		for _, col := range []int{3, 4, 5, 6, 7, 8} {
			cell, _ := excelize.CoordinatesToCellName(col, row)
			pics, err := f.GetPictures(sheetName, cell)
			if err != nil || len(pics) == 0 {
				continue
			}
			pic := pics[0]
			result[row] = imageRef{Data: pic.File, Ext: pic.Extension}
			break // 每行只取一张
		}
	}

	return result
}

// parseMaxRow 从 "A1:D33" 这类范围字符串提取最大行号
func parseMaxRow(dim string) int {
	if dim == "" {
		return 0
	}
	parts := strings.Split(dim, ":")
	if len(parts) != 2 {
		return 0
	}
	// 第二段如 "D33" -> 33
	end := parts[1]
	rowStart := -1
	for i, r := range end {
		if r >= '0' && r <= '9' {
			rowStart = i
			break
		}
	}
	if rowStart < 0 {
		return 0
	}
	n, err := strconv.Atoi(end[rowStart:])
	if err != nil {
		return 0
	}
	return n
}

// ---- 主处理函数 ----

// Process 读取订单 Excel 并按皮质壳配置聚合分配
func Process(filename, configPath string) (*Result, error) {
	if configPath == "" {
		path := common.LoadConfigPath(configPathName)
		if path == "" {
			return nil, fmt.Errorf("未指定配置文件路径")
		}
		configPath = path
	}

	engine, err := LoadEngine(configPath)
	if err != nil {
		return nil, err
	}

	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, fmt.Errorf("打开订单文件失败: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetList()[0]
	allRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("读取订单Sheet失败: %w", err)
	}
	if len(allRows) < 2 {
		return nil, fmt.Errorf("订单数据行不足")
	}

	headers := allRows[0]
	// 用 GetCellValue 修正商品ID列
	colProductID := common.FindColumn(headers, "商品id")
	dataRows := allRows[1:]
	if colProductID >= 0 {
		for i := range dataRows {
			cell, _ := excelize.CoordinatesToCellName(colProductID+1, i+2)
			pid := strings.TrimSpace(common.GetCellValueSafe(f, sheetName, cell))
			if pid != "" && colProductID < len(dataRows[i]) {
				dataRows[i][colProductID] = pid
			}
		}
	}

	result := ProcessData(dataRows, headers, engine)

	// 输出文件
	absPath, _ := filepath.Abs(filename)
	excelDir := filepath.Dir(absPath)
	excelName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	outputDir := filepath.Join(excelDir, excelName+"_output")
	os.MkdirAll(outputDir, 0755)

	outputPath := filepath.Join(outputDir, "皮质壳分配.xlsx")
	if err := writeOutput(outputPath, engine, result); err != nil {
		return nil, fmt.Errorf("生成输出文件失败: %w", err)
	}
	result.OutputPath = outputPath

	return result, nil
}

// ProcessData 对已解析的订单数据执行聚合分配，不涉及文件 I/O。
// 聚合键：(商品ID, SKU名称, 手机型号) 三者都相同的订单合并数量。
func ProcessData(dataRows [][]string, headers []string, engine *Engine) *Result {
	colProductID := common.FindColumn(headers, "商品id")
	colSpec := common.FindColumn(headers, "商品规格")
	colQty := common.FindColumn(headers, "商品数量")

	result := &Result{
		StallAggregates: make(map[string][]AggregateRow),
		Total:           len(dataRows),
	}

	// 按 (商品ID, SKU, 型号) 聚合
	type aggKey struct {
		Stall    string
		ProductID string
		SKU      string
		Model    string
	}
	aggMap := make(map[aggKey]*AggregateRow)
	aggOrder := []aggKey{} // 保留插入顺序

	for _, row := range dataRows {
		productID := ""
		if colProductID >= 0 && colProductID < len(row) {
			productID = strings.TrimSpace(row[colProductID])
		}
		spec := ""
		if colSpec >= 0 && colSpec < len(row) {
			spec = strings.TrimSpace(row[colSpec])
		}
		qty := 1
		if colQty >= 0 && colQty < len(row) {
			qty = parseQty(row[colQty])
		}

		model, skuName := common.ParseSpec(spec)
		imageKey := strings.ToLower(productID + "|" + skuName)

		item, ok := engine.Items[imageKey]
		if !ok {
			result.Unmatched = append(result.Unmatched, row)
			continue
		}

		key := aggKey{Stall: item.Stall, ProductID: productID, SKU: skuName, Model: model}
		if existing, exists := aggMap[key]; exists {
			existing.Quantity += qty
		} else {
			aggMap[key] = &AggregateRow{
				ProductID: productID,
				SKU:       skuName,
				Model:     model,
				Quantity:  qty,
				ImageKey:  imageKey,
			}
			aggOrder = append(aggOrder, key)
		}
	}

	// 按档口分组，保持 Sheet 顺序
	for _, key := range aggOrder {
		row := aggMap[key]
		result.StallAggregates[key.Stall] = append(result.StallAggregates[key.Stall], *row)
	}

	return result
}

func parseQty(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 1
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 1
	}
	if n < 1 {
		return 1
	}
	return n
}

// ---- 输出 ----

func writeOutput(outputPath string, engine *Engine, result *Result) error {
	out := excelize.NewFile()
	defer out.Close()

	firstWritten := false
	for _, stallName := range engine.Stalls {
		rows, ok := result.StallAggregates[stallName]
		if !ok || len(rows) == 0 {
			continue
		}

		sheetName := stallName
		if !firstWritten {
			out.SetSheetName("Sheet1", sheetName)
			firstWritten = true
		} else {
			if _, err := out.NewSheet(sheetName); err != nil {
				return err
			}
		}

		writeStallSheet(out, sheetName, engine, rows)
	}

	out.SetActiveSheet(0)
	return out.SaveAs(outputPath)
}

// writeStallSheet 写单个档口 sheet：表头 [型号, 数量, 图片]
func writeStallSheet(out *excelize.File, sheetName string, engine *Engine, rows []AggregateRow) {
	headers := []string{"型号", "数量", "图片"}
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		out.SetCellValue(sheetName, cell, h)
	}

	// 目标显示像素
	const targetPx = 100

	type imgSize struct{ w, h int }
	sizes := make([]imgSize, len(rows))

	for i, r := range rows {
		item, ok := engine.Items[r.ImageKey]
		if !ok || len(item.ImageBytes) == 0 {
			sizes[i] = imgSize{targetPx, targetPx}
			continue
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(item.ImageBytes))
		if err != nil || cfg.Width == 0 || cfg.Height == 0 {
			sizes[i] = imgSize{targetPx, targetPx}
			continue
		}
		var w, h int
		if cfg.Width >= cfg.Height {
			w = targetPx
			h = cfg.Height * targetPx / cfg.Width
		} else {
			h = targetPx
			w = cfg.Width * targetPx / cfg.Height
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		sizes[i] = imgSize{w, h}
	}

	// 列宽/行高按最大图尺寸 + 余量 (1px ≈ 0.75pt)
	maxW, maxH := 0, 0
	for _, s := range sizes {
		if s.w > maxW {
			maxW = s.w
		}
		if s.h > maxH {
			maxH = s.h
		}
	}
	colWidth := float64(maxW+20) * 0.75 // 约 7 字符
	if colWidth < 18 {
		colWidth = 18
	}
	out.SetColWidth(sheetName, "A", "A", 25)
	out.SetColWidth(sheetName, "B", "B", 10)
	out.SetColWidth(sheetName, "C", "C", colWidth)
	rowHeight := float64(maxH+20) * 0.75
	if rowHeight < 20 {
		rowHeight = 20
	}

	for i := range rows {
		rowNum := i + 2
		out.SetRowHeight(sheetName, rowNum, rowHeight)
	}

	for i, r := range rows {
		rowNum := i + 2
		out.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), r.Model)
		out.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), r.Quantity)

		item, ok := engine.Items[r.ImageKey]
		if !ok || len(item.ImageBytes) == 0 {
			continue
		}
		ext := item.ImageExt
		if ext == "" {
			ext = ".png"
		}
		cell := fmt.Sprintf("C%d", rowNum)

		// 计算 scale 使图正好等于 sizes[i]
		cfg, _, err := image.DecodeConfig(bytes.NewReader(item.ImageBytes))
		if err != nil || cfg.Width == 0 {
			continue
		}
		scaleX := float64(sizes[i].w) / float64(cfg.Width)
		scaleY := float64(sizes[i].h) / float64(cfg.Height)

		_ = out.AddPictureFromBytes(sheetName, cell, &excelize.Picture{
			Extension: ext,
			File:      item.ImageBytes,
			Format: &excelize.GraphicOptions{
				ScaleX: scaleX,
				ScaleY: scaleY,
				OffsetX: 5,
				OffsetY: 5,
			},
		})
	}
}

// ---- 配置路径持久化 ----

const configPathName = "pizhi_config.json"

// ConfigPath 返回配置文件的完整路径（可执行文件同目录）
func ConfigPath() string {
	return common.ConfigPath(configPathName)
}

// SaveConfigPath 保存皮质壳配置文件路径
func SaveConfigPath(path string) error {
	return common.SaveConfigPath(configPathName, path)
}

// LoadConfigPath 加载皮质壳配置文件路径
func LoadConfigPath() string {
	return common.LoadConfigPath(configPathName)
}