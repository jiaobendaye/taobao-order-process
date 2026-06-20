// Package common 工具函数，被 dangkou / peijian 等模块共用
package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// ---- Excel helpers ----

// FindColumn 在表头中查找指定列名的索引（大小写不敏感）
func FindColumn(headers []string, name string) int {
	for i, h := range headers {
		if strings.EqualFold(strings.TrimSpace(h), name) {
			return i
		}
	}
	return -1
}

// GetCellValueSafe 安全读取单元格值（处理合并单元格等情况）
func GetCellValueSafe(f *excelize.File, sheet, cell string) string {
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

// StripInvisible 去除字符串两端的不可见字符（BOM、零宽空格等）
func StripInvisible(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		// U+FEFF BOM, U+200B ZWSP, U+200C ZWNJ, U+200D ZWJ, U+200E LRM, U+200F RLM
		return r == 0xFEFF || r == 0x200B || r == 0x200C ||
			r == 0x200D || r == 0x200E || r == 0x200F
	})
}

// ParseSpec 解析商品规格，返回 (型号, SKU名称)
// 商品规格格式: "{Phone Model}|{SKU Name}[{Variant}]"
func ParseSpec(spec string) (model, skuName string) {
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

// ---- 配置路径持久化 ----

// ConfigPath 返回配置文件的完整路径（可执行文件同目录）
func ConfigPath(name string) string {
	execPath, _ := os.Executable()
	return filepath.Join(filepath.Dir(execPath), name)
}

// ConfigSearchPaths 返回配置文件搜索路径列表
func ConfigSearchPaths(name string) []string {
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	return []string{
		filepath.Join(execDir, name),
		name,
	}
}

// LoadConfigPath 从 JSON 配置文件加载路径。
// configName 为 JSON 文件名，返回保存的路径；未找到返回空字符串。
func LoadConfigPath(configName string) string {
	for _, p := range ConfigSearchPaths(configName) {
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
			if _, err := os.Stat(cfg.Path); err == nil {
				return cfg.Path
			}
		}
	}
	return ""
}

// SaveConfigPath 保存路径到 JSON 配置文件
func SaveConfigPath(configName, path string) error {
	data, err := json.MarshalIndent(map[string]string{"path": path}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(configName), data, 0644)
}

// LoadConfigPathOrError 与 LoadConfigPath 相同但返回 error
func LoadConfigPathOrError(configName string) (string, error) {
	path := LoadConfigPath(configName)
	if path == "" {
		return "", fmt.Errorf("未找到已保存的配置文件路径: %s", configName)
	}
	return path, nil
}
