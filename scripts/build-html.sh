#!/bin/bash
# 打包脚本：将 web/ + phonecase.wasm 打包为单 HTML 文件
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/build/bin"
WEB_DIR="$PROJECT_DIR/web"

HTML_OUT="$BUILD_DIR/phonecase-tools.html"
TEMPLATE="$WEB_DIR/index.html"
WASM_FILE="$BUILD_DIR/phonecase.wasm"
WASM_EXEC_JS=""
for p in "$(go env GOROOT)/misc/wasm/wasm_exec.js" "$(go env GOROOT)/lib/wasm/wasm_exec.js"; do
  [ -f "$p" ] && { WASM_EXEC_JS="$p"; break; }
done

# 检查文件
[ -f "$WASM_FILE" ] || { echo "错误: 请先运行 GOOS=js GOARCH=wasm go build -o $WASM_FILE ./wasm/"; exit 1; }
[ -f "$WASM_EXEC_JS" ] || { echo "错误: 找不到 wasm_exec.js: $WASM_EXEC_JS"; exit 1; }

echo "=== 打包单 HTML 文件 ==="

# 使用 Python 进行字符串替换（bash 处理大字符串有问题）
python3 << PYEOF
import base64

# 读取模板
with open("$TEMPLATE", "r") as f:
    html = f.read()

# 内联 CSS
with open("$WEB_DIR/style.css", "r") as f:
    css = f.read()
html = html.replace("__CSS__", css)

# 内联 wasm_exec.js
with open("$WASM_EXEC_JS", "r") as f:
    wasm_exec = f.read()
html = html.replace("__WASM_EXEC_JS__", wasm_exec)

# 内联 JS 文件（按依赖顺序）
js_files = ["$WEB_DIR/excel.js", "$WEB_DIR/config.js", "$WEB_DIR/wasm-bridge.js", "$WEB_DIR/ui.js", "$WEB_DIR/app.js"]
js_all = ""
for f in js_files:
    with open(f, "r") as fh:
        js_all += "\n" + fh.read()
html = html.replace("__JS__", js_all)

# 内联 wasm (base64)
print("编码 wasm (base64)...")
with open("$WASM_FILE", "rb") as f:
    wasm_bytes = f.read()
wasm_b64 = base64.b64encode(wasm_bytes).decode("ascii")
wasm_data_url = "data:application/wasm;base64," + wasm_b64
html = html.replace("__WASM_DATA_URL__", wasm_data_url)

# 写入输出
with open("$HTML_OUT", "w") as f:
    f.write(html)

wasm_size_mb = len(wasm_bytes) / (1024 * 1024)
html_size_mb = len(html) / (1024 * 1024)
print(f"完成: $HTML_OUT ({html_size_mb:.1f} MB)")
print(f"  wasm: {wasm_size_mb:.1f} MB, base64: {len(wasm_b64)/1024/1024:.1f} MB")
PYEOF
