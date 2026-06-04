.PHONY: all linux windows clean

APP_NAME  := phonecase-tools
OUT_DIR   := build/bin
WAILS     := $(HOME)/go/bin/wails

# 构建所有
all: linux windows

# CLI 已集成到主程序
cli:
	@echo "CLI 已集成到 phonecase-tools 中"
	@echo "用法: ./$(OUT_DIR)/$(APP_NAME) filter|dangkou|peijian <文件>"

# Linux
linux:
	@echo "=== 构建 Linux ==="
	$(WAILS) build -tags webkit2_41 -ldflags="-s -w"
	@echo "完成: $(OUT_DIR)/$(APP_NAME)"

# Windows
windows:
	@echo "=== 构建 Windows ==="
	$(WAILS) build -platform windows/amd64 -webview2 embed -ldflags="-s -w -H windowsgui"
	@echo "完成: $(OUT_DIR)/$(APP_NAME).exe"

# 开发模式
dev:
	$(WAILS) dev -tags webkit2_41

# 清理
clean:
	rm -rf $(OUT_DIR)
	@echo "清理完成"
