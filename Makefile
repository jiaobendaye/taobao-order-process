.PHONY: all linux windows clean wasm

APP_NAME  := phonecase-tools
OUT_DIR   := build/bin
WAILS     := $(HOME)/go/bin/wails

# 构建所有
all: linux windows wasm

# CLI 已集成到主程序
cli:
	@echo "CLI 已集成到 phonecase-tools 中"
	@echo "用法: ./$(OUT_DIR)/$(APP_NAME) filter|dangkou|peijian <文件>"

# Linux
linux:
	@echo "=== 构建 Linux ==="
	$(WAILS) build -platform linux/amd64 -tags webkit2_41 -ldflags="-s -w"
	@echo "完成: $(OUT_DIR)/$(APP_NAME)"

# Windows
windows:
	@echo "=== 构建 Windows ==="
	$(WAILS) build -platform windows/amd64 -webview2 embed -ldflags="-s -w -H windowsgui"
	@echo "完成: $(OUT_DIR)/$(APP_NAME).exe"

# 开发模式
dev:
	$(WAILS) dev -tags webkit2_41

# macOS（需在 Mac 上构建）
#   make macos         → 当前架构（ARM Mac 编译 ARM，Intel Mac 编译 Intel）
#   make macos-intel   → 强制编译 Intel(x86_64) 包（可在 ARM Mac 上交叉编译）
macos:
	@uname -s | grep -q Darwin || { echo "错误: macOS 构建必须在 Mac 上运行"; exit 1; }
	@echo "=== 构建 macOS（当前架构）==="
	$(WAILS) build -o "$(OUT_DIR)/$(APP_NAME)-$(shell uname -m)" -ldflags="-s -w"
	@echo "完成: $(OUT_DIR)/$(APP_NAME)-$(shell uname -m)"

macos-intel:
	@uname -s | grep -q Darwin || { echo "错误: macOS 构建必须在 Mac 上运行"; exit 1; }
	@echo "=== 构建 macOS（Intel x86_64）==="
	$(WAILS) build -platform darwin/amd64 -o "$(OUT_DIR)/$(APP_NAME)-x86_64" -ldflags="-s -w"
	@echo "完成: $(OUT_DIR)/$(APP_NAME)-x86_64"

# 清理
clean:
	rm -rf $(OUT_DIR)
	@echo "清理完成"

# WebAssembly 单 HTML 版本
wasm:
	@echo "=== 编译 Go → Wasm ==="
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o $(OUT_DIR)/phonecase.wasm ./wasm/
	@echo "Wasm 完成: $(OUT_DIR)/phonecase.wasm"
	@echo "=== 打包单 HTML ==="
	bash scripts/build-html.sh
