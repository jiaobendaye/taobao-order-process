//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"taobao/internal/dangkou"
	"taobao/internal/filter"
	"taobao/internal/peijian"
)

// ---- Filter ----

type filterArgs struct {
	Rows [][]string    `json:"rows"`
	Conf filter.Config `json:"config"`
}

type filterResult struct {
	Data  *filter.Result `json:"data"`
	Error string         `json:"error,omitempty"`
}

func filterProcess(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return mustMarshal(filterResult{Error: "filterProcess: missing JSON argument"})
	}
	var a filterArgs
	if err := json.Unmarshal([]byte(args[0].String()), &a); err != nil {
		return mustMarshal(filterResult{Error: fmt.Sprintf("filterProcess: invalid JSON: %v", err)})
	}

	// 解析 rows 为 []filter.RowData
	parsed, err := filter.ParseRows(a.Rows)
	if err != nil {
		return mustMarshal(filterResult{Error: fmt.Sprintf("filterProcess: parse rows: %v", err)})
	}

	data := filter.ProcessData(parsed, &a.Conf)
	return mustMarshal(filterResult{Data: data})
}

// ---- Dangkou ----

type dangkouArgs struct {
	Rows      [][]string           `json:"rows"`
	Headers   []string             `json:"headers"`
	EngineJSON dangkouEngineJSON   `json:"engine"`
}

type dangkouResult struct {
	Data  *dangkou.Result `json:"data"`
	Error string          `json:"error,omitempty"`
}

func dangkouProcess(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return mustMarshal(dangkouResult{Error: "dangkouProcess: missing JSON argument"})
	}
	var a dangkouArgs
	if err := json.Unmarshal([]byte(args[0].String()), &a); err != nil {
		return mustMarshal(dangkouResult{Error: fmt.Sprintf("dangkouProcess: invalid JSON: %v", err)})
	}

	engine := a.EngineJSON.toEngine()
	data := dangkou.ProcessData(a.Rows, a.Headers, engine)
	return mustMarshal(dangkouResult{Data: data})
}

// ---- Peijian ----

type peijianArgs struct {
	Rows       [][]string         `json:"rows"`
	Headers    []string           `json:"headers"`
	EngineJSON peijianEngineJSON  `json:"engine"`
}

type peijianResult struct {
	Data  *peijian.Result `json:"data"`
	Error string          `json:"error,omitempty"`
}

func peijianProcess(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return mustMarshal(peijianResult{Error: "peijianProcess: missing JSON argument"})
	}
	var a peijianArgs
	if err := json.Unmarshal([]byte(args[0].String()), &a); err != nil {
		return mustMarshal(peijianResult{Error: fmt.Sprintf("peijianProcess: invalid JSON: %v", err)})
	}

	engine := a.EngineJSON.toEngine()
	data := peijian.ProcessData(a.Rows, a.Headers, engine)
	return mustMarshal(peijianResult{Data: data})
}

// ---- helpers ----

func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"marshal failed"}`
	}
	return string(b)
}
