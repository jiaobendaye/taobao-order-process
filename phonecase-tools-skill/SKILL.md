---
name: phonecase-tools
description: Use phonecase-tools CLI to process Taobao phone case order Excel files — filter, dangkou assignment, and accessory extraction workflows.
---

# phonecase-tools CLI Usage

## Overview

`phonecase-tools` processes 淘宝手机壳订单 Excel files through a pipeline of CLI commands. The binary is at `build/bin/phonecase-tools` (Linux) or `build/bin/phonecase-tools.exe` (Windows).

**Absolute path**: `/home/jiaobendaye/lab/taobao/order-process/build/bin/phonecase-tools`

## Processing Pipeline

A complete order processing workflow follows this sequence:

```
原始订单.xlsx
    │
    ├─→ [1] filter ───→ 筛选结果.xlsx (4 sheets: 多件订单/疑难单/正常手机壳/单独配件)
    │                        │
    │   ┌────────────────────┼────────────────────┐
    │   ↓                    ↓                    ↓
    │ 正常手机壳          疑难单+配件         多件订单
    │   │                                        (人工处理)
    │   ├─→ [2] dangkou ──→ 档口分配.xlsx
    │   │   (需要自设编码.xlsx)
    │   │
    │   └─→ [2] peijian extract ──→ pending.xlsx
    │             │                    (简单订单/待处理/无配件)
    │             │
    │             └─→ [3] peijian merge ──→ result.xlsx
    │                    (汇总配件统计)
    │
    └─→ [可选] dangkou <筛选结果.xlsx> ──→ 档口分配.xlsx
```

## Commands

### 1. filter — 订单筛选

**Purpose**: Classify orders into 4 categories.

```bash
phonecase-tools filter <Excel文件路径>
```

**Input**: 订单 Excel file. Required columns: `店铺名称`, `订单编号`, `子订单编号`, `快递单号`, `买家留言`, `卖家备注`, `商品商家编码`, `商品规格`, `商品数量`

**Output**: `<原文件名>_output/筛选结果.xlsx` with 4 sheets:
| Sheet | 内容 |
|-------|------|
| 多件订单 | SubOrderID ≠ OrderID，按 OrderID 排序 |
| 疑难单 | 有买家留言/卖家备注/含疑难关键词，按编码排序 |
| 单独配件 | 规格含配件关键词且不含 `+`，按编码排序 |
| 正常手机壳 | 其余订单，按编码分组（不同编码间空行隔开） |

**CLI output example**:
```
已生成 /path/to/xxx_output/筛选结果.xlsx
  多件订单: 5条
  疑难单: 3条
  正常手机壳: 20条
  单独配件: 2条
  总计: 30条
```

**Configuration**: `keywords.json` — filter keywords configuration
```json
{
  "doubtKeywords": ["其他", "咨询客服", "备注", "diy"],
  "accessoryKeywords": ["支架", "绳", "链", "吸盘", "串珠", "相机", "纽扣", "腕带", "贴纸", "卡包"]
}
```
Edit `build/bin/keywords.json` directly, or use GUI 齿轮按钮.

### 2. dangkou — 档口分配

**Purpose**: Match orders to stalls (档口) based on `自设编码.xlsx` configuration.

```bash
phonecase-tools dangkou <订单Excel文件> [自设编码.xlsx路径]
```

**Input**: 
- Order Excel (must have `商品ID` and `商品规格` columns)
- 自设编码.xlsx — stall configuration file (optional if already configured via GUI)

**自设编码 Excel 格式**:
- Sheet 1: `商品ID | SKU名称 | 自设编码` 三列映射表
- Sheet 2+: 每列头为一个自设编码，下方行是该编码支持的手机型号（空格会被去除）

**Output**: `<原文件名>_output/档口分配.xlsx` — each stall as a sheet, plus:
| Sheet | 内容 |
|-------|------|
| 档口名 (多个) | 匹配到该档口的订单 |
| 未分配档口 | 有自设编码但无匹配档口的订单 |
| 无匹配自设编码 | 商品ID+SKU名称找不到对应自设编码的订单 |

**Configuration**: `dangkou_config.json` — stores path to 自设编码.xlsx
```json
{"path": "/path/to/自设编码.xlsx"}
```

### 3. peijian extract — 配件提取

**Purpose**: Extract accessory information from order specifications.

```bash
phonecase-tools peijian extract <Excel文件>
```

**Input**: Order Excel. Required columns (configurable via `columns.json`): `商品规格`, `买家留言`, `卖家备注`, `商品数量`

**Output**: `<原文件名>_output/pending.xlsx` with 3 sheets:
| Sheet | 内容 |
|-------|------| | 简单订单 | 含`+`且无备注 → 已自动提取配件（配件名称/颜色/数量列） |
| 待处理订单 | 可能有配件但需人工确认（有备注、含DIY/搭配等） |
| 无配件订单 | 无配件信息的订单 |

Multiple accessories in one order → multiple rows (blue highlight for the group).

**Configuration**: 
- `parts.json` — accessory name list (suffix matching):
```json
{
  "accessories": ["旋转折叠支架", "泡泡软胶支架", "推拉支架", "软胶吸盘", "吸盘", "撞色串珠", "串珠", "编织手提绳", "腕带", "爱心镜", "镜"]
}
```
- `columns.json` — column name mapping:
```json
{"spec": "商品规格", "buyerMsg": "买家留言", "sellerNote": "卖家备注", "quantity": "商品数量"}
```

### 4. peijian merge — 配件汇总

**Purpose**: Aggregate accessory counts from `pending.xlsx` (after human review).

```bash
phonecase-tools peijian merge <pending.xlsx路径>
```

**Input**: `pending.xlsx` (output of `peijian extract`). Reads all sheets, looking for columns: `配件名称 | 颜色 | 配件数量`

**Output**: Same directory's `result.xlsx` — single "配件汇总" sheet:
```
配件名称 | 颜色 | 数量
支架     | 黑色 | 12
吸盘     | 无   | 8
...
```
Entries sorted by quantity descending.

**CLI output example**:
```
配件统计汇总
──────────────────────────────────────────────────
旋转折叠支架              无               12
软胶吸盘                  无               8
撞色串珠                  无               5
──────────────────────────────────────────────────
共 3 种配件，总计 25 件

已生成 /path/to/result.xlsx
```

## Workflow Patterns

### Pattern A: Filter → Dangkou (Normal Phone Cases)
```bash
# Step 1: Filter orders
./build/bin/phonecase-tools filter data/xxx.xlsx

# Step 2: Assign stalls (takes largest sheet "正常手机壳")
./build/bin/phonecase-tools dangkou data/xxx_output/筛选结果.xlsx data/自设编码.xlsx
```

### Pattern B: Filter → Peijian (Accessory Handling)
```bash
# Step 1: Filter
./build/bin/phonecase-tools filter data/xxx.xlsx

# Step 2: Extract accessories
./build/bin/phonecase-tools peijian extract data/xxx_output/筛选结果.xlsx

# Step 3: HUMAN REVIEW — edit pending.xlsx if needed

# Step 4: Merge stats
./build/bin/phonecase-tools peijian merge data/xxx_output/pending.xlsx
```

### Pattern C: Direct Dangkou (without filter)
```bash
# For orders that have clear product IDs and specs
./build/bin/phonecase-tools dangkou data/xxx.xlsx data/自设编码.xlsx
```

## Configuration Management

Config files live alongside the binary in `build/bin/`:
```
build/bin/
├── phonecase-tools          # binary
├── keywords.json            # filter config
├── parts.json               # peijian accessory list
├── columns.json             # peijian column mapping
├── dangkou_config.json      # path to 自设编码.xlsx
└── phonecase-tools.log      # runtime log
```

To update configs programmatically:
```bash
# Edit keywords (filter)
cat > build/bin/keywords.json << 'EOF'
{
  "doubtKeywords": ["其他", "diy", "特殊"],
  "accessoryKeywords": ["支架", "绳", "链", "吸盘"]
}
EOF

# Edit parts (peijian)
cat > build/bin/parts.json << 'EOF'
{
  "accessories": ["支架", "吸盘", "串珠", "腕带"]
}
EOF

# Edit dangkou config
cat > build/bin/dangkou_config.json << 'EOF'
{"path": "/absolute/path/to/自设编码.xlsx"}
EOF
```

## Error Handling

| Exit Code | Meaning |
|-----------|---------|
| 0 | Success |
| 1 | Error (see stderr) |

**Common errors**:
- `数据行不足` — Excel has only header row or is empty
- `未找到「xxx」列` — Required column missing from Excel. Check headers match expected names
- `打开配置文件失败` — 自设编码.xlsx not found or corrupted
- `未指定配置文件路径` — dangkou needs 自设编码.xlsx, configure it first
- `打开订单文件失败` — input Excel not found or not a valid xlsx

## Testing the CLI

```bash
# Test with sample data
./build/bin/phonecase-tools filter "data/发货单20260605150646共计11条.xlsx"

# Check output
ls data/发货单20260605150646共计11条_output/

# Test dangkou
./build/bin/phonecase-tools dangkou \
  "data/发货单20260605150646共计11条.xlsx" \
  "data/自设编码(完善中）(1).xlsx"
```

## Important Rules

1. **Always use absolute paths** when invoking the CLI — relative paths may not resolve correctly depending on working directory
2. **Filter before dangkou/peijian**: The filter step separates orders, making downstream processing cleaner. Dangkou is typically used on "正常手机壳" sheet but can also take any filter output
3. **Don't skip human review in peijian**: The `extract` → `merge` pipeline is designed for human review of the pending sheet before merge
4. **Output directories auto-create**: `_output` directories are created alongside the input file, named `<input_name>_output/`
5. **Config files are JSON**: Always write valid JSON. The tool reads configs from `build/bin/` relative to the binary, or from CWD as fallback
6. **Never run as bare `phonecase-tools` without args** — this launches the Wails GUI desktop app, not the CLI
7. **Use `go build` to recompile**: If you modify source code, rebuild with `make linux` or `make windows` before testing CLI changes
