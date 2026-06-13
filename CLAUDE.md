# 手机壳订单处理工具 (Phone Case Order Processing Tool)

## 项目概述

这是一个基于 Wails v2 的桌面应用 + CLI 工具，用于处理淘宝手机壳订单的 Excel 文件。支持三大功能：
1. **订单筛选** (`filter`) — 将订单按多件/疑难/正常/配件分类
2. **档口分配** (`dangkou`) — 按自设编码将订单分配给不同档口
3. **配件提取** (`peijian`) — 从订单规格中提取并汇总配件信息

## 技术栈

- **后端**: Go 1.24+, Wails v2.12.0
- **前端**: 原生 HTML/CSS/JS（无框架），`frontend/` 目录
- **Excel 处理**: `github.com/xuri/excelize/v2`
- **模块名**: `taobao`（`go.mod`）
- **二进制名**: `phonecase-tools`
- **无参数启动** → GUI 桌面模式（Wails）；**有参数** → CLI 模式

## 项目结构

```
.
├── main.go              # 入口：GUI/CLI 路由
├── app.go               # 后端 App 结构体，暴露给前端的绑定方法
├── frontend/            # 前端静态资源 (嵌入到二进制)
│   ├── index.html       # 主页面
│   ├── main.js          # 前端逻辑 (原生 JS)
│   ├── style.css        # 样式
│   └── wailsjs/         # Wails 自动生成的 JS 绑定
├── internal/
│   ├── filter/          # 订单筛选模块
│   │   └── filter.go    # 分类逻辑 + Excel 读写
│   ├── dangkou/         # 档口分配模块
│   │   ├── dangkou.go       # 引擎 + 匹配 + 主流程
│   │   └── dangkou_test.go  # 单元测试
│   ├── peijian/         # 配件提取模块
│   │   ├── peijian.go       # 提取 + 汇总 + 分类
│   │   └── peijian_test.go  # 单元测试
│   └── logger/          # 简洁的日志（文件+控制台）
│       └── logger.go
├── build/               # 构建产物和资源
│   └── bin/             # 编译输出 + 运行时的配置文件
├── data/                # 测试用的 Excel 数据文件
├── openspec/            # OpenSpec 变更记录（提案/设计/任务）
├── Makefile             # 构建命令
├── wails.json           # Wails 项目配置
└── go.mod / go.sum
```

## 构建命令

```bash
# Linux 构建
make linux                 # → build/bin/phonecase-tools

# Windows 构建
make windows               # → build/bin/phonecase-tools.exe

# macOS 构建（需在 Mac 上运行）
make macos                 # 当前架构
make macos-intel           # Intel x86_64

# 开发模式（热重载）
make dev

# 清理
make clean
```

构建产物在 `build/bin/` 目录，配置文件（`keywords.json`、`parts.json`、`columns.json`、`dangkou_config.json`）和日志文件与该目录中的可执行文件同目录。

## 测试

```bash
go test ./...                       # 运行所有测试
go test ./internal/dangkou/ -v      # 档口模块测试
go test ./internal/peijian/ -v      # 配件模块测试
```

## CLI 用法

```bash
# 订单筛选
phonecase-tools filter <Excel文件>

# 档口分配
phonecase-tools dangkou <订单Excel文件> [自设编码.xlsx]

# 配件提取
phonecase-tools peijian extract <Excel文件>

# 配件汇总
phonecase-tools peijian merge <pending.xlsx>
```

## 关键设计

### 订单筛选 (filter)
- `Config` 包含 `DoubtKeywords`（疑难关键词）和 `AccessoryKeywords`（配件关键词）
- 分类优先级：多件订单 > 疑难单 > 单独配件 > 正常手机壳
- 多件订单判断：`SubOrderID != "" && OrderID != "" && SubOrderID != OrderID`
- 输入 Excel 的表头通过反射 + `xlsx` tag 映射到 `RowData` 结构体（大小写不敏感匹配）
- 排序优先级：原有排序键 > 付款时间（多件按订单编号，其余按商家编码）
- 配置从 `keywords.json` 加载，保存到可执行文件同目录
- 输出 Excel 包含 4 个 Sheet，正常手机壳按编码分组（不同编码之间空行隔开）

### 档口分配 (dangkou)
- `Engine` 加载自设编码 Excel 文件（2层映射）：
  1. Sheet 1（自设编码表）：`商品ID|SKU名称` → 自设编码
  2. 后续 Sheets：自设编码 → 型号列表，每个 Sheet 名即为档口名
- 匹配流程：`parseSpec` 解析规格 → `LookupZisheBianma` 查编码 → `FindStall` 按编码+型号找档口
- `parseSpec` 解析格式 `"{Phone Model}|{SKU Name}[{Variant}]"`，型号会去掉所有空格
- `StripBracketSuffix` 去除 `[...]` 和 `【...】` 后缀
- 商品ID 使用 `GetCellValue`（非 `GetRows`）读取以避免大数字精度问题
- 档口按 Sheet 顺序决定优先级，匹配到第一个即停止
- 不匹配的分为「无匹配自设编码」和「未分配档口」两类
- 配置文件路径保存在 `dangkou_config.json`

### 配件提取 (peijian)
- 两阶段处理：`Extract`（提取）→ `Merge`（汇总）
- `Extract` 根据配件关键词对订单分类：简单订单 > 待处理订单 > 无配件订单
- 简单订单：含 `+` 分隔符、无买家留言、无卖家备注、无备注/咨询关键词
- 配件关键词通过 `parts.json` 配置，默认包括支架、吸盘、串珠等
- `ExtractParts` 解析 `+` 分隔的规格段，`matchPart` 按后缀匹配配件名
- `Merge` 汇总 pending.xlsx 的所有 Sheet 中的配件，按名称+颜色聚合，按数量降序排列
- 列名可通过 `columns.json` 自定义（spec、buyerMsg、sellerNote、quantity）

### 配置系统
- 所有模块的配置文件都保存在可执行文件同目录
- 使用 `os.Executable()` 获取路径，`configSearchPaths` 提供多路径搜索（exe同目录 → 当前目录）
- 前端通过 Wails 绑定方法获取/保存配置
- `keywords.json`、`parts.json`、`columns.json` 直接替换；`dangkou_config.json` 存储文件路径

### GUI 模式
- Wails 框架，前端通过 `window.go.main.App.*` 调用后端
- 支持拖拽文件（base64 编码传给后端写临时文件，或使用 `file.path` 直接传递）
- 支持原生文件选择对话框（`runtime.OpenFileDialog`）
- 三个工具按钮 + 各自的配置齿轮按钮
- 结果展示区统一显示统计卡片

## 代码规范

- **编码风格**: 标准 Go 风格，包级私有函数小写开头，导出类型大写开头
- **注释**: 中文注释为主，包级注释使用英文 `// Package xxx` 格式
- **测试**: 使用标准 `testing` 包，表驱动测试（table-driven tests），测试函数命名 `TestXxx`
- **错误处理**: 使用 `fmt.Errorf` 包装错误，返回 error
- **日志**: 通过 `logger` 包统一输出，格式 `[时间] [级别] 消息`
- **配置文件**: JSON 格式，双重存储（前端可通过 Wails 绑定读写，后端直接读写文件）
- **Excel**: 表头列名大小写不敏感匹配（`strings.EqualFold`），支持列缺失（某些列非必需）

## 常见开发任务

### 添加新的订单分类规则
1. 修改 `internal/filter/filter.go` 的 `ProcessWithConfig` 函数
2. 在 `RowData` 或 `Config` 中添加必要字段
3. 更新 `writeOutput` 添加新 Sheet
4. 更新 CLI 输出 `main.go` 的 `runCLI` case "filter"

### 添加新的配件关键词
1. 编辑运行目录下的 `parts.json`（通过 GUI 配置面板 → 齿轮按钮）
2. 或修改 `internal/peijian/peijian.go` 的 `DefaultPartsConfig`
3. 关键词匹配逻辑在 `matchPart` 中（后缀匹配）

### 添加新的档口
1. 在自设编码 Excel 文件中添加新的 Sheet（Sheet 名即为档口名）
2. Sheet 格式：第一行为自设编码（表头），下方为该编码支持的型号列表
3. 无需修改代码

### 修改前端 UI
1. 修改 `frontend/index.html`（结构）、`frontend/style.css`（样式）、`frontend/main.js`（逻辑）
2. 注意前端通过 `window.go.main.App.<MethodName>` 调用后端方法
3. 前端内嵌到二进制中（`//go:embed all:frontend`），修改后重新构建即可
