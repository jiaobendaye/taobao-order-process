// Package dangkou 手机壳档口分配
//
// 根据自设编码 Excel 配置文件将订单分配到对应档口。
// 配置文件 Sheet 1（自设编码）为 (商品ID, SKU名称) → 自设编码 映射表，
// 后续每个 Sheet 为一个档口（Sheet 名即为档口名），
// 按 Sheet 顺序决定档口优先级。
package dangkou

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// ---- 类型定义 ----

// StallConfig 单个档口的配置（从 Excel 的一个 Sheet 读取）
type StallConfig struct {
	Name     string              // 档口名称（Sheet 名）
	Priority int                 // 优先级（0-based Sheet 顺序）
	Codes    map[string][]string // 自设编码 → 支持的手机型号列表
}

// Engine 档口匹配引擎
type Engine struct {
	mapping map[string]string // "商品ID|SKU名称" → 自设编码
	Stalls  []StallConfig     // 按优先级排序的档口列表
}

// Result 档口分配结果
type Result struct {
	StallOrders map[string][][]string `json:"-"`       // 档口名 → 订单行列表
	NoCodeMatch [][]string            `json:"-"`       // 无匹配自设编码的行
	Unassigned  [][]string            `json:"-"`       // 有自设编码但无档口匹配的行
	Summary     map[string]int        `json:"summary"` // 档口名 → 数量
	OutputDir   string                `json:"outputDir"`
	Total       int                   `json:"total"`
}

// ---- 引擎加载 ----

// LoadEngine 从自设编码 Excel 文件加载匹配引擎
func LoadEngine(configPath string) (*Engine, error) {
	f, err := excelize.OpenFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) < 2 {
		return nil, fmt.Errorf("配置文件至少需要2个Sheet（自设编码 + 至少1个档口）")
	}

	engine := &Engine{}

	// Sheet 1: 自设编码映射表
	mapping, err := loadMapping(f, sheets[0])
	if err != nil {
		return nil, fmt.Errorf("读取自设编码映射失败: %w", err)
	}
	engine.mapping = mapping

	// 后续 Sheets: 各档口配置
	stalls, err := loadStallConfigs(f, sheets[1:])
	if err != nil {
		return nil, fmt.Errorf("读取档口配置失败: %w", err)
	}
	engine.Stalls = stalls

	return engine, nil
}

// loadMapping 从 Sheet 1 读取 (商品ID, SKU名称) → 自设编码 映射
func loadMapping(f *excelize.File, sheetName string) (map[string]string, error) {
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("自设编码Sheet数据行不足")
	}

	// 找列
	headers := rows[0]
	colProductID := findColumn(headers, "商品ID")
	colSKU := findColumn(headers, "SKU名称")
	colCode := findColumn(headers, "自设编码")
	if colProductID < 0 {
		return nil, fmt.Errorf("未找到「商品ID」列")
	}
	if colSKU < 0 {
		return nil, fmt.Errorf("未找到「SKU名称」列")
	}
	if colCode < 0 {
		return nil, fmt.Errorf("未找到「自设编码」列")
	}

	// 同时用 GetCellValue 读取商品ID（避免科学计数法问题）
	// 先用 GetRows 的行数循环，再用 GetCellValue 取商品ID
	mapping := make(map[string]string, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := rows[i]

		// 商品ID 使用 GetCellValue 读取以避免大数字精度问题
		productIDCell, _ := excelize.CoordinatesToCellName(colProductID+1, i+1)
		productID := strings.TrimSpace(getCellValueSafe(f, sheetName, productIDCell))
		if productID == "" {
			// 回退到 GetRows 的值
			if colProductID < len(row) {
				productID = strings.TrimSpace(row[colProductID])
			}
		}

		skuName := ""
		if colSKU < len(row) {
			skuName = strings.TrimSpace(row[colSKU])
		}

		code := ""
		if colCode < len(row) {
			code = strings.TrimSpace(row[colCode])
		}

		if productID == "" || skuName == "" || code == "" {
			continue
		}

		key := strings.ToLower(productID + "|" + skuName)
		mapping[key] = code
	}

	if len(mapping) == 0 {
		return nil, fmt.Errorf("自设编码映射表为空")
	}

	return mapping, nil
}

// loadStallConfigs 从 Sheets 2+ 读取各档口配置（列式布局）
//
// 每个档口 Sheet 的格式：
//   - Row 0: 各列的 自设编码（表头）
//   - Row 1+: 每列为该自设编码对应的型号列表
func loadStallConfigs(f *excelize.File, sheetNames []string) ([]StallConfig, error) {
	stalls := make([]StallConfig, 0, len(sheetNames))

	for i, sheetName := range sheetNames {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("读取Sheet「%s」失败: %w", sheetName, err)
		}
		if len(rows) < 2 {
			continue // 空档口跳过
		}

		// Row 0: 自设编码表头，每个非空单元格是一个自设编码
		headerRow := rows[0]
		codes := make(map[string][]string)

		for col, code := range headerRow {
			code = strings.ToLower(strings.TrimSpace(code))
			if code == "" {
				continue
			}

			// 从 Row 1 开始读取该列下的所有型号
			var models []string
			for j := 1; j < len(rows); j++ {
				if col >= len(rows[j]) {
					continue
				}
				model := strings.TrimSpace(rows[j][col])
				if model == "" {
					continue
				}
				model = stripInvisible(model)
				model = strings.ReplaceAll(model, " ", "")
				if model != "" {
					models = append(models, model)
				}
			}
			codes[code] = models
		}

		stalls = append(stalls, StallConfig{
			Name:     sheetName,
			Priority: i,
			Codes:    codes,
		})
	}

	return stalls, nil
}

// getCellValueSafe 安全读取单元格值（处理合并单元格等情况）
func getCellValueSafe(f *excelize.File, sheet, cell string) string {
	v, err := f.GetCellValue(sheet, cell)
	if err != nil {
		return ""
	}
	return v
}

// ---- 字符串处理 ----

// StripBracketSuffix 去除字符串末尾的 [...] 或 【...】 后缀
func StripBracketSuffix(s string) string {
	s = strings.TrimSpace(s)
	// 去除末尾 [...]
	for {
		idx := strings.LastIndex(s, "[")
		if idx < 0 {
			break
		}
		closeIdx := strings.LastIndex(s, "]")
		if closeIdx == len(s)-1 && closeIdx > idx {
			s = strings.TrimSpace(s[:idx])
		} else {
			break
		}
	}
	// 去除末尾 【...】
	for {
		idx := strings.LastIndex(s, "【")
		if idx < 0 {
			break
		}
		closeIdx := strings.LastIndex(s, "】")
		if closeIdx == len(s)-utf8.RuneLen('】') && closeIdx > idx {
			s = strings.TrimSpace(s[:idx])
		} else {
			break
		}
	}
	return s
}

// stripInvisible 去除字符串两端的不可见字符（BOM、零宽空格等）
func stripInvisible(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		// U+FEFF BOM, U+200B ZWSP, U+200C ZWNJ, U+200D ZWJ, U+200E LRM, U+200F RLM
		return r == 0xFEFF || r == 0x200B || r == 0x200C ||
			r == 0x200D || r == 0x200E || r == 0x200F
	})
}

// parseSpec 解析商品规格，返回 (型号, SKU名称)
// 商品规格格式: "{Phone Model}|{SKU Name}[{Variant}]"
func parseSpec(spec string) (model, skuName string) {
	spec = strings.TrimSpace(spec)
	if idx := strings.Index(spec, "|"); idx >= 0 {
		model = strings.ReplaceAll(strings.TrimSpace(spec[:idx]), " ", "")
		skuName = StripBracketSuffix(spec[idx+1:])
	} else {
		model = ""
		skuName = StripBracketSuffix(spec)
	}
	return
}

// ---- 匹配方法 ----

// LookupZisheBianma 根据商品ID和SKU名称查找自设编码。
// 返回空字符串表示未找到。
func (e *Engine) LookupZisheBianma(productID, skuName string) string {
	key := strings.ToLower(productID + "|" + skuName)
	return e.mapping[key]
}

// FindStall 按档口优先级查找匹配该自设编码和型号的档口。
// model 为空时跳过型号检查。
// 返回档口名称；若所有档口均不匹配则返回空字符串。
func (e *Engine) FindStall(zisheBianma, model string) string {
	lowerCode := strings.ToLower(zisheBianma)
	lowerModel := strings.ToLower(model)
	for _, stall := range e.Stalls {
		models, ok := stall.Codes[lowerCode]
		if !ok {
			continue
		}
		if model == "" {
			return stall.Name
		}
		for _, m := range models {
			if strings.EqualFold(m, lowerModel) {
				return stall.Name
			}
		}
	}
	return ""
}

// ---- 主处理函数 ----

// Process 读取订单 Excel 并按自设编码配置分配档口。
// configPath 为自设编码.xlsx 的路径；若为空则尝试从 dangkou_config.json 读取。
func Process(filename, configPath string) (*Result, error) {
	// 加载配置路径
	if configPath == "" {
		path, err := loadConfigPath()
		if err != nil {
			return nil, fmt.Errorf("未指定配置文件路径，且无法加载已保存的路径: %w\n请在GUI中先选择自设编码文件，或通过命令行传入: phonecase-tools dangkou <订单文件> <自设编码.xlsx>", err)
		}
		configPath = path
	}

	// 加载引擎
	engine, err := LoadEngine(configPath)
	if err != nil {
		return nil, err
	}
	// logger.Info("已加载自设编码配置: %v", engine)

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
	colProductID := findColumn(headers, "商品id")
	colSpec := findColumn(headers, "商品规格")
	if colProductID < 0 {
		return nil, fmt.Errorf("未找到「商品id」列")
	}
	if colSpec < 0 {
		return nil, fmt.Errorf("未找到「商品规格」列")
	}

	result := &Result{
		StallOrders: make(map[string][][]string),
		Summary:     make(map[string]int),
		Total:       len(rows) - 1,
	}

	// 逐行处理
	for i := 1; i < len(rows); i++ {
		row := rows[i]

		// 读取商品ID（使用 GetCellValue 避免精度问题）
		productIDCell, _ := excelize.CoordinatesToCellName(colProductID+1, i+1)
		productID := strings.TrimSpace(getCellValueSafe(f, f.GetSheetList()[0], productIDCell))
		if productID == "" && colProductID < len(row) {
			productID = strings.TrimSpace(row[colProductID])
		}

		// 读取商品规格
		spec := ""
		if colSpec < len(row) {
			spec = strings.TrimSpace(row[colSpec])
		}

		// 解析规格
		model, skuName := parseSpec(spec)

		// 查找自设编码
		zisheBianma := engine.LookupZisheBianma(productID, skuName)
		if zisheBianma == "" {
			// 无匹配自设编码
			result.NoCodeMatch = append(result.NoCodeMatch, row)
			continue
		}

		// 查找档口（按自设编码 + 型号匹配）
		stall := engine.FindStall(zisheBianma, model)
		if stall == "" {
			// 有自设编码但无档口匹配
			result.Unassigned = append(result.Unassigned, row)
		} else {
			result.StallOrders[stall] = append(result.StallOrders[stall], row)
		}
	}

	// 统计
	for _, stall := range engine.Stalls {
		result.Summary[stall.Name] = len(result.StallOrders[stall.Name])
	}
	result.Summary["无匹配自设编码"] = len(result.NoCodeMatch)
	result.Summary["未分配档口"] = len(result.Unassigned)

	// 输出
	absPath, _ := filepath.Abs(filename)
	excelDir := filepath.Dir(absPath)
	excelName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	outputDir := filepath.Join(excelDir, excelName+"_output")
	os.MkdirAll(outputDir, 0755)
	result.OutputDir = outputDir

	outputPath := filepath.Join(outputDir, "档口分配.xlsx")
	if err := writeOutput(outputPath, headers, engine, result); err != nil {
		return nil, fmt.Errorf("生成输出文件失败: %w", err)
	}

	return result, nil
}

// ---- 输出 ----

func writeOutput(outputPath string, headers []string, engine *Engine, result *Result) error {
	f := excelize.NewFile()
	defer f.Close()

	firstSheet := true

	// 按档口优先级顺序写 sheet
	for _, stall := range engine.Stalls {
		orders, ok := result.StallOrders[stall.Name]
		if !ok || len(orders) == 0 {
			continue
		}
		if firstSheet {
			f.SetSheetName("Sheet1", stall.Name)
			writeSheet(f, stall.Name, headers, orders)
			firstSheet = false
		} else {
			if _, err := f.NewSheet(stall.Name); err != nil {
				return err
			}
			writeSheet(f, stall.Name, headers, orders)
		}
	}

	// 未分配档口
	if len(result.Unassigned) > 0 {
		if firstSheet {
			f.SetSheetName("Sheet1", "未分配档口")
			writeSheet(f, "未分配档口", headers, result.Unassigned)
			firstSheet = false
		} else {
			if _, err := f.NewSheet("未分配档口"); err != nil {
				return err
			}
			writeSheet(f, "未分配档口", headers, result.Unassigned)
		}
	}

	// 无匹配自设编码
	if len(result.NoCodeMatch) > 0 {
		if firstSheet {
			f.SetSheetName("Sheet1", "无匹配自设编码")
			writeSheet(f, "无匹配自设编码", headers, result.NoCodeMatch)
		} else {
			if _, err := f.NewSheet("无匹配自设编码"); err != nil {
				return err
			}
			writeSheet(f, "无匹配自设编码", headers, result.NoCodeMatch)
		}
	}

	f.SetActiveSheet(0)
	return f.SaveAs(outputPath)
}

func writeSheet(f *excelize.File, name string, headers []string, rows [][]string) {
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(name, cell, h)
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(name, cell, val)
		}
	}
}

// ---- 辅助函数 ----

// findColumn 在表头中查找指定列名的索引
func findColumn(headers []string, name string) int {
	for i, h := range headers {
		if strings.EqualFold(strings.TrimSpace(h), name) {
			return i
		}
	}
	return -1
}

// ---- 配置路径持久化 ----

// dangkouConfigName 存储自设编码文件路径的 JSON 文件名
const dangkouConfigName = "dangkou_config.json"

// ConfigPath 返回配置文件的完整路径（可执行文件同目录）
func ConfigPath(name string) string {
	execPath, _ := os.Executable()
	return filepath.Join(filepath.Dir(execPath), name)
}

// SaveConfigPath 保存自设编码文件路径到 dangkou_config.json
func SaveConfigPath(path string) error {
	data, err := json.MarshalIndent(map[string]string{"path": path}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(dangkouConfigName), data, 0644)
}

// LoadConfigPath 从 dangkou_config.json 加载自设编码文件路径。
// 返回空字符串表示未找到已保存的路径。
func LoadConfigPath() string {
	path, err := loadConfigPath()
	if err != nil {
		return ""
	}
	return path
}

// loadConfigPath 从 dangkou_config.json 加载自设编码文件路径
func loadConfigPath() (string, error) {
	for _, p := range configSearchPaths(dangkouConfigName) {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		if cfg.Path != "" {
			// 验证文件是否存在
			if _, err := os.Stat(cfg.Path); err == nil {
				return cfg.Path, nil
			}
			// 文件不存在，继续尝试其他搜索路径
		}
	}
	return "", fmt.Errorf("未找到已保存的自设编码文件路径")
}

func configSearchPaths(name string) []string {
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	return []string{
		filepath.Join(execDir, name),
		name,
	}
}
