// Package main 为 WebAssembly 提供 Go→JS 桥接入口
//
// 编译: GOOS=js GOARCH=wasm go build -o phonecase.wasm ./wasm/
//
// 暴露三个全局 JS 函数:
//   goFilterProcess(jsonArgs)   → JSON result
//   goDangkouProcess(jsonArgs)  → JSON result
//   goPeijianProcess(jsonArgs)  → JSON result
//
//go:build js && wasm

package main

import (
	"syscall/js"
)

func main() {
	c := make(chan struct{})

	// 注册全局 JS 函数
	js.Global().Set("goFilterProcess", js.FuncOf(filterProcess))
	js.Global().Set("goDangkouProcess", js.FuncOf(dangkouProcess))
	js.Global().Set("goPeijianProcess", js.FuncOf(peijianProcess))

	// 发送信号表明 Wasm 已就绪
	notifyReady()

	<-c // 保持 Wasm 运行
}

// notifyReady 调用 JS 全局钩子 onWasmReady()
func notifyReady() {
	if readyFn := js.Global().Get("onWasmReady"); readyFn.Type() == js.TypeFunction {
		readyFn.Invoke()
	}
}
