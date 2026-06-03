// Package dangkou 手机壳档口分配
//
// 根据型号（苹果/国产）和规格关键词将订单分配到对应档口
package dangkou

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// StallRule 单个档口的匹配规则
type StallRule struct {
	Name      string   `json:"name"`      // 档口名称
	AppleModel bool    `json:"appleModel"` // 是否匹配苹果型号
	Keywords  []string `json:"keywords"`  // 规格关键词（任一命中即匹配）
}

// Config 档口配置
type Config struct {
	CodeFilter string      `json:"codeFilter"` // 商品商家编码必须包含的关键词，空=不过滤
	Stalls     []StallRule `json:"stalls"`     // 档口规则列表，按顺序匹配
}

// DefaultConfig 返回内置默认配置
func DefaultConfig() Config {
	return Config{
		CodeFilter: "液态",
		Stalls: []StallRule{
			{Name: "科威达档口", AppleModel: true, Keywords: nil},
			{Name: "威金斯档口", AppleModel: false, Keywords: []string{"黑加仑", "海葡萄", "流光樱粉"}},
			{Name: "酷霓档口", AppleModel: false, Keywords: []string{"灰粉", "芭蕾粉"}},
			{Name: "飞鹏达档口", AppleModel: false, Keywords: []string{"Reno"}},
		},
	}
}

// LoadConfig 按优先级加载配置：可执行文件同目录 > 当前目录 > 默认
func LoadConfig() Config {
	cfg := DefaultConfig()

	for _, p := range configSearchPaths("stalls.json") {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var loaded Config
		if err := json.Unmarshal(data, &loaded); err != nil {
			continue
		}
		if len(loaded.Stalls) > 0 {
			cfg.Stalls = loaded.Stalls
		}
		cfg.CodeFilter = loaded.CodeFilter
		return cfg
	}
	return cfg
}

// SaveConfig 保存配置到可执行文件同目录
func SaveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath("stalls.json"), data, 0644)
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

// Result 档口分配结果
type Result struct {
	StallOrders map[string][][]string `json:"-"`       // 档口名 → 订单行列表
	Unassigned  [][]string            `json:"-"`       // 未分配的行
	Summary     map[string]int        `json:"summary"` // 档口名 → 数量（含"未分配"）
	OutputDir   string                `json:"outputDir"`
	Total       int                   `json:"total"`
}

// ---- 分类逻辑 ----

func isAppleModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(model, "iphone") || strings.HasPrefix(model, "苹果")
}

// classifyStall 按配置规则分类
func classifyStall(spec string, cfg *Config) string {
	model := spec
	detail := ""
	if idx := strings.Index(spec, "|"); idx >= 0 {
		model = spec[:idx]
		detail = spec[idx+1:]
	}

	fullSpec := model + detail

	for _, rule := range cfg.Stalls {
		if rule.AppleModel && isAppleModel(model) {
			return rule.Name
		}
		for _, kw := range rule.Keywords {
			if strings.Contains(fullSpec, kw) {
				return rule.Name
			}
		}
	}
	return "未分配"
}

// ---- 主处理函数 ----

// Process 读取 Excel 并分配档口
func Process(filename string) (*Result, error) {
	cfg := LoadConfig()
	return ProcessWithConfig(filename, &cfg)
}

// ProcessWithConfig 使用指定配置处理
func ProcessWithConfig(filename string, cfg *Config) (*Result, error) {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, fmt.Errorf("打开Excel失败: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetList()[0])
	if err != nil {
		return nil, fmt.Errorf("读取sheet失败: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("数据行不足")
	}

	headers := rows[0]

	specCol := findColumn(headers, "商品规格")
	codeCol := findColumn(headers, "商品商家编码")
	if specCol < 0 {
		return nil, fmt.Errorf("未找到「商品规格」列")
	}

	result := &Result{
		StallOrders: make(map[string][][]string),
		Summary:     make(map[string]int),
		Total:       len(rows) - 1,
	}

	for i := 1; i < len(rows); i++ {
		row := rows[i]

		// codeFilter 检查
		code := ""
		if codeCol >= 0 && codeCol < len(row) {
			code = strings.TrimSpace(row[codeCol])
		}
		if cfg.CodeFilter != "" && !strings.Contains(code, cfg.CodeFilter) {
			result.Unassigned = append(result.Unassigned, row)
			continue
		}

		spec := ""
		if specCol < len(row) {
			spec = strings.TrimSpace(row[specCol])
		}

		stall := classifyStall(spec, cfg)
		if stall == "未分配" {
			result.Unassigned = append(result.Unassigned, row)
		} else {
			result.StallOrders[stall] = append(result.StallOrders[stall], row)
		}
	}

	// 统计
	for _, rule := range cfg.Stalls {
		result.Summary[rule.Name] = len(result.StallOrders[rule.Name])
	}
	result.Summary["未分配"] = len(result.Unassigned)

	// 输出
	absPath, _ := filepath.Abs(filename)
	excelDir := filepath.Dir(absPath)
	excelName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	outputDir := filepath.Join(excelDir, excelName+"_output")
	os.MkdirAll(outputDir, 0755)
	result.OutputDir = outputDir

	outputPath := filepath.Join(outputDir, "档口分配.xlsx")
	if err := writeOutput(outputPath, headers, cfg, result); err != nil {
		return nil, fmt.Errorf("生成输出文件失败: %w", err)
	}

	return result, nil
}

// ---- 辅助函数 ----

func findColumn(headers []string, name string) int {
	for i, h := range headers {
		if strings.TrimSpace(h) == name {
			return i
		}
	}
	return -1
}

func writeOutput(outputPath string, headers []string, cfg *Config, result *Result) error {
	f := excelize.NewFile()
	defer f.Close()

	firstSheet := true

	// 按配置顺序写档口 sheet
	for _, rule := range cfg.Stalls {
		orders, ok := result.StallOrders[rule.Name]
		if !ok || len(orders) == 0 {
			continue
		}
		if firstSheet {
			f.SetSheetName("Sheet1", rule.Name)
			writeSheet(f, rule.Name, headers, orders)
			firstSheet = false
		} else {
			f.NewSheet(rule.Name)
			writeSheet(f, rule.Name, headers, orders)
		}
	}

	// 未分配
	if len(result.Unassigned) > 0 {
		if firstSheet {
			f.SetSheetName("Sheet1", "未分配")
			writeSheet(f, "未分配", headers, result.Unassigned)
		} else {
			f.NewSheet("未分配")
			writeSheet(f, "未分配", headers, result.Unassigned)
		}
	}

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
