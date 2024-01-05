package main

import (
	"bytes"
	"encoding/json"
)

type AppArgs struct {
	Target string

	Technicals []string

	DictFile    string
	BruteLength string
	FofaToken   string

	OutputFile    string
	TaskCount     uint
	CheckWildcard bool
	Nameserver    []string
	FetchTitle    bool

	Debug       bool
	HasWildcard bool
}

var appArgs AppArgs

func (a *AppArgs) PrettyString() string {
	bs, _ := json.Marshal(a)
	var out bytes.Buffer
	json.Indent(&out, bs, "", "    ")
	return out.String()
}
