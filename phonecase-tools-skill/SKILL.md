---
name: phonecase-tools
description: Use the phonecase-tools CLI to process Taobao phone case order Excel files — filter classification, stall (档口) assignment, accessory extraction, and leather-shell aggregation workflows.
---

# phonecase-tools CLI Usage

## Overview

`phonecase-tools` 是一款基于 Wails 的桌面 + 命令行二合一工具，用于处理淘宝手机壳订单 Excel。

- **二进制路径（Linux）**：`/home/jiaobendaye/lab/taobao/order-process/build/bin/phonecase-tools`
- **二进制路径（Windows）**：`build/bin/phonecase-tools.exe`
- **无参数启动** → 桌面 GUI 模式（Wails WebView）
- **带子命令启动** → CLI 模式

当前 CLI 提供 **四个** 子命令：

| 子命令 | 作用 |
|--------|------|
| `filter`  | 把订单 Excel 按规则分为 4 类 |
| `dangkou`| 按自设编码把订单分配到各档口 |
| `peijian`| 从订单中提取配件并分配到档口 |
| `pizhi`  | 皮质壳按 (商品ID, SKU, 型号) 聚合并附带图片输出 |

> ⚠ **不要** 不带参数运行二进制 — 这会启动 GUI 桌面应用而不是 CLI。

---

## 共用约定

- 所有子命令都**绝对路径优先**调用，CLI 内部使用 `filepath.Abs` 解析。
- 输出目录默认在输入 Excel 同级目录下创建 `<输入文件名>_output/`。
- 配置文件统一存放在**可执行文件同目录**（`build/bin/`）：
  ```
  build/bin/
  ├── phonecase-tools              # 二进制
  ├── keywords.json                # filter 关键词
  ├── dangkou_config.json          # 自设编码文件路径
  ├── peijian_config.json          # 配件编码文件路径
  ├── pizhi_config.json            # 皮质壳配置文件路径
  └── phonecase-tools.log          # 运行日志
  ```
- 任何 `<订单 Excel>` 必须**至少包含表头 + 1 行数据**，否则返回「数据行不足」错误。
- 列名匹配**大小写不敏感**，多余空格会被自动忽略。
- 商品ID 通过 `excelize.GetCellValue` 读取（避免大数字的科学计数法精度问题）。

---

## 1. `filter` — 订单筛选

**作用**：把订单 Excel 按规则分到 4 个 Sheet：多件订单 / 疑难单 / 正常手机壳 / 单独配件。

### 用法

```bash
phonecase-tools filter <订单Excel文件>
# 例
./build/bin/phonecase-tools filter /abs/path/to/发货单.xlsx
```

### 输入

订单 Excel 必需列（表头大小写不敏感）：

| 列名 | 说明 |
|------|------|
| `店铺名称` | |
| `订单编号` | |
| `子订单编号` | |
| `买家昵称` | 用于判断「同收件人多件」 |
| `收件人姓名` | |
| `收件人手机号` | |
| `收件人详细地址` | |
| `付款时间` | 排序键 |
| `买家留言` | |
| `卖家备注` | |
| `商品商家编码` | 排序/分组键 |
| `商品规格` | |
| `商品数量` | |

### 分类规则（**优先级从高到低**）

1. **多件订单** — 满足任一：
   - `子订单编号 ≠ ""` 且 `订单编号 ≠ ""` 且 `子订单编号 ≠ 订单编号`
   - 同一收件人复合键 (`买家昵称|收件人姓名|手机号|地址`) 出现 ≥ 2 次
2. **疑难单** — 满足任一：
   - `买家留言 ≠ ""` 或 `卖家备注 ≠ ""`
   - `商品规格` 含 `doubtKeywords` 任一关键词（不区分大小写）
3. **单独配件** — 规格**不含 `+`** 且含 `accessoryKeywords` 任一关键词
4. **正常手机壳** — 其余订单

### 输出

`/abs/path/to/发货单_output/筛选结果.xlsx`，包含 4 个 Sheet：

| Sheet | 排序 |
|-------|------|
| `多件订单` | 按 `订单编号` 分组，组内按 `付款时间` |
| `疑难单` | 按 `商品商家编码` 分组，组内按 `付款时间` |
| `单独配件` | 按 `商品商家编码` 分组，组内按 `付款时间` |
| `正常手机壳` | 按 `商品商家编码` 分组（不同编码间空行隔开），组内按 `付款时间` |

### CLI 输出示例

```
已生成 /abs/path/to/发货单_output/筛选结果.xlsx
  多件订单: 5条
  疑难单: 3条
  正常手机壳: 20条
  单独配件: 2条
  总计: 30条
```

### 配置：`keywords.json`

存放位置：`build/bin/keywords.json`（也可放当前目录）。**整套替换**而非追加。

```json
{
  "doubtKeywords": ["其他", "咨询客服", "备注", "diy"],
  "accessoryKeywords": ["支架", "绳", "链", "吸盘", "串珠", "相机", "纽扣", "腕带", "贴纸", "卡包"]
}
```

调整方式：
- **CLI**：直接编辑 `build/bin/keywords.json`
- **GUI**：点击「订单筛选」旁的 ⚙ 齿轮按钮可视化编辑

默认值见 `internal/filter/filter.go` 的 `DefaultConfig()`。

---

## 2. `dangkou` — 档口分配

**作用**：把订单按 `自设编码.xlsx` 配置分配到对应档口。

### 用法

```bash
phonecase-tools dangkou <订单Excel文件> [自设编码.xlsx]
# 例
./build/bin/phonecase-tools dangkou /abs/订单.xlsx /abs/自设编码.xlsx

# 若 dangkou_config.json 已存路径，则第二个参数可省略
./build/bin/phonecase-tools dangkou /abs/订单.xlsx
```

### 自设编码 Excel 格式

- **Sheet 1**：自设编码映射表（三列）
  | 商品ID | SKU名称 | 自设编码 |
  |--------|---------|-----------|
  | xxx    | skuA    | ZS001     |

- **Sheet 2+**（每 Sheet 一个档口，**Sheet 名 = 档口名**，优先级按 Sheet 顺序）：
  - 第 1 行：列头为多个自设编码（横向排列）
  - 第 2 行起：每列是该自设编码对应的**手机型号列表**
  - 型号会去除所有空格、不可见字符（BOM/ZWSP 等）

### 订单必需列

`商品id`、`商品规格`、`订单编号`。

商品规格格式：`{手机型号}|{SKU名称}[{颜色变种}]` — 例：`iPhone 15 Pro|透明壳-星空[黑]`
解析后：型号 = `iPhone15Pro`，SKU = `透明壳-星空`（去掉 `[黑]` 后缀）。

### 匹配流程

```
订单行
  → 商品ID+SKU名称  →  查映射表 → 自设编码
  → 规格解析        →  型号 + SKU
  → (自设编码, 型号) → 按档口优先级找匹配的 Sheet → 档口名
```

匹配到第一个档口即停止（按 Sheet 顺序）。不匹配会落入以下兜底分类：
- **无匹配自设编码**：商品ID+SKU 找不到对应自设编码
- **未分配档口**：找到了自设编码但所有档口都没有对应型号

### 输出

`/abs/path/to/订单_output/档口分配.xlsx`

| Sheet | 内容 |
|-------|------|
| `汇总`（第一个） | 列头=档口名，每列下方纵向列出该档口的**订单编号** |
| `<档口名>`（多个） | 该档口的完整订单明细（保留所有原始列） |
| `未分配档口` | 有自设编码但找不到对应档口的订单 |
| `无匹配自设编码` | 商品ID+SKU 在映射表中找不到 |

### 配置：`dangkou_config.json`

存放路径配置：

```json
{ "path": "/abs/path/to/自设编码.xlsx" }
```

调整方式：
- **CLI**：`echo '{"path": "/abs/path/to/自设编码.xlsx"}' > build/bin/dangkou_config.json`
- **GUI**：点击「档口分配」旁的 ⚙ → 弹出文件选择对话框，**会自动校验格式并保存**

> 注意：`LoadConfigPath` 会验证路径是否存在，所以路径变更后请同步更新配置。

---

## 3. `peijian` — 配件提取与分配

**作用**：从订单规格中提取配件，按 `配件编码.xlsx` 分配到各档口，同时按配件名 + 数量聚合并降序输出。

### 用法

```bash
phonecase-tools peijian <订单Excel文件> [配件编码.xlsx]
# 例
./build/bin/phonecase-tools peijian /abs/订单.xlsx /abs/配件编码.xlsx
```

> 注意：CLI 只有 `peijian` 一个子命令，没有 `peijian extract` / `peijian merge`。

### 配件编码 Excel 格式

- **Sheet 1**（自设编码映射）：
  | 商品ID | SKU名称 | 编码1 | 编码2 | 编码3 | 编码4 | 编码5 |
  |--------|---------|-------|-------|-------|-------|-------|
  | xxx    | 壳+支架+腕带 | ZS001 | ZS002 | ZS003 |       |       |

  - SKU 用 `+` 分隔：`+` 前为手机壳（忽略），`+` 后每段是 1 个配件
  - 无 `+` 时整段 SKU 视为 1 个配件
  - 列 `编码N` 的**数量必须 = 配件段数**，不一致会直接报错

- **Sheet 2+**（每 Sheet 一个档口，列式布局）：
  - 第 1 行：列头 = 档口名
  - 第 2 行起：每列是该档口的**自设编码列表**

### 订单必需列

`商品id`、`商品规格`、`商品数量`、`店铺名称`、`订单编号`。

### 配件提取规则

1. 拆 `商品规格` → `(型号, SKU名称)`
2. 用 `商品ID+SKU名称` 在 Sheet 1 查 → 得到 `[自设编码]` 列表
3. 拆 SKU 用 `+` → 得到 `配件名` 列表
4. 配件 i 用 `编码i` 在 Sheet 2+ 找档口
5. 按 (档口, 配件名) 聚合数量（**汇总 sheet 按数量降序**）

无匹配/未分配：
- `无匹配自设编码`：SKU 在映射表中找不到
- `未分配档口`：找到了编码但不在任何档口

### 输出

`/abs/path/to/订单_output/配件分配.xlsx`

| Sheet | 内容 |
|-------|------|
| `汇总`（第一个） | 列头=档口名（只显示有数据的档口），下方为 `配件名称 x数量`（按数量降序） |
| `<档口名>`（多个） | 配件明细。表头 = `[店铺名称, 订单编号, 商品id, 商品规格, 商品数量, 配件名称]` |
| `未分配档口` | 保留原始列 |
| `无匹配自设编码` | 保留原始列 |

### 配置：`peijian_config.json`

```json
{ "path": "/abs/path/to/配件编码.xlsx" }
```

调整方式同 `dangkou_config.json`。

---

## 4. `pizhi` — 皮质壳档口分配（NEW）

**作用**：把皮质壳订单按 `(商品ID, SKU名称, 手机型号)` **聚合数量**后分配到档口，附带从配置表读取的**图片**输出。

### 用法

```bash
phonecase-tools pizhi <订单Excel文件> [皮质壳配置表.xlsx]
# 例
./build/bin/phonecase-tools pizhi /abs/订单.xlsx /abs/皮质壳配置表.xlsx
```

### 皮质壳配置表 Excel 格式

- **每 Sheet = 一个档口**（Sheet 名 = 档口名，按 Sheet 顺序定优先级）
- 表头：`| 商品ID | sku名称 | (图片) |`
  - 商品ID 用 `GetCellValue` 读取避免科学计数法
  - **图片嵌入在 C 列**（自动检测范围 C~H 列）
- 跳过名字以 `WpsReserved` 开头的内部 sheet

### 订单必需列

`商品id`、`商品规格`、`商品数量`。

### 处理流程

```
订单行
  → 商品ID + (从规格解析出的 SKU)
  → 查 Engine.Items[(商品ID|SKU)] → 档口 + 图片
  → 按 (档口, 商品ID, SKU, 型号) 聚合数量
  → 写出对应 Sheet：[型号, 数量, 图片]
```

未匹配 `(商品ID, SKU)` 的订单统一进入 **`未匹配订单`**（不写入输出文件，保留在 `Result.Unmatched`）。

### 输出

`/abs/path/to/订单_output/皮质壳分配.xlsx`

- 每个档口一个 Sheet，**只输出有数据的档口**
- 表头：`型号 | 数量 | 图片`
- 自动等比缩放图片到目标宽度 ~100 px（按列宽/行高调整）
- 第一列宽 25，第二列宽 10，第三列按最大图片尺寸调整

### CLI 输出示例

```
已生成 /abs/path/to/订单_output/皮质壳分配.xlsx
  总订单: 50条
  档口A: 12 个型号
  档口B: 8 个型号
```

### 配置：`pizhi_config.json`

```json
{ "path": "/abs/path/to/皮质壳配置表.xlsx" }
```

调整方式同 `dangkou_config.json` / `peijian_config.json`。

---

## 独立调用

四个子命令**完全独立**，没有强制顺序或依赖关系。每个命令接收一份订单 Excel +（可选的）配置 Excel，输出一份结果 Excel 到 `<输入名>_output/`。

输入的订单 Excel 来源不限 — 既可以是淘宝导出的原始订单，也可以是 `filter` 输出的 `筛选结果.xlsx` 的某一个 Sheet 单独跑下游，也可以是别的工具处理过的 Excel。

### 调用示例（互相独立，按需选用）

```bash
# 筛选 —— 把订单按 4 类分到不同 Sheet
./build/bin/phonecase-tools filter /abs/原始订单.xlsx

# 档口分配 —— 需要「自设编码」配置
./build/bin/phonecase-tools dangkou /abs/订单.xlsx /abs/自设编码.xlsx

# 配件提取 —— 需要「配件编码」配置
./build/bin/phonecase-tools peijian /abs/订单.xlsx /abs/配件编码.xlsx

# 皮质壳分配 —— 需要「皮质壳配置表」配置
./build/bin/phonecase-tools pizhi  /abs/订单.xlsx /abs/皮质壳配置表.xlsx
```

每个 `<订单.xlsx>` 可以是任意一个订单文件，没有「必须先 filter 才能 dangkou」这种前置关系。

---

## 错误处理

### 退出码

| 退出码 | 含义 |
|--------|------|
| 0 | 成功 |
| 1 | 异常（详情在 stderr） |

### 常见错误及排查

| 错误信息 | 原因 / 修复 |
|----------|-------------|
| `数据行不足` | Excel 只有表头或为空 |
| `未找到「商品ID」列` | 表头缺必需列，检查列名拼写 |
| `未找到任何「编码N」列` | 配件编码配置 Sheet 1 缺 `编码1`/`编码2` 之类列 |
| `打开配置文件失败` | 自设编码/配件编码/皮质壳配置表打不开或损坏 |
| `未指定配置文件路径` / `配置未找到` | 配置文件路径未设置，先用 GUI ⚙ 选择或 CLI 第二个参数传入 |
| `打开订单文件失败` | 输入 Excel 路径错或非 .xlsx |
| `第N行 SKU「xx」有 M 个配件，但编码列有 K 个编码，数量不一致` | 配件编码列数与 `+` 段数不匹配 |

---

## 开发与构建

```bash
# 编译
make linux       # Linux
make windows     # Windows
make dev         # Wails 开发模式（热重载 GUI）

# 测试
go test ./...                          # 全部
go test ./internal/dangkou/ -v         # 档口
go test ./internal/peijian/ -v         # 配件
go test ./internal/pizhi/   -v         # 皮质壳
```

修改源码后**必须重新编译**才能用 CLI 测试新版。

---

## 重要规则

1. **始终用绝对路径**调用 CLI，避免工作目录变化导致找不到输入文件。
2. **四个子命令互相独立**：`filter` / `dangkou` / `peijian` / `pizhi` 没有前后依赖关系，输入文件来源不限。
3. **配置文件存路径而非内容**：`dangkou_config.json` / `peijian_config.json` / `pizhi_config.json` 只存路径，具体配置从那个 Excel 文件读取。
4. **`peijian` 对数量严格匹配**：配件段数 ≠ 编码列数会直接报错 — 修改 `配件编码.xlsx` 时务必保持一致。
5. **`pizhi` 的图片读取**：配置表 C 列必须真有嵌入图片（不能只是文字），否则该行的图片列会空白。
6. **不要不带参数运行二进制**，否则会启动 GUI 桌面应用而不是 CLI。
7. **GUI 中通过 Wails 绑定暴露的后端方法**（`window.go.main.App.*`）：`RunFilter` / `RunDangkou` / `RunPeijianProcess` / `RunPizhiProcess` / `GetXxxConfigPath` / `SaveXxxConfigPath` / `SelectXxxConfigFile` 等。
