package main

import (
	"bytes"
	"strings"
	"testing"
)

const stateSource = `package p

import (
	"sync"
	"sync/atomic"
)

var table = map[string]int{"a": 1} // yalnızca okunur
var initOnly string                // yalnızca init'te yazılır
var counter int                    // Inc yazar
var registry = map[string]int{}    // Register öğe yazar
var warnOnce sync.Once             // eşzamanlı
var hits atomic.Int64              // eşzamanlı
var hook = func() {}               // yalnızca testler değiştirir
var lazy = func() { counter2++ }   // başlatıcıdaki işlev sonra çalışır
var counter2 int
var given = map[string]bool{}      // delete ile boşaltılır
var ticks int64                    // atomic.AddInt64(&ticks, 1)
var aliased [4]float32             // yalnızca adresi alınır
var guarded struct {               // kendi kilidiyle korunur
	mu sync.Mutex
	n  int
}

func init() { initOnly = "x" }

func Inc()                    { counter++ }
func Register(k string, v int) { registry[k] = v }
func Forget(k string)         { delete(given, k) }
func Get() int                { hook(); lazy(); return table["a"] + counter + len(initOnly) }

type T struct{ n int }

var current T

func (t *T) Set()  { current.n = 1 }
func Leak() *int   { return &counter }
func Tick()        { atomic.AddInt64(&ticks, 1) }
func Alias() int   { p := &aliased; return len(p) }
func Guard()       { guarded.mu.Lock(); guarded.n++; guarded.mu.Unlock() }
`

// Süreç geneli değişebilir durum: init dışında yazılan değişkenler, adı ve nasıl yazıldığıyla; yalnızca
// okunanlar, init'te yazılanlar listelenmez; sync/atomic tipliler ayrı işaretlenir.
func TestPackageStateListsProcessWideMutableVariables(t *testing.T) {
	root := auditModule(t, map[string]string{
		"p.go":      stateSource,
		"p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestHook(t *testing.T) { hook = func() {} }\n",
	})
	res := auditJSON(t, "package-state", root)
	kinds := map[string]string{}
	how := map[string]string{}
	for _, v := range res["variables"].([]any) {
		m := v.(map[string]any)
		name := m["name"].(string)
		kinds[name] = m["kind"].(string)
		if ws, ok := m["writers"].([]any); ok && len(ws) > 0 {
			how[name] = ws[0].(map[string]any)["how"].(string) + "@" + ws[0].(map[string]any)["func"].(string)
		}
	}
	want := map[string]string{"counter": "mutable", "registry": "mutable", "given": "mutable", "current": "mutable",
		"counter2": "mutable", "hook": "test-hook", "warnOnce": "synchronized", "hits": "synchronized",
		"ticks": "synchronized", "guarded": "synchronized", "aliased": "address-taken"}
	for k, v := range want {
		if kinds[k] != v {
			t.Errorf("%s: %q, want %q (all %v)", k, kinds[k], v, kinds)
		}
	}
	for _, quiet := range []string{"table", "initOnly", "lazy"} {
		if _, bad := kinds[quiet]; bad {
			t.Errorf("%s is read-only or set at init: must not be listed", quiet)
		}
	}
	for name, h := range map[string]string{"registry": "element/field write@Register", "given": "delete/clear@Forget", "current": "element/field write@T.Set", "counter2": "inc/dec@(var initializer func)"} {
		if how[name] != h {
			t.Errorf("%s written %q, want %q", name, how[name], h)
		}
	}
	if int(res["read_only"].(float64)) != 3 {
		t.Errorf("table, initOnly and lazy are only read after init: %v", res["read_only"])
	}
	var out, errb bytes.Buffer
	run([]string{"package-state", "--root", root}, &out, &errb)
	if !strings.Contains(out.String(), "mutable (") || !strings.Contains(out.String(), "Register (element/field write)") {
		t.Errorf("text:\n%s", out.String())
	}
}
