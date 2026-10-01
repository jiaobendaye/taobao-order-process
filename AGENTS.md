# 手机壳订单处理工具 (Phone Case Order Processing Tool)

## 项目概述

基于 Wails v2 的桌面应用 + CLI 工具 + 浏览器单 HTML 版（Go → Wasm），用于处理淘宝手机壳订单的 Excel 文件。支持五大功能：
1. **订单筛选** (`filter`) — 将订单按多件/疑难/正常/配件分类
2. **档口分配** (`dangkou`) — 按自设编码将订单分配给不同档口，并生成拿货档口表
3. **配件提取** (`peijian`) — 从订单规格中提取配件并按自设编码分配到档口
4. **皮质壳分配** (`pizhi`) — 按 (商品ID, SKU名称) 将皮质壳订单分配到档口，输出带图片
5. **打图分配** (`datu`) — 按商品ID 将订单分配到打图工厂，解析 `【素材-编码】`

## 技术栈

- **后端**: Go（`go.mod` 声明 `go 1.26.3`），Wails v2.12.0
- **桌面前端**: 原生 HTML/CSS/JS（无框架），`frontend/` 目录
- **浏览器版**: `wasm/`（Go → Wasm 桥接）+ `web/`（原生 JS，SheetJS 读写 Excel），打包为单 HTML
- **Excel 处理**: `github.com/xuri/excelize/v2`
- **模块名**: `taobao`（`go.mod`）
- **二进制名**: `phonecase-tools`
- **无参数启动** → GUI 桌面模式（Wails）；**有参数** → CLI 模式

## 项目结构

```
.
├── main.go              # 入口：GUI/CLI 路由
├── app.go               # 后端 App 结构体，暴露给桌面前端的绑定方法
├── frontend/            # 桌面前端静态资源 (//go:embed 嵌入到二进制)
│   ├── index.html / main.js / style.css
│   └── wailsjs/         # Wails 自动生成的 JS 绑定
├── internal/
│   ├── common/          # 共用工具：列查找、单元格读取、规格解析、配置路径持久化
│   ├── filter/          # 订单筛选（分类逻辑 + Excel 读写）
│   ├── dangkou/         # 档口分配
│   ├── peijian/         # 配件提取 + 档口分配
│   ├── pizhi/           # 皮质壳档口分配（含嵌入图片）
│   ├── datu/            # 打图工厂分配
│   └── logger/          # 简洁的日志（文件+控制台）
├── wasm/                # Go → Wasm 入口，向 JS 暴露 go<Module>Process 函数
│   ├── main.go          # 注册全局函数，通知 onWasmReady
│   ├── bridge.go        # JSON 入参/出参桥接，调用各模块 ProcessData
│   └── engine_json.go   # JS 侧解析好的配置 → 各模块 Engine
├── web/                 # 浏览器版前端（原生 JS）
│   ├── index.html       # 模板（__CSS__/__JS__/wasm 占位符，由打包脚本替换）
│   ├── excel.js         # SheetJS 封装（读 Excel → 二维数组，二维数组 → 下载）
│   ├── config.js        # localStorage 配置持久化
│   ├── wasm-bridge.js   # Wasm 加载与调用
│   ├── ui.js / app.js   # UI 与主流程（配置 Excel 在 JS 侧解析成 engine JSON）
│   └── app.test.js      # node --test 测试
├── scripts/build-html.sh  # 将 web/ + wasm + wasm_exec.js 内联为单 HTML
├── phonecase-tools-skill/SKILL.md  # 供 AI 代理使用 CLI 的技能说明
├── docs/                # REPORT.md 与架构图（已 gitignore）
├── build/bin/           # 编译输出 + 运行时配置/日志（已 gitignore）
├── .github/workflows/release.yml  # tag v* 触发多平台构建并发布 Release
├── Makefile             # 构建命令
├── wails.json           # Wails 项目配置
└── go.mod / go.sum
```

`.gitignore` 忽略了 `*.xlsx`、`data/`、`build/`、`docs/`、`openspec/` 等，测试数据和配置表不在仓库中。

## 构建命令

```bash
make linux          # → build/bin/phonecase-tools（webkit2_41）
make windows        # → build/bin/phonecase-tools.exe
make macos          # 当前架构（需在 Mac 上运行）
make macos-intel    # Intel x86_64（需在 Mac 上运行）
make wasm           # → build/bin/phonecase.wasm + build/bin/phonecase-tools.html（单 HTML）
make all            # linux + windows + wasm
make dev            # 开发模式（热重载）
make clean          # 删除 build/bin
```

Makefile 使用 `$(HOME)/go/bin/wails`。构建产物、运行时配置（`keywords.json`、`*_config.json`）和日志都在 `build/bin/`，与可执行文件同目录。

## 测试

```bash
make test                           # Go + JS 全部测试
go test ./...                       # Go 测试（common/dangkou/peijian/pizhi/datu 有测试，filter 无）
go test ./internal/<模块>/ -v       # 单模块
node --test web/*.test.js           # 浏览器版 JS 测试
```

## CLI 用法

所有子命令的配置文件都**必须**通过参数传入（CLI 不读取已保存的配置路径）：

```bash
phonecase-tools filter  <Excel文件>
phonecase-tools dangkou <订单Excel文件> <自设编码.xlsx>
phonecase-tools peijian <订单Excel文件> <配件编码.xlsx>
phonecase-tools pizhi   <订单Excel文件> <皮质壳配置表.xlsx>
phonecase-tools datu    <订单Excel文件> <打图工厂配置表.xlsx>
```

输出统一写到订单文件同级的 `<订单名>_output/` 目录。

## 关键设计

### 通用约定（`internal/common`）
- 每个模块都分两层：`Process(filename, configPath)` 负责 I/O，`ProcessData(dataRows, headers, engine)` 是纯逻辑。桌面、CLI、Wasm 共用 `ProcessData`
- 商品规格用 `SplitSpec` 按 `|` 拆成两段，并去掉 `[...]`/`【...】` 后缀，不预设哪一段是型号、哪一段是 SKU。dangkou/peijian/pizhi 依次用两段去查配置表，**命中的那一段当 SKU，另一段当型号**，所以 `型号|SKU` 和 `SKU|型号` 两种写法都支持（`ParseSpec` 只按 `型号|SKU` 解析，目前仅 datu 使用）
- 商品ID 用 `GetCellValueSafe` 重新读取，覆盖 `GetRows` 读到的值，避免科学计数法和大数字精度问题
- 表头列名用 `FindColumn` 匹配，大小写不敏感，允许列缺失
- 配置 Excel 加载时跳过 `WpsReserved*` 开头的 sheet（WPS 内部保留 sheet）
- 配置路径持久化：`ConfigPath` / `ConfigSearchPaths`（先找 exe 同目录，再找当前目录）、`LoadConfigPath` / `SaveConfigPath`

### 订单筛选 (filter)
- `Config` 包含 `DoubtKeywords`（疑难关键词）和 `AccessoryKeywords`（配件关键词），从 `keywords.json` 加载，保存时直接覆盖
- 分类优先级：多件订单 > 疑难单 > 单独配件 > 正常手机壳
- 多件订单判断：`SubOrderID != "" && OrderID != "" && SubOrderID != OrderID`
- 输入表头通过反射 + `xlsx` tag 映射到 `RowData`（大小写不敏感）
- 排序：先按原有排序键，再按付款时间（多件按订单编号，其余按商家编码）
- 输出 `筛选结果.xlsx`，4 个 Sheet：多件订单、疑难单、单独配件、正常手机壳（按编码分组，组间空行）

### 档口分配 (dangkou)
- `Engine` 从自设编码 Excel 加载：
  1. Sheet 1：`商品ID|SKU名称` → 自设编码
  2. 后续 Sheets：列式布局，第 1 行为自设编码，下方为该编码支持的型号；Sheet 名即档口名，**空档口 Sheet 跳过**
- 匹配：`SplitSpec` → `LookupZisheBianma` 查编码（型号去掉全部空格）→ `FindStall` 按编码+型号找档口；按 Sheet 顺序决定优先级，命中第一个即停止
- 查不到编码的归入「无匹配自设编码」，找不到档口的归入「未分配档口」
- 输出 `档口分配.xlsx`：首个 Sheet 是「汇总」（每列一个档口，下方列出订单编号），后面是各档口明细（原始完整行）、未分配档口、无匹配自设编码
- 同时输出 `拿货档口.xlsx`：列出有订单的档口，用 `ParseStallName` 把档口名按 `-` 拆成 市场/档口号/档口名称（如 `经济-4-国产哥GCG`），按市场升序排序。表头：`产品数量|产品图片|市场|档口号|档口名称|支付状态|拿货备注`
- 配置文件路径保存在 `dangkou_config.json`

### 配件提取 (peijian)
- `Engine` 从「配件编码.xlsx」加载：
  1. Sheet 1：表头 `商品ID | SKU名称 | 编码1 | 编码2 | ...`，映射 `"商品ID|SKU名称"`（小写）→ `[]自设编码`（按编码列顺序）
  2. Sheet 2+：列式布局，第 1 行为档口名，下方为该档口的自设编码；`StallOrder` 按列顺序决定优先级
  3. 可选的「配件别名」Sheet：列式布局，每列一种配件，**第 1 行为标准名**，同列下方为别名。`loadAliasMapping` 生成 别名→标准名 映射，这个 Sheet 不算档口 Sheet
- 加载时**强校验**：每个 SKU 的配件数必须等于它的编码列数，不一致直接报错（`loadPeijianMapping`）
- `extractAccessories` 按 `+` 分割 SKU 名称：
  - 有 `+`：`+` 前面是手机壳（忽略），后面每一段是一个配件
  - 无 `+`：整个 SKU 就是配件
  - 每个配件名经 `cleanAccessoryName` 清洗：去掉开头的「单独」，去掉结尾的「不含壳」（前面可带 `-` 或空格），最后 `TrimSpace`
- `ProcessData`：
  1. `SplitSpec` 拆规格，两段分别拼 `商品ID|段`（小写）查 `Mapping`，命中的那段当 SKU
  2. 查不到归入 `NoMatch`（无匹配自设编码）；配件数≠编码数归入 `Unassigned`
  3. 每个配件用对应编码查 `Stalls` 找档口。找不到归入 `Unassigned`（未分配档口）；找到则经 `Aliases` 归一成标准名，记入 `StallOrders[档口]`，并累加 `Summary[档口] += 商品数量`
- 输出 `配件分配.xlsx`：
  - 汇总 Sheet（第一个）：每列一个有订单的档口，下方是 `配件名 x数量`，按数量降序
  - 单独配件 Sheet（紧跟汇总）：SKU 名称不含 `+` 的订单原始完整行（这些订单照常参与档口分配，只是额外单独输出一份）
  - 每个档口一个明细 Sheet：`店铺名称|订单编号|商品id|商品规格|商品数量|配件名称`
  - `未分配档口` / `无匹配自设编码`：输出原始完整行
- 配件编码文件路径保存在 `peijian_config.json`

### 皮质壳分配 (pizhi)
- `Engine` 从「皮质壳配置表.xlsx」加载：每个 Sheet 是一个档口（Sheet 名即档口名，`Stalls` 按 Sheet 顺序排列）。每行是 `商品ID | SKU名称 | 图片`，图片**嵌入**在单元格里（默认 C 列）。`mapImagesByRow` 按行号提取图片字节，每行只取一张
- `Items`：`"商品ID|SKU"`（小写）→ `ConfigItem{Stall, ImageBytes, ImageExt}`
- `ProcessData` 逐行处理、不聚合：两段规格都去查 `Items` 确定 SKU 和型号，**查不到的订单静默跳过**；同时带上商品数量、买家留言、卖家备注
- 输出 `皮质壳分配.xlsx`：每个有订单的档口一个 Sheet，表头 `型号|数量|图片|买家留言|卖家备注`。图片缩放到约 100px 嵌入单元格，留言和备注列自动换行
- 配置文件路径保存在 `pizhi_config.json`

### 打图分配 (datu)
- `Engine` 从「打图工厂配置表.xlsx」加载：只读第一个非 `WpsReserved` 的 Sheet（「打图工厂编码」）。每列是一个工厂，第 1 行是工厂名，下方是该工厂负责的商品ID。生成 `FactoryByProductID`（小写），`Factories` 按列顺序排列
- 只按商品ID 匹配，查不到的订单静默跳过。型号用 `ParseSpec` 取 `|` 前一段（去空格、去括号），即假定规格为 `型号|SKU`
- `ParseDatuCode` 解析「商品规格商家编码」列里**最后一个** `【素材-编码】`，例如 `【PH仓】【DYT彩银白色-DTY7958】` → (`DYT彩银白色`, `DTY7958`)。格式不符时素材和编码都为空
- 一个订单输出一行，不聚合；姓名列固定为 `DefaultName`（`凡凡`）
- 输出 `打图结果.xlsx`：每个工厂一个 Sheet，按付款时间升序排列，表头 `订单号|序号|编码|手机型号|素材|数量|姓名|付款时间|买家留言|卖家备注`
- 配置文件路径保存在 `datu_config.json`

### 配置系统
- 桌面/CLI 的配置文件都在可执行文件同目录（用 `os.Executable()` 定位），找不到时再找当前目录
- `keywords.json` 存筛选关键词，保存时直接覆盖。`dangkou_config.json`、`peijian_config.json`、`pizhi_config.json`、`datu_config.json` 只存对应配置 Excel 的**路径**
- GUI 通过 `Get/Save/Select<Module>ConfigPath` 绑定读写这些路径；CLI 不读它们，配置文件由参数传入

### GUI 模式（桌面）
- Wails 框架，前端通过 `window.go.main.App.*` 调用后端：`RunFilter`、`RunDangkou`、`RunPeijianProcess`、`RunPizhiProcess`、`RunDatuProcess`、`OpenDir`、`SelectFile`、`HandleDroppedFile`
- 支持拖拽文件：有 `file.path` 时直接传路径，没有时把 base64 内容交给 `HandleDroppedFile` 写成临时文件
- 支持原生文件选择对话框（`runtime.OpenFileDialog`）
- 每个工具一个按钮，各配一个设置齿轮；结果区统一显示统计卡片
- 前端用 `//go:embed all:frontend` 嵌入二进制，改完要重新构建

### 浏览器版（wasm + web）
- `wasm/main.go` 在 JS 全局注册 `goFilterProcess`、`goDangkouProcess`、`goPeijianProcess`、`goDatuProcess`、`goPizhiProcess`，加载完成后调用 `onWasmReady`
- 入参和出参都是 JSON（`{rows, headers, engine}` / `{data, error}`）。**配置 Excel 在 JS 侧解析**（`web/app.js` 的 `parse<Module>ConfigSheet`），生成 engine JSON 后，由 `wasm/engine_json.go` 转成各模块的 `Engine`，再调用 `ProcessData`
- 读写 Excel 用 SheetJS（CDN）。读取时设 `raw: false`，避免商品ID 精度丢失。皮质壳配置要提取嵌入图片，改用 `hucre`（`esm.sh` 动态 import）
- 配置持久化：filter 和 datu 的配置较小，存 `localStorage`；dangkou、peijian、pizhi 的配置较大，只放内存，刷新页面后要重新上传
- **改了模块的 Engine 结构或配置格式，要同步修改 `wasm/engine_json.go` 和 `web/app.js` 里的解析逻辑**
- `make wasm` 先编译 wasm，再用 `scripts/build-html.sh`（内含 Python）把 CSS、JS、`wasm_exec.js`、base64 编码的 wasm 内联进 `web/index.html` 模板，生成 `build/bin/phonecase-tools.html`

## 代码规范

- **编码风格**: 标准 Go 风格，包级私有函数小写开头，导出类型大写开头
- **注释**: 以中文为主，包注释用 `// Package xxx` 开头
- **测试**: 用标准 `testing` 包写表驱动测试，函数命名 `TestXxx`；纯逻辑放在 `ProcessData` 里方便测试
- **错误处理**: 用 `fmt.Errorf` 包装（`%w`）并返回 error
- **日志**: 统一通过 `logger` 包输出，格式 `[时间] [级别] 消息`
- **Excel**: 表头列名大小写不敏感匹配，允许某些列缺失；新的公共逻辑放进 `internal/common`

## 常见开发任务

### 添加新的订单分类规则（filter）
1. 修改 `internal/filter/filter.go` 的 `ProcessWithConfig`
2. 按需在 `RowData` 或 `Config` 中加字段
3. 更新 `writeOutput`，加上新 Sheet
4. 更新 `main.go` 里 `runCLI` 的 `case "filter"` 输出

### 新增/修改配置映射（无需改代码）
- 档口：在自设编码 Excel 里加一个 Sheet（Sheet 名即档口名）
- 配件：「配件编码.xlsx」Sheet 1 加一行 `商品ID | SKU名称 | 编码1 | ...`（编码列数必须等于配件数），再在 Sheet 2+ 对应档口列下加上该编码
- 皮质壳：在对应档口 Sheet 里加一行 `商品ID | SKU名称 | 图片`
- 打图：在对应工厂列下加上商品ID

### 新增一个处理模块
1. 新建 `internal/<模块>/`，实现 `LoadEngine`、`Process`、`ProcessData`、`writeOutput`、配置路径持久化，并写测试
2. `main.go`：加 CLI 子命令和用法说明
3. `app.go` + `frontend/`：加 `Run*`、`Get/Save/Select*ConfigPath` 绑定和按钮
4. `wasm/bridge.go`、`wasm/engine_json.go`、`wasm/main.go`、`web/`：加浏览器版支持
5. 同步更新本文件和 `phonecase-tools-skill/SKILL.md`

### 修改前端 UI
- 桌面：`frontend/index.html`、`style.css`、`main.js`，通过 `window.go.main.App.<MethodName>` 调用后端
- 浏览器：`web/`，改完运行 `make wasm` 重新打包
