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
    │   └─→ [2] peijian ──→ 配件分配.xlsx
    │         (需要配件编码.xlsx)
    │
    └─→ [可选] dangkou <筛选结果.xlsx> ──→ 档口分配.xlsx
```

## Commands

### 1. filter — 订单筛选

**Purpose**: Classify orders into 4 categories.

```bash
phonecase-tools filter <Excel文件路径>
```

**Input**: 订单 Excel file. Required columns: `店铺名称`, `订单编号`, `子订单编号`, `付款时间`, `买家留言`, `卖家备注`, `商品商家编码`, `商品规格`, `商品数量`

**Output**: `<原文件名>_output/筛选结果.xlsx` with 4 sheets:
| Sheet | 内容 |
|-------|------|
| 多件订单 | SubOrderID ≠ OrderID，按 OrderID 排序，同订单按付款时间排序 |
| 疑难单 | 有买家留言/卖家备注/含疑难关键词，按编码再按付款时间排序 |
| 单独配件 | 规格含配件关键词且不含 `+`，按编码再按付款时间排序 |
| 正常手机壳 | 其余订单，按编码分组（不同编码间空行隔开），同编码按付款时间排序 |

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

**Output**: `<原文件名>_output/档口分配.xlsx`:
| Sheet | 内容 |
|-------|------|
| 汇总 | 列头=档口名，每列下方=订单编号列表（第一个sheet） |
| 档口名 (多个) | 匹配到该档口的完整订单 |
| 未分配档口 | 有自设编码但无匹配档口的订单 |
| 无匹配自设编码 | 商品ID+SKU名称找不到对应自设编码的订单 |

**Configuration**: `dangkou_config.json` — stores path to 自设编码.xlsx
```json
{"path": "/path/to/自设编码.xlsx"}
```

### 3. peijian — 配件提取

**Purpose**: Extract accessories from orders and assign them to stalls based on `配件编码.xlsx`.

```bash
phonecase-tools peijian <订单Excel文件> [配件编码.xlsx]
```

**Input**: 
- Order Excel (must have `商品id` and `商品规格` columns)
- 配件编码.xlsx — accessory-to-stall config file (optional if already configured via GUI)

**配件编码 Excel 格式**:
- Sheet 1 (`支架-自设编码`): `商品ID | SKU名称 | 编码1 | 编码2 | ... | 编码5` — SKU到自设编码的多对多映射
  - `+` 前为手机壳（忽略），`+` 后每段为一个配件，按位置对应编码列
  - 无 `+` 时整个 SKU 为配件名，对应编码1
- Sheet 2 (`档口分配`): 列式布局 — Row 0 为档口名，下方行为该档口的自设编码

**Output**: `<原文件名>_output/配件分配.xlsx`:
| Sheet | 内容 |
|-------|------|
| 汇总 | 列头=档口名，每列下方=`配件名称 x数量` 按数量降序（第一个sheet） |
| 档口名 (多个) | 分配至该档口的配件详情（店铺名称/订单编号/商品ID/商品规格/配件名称/商品数量） |
| 未分配档口 | 有自设编码但不在任何档口的订单 |
| 无匹配自设编码 | SKU未匹配到自设编码的订单 |

**Configuration**: `peijian_config.json` — stores path to 配件编码.xlsx
```json
{"path": "/path/to/配件编码.xlsx"}
```

## Workflow Patterns

### Pattern A: Filter → Dangkou (Normal Phone Cases)
```bash
# Step 1: Filter orders
./build/bin/phonecase-tools filter data/xxx.xlsx

# Step 2: Assign stalls (takes largest sheet "正常手机壳")
./build/bin/phonecase-tools dangkou data/xxx_output/筛选结果.xlsx data/自设编码.xlsx
```

### Pattern B: Filter → Peijian (Accessory Extraction)
```bash
# Step 1: Filter
./build/bin/phonecase-tools filter data/xxx.xlsx

# Step 2: Extract accessories and assign to stalls
./build/bin/phonecase-tools peijian data/xxx_output/筛选结果.xlsx data/配件编码.xlsx
```

### Pattern C: Direct Processing (without filter)
```bash
# Direct dangkou
./build/bin/phonecase-tools dangkou data/xxx.xlsx data/自设编码.xlsx

# Direct peijian
./build/bin/phonecase-tools peijian data/xxx.xlsx data/配件编码.xlsx
```

## Configuration Management

Config files live alongside the binary in `build/bin/`:
```
build/bin/
├── phonecase-tools          # binary
├── keywords.json            # filter config
├── dangkou_config.json      # path to 自设编码.xlsx
├── peijian_config.json      # path to 配件编码.xlsx
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

# Edit dangkou config
cat > build/bin/dangkou_config.json << 'EOF'
{"path": "/absolute/path/to/自设编码.xlsx"}
EOF

# Edit peijian config
cat > build/bin/peijian_config.json << 'EOF'
{"path": "/absolute/path/to/配件编码.xlsx"}
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
- `打开配置文件失败` — 自设编码.xlsx / 配件编码.xlsx not found or corrupted
- `未指定配置文件路径` — Config file path not set, configure it via GUI first
- `打开订单文件失败` — input Excel not found or not a valid xlsx

## Testing the CLI

```bash
# Build
make linux

# Test with sample data
./build/bin/phonecase-tools filter "data/发货单20260605150646共计11条.xlsx"

# Check output
ls data/发货单20260605150646共计11条_output/

# Test dangkou
./build/bin/phonecase-tools dangkou \
  "data/发货单20260605150646共计11条.xlsx" \
  "data/自设编码(完善中）(1).xlsx"

# Test peijian
./build/bin/phonecase-tools peijian \
  "data/发货单20260620094852共计237条.xlsx" \
  "data/配件编码测试.xlsx"
```

## Important Rules

1. **Always use absolute paths** when invoking the CLI — relative paths may not resolve correctly depending on working directory
2. **Filter before dangkou/peijian**: The filter step separates orders, making downstream processing cleaner
3. **Output directories auto-create**: `_output` directories are created alongside the input file, named `<input_name>_output/`
4. **Config files store paths**: `dangkou_config.json` and `peijian_config.json` store the path to the respective Excel config files. Use the GUI gear buttons to select them
5. **Never run as bare `phonecase-tools` without args** — this launches the Wails GUI desktop app, not the CLI
6. **Use `make linux` to recompile**: If you modify source code, rebuild with `make linux` (or `make windows`) before testing CLI changes
