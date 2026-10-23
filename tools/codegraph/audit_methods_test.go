package main

import (
	"bytes"
	"strings"
	"testing"
)

const hooksSource = `package p

// OperatorHook ve TaskOperatorHook: yorumlayıcı önce ikincisini dener (TensorValue.Operator olayı).
type OperatorHook interface{ Operator(a int) int }
type TaskOperatorHook interface{ TaskOperator(a int) int }

type T struct{}

func (T) Operator(a int) int     { return a }
func (T) TaskOperator(a int) int { return a }

var (
	_ OperatorHook     = T{}
	_ TaskOperatorHook = T{}
)

func apply(v any) int {
	if k, ok := v.(TaskOperatorHook); ok {
		return k.TaskOperator(1)
	}
	if k, ok := v.(OperatorHook); ok {
		return k.Operator(1)
	}
	return 0
}

// Run apply'ı çağırır.
func Run() int { return apply(T{}) }

// U yalnızca Pinger denetimiyle tutulur; Pinger.Ping hiçbir yerde çağrılmaz.
type U struct{}

func (U) Ping() {}

type Pinger interface{ Ping() }

var _ Pinger = U{}

// V'nin metodunu kimse çağırmaz.
type V struct{}

func (V) Orphan() {}

// W'nin metodu yalnızca testten çağrılır.
type W struct{}

func (W) OnlyTest() {}

// X canlı: doğrudan çağrılır. Y canlı: arayüz üzerinden çağrılır.
type X struct{}

func (X) Live() {}

type Runner interface{ Go() }
type Y struct{}

func (Y) Go() {}

var _ Runner = Y{}

// Z yalnızca kapalı arayüz işaretçisidir: çağrılmaması beklenir, listelenmez.
type sealed interface{ mark() }
type Z struct{}

func (Z) mark() {}

var _ sealed = Z{}

func Use(r Runner) { X{}.Live(); r.Go() }
`

// Arayüz denetimiyle tutulan ölü yol: TensorValue.Operator olayının küçük modeli. Arayüz üzerinden
// çağrılan, doğrudan çağrılan ve dış sözleşme adlı metotlar listelenmez.
func TestDeadMethodsFindsInterfaceKeptMethods(t *testing.T) {
	root := auditModule(t, map[string]string{
		"p.go":      hooksSource,
		"p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestW(t *testing.T) { W{}.OnlyTest() }\n",
	})
	res := auditJSON(t, "dead-methods", root)
	got := map[string]string{}
	for _, v := range res["methods"].([]any) {
		m := v.(map[string]any)
		got[m["method"].(string)] = m["kind"].(string)
	}
	want := map[string]string{
		"audit.T.Operator": "shadowed", "audit.U.Ping": "assertion-only", "audit.V.Orphan": "unreferenced", "audit.W.OnlyTest": "tests-only",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	for _, live := range []string{"audit.T.TaskOperator", "audit.X.Live", "audit.Y.Go", "audit.Z.mark"} {
		if _, bad := got[live]; bad {
			t.Errorf("%s is reached and must not be listed", live)
		}
	}
	var out, errb bytes.Buffer
	run([]string{"dead-methods", "--root", root}, &out, &errb)
	if !strings.Contains(out.String(), "shadowed (1)") || !strings.Contains(out.String(), "returns first") {
		t.Errorf("text output names the shadowing check:\n%s", out.String())
	}
}
