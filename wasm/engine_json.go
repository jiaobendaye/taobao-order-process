//go:build js && wasm

package main

import (
	"taobao/internal/dangkou"
	"taobao/internal/peijian"
)

// ---- Dangkou Engine JSON types ----

type dangkouEngineJSON struct {
	Mapping map[string]string          `json:"mapping"`
	Stalls  []dangkouStallConfigJSON   `json:"stalls"`
}

type dangkouStallConfigJSON struct {
	Name     string              `json:"name"`
	Priority int                 `json:"priority"`
	Codes    map[string][]string `json:"codes"`
}

func (e *dangkouEngineJSON) toEngine() *dangkou.Engine {
	stalls := make([]dangkou.StallConfig, len(e.Stalls))
	for i, s := range e.Stalls {
		stalls[i] = dangkou.StallConfig{
			Name:     s.Name,
			Priority: s.Priority,
			Codes:    s.Codes,
		}
	}
	return &dangkou.Engine{
		Mapping: e.Mapping,
		Stalls:  stalls,
	}
}

// ---- Peijian Engine JSON types ----

type peijianEngineJSON struct {
	Mapping    map[string][]string `json:"mapping"`
	Stalls     map[string]string   `json:"stalls"`
	StallOrder []string            `json:"stallOrder"`
}

func (e *peijianEngineJSON) toEngine() *peijian.Engine {
	return &peijian.Engine{
		Mapping:    e.Mapping,
		Stalls:     e.Stalls,
		StallOrder: e.StallOrder,
	}
}
