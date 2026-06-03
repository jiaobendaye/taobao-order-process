#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

build_skill() {
    local name=$1
    local script_dir="$SCRIPT_DIR/skills/$name/scripts"
    local linux_bin="$script_dir/$name"
    local windows_bin="$script_dir/$name.exe"

    echo "=== 编译 $name ==="

    echo "  编译 Linux 版本..."
    (cd "$script_dir" && GOOS=linux GOARCH=amd64 go build -o "$linux_bin" main.go)
    if [ $? -ne 0 ]; then
        echo "  Linux 编译失败"
        return 1
    fi

    echo "  编译 Windows 版本..."
    (cd "$script_dir" && GOOS=windows GOARCH=amd64 go build -o "$windows_bin" main.go)
    if [ $? -ne 0 ]; then
        echo "  Windows 编译失败"
        return 1
    fi

    echo "  完成: $linux_bin"
    echo "        $windows_bin"
    return 0
}

build_skill "excel-parts-extractor"
if [ $? -ne 0 ]; then
    exit 1
fi

build_skill "excel-phonecase-filter"
if [ $? -ne 0 ]; then
    exit 1
fi

echo ""
echo "所有编译完成"