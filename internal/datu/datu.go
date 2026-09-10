// Package datu 打图工厂订单分配
//
// 根据打图工厂配置表将订单按 (工厂, 编码, 手机型号, 素材) 聚合后分配到对应打图工厂。
// 配置文件格式：唯一 Sheet「打图工厂编码」，每列一个工厂（列头 = 工厂名），
// 列下方为该工厂支持的商品 ID 列表。
package datu

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"taobao/internal/common"
)

// DefaultName 输出 Excel 中「姓名」列固定值
const DefaultName = "凡凡"

// ---- 类型定义 ----

// Engine 打图工厂匹配引擎
type Engine struct {
	FactoryByProductID map[string]string `json:"factoryByProductID"` // 商品ID（小写）→ 工厂名
	Factories          []string          `json:"factories"`          // 按配置表列顺序排列的工厂名
}

// OutputRow 单条订单的输出行（一行一订单，不聚合）
type OutputRow struct {
	Code        string `json:"code"`        // 编码（解析自 【素材-编码】）
	Model       string `json:"model"`       // 手机型号
	Material    string `json:"material"`    // 素材
	Quantity    int    `json:"quantity"`    // 单订单数量（不累加）
	Name        string `json:"name"`        // 姓名（固定 凡凡）
	PaymentTime string `json:"paymentTime"` // 付款时间（订单原值透传，列缺失时为空字符串）
	BuyerNote   string `json:"buyerNote"`   // 买家留言（订单原值透传，列缺失时为空字符串）
	SellerNote  string `json:"sellerNote"`  // 卖家备注（订单原值透传，列缺失时为空字符串）
}

// Result 处理结果
type Result struct {
	FactoryOrders map[string][]OutputRow `json:"factoryOrders"` // 工厂名 → 订单行列表（一行一订单）
	OutputPath    string                 `json:"outputPath"`
	Total         int                    `json:"total"` // 输出行总数（匹配后）
}

// ---- 引擎加载 ----

// LoadEngine 从打图工厂配置 Excel 加载匹配引擎
//
// 配置表格式：
//   - 唯一 Sheet「打图工厂编码」
//   - 第 1 行：列头 = 工厂名
//   - 第 2 行起：每列单元格 = 一个商品 ID
func LoadEngine(configPath string) (*Engine, error) {
	f, err := excelize.OpenFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("打开打图工厂配置文件失败: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("打图工厂配置文件无 Sheet")
	}
	// WPS 内部保留 sheet 跳过
	targetSheet := ""
	for _, s := range sheets {
		if strings.HasPrefix(s, "WpsReserved") {
			continue
		}
		targetSheet = s
		break
	}
	if targetSheet == "" {
		return nil, fmt.Errorf("打图工厂配置文件无有效 Sheet")
	}

	rows, err := f.GetRows(targetSheet)
	if err != nil {
		return nil, fmt.Errorf("读取Sheet「%s」失败: %w", targetSheet, err)
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("打图工厂配置文件为空")
	}

	headers := rows[0]
	engine := &Engine{
		FactoryByProductID: make(map[string]string),
	}

	// 按列扫描：每列第一行（headers[col]）= 工厂名
	for colIdx, factoryHeader := range headers {
		factoryName := strings.TrimSpace(factoryHeader)
		if factoryName == "" {
			continue
		}
		// 仅在第一次见到该工厂名时记录顺序（避免重复）
		alreadySeen := false
		for _, existing := range engine.Factories {
			if existing == factoryName {
				alreadySeen = true
				break
			}
		}
		if !alreadySeen {
			engine.Factories = append(engine.Factories, factoryName)
		}

		// 该列下方行 = 商品 ID 列表
		for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
			if colIdx >= len(rows[rowIdx]) {
				continue
			}
			// 商品 ID 用 GetCellValue 读取以避免大数字精度问题
			cellName, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			productID := strings.TrimSpace(common.GetCellValueSafe(f, targetSheet, cellName))
			if productID == "" {
				// 回退到 GetRows 的值
				productID = strings.TrimSpace(rows[rowIdx][colIdx])
			}
			if productID == "" {
				continue
			}
			key := strings.ToLower(productID)
			engine.FactoryByProductID[key] = factoryName
		}
	}

	if len(engine.Factories) == 0 {
		return nil, fmt.Errorf("配置表为空，未找到任何工厂")
	}
	if len(engine.FactoryByProductID) == 0 {
		return nil, fmt.Errorf("配置表为空，未找到任何商品ID")
	}

	return engine, nil
}

// LookupFactory 根据商品ID查找工厂名。返回空字符串表示未找到。
func (e *Engine) LookupFactory(productID string) string {
	if e == nil {
		return ""
	}
	return e.FactoryByProductID[strings.ToLower(strings.TrimSpace(productID))]
}

// ---- 解析 【素材-编码】 ----

// ParseDatuCode 解析 【素材-编码】 格式
//
// 当存在多个 【...】 时，只取最后一个 【...】 进行解析。
// 符合: 【DYT彩银白色-DTY7958】 -> ("DYT彩银白色", "DTY7958")
//       【PH仓】【DYT彩银白色-DTY7958】 -> ("DYT彩银白色", "DTY7958")
// 不符合 (如 【PH皮质】 / 【DTY彩银白色DTY7958】 / 空字符串) -> ("", "")
func ParseDatuCode(s string) (material, code string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	// 取最后一个 【...】 片段
	lastOpen := strings.LastIndex(s, "【")
	lastClose := strings.LastIndex(s, "】")
	if lastOpen < 0 || lastClose <= lastOpen {
		return "", ""
	}
	inner := s[lastOpen+len("【") : lastClose]
	parts := strings.SplitN(inner, "-", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

// ---- 主处理函数 ----

// Process 读取订单 Excel 并按打图工厂配置聚合分配
func Process(filename, configPath string) (*Result, error) {
	if configPath == "" {
		return nil, fmt.Errorf("打图工厂配置文件不能为空\n用法: phonecase-tools datu <订单文件> <打图工厂配置表.xlsx>")
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

	outputPath := filepath.Join(outputDir, "打图结果.xlsx")
	if err := writeOutput(outputPath, engine, result); err != nil {
		return nil, fmt.Errorf("生成输出文件失败: %w", err)
	}
	result.OutputPath = outputPath

	return result, nil
}

// ProcessData 对已解析的订单数据执行打图分配，不涉及文件 I/O。
//
// 每个匹配订单输出 1 行（不再按 (工厂, 编码, 型号, 素材) 聚合）。
// 编码/素材可为空（不匹配 【素材-编码】 格式时）。
// 商品 ID 不在 Engine 中的订单静默跳过。
func ProcessData(dataRows [][]string, headers []string, engine *Engine) *Result {
	colProductID := common.FindColumn(headers, "商品id")
	colSpec := common.FindColumn(headers, "商品规格")
	colDatuCode := common.FindColumn(headers, "商品规格商家编码")
	colQty := common.FindColumn(headers, "商品数量")
	colPayTime := common.FindColumn(headers, "付款时间")
	colBuyerNote := common.FindColumn(headers, "买家留言")
	colSellerNote := common.FindColumn(headers, "卖家备注")

	result := &Result{
		FactoryOrders: make(map[string][]OutputRow),
		Total:         0,
	}

	for _, row := range dataRows {
		productID := ""
		if colProductID >= 0 && colProductID < len(row) {
			productID = strings.TrimSpace(row[colProductID])
		}

		factory := engine.LookupFactory(productID)
		if factory == "" {
			continue
		}

		spec := ""
		if colSpec >= 0 && colSpec < len(row) {
			spec = strings.TrimSpace(row[colSpec])
		}
		model, _ := common.ParseSpec(spec)
		// ParseSpec 已 strip 末尾 [...] / 【...】，符合我们对手机型号的预期

		material, code := "", ""
		if colDatuCode >= 0 && colDatuCode < len(row) {
			material, code = ParseDatuCode(strings.TrimSpace(row[colDatuCode]))
		}

		qty := 1
		if colQty >= 0 && colQty < len(row) {
			qty = parseQty(row[colQty])
		}

		payTime := readCell(row, colPayTime)
		buyerNote := readCell(row, colBuyerNote)
		sellerNote := readCell(row, colSellerNote)

		result.FactoryOrders[factory] = append(result.FactoryOrders[factory], OutputRow{
			Code:        code,
			Model:       model,
			Material:    material,
			Quantity:    qty,
			Name:        DefaultName,
			PaymentTime: payTime,
			BuyerNote:   buyerNote,
			SellerNote:  sellerNote,
		})
		result.Total++
	}

	return result
}

// readCell 读取指定列的单元格字符串值；列缺失或单元格为空时返回空字符串。
func readCell(row []string, colIdx int) string {
	if colIdx < 0 || colIdx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[colIdx])
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
	for _, factoryName := range engine.Factories {
		rows, ok := result.FactoryOrders[factoryName]
		if !ok || len(rows) == 0 {
			continue
		}

		// 按付款时间升序排序
		sort.SliceStable(rows, func(i, j int) bool {
			return rows[i].PaymentTime < rows[j].PaymentTime
		})

		sheetName := factoryName
		if !firstWritten {
			if err := out.SetSheetName("Sheet1", sheetName); err != nil {
				return err
			}
			firstWritten = true
		} else {
			if _, err := out.NewSheet(sheetName); err != nil {
				return err
			}
		}

		writeFactorySheet(out, sheetName, rows)
	}

	out.SetActiveSheet(0)
	return out.SaveAs(outputPath)
}

// writeFactorySheet 写单个工厂 sheet：表头 [序号, 编码, 手机型号, 素材, 数量, 姓名, 付款时间, 买家留言, 卖家备注]
func writeFactorySheet(out *excelize.File, sheetName string, rows []OutputRow) {
	headers := []string{"序号", "编码", "手机型号", "素材", "数量", "姓名", "付款时间", "买家留言", "卖家备注"}
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = out.SetCellValue(sheetName, cell, h)
	}

	for i, r := range rows {
		rowNum := i + 2
		_ = out.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), i+1)
		_ = out.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), r.Code)
		_ = out.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), r.Model)
		_ = out.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), r.Material)
		_ = out.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), r.Quantity)
		_ = out.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), r.Name)
		_ = out.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), r.PaymentTime)
		_ = out.SetCellValue(sheetName, fmt.Sprintf("H%d", rowNum), r.BuyerNote)
		_ = out.SetCellValue(sheetName, fmt.Sprintf("I%d", rowNum), r.SellerNote)
	}
}

// ---- 配置路径持久化 ----

const configPathName = "datu_config.json"

// ConfigPath 返回配置文件的完整路径（可执行文件同目录）
func ConfigPath() string {
	return common.ConfigPath(configPathName)
}

// SaveConfigPath 保存打图工厂配置文件路径
func SaveConfigPath(path string) error {
	return common.SaveConfigPath(configPathName, path)
}

// LoadConfigPath 加载打图工厂配置文件路径
func LoadConfigPath() string {
	return common.LoadConfigPath(configPathName)
}
