package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindColumn(t *testing.T) {
	headers := []string{" 店铺名称 ", "订单编号", "商品id", "商品规格"}
	tests := []struct {
		name   string
		target string
		want   int
	}{
		{"exact match", "订单编号", 1},
		{"case insensitive", "商品ID", 2},
		{"with spaces", "店铺名称", 0},
		{"case and space insensitive", "商品规格", 3},
		{"not found", "不存在", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindColumn(headers, tt.target)
			if got != tt.want {
				t.Errorf("FindColumn(%q) = %d, want %d", tt.target, got, tt.want)
			}
		})
	}
}

func TestStripBracketSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"透明壳[黑色]", "透明壳"},
		{"透明壳[黑色][大号]", "透明壳"},
		{"透明壳【红色】", "透明壳"},
		{"普通壳", "普通壳"},
		{"壳 [中] [大] ", "壳"},
		{"壳【中】【大】", "壳"},
		{"[not at end] middle", "[not at end] middle"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := StripBracketSuffix(tt.input)
			if got != tt.want {
				t.Errorf("StripBracketSuffix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripInvisible(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{string(rune(0xFEFF)) + "hello" + string(rune(0x200B)), "hello"},
		{string(rune(0x200B)) + string(rune(0x200C)) + "test" + string(rune(0x200D)) + string(rune(0x200E)), "test"},
		{"normal text", "normal text"},
		{"", ""},
		{string(rune(0x200B)) + string(rune(0x200C)) + string(rune(0x200D)), ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := StripInvisible(tt.input)
			if got != tt.want {
				t.Errorf("StripInvisible(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseSpec(t *testing.T) {
	tests := []struct {
		spec      string
		wantModel string
		wantSKU   string
	}{
		{"iPhone15Pro|透明壳[黑色]", "iPhone15Pro", "透明壳"},
		{"华为Pura 70 Pro+|薄荷波点+薄荷糖支架", "华为Pura70Pro+", "薄荷波点+薄荷糖支架"},
		{"iPhone 15 Pro Max|奶油蓝+红苹果支架+红色手提绳", "iPhone15ProMax", "奶油蓝+红苹果支架+红色手提绳"},
		{"simple|SKU", "simple", "SKU"},
		{"no pipe just SKU[红色]", "", "no pipe just SKU"},
		{"|", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			model, sku := ParseSpec(tt.spec)
			if model != tt.wantModel {
				t.Errorf("ParseSpec(%q) model = %q, want %q", tt.spec, model, tt.wantModel)
			}
			if sku != tt.wantSKU {
				t.Errorf("ParseSpec(%q) sku = %q, want %q", tt.spec, sku, tt.wantSKU)
			}
		})
	}
}

func TestConfigPathAndSearchPaths(t *testing.T) {
	// ConfigPath should return a non-empty path
	path := ConfigPath("test.json")
	if path == "" {
		t.Error("ConfigPath returned empty string")
	}

	// ConfigSearchPaths should return 2 paths
	paths := ConfigSearchPaths("test.json")
	if len(paths) != 2 {
		t.Errorf("ConfigSearchPaths returned %d paths, want 2", len(paths))
	}
	// Both paths should end with test.json
	for _, p := range paths {
		if filepath.Base(p) != "test.json" {
			t.Errorf("ConfigSearchPaths path %q does not end with test.json", p)
		}
	}
}

func TestSaveAndLoadConfigPath(t *testing.T) {
	// Use a temp config name to avoid clobbering real config
	configName := "common_test_config.json"
	cleanup := func() {
		for _, p := range ConfigSearchPaths(configName) {
			os.Remove(p)
		}
	}
	defer cleanup()

	// Initially should be empty
	path := LoadConfigPath(configName)
	if path != "" {
		t.Logf("pre-existing config path: %s, will overwrite", path)
	}

	// Save a test path
	testPath := "/tmp/test_peijian_config.xlsx"
	err := SaveConfigPath(configName, testPath)
	if err != nil {
		t.Fatalf("SaveConfigPath failed: %v", err)
	}

	// Load should return the saved path only if the file exists
	// Since /tmp/test_peijian_config.xlsx doesn't exist, LoadConfigPath returns ""
	// Create the file first
	err = os.WriteFile(testPath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(testPath)

	loaded := LoadConfigPath(configName)
	if loaded != testPath {
		t.Errorf("LoadConfigPath = %q, want %q", loaded, testPath)
	}
}
