//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"taobao/internal/common"
	"taobao/internal/dangkou"
	"taobao/internal/datu"
	"taobao/internal/filter"
	"taobao/internal/peijian"
	"taobao/internal/pizhi"
)

// ---- Filter ----

func normalizeOrderRows(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 { return mustMarshal(struct{ Error string `json:"error"` }{Error: "缺少订单数据"}) }
	var input struct{ Rows [][]string `json:"rows"` }
	if err := json.Unmarshal([]byte(args[0].String()), &input); err != nil {
		return mustMarshal(struct{ Error string `json:"error"` }{Error: err.Error()})
	}
	rows, err := common.NormalizeOrderRows(input.Rows)
	if err != nil { return mustMarshal(struct{ Error string `json:"error"` }{Error: err.Error()}) }
	return mustMarshal(struct{ Data [][]string `json:"data"` }{Data: rows})
}

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
	Rows       [][]string        `json:"rows"`
	Headers    []string          `json:"headers"`
	EngineJSON dangkouEngineJSON `json:"engine"`
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

	allRows, err := common.NormalizeOrderRows(append([][]string{a.Headers}, a.Rows...))
	if err != nil {
		return mustMarshal(dangkouResult{Error: err.Error()})
	}
	engine := a.EngineJSON.toEngine()
	data := dangkou.ProcessData(allRows[1:], allRows[0], engine)
	return mustMarshal(dangkouResult{Data: data})
}

// ---- Peijian ----

type peijianArgs struct {
	Rows       [][]string        `json:"rows"`
	Headers    []string          `json:"headers"`
	EngineJSON peijianEngineJSON `json:"engine"`
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

	allRows, err := common.NormalizeOrderRows(append([][]string{a.Headers}, a.Rows...))
	if err != nil {
		return mustMarshal(peijianResult{Error: err.Error()})
	}
	engine := a.EngineJSON.toEngine()
	data := peijian.ProcessData(allRows[1:], allRows[0], engine)
	return mustMarshal(peijianResult{Data: data})
}

// ---- Datu ----

type datuArgs struct {
	Rows       [][]string     `json:"rows"`
	Headers    []string       `json:"headers"`
	EngineJSON datuEngineJSON `json:"engine"`
}

type datuResult struct {
	Data  *datu.Result `json:"data"`
	Error string       `json:"error,omitempty"`
}

func datuProcess(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return mustMarshal(datuResult{Error: "datuProcess: missing JSON argument"})
	}
	var a datuArgs
	if err := json.Unmarshal([]byte(args[0].String()), &a); err != nil {
		return mustMarshal(datuResult{Error: fmt.Sprintf("datuProcess: invalid JSON: %v", err)})
	}

	allRows, err := common.NormalizeOrderRows(append([][]string{a.Headers}, a.Rows...))
	if err != nil {
		return mustMarshal(datuResult{Error: err.Error()})
	}
	engine := a.EngineJSON.toEngine()
	data := datu.ProcessData(allRows[1:], allRows[0], engine)
	return mustMarshal(datuResult{Data: data})
}

// ---- Pizhi ----

type pizhiArgs struct {
	Rows       [][]string      `json:"rows"`
	Headers    []string        `json:"headers"`
	EngineJSON pizhiEngineJSON `json:"engine"`
}

type pizhiResult struct {
	Data  *pizhi.Result `json:"data"`
	Error string        `json:"error,omitempty"`
}

func pizhiProcess(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return mustMarshal(pizhiResult{Error: "pizhiProcess: missing JSON argument"})
	}
	var a pizhiArgs
	if err := json.Unmarshal([]byte(args[0].String()), &a); err != nil {
		return mustMarshal(pizhiResult{Error: fmt.Sprintf("pizhiProcess: invalid JSON: %v", err)})
	}

	allRows, err := common.NormalizeOrderRows(append([][]string{a.Headers}, a.Rows...))
	if err != nil {
		return mustMarshal(pizhiResult{Error: err.Error()})
	}
	engine := a.EngineJSON.toEngine()
	data := pizhi.ProcessData(allRows[1:], allRows[0], engine)
	return mustMarshal(pizhiResult{Data: data})
}

// ---- helpers ----

func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"marshal failed"}`
	}
	return string(b)
}
