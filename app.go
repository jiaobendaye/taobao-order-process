package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	gort "runtime"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"taobao/internal/dangkou"
	"taobao/internal/filter"
	"taobao/internal/logger"
	"taobao/internal/peijian"
)

// App 后端应用
type App struct {
	ctx context.Context
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ---- 配置管理 ----

// FilterConfig 筛选配置（JSON友好，导出所有字段）
type FilterConfig struct {
	DoubtKeywords     []string `json:"doubtKeywords"`
	AccessoryKeywords []string `json:"accessoryKeywords"`
}

// GetFilterConfig 获取当前筛选配置
func (a *App) GetFilterConfig() FilterConfig {
	cfg := filter.LoadConfig()
	return FilterConfig{
		DoubtKeywords:     cfg.DoubtKeywords,
		AccessoryKeywords: cfg.AccessoryKeywords,
	}
}

// SaveFilterConfig 保存筛选配置
func (a *App) SaveFilterConfig(cfg FilterConfig) error {
	logger.Info("保存筛选配置: doubtKeywords=%d条 accessoryKeywords=%d条",
		len(cfg.DoubtKeywords), len(cfg.AccessoryKeywords))
	return filter.SaveConfig(&filter.Config{
		DoubtKeywords:     cfg.DoubtKeywords,
		AccessoryKeywords: cfg.AccessoryKeywords,
	})
}

// DangkouConfigStore 自设编码文件路径的简单存储
type DangkouConfigStore struct {
	Path string `json:"path"`
}

// GetDangkouConfigPath 获取已保存的自设编码文件路径
func (a *App) GetDangkouConfigPath() string {
	return dangkou.LoadConfigPath()
}

// SaveDangkouConfigPath 保存自设编码文件路径（先校验文件格式）
func (a *App) SaveDangkouConfigPath(path string) error {
	if path == "" {
		return fmt.Errorf("编码文件路径不能为空")
	}
	// 校验文件格式是否正确
	if _, err := dangkou.LoadEngine(path); err != nil {
		return fmt.Errorf("编码文件格式错误: %w", err)
	}
	logger.Info("保存档口配置路径: %s", path)
	return dangkou.SaveConfigPath(path)
}

// SelectDangkouConfigFile 打开文件选择对话框选择自设编码.xlsx，校验通过后自动保存
func (a *App) SelectDangkouConfigFile() (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择自设编码 Excel 文件",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Excel文件 (*.xlsx)", Pattern: "*.xlsx"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return "", nil // 用户取消选择，不报错
	}
	// 校验格式并保存
	if err := a.SaveDangkouConfigPath(path); err != nil {
		return "", err
	}
	return path, nil
}

// ---- 配件配置管理 ----

// GetPeijianConfigPath 获取已保存的配件编码文件路径
func (a *App) GetPeijianConfigPath() string {
	return peijian.LoadConfigPath()
}

// SavePeijianConfigPath 保存配件编码文件路径（先校验文件格式）
func (a *App) SavePeijianConfigPath(path string) error {
	if path == "" {
		return fmt.Errorf("配件编码文件路径不能为空")
	}
	if _, err := peijian.LoadEngine(path); err != nil {
		return fmt.Errorf("配件编码文件格式错误: %w", err)
	}
	logger.Info("保存配件配置路径: %s", path)
	return peijian.SaveConfigPath(path)
}

// SelectPeijianConfigFile 打开文件选择对话框选择配件编码.xlsx，校验通过后自动保存
func (a *App) SelectPeijianConfigFile() (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择配件编码 Excel 文件",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Excel文件 (*.xlsx)", Pattern: "*.xlsx"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return "", nil
	}
	if err := a.SavePeijianConfigPath(path); err != nil {
		return "", err
	}
	return path, nil
}

// ---- 处理结果 ----

// FilterResult 筛选结果
type FilterResult struct {
	Success   bool        `json:"success"`
	Error     string      `json:"error,omitempty"`
	Summary   interface{} `json:"summary"`
	OutputDir string      `json:"outputDir"`
}

// DangkouResult 档口分配结果
type DangkouResult struct {
	Success   bool           `json:"success"`
	Error     string         `json:"error,omitempty"`
	Summary   map[string]int `json:"summary"`
	Total     int            `json:"total"`
	OutputDir string         `json:"outputDir"`
}

// PeijianResult 配件提取分配结果
type PeijianResult struct {
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
	Summary    map[string]int `json:"summary"`
	Total      int            `json:"total"`
	OutputDir  string         `json:"outputDir"`
	OutputPath string         `json:"outputPath"`
}

// ---- 工具方法 ----

// RunFilter 执行订单筛选
func (a *App) RunFilter(filePath string) FilterResult {
	logger.Info("开始订单筛选: %s", filePath)
	result, err := filter.Process(filePath)
	if err != nil {
		logger.Error("订单筛选失败: %v", err)
		return FilterResult{Success: false, Error: err.Error()}
	}
	logger.Info("订单筛选完成: 多件=%d 疑难=%d 正常=%d 配件=%d",
		result.Summary.MultiOrders, result.Summary.DoubtfulOrders,
		result.Summary.NormalOrders, result.Summary.AccessoryRows)
	return FilterResult{
		Success:   true,
		Summary:   result.Summary,
		OutputDir: result.OutputDir,
	}
}

// RunDangkou 执行档口分配
func (a *App) RunDangkou(filePath string) DangkouResult {
	logger.Info("开始档口分配: %s", filePath)
	configPath := a.GetDangkouConfigPath()
	if configPath == "" {
		return DangkouResult{Success: false, Error: "请先配置自设编码文件（点击档口分配旁的⚙按钮）"}
	}
	result, err := dangkou.Process(filePath, configPath)
	if err != nil {
		logger.Error("档口分配失败: %v", err)
		return DangkouResult{Success: false, Error: err.Error()}
	}
	logger.Info("档口分配完成: %v", result.Summary)
	return DangkouResult{
		Success:   true,
		Summary:   result.Summary,
		Total:     result.Total,
		OutputDir: result.OutputDir,
	}
}

// RunPeijianProcess 执行配件提取与档口分配
func (a *App) RunPeijianProcess(filePath string) PeijianResult {
	logger.Info("开始配件提取: %s", filePath)
	configPath := a.GetPeijianConfigPath()
	if configPath == "" {
		return PeijianResult{Success: false, Error: "请先配置配件编码文件（点击配件提取旁的⚙按钮）"}
	}
	result, err := peijian.Process(filePath, configPath)
	if err != nil {
		logger.Error("配件提取失败: %v", err)
		return PeijianResult{Success: false, Error: err.Error()}
	}
	logger.Info("配件提取完成: %v", result.Summary)
	return PeijianResult{
		Success:    true,
		Summary:    result.Summary,
		Total:      result.Total,
		OutputDir:  result.OutputDir,
		OutputPath: result.OutputPath,
	}
}

// OpenDir 在文件管理器中打开目录
func (a *App) OpenDir(dirPath string) error {
	var cmd *exec.Cmd
	switch gort.GOOS {
	case "darwin":
		cmd = exec.Command("open", dirPath)
	case "windows":
		cmd = exec.Command("explorer", dirPath)
	default:
		cmd = exec.Command("xdg-open", dirPath)
	}
	return cmd.Start()
}

// SelectFile 打开原生文件选择对话框，返回选中的文件路径
func (a *App) SelectFile() string {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择 Excel 文件",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Excel文件 (*.xlsx)", Pattern: "*.xlsx"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// HandleDroppedFile 前端拖拽文件后，将内容写入临时文件并返回路径
func (a *App) HandleDroppedFile(filename string, b64data string) string {
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		logger.Error("解码拖拽文件失败: %v", err)
		return ""
	}
	tmpDir := os.TempDir()
	tmpPath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		logger.Error("写入拖拽文件失败: %v", err)
		return ""
	}
	logger.Info("拖拽文件已保存: %s", tmpPath)
	return tmpPath
}
