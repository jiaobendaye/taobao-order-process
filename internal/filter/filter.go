// Package filter 手机壳订单筛选
//
// 将 Excel 订单按规则分为：多件订单、疑难单、正常手机壳、单独配件四个类别
package filter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Config 筛选配置
type Config struct {
	DoubtKeywords     []string `json:"doubtKeywords"`
	AccessoryKeywords []string `json:"accessoryKeywords"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		DoubtKeywords:     []string{"其他", "咨询客服", "备注", "diy"},
		AccessoryKeywords: []string{"支架", "绳", "链", "吸盘", "串珠", "相机", "纽扣", "腕带", "贴纸", "卡包"},
	}
}

// MergeConfig 从 JSON 文件合并配置到默认配置上（追加模式，用于加载外部配置）
// Deprecated: 使用 LoadConfig 代替，它直接替换整个配置
func MergeConfig(configPath string, cfg *Config) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}
	if len(loaded.DoubtKeywords) > 0 {
		cfg.DoubtKeywords = loaded.DoubtKeywords
	}
	if len(loaded.AccessoryKeywords) > 0 {
		cfg.AccessoryKeywords = loaded.AccessoryKeywords
	}
}

// LoadConfig 按优先级查找并加载配置文件（直接替换模式）
func LoadConfig() Config {
	cfg := DefaultConfig()

	for _, p := range configSearchPaths("keywords.json") {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var loaded Config
		if err := json.Unmarshal(data, &loaded); err != nil {
			continue
		}
		if len(loaded.DoubtKeywords) > 0 {
			cfg.DoubtKeywords = loaded.DoubtKeywords
		}
		if len(loaded.AccessoryKeywords) > 0 {
			cfg.AccessoryKeywords = loaded.AccessoryKeywords
		}
		break // use first found config
	}
	return cfg
}

// SaveConfig 保存配置到可执行文件同目录
func SaveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath("keywords.json"), data, 0644)
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

// RowData Excel 中的一行订单数据
type RowData struct {
	ShopName        string `xlsx:"店铺名称"`
	OrderID         string `xlsx:"订单编号"`
	SubOrderID      string `xlsx:"子订单编号"`
	BuyerNick       string `xlsx:"买家昵称"`
	ReceiverName    string `xlsx:"收件人姓名"`
	ReceiverPhone   string `xlsx:"收件人手机号"`
	ReceiverAddress string `xlsx:"收件人详细地址"`
	PaymentTime     string `xlsx:"付款时间"`
	BuyerMsg        string `xlsx:"买家留言"`
	SellerNote      string `xlsx:"卖家备注"`
	Code            string `xlsx:"商品商家编码"`
	Spec            string `xlsx:"商品规格"`
	Quantity        int    `xlsx:"商品数量"`
}

// Result 筛选结果
type Result struct {
	MultiOrders    []RowData `json:"-"`
	DoubtfulOrders []RowData `json:"-"`
	NormalOrders   []RowData `json:"-"`
	AccessoryRows  []RowData `json:"-"`
	OutputDir      string    `json:"outputDir"`
	Summary        Summary   `json:"summary"`
}

// Summary 统计摘要
type Summary struct {
	MultiOrders    int `json:"multiOrders"`
	DoubtfulOrders int `json:"doubtfulOrders"`
	NormalOrders   int `json:"normalOrders"`
	AccessoryRows  int `json:"accessoryRows"`
	Total          int `json:"total"`
}

// fieldMapper 表头字段名 → 结构体字段索引 的映射
type fieldMapper struct {
	headerToFieldIdx map[string]int
	colToFieldIdx    map[int]int
}

func newFieldMapper(sample *RowData) *fieldMapper {
	t := reflect.TypeOf(*sample)
	headerToFieldIdx := make(map[string]int)
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("xlsx")
		if tag != "" {
			headerToFieldIdx[tag] = i
		}
	}
	return &fieldMapper{
		headerToFieldIdx: headerToFieldIdx,
		colToFieldIdx:    make(map[int]int),
	}
}

func scanRow(data []string, mapper *fieldMapper) *RowData {
	row := &RowData{}
	v := reflect.ValueOf(row).Elem()
	for colIdx := range data {
		fieldIdx, ok := mapper.colToFieldIdx[colIdx]
		if !ok {
			continue
		}
		if fieldIdx < v.NumField() {
			f := v.Field(fieldIdx)
			if f.CanSet() {
				val := data[colIdx]
				switch f.Kind() {
				case reflect.Int:
					if qty, err := strconv.Atoi(val); err == nil {
						f.SetInt(int64(qty))
					}
				case reflect.String:
					f.SetString(val)
				}
			}
		}
	}
	return row
}

// ---- 分类逻辑 ----

func isMultiOrder(row *RowData) bool {
	return row.SubOrderID != "" && row.OrderID != "" && row.SubOrderID != row.OrderID
}

// recipientKey 生成收件人复合键（买家昵称 + 三个收件人信息）
func recipientKey(row *RowData) string {
	return row.BuyerNick + "|" + row.ReceiverName + "|" + row.ReceiverPhone + "|" + row.ReceiverAddress
}

// isRecipientMulti 判断该行是否属于收件人信息相同的多件订单组
func isRecipientMulti(row *RowData, multiCount map[string]int) bool {
	key := recipientKey(row)
	return key != "||||" && multiCount[key] > 1
}

func isDoubtful(row *RowData, cfg *Config) bool {
	if row.BuyerMsg != "" || row.SellerNote != "" {
		return true
	}
	for _, kw := range cfg.DoubtKeywords {
		if strings.Contains(strings.ToLower(row.Spec), strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func isAccessoryOnly(row *RowData, cfg *Config) bool {
	if strings.Contains(row.Spec, "+") {
		return false
	}
	for _, kw := range cfg.AccessoryKeywords {
		if strings.Contains(strings.ToLower(row.Spec), strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// ---- 主处理函数 ----

// Process 读取 Excel 并分类，输出到 _output 目录
func Process(filename string) (*Result, error) {
	cfg := LoadConfig()
	return ProcessWithConfig(filename, &cfg)
}

// ProcessWithConfig 使用指定配置处理
func ProcessWithConfig(filename string, cfg *Config) (*Result, error) {
	rows, err := readExcel(filename)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Summary: Summary{Total: len(rows)},
	}

	// 第一遍：统计收件人相同的订单组（买家昵称+收件人姓名+手机号+地址都相同）
	recipientCount := make(map[string]int)
	for i := range rows {
		key := recipientKey(&rows[i])
		if key != "||||" {
			recipientCount[key]++
		}
	}

	for _, row := range rows {
		if isDoubtful(&row, cfg) {
			result.DoubtfulOrders = append(result.DoubtfulOrders, row)
		} else if isMultiOrder(&row) || isRecipientMulti(&row, recipientCount) {
			result.MultiOrders = append(result.MultiOrders, row)
		} else if isAccessoryOnly(&row, cfg) {
			result.AccessoryRows = append(result.AccessoryRows, row)
		} else {
			result.NormalOrders = append(result.NormalOrders, row)
		}
	}

	// 排序
	// 排序：先按原有逻辑，再按付款时间
	sort.Slice(result.MultiOrders, func(i, j int) bool {
		oi, oj := result.MultiOrders[i].OrderID, result.MultiOrders[j].OrderID
		if oi != oj {
			return oi < oj
		}
		return result.MultiOrders[i].PaymentTime < result.MultiOrders[j].PaymentTime
	})
	sort.Slice(result.DoubtfulOrders, func(i, j int) bool {
		ci, cj := result.DoubtfulOrders[i].Code, result.DoubtfulOrders[j].Code
		if ci != cj {
			return ci < cj
		}
		return result.DoubtfulOrders[i].PaymentTime < result.DoubtfulOrders[j].PaymentTime
	})
	sort.Slice(result.NormalOrders, func(i, j int) bool {
		ci, cj := result.NormalOrders[i].Code, result.NormalOrders[j].Code
		if ci != cj {
			return ci < cj
		}
		return result.NormalOrders[i].PaymentTime < result.NormalOrders[j].PaymentTime
	})
	sort.Slice(result.AccessoryRows, func(i, j int) bool {
		ci, cj := result.AccessoryRows[i].Code, result.AccessoryRows[j].Code
		if ci != cj {
			return ci < cj
		}
		return result.AccessoryRows[i].PaymentTime < result.AccessoryRows[j].PaymentTime
	})

	result.Summary.MultiOrders = len(result.MultiOrders)
	result.Summary.DoubtfulOrders = len(result.DoubtfulOrders)
	result.Summary.NormalOrders = len(result.NormalOrders)
	result.Summary.AccessoryRows = len(result.AccessoryRows)

	// 输出到 _output/ 目录
	absPath, _ := filepath.Abs(filename)
	excelDir := filepath.Dir(absPath)
	excelName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	outputDir := filepath.Join(excelDir, excelName+"_output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}
	result.OutputDir = outputDir

	if err := writeOutput(filepath.Join(outputDir, "筛选结果.xlsx"), result); err != nil {
		return nil, err
	}

	return result, nil
}

// ---- Excel 读写 ----

func readExcel(filename string) ([]RowData, error) {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetList()[0])
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("数据行不足")
	}

	mapper := newFieldMapper(&RowData{})
	for colIdx, header := range rows[0] {
		for tag, fieldIdx := range mapper.headerToFieldIdx {
			if strings.EqualFold(strings.TrimSpace(header), tag) {
				mapper.colToFieldIdx[colIdx] = fieldIdx
				break
			}
		}
	}

	var result []RowData
	for i := 1; i < len(rows); i++ {
		row := scanRow(rows[i], mapper)
		if row.Code == "" {
			row.Code = "未知"
		}
		result = append(result, *row)
	}
	return result, nil
}

func writeOutput(filename string, result *Result) error {
	f := excelize.NewFile()
	defer f.Close()

	f.SetSheetName("Sheet1", "多件订单")
	writeSheet(f, "多件订单", result.MultiOrders)

	createSheet(f, "疑难单", result.DoubtfulOrders)
	createSheet(f, "单独配件", result.AccessoryRows)
	createSheetWithGrouping(f, "正常手机壳", result.NormalOrders)

	return f.SaveAs(filename)
}

func writeSheet(f *excelize.File, name string, rows []RowData) {
	headers := []string{"店铺名称", "订单编号", "子订单编号", "买家昵称", "收件人姓名", "收件人手机号", "收件人详细地址", "付款时间", "买家留言", "卖家备注", "商品商家编码", "商品规格", "商品数量"}
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(name, cell, h)
	}

	for rowIdx, row := range rows {
		values := []string{
			row.ShopName, row.OrderID, row.SubOrderID, row.BuyerNick, row.ReceiverName, row.ReceiverPhone, row.ReceiverAddress,
			row.PaymentTime, row.BuyerMsg, row.SellerNote, row.Code, row.Spec,
			fmt.Sprintf("%d", row.Quantity),
		}
		for colIdx, v := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(name, cell, v)
		}
	}
}

func createSheet(f *excelize.File, name string, rows []RowData) {
	f.NewSheet(name)
	writeSheet(f, name, rows)
}

func createSheetWithGrouping(f *excelize.File, name string, rows []RowData) {
	f.NewSheet(name)

	headers := []string{"店铺名称", "订单编号", "子订单编号", "买家昵称", "收件人姓名", "收件人手机号", "收件人详细地址", "付款时间", "买家留言", "卖家备注", "商品商家编码", "商品规格", "商品数量"}
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(name, cell, h)
	}

	rowIdx := 2
	var lastCode string
	for _, row := range rows {
		if lastCode != "" && row.Code != lastCode {
			rowIdx++ // 编码变化时空行隔开
		}
		lastCode = row.Code

		values := []string{
			row.ShopName, row.OrderID, row.SubOrderID, row.BuyerNick, row.ReceiverName, row.ReceiverPhone, row.ReceiverAddress,
			row.PaymentTime, row.BuyerMsg, row.SellerNote, row.Code, row.Spec,
			fmt.Sprintf("%d", row.Quantity),
		}
		for colIdx, v := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
			f.SetCellValue(name, cell, v)
		}
		rowIdx++
	}
}
