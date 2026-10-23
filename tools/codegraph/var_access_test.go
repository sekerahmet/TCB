package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

var varAccessFiles = map[string]string{
	"a/a.go": `package a

import "sync/atomic"

var counter int

var table = map[string]int{}

var n int64

var cfg struct{ n int }

var initial = load()

var Exported int

func load() int { return 1 }

func Set() { counter = 1 }

func Add() {
	counter += 2
	counter++
}

func Put(k string) {
	table[k] = 1
	delete(table, k)
}

func Hit() { atomic.AddInt64(&n, 1) }

func Ptr() *int { return &counter }

func SetCfg() { cfg.n = 3 }

func Read() int { return counter + len(table) + initial }

func init() { counter = 9 }
`,
	"b/b.go": `package b

import "example.com/audit/a"

func Poke() { a.Exported = 5 }

func Peek() int { return a.Exported }
`,
}

// Kural: paket düzeyi değişkenlerde okuma ve yazma ayrılır, yapı alanlarındaki gibi: atama, işlemli atama, ++/--,
// öğe/alan yazımı, delete, adres alma, sync/atomic ve başka paketten nitelikli yazım `writes_var`; okumalar
// `reads_var`; `init` işlevindeki yazım ve değişkenin başlatıcısı `inits_var`. Kip kenarın erişim yerlerinde durur.
func TestPackageVariablesSeparateReadsFromWrites(t *testing.T) {
	g, err := buildGraph(auditModule(t, varAccessFiles))
	if err != nil {
		t.Fatal(err)
	}
	const a = "go:func:example.com/audit/a."
	const v = "go:var:example.com/audit/a."
	modes := func(from, to, kind string) string {
		e := edge(g, from, to, kind)
		if e == nil {
			return "-"
		}
		var ms []string
		for _, s := range e.Sites {
			ms = append(ms, s.Mode)
		}
		sort.Strings(ms)
		return strings.Join(ms, ",")
	}
	for _, c := range []struct{ from, to, kind, want string }{
		{a + "Set", v + "counter", edgeWritesVar, "write"},
		{a + "Add", v + "counter", edgeWritesVar, "read+write,read+write"},
		{a + "Add", v + "counter", edgeReadsVar, "read,read"},
		{a + "Put", v + "table", edgeWritesVar, "elem-write,elem-write"},
		{a + "Hit", v + "n", edgeWritesVar, "atomic"},
		{a + "Ptr", v + "counter", edgeWritesVar, "addr"},
		{a + "SetCfg", v + "cfg", edgeWritesVar, "elem-write"},
		{a + "Read", v + "counter", edgeReadsVar, "read"},
		{a + "Read", v + "counter", edgeWritesVar, "-"},
		{"go:func:example.com/audit/b.Poke", v + "Exported", edgeWritesVar, "write"},
		{"go:func:example.com/audit/b.Peek", v + "Exported", edgeReadsVar, "read"},
		{v + "initial", v + "initial", edgeInitsVar, "initializer"},
		{v + "table", v + "table", edgeInitsVar, "initializer"},
	} {
		if got := modes(c.from, c.to, c.kind); got != c.want {
			t.Errorf("%s -%s-> %s: modes %q, want %q", c.from, c.kind, c.to, got, c.want)
		}
	}
	var initFrom string
	for _, e := range g.Edges {
		if e.Kind == edgeInitsVar && e.To == v+"counter" && e.From != e.To {
			initFrom = e.From
		}
	}
	if !strings.Contains(initFrom, ".init#") {
		t.Errorf("a write inside func init is inits_var from the init function, got %q", initFrom)
	}
	if edge(g, a+"Set", v+"counter", edgeUses) != nil {
		t.Error("variables are read or written, not just used")
	}
}

// Kural: `package-state` ve grafik "yazma nedir" kuralını paylaşır (writeTargets): package-state'in yazanları (init ve
// başlatıcı hariç) grafikteki `writes_var` erişim yerleriyle birebir aynıdır, hiçbiri fazla ya da eksik değildir.
func TestPackageStateAndGraphAgreeOnWriters(t *testing.T) {
	root := auditModule(t, varAccessFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	fromGraph := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind != edgeWritesVar {
			continue
		}
		for _, s := range e.Sites {
			fromGraph[fmt.Sprintf("%s@%s:%d", e.To[strings.LastIndex(e.To, ".")+1:], s.File, s.Line)] = true
		}
	}
	m, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	vars, _ := findPackageState(m)
	fromAudit := map[string]bool{}
	for _, sv := range vars {
		for _, w := range sv.Writers {
			fromAudit[fmt.Sprintf("%s@%s:%d", sv.Name, w.File, w.Line)] = true
		}
	}
	if len(fromAudit) < 6 {
		t.Fatalf("package-state found only %d writes: %v", len(fromAudit), fromAudit)
	}
	for k := range fromAudit {
		if !fromGraph[k] {
			t.Errorf("package-state writer %s is not a writes_var site", k)
		}
	}
	for k := range fromGraph {
		if !fromAudit[k] {
			t.Errorf("writes_var site %s is not a package-state writer", k)
		}
	}
}
