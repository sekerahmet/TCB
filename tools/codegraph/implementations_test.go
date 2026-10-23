package main

import (
	"strings"
	"testing"
)

var dispatchFiles = map[string]string{
	"tensor/backend.go": `package tensor

// Backend tensör işlemlerinin geçtiği soyutlamadır.
type Backend interface {
	Softmax(x int) int
	Sum(x int) int
}

type cpuBackend struct{}

func (cpuBackend) Softmax(x int) int { return x }
func (cpuBackend) Sum(x int) int     { return x }

var active Backend = cpuBackend{}

// SoftmaxOp arayüz üzerinden çağırır.
func SoftmaxOp(x int) int { return active.Softmax(x) }
`,
	"gpu/backend.go": `package gpu

type backend struct{ inner }

type inner struct{}

func (inner) Sum(x int) int { return x }

func (backend) Softmax(x int) int { return x + 1 }
`,
	"gpu/register.go": `package gpu

import "example.com/audit/tensor"

var _ tensor.Backend = backend{}
`,
	"tensor/backend_test.go": `package tensor

import "testing"

// Softmax sonucu girdiyi korur.
func TestSoftmaxOp(t *testing.T) { _ = SoftmaxOp(1) }
`,
}

// Kural: bir arayüz yöntemi, onu uygulayan her yönteme INFERRED `dispatches_to` kenarı verir (gömülü alandan gelen
// uygulama dahil); `query` uygulamaları listeler, `callees` çağrının arayüzde kesilmediğini gösterir, `path` yönlü yolu
// uygulamaya kadar sürer ve `tests-for` uygulamanın testlerini arayüz çağrısının testlerinden bulur. Kural tek yerde
// (moduleImplementations) durur; dead-methods aynısını kullanır.
func TestInterfaceMethodsDispatchToTheirImplementations(t *testing.T) {
	root := auditModule(t, dispatchFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	const tens = "go:func:example.com/audit/tensor."
	mustEdge(t, g, tens+"Backend.Softmax", tens+"cpuBackend.Softmax", edgeDispatches, inferred)
	mustEdge(t, g, tens+"Backend.Softmax", "go:func:example.com/audit/gpu.backend.Softmax", edgeDispatches, inferred)
	mustEdge(t, g, tens+"Backend.Sum", "go:func:example.com/audit/gpu.inner.Sum", edgeDispatches, inferred) // gömülü alandan
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	ask := func(args ...string) string {
		out.Reset()
		if code := run(append(args, "--root", root, "--no-rebuild"), &out, &errb); code != exitOK {
			t.Fatalf("%v: %d %s", args, code, errb.String())
		}
		return out.String()
	}
	if got := ask("query", "tensor.Backend.Softmax"); !strings.Contains(got, "dispatches_to (2)") {
		t.Errorf("query lists both implementations:\n%s", got)
	}
	if got := ask("callees", "tensor.SoftmaxOp"); !strings.Contains(got, "dispatches to: gpu.backend.Softmax, tensor.cpuBackend.Softmax") {
		t.Errorf("callees shows the dispatch:\n%s", got)
	}
	if got := ask("path", "tensor.SoftmaxOp", "tensor.cpuBackend.Softmax"); !strings.Contains(got, "(directed, 2 steps)") {
		t.Errorf("path follows the dispatch:\n%s", got)
	}
	if got := ask("tests-for", "tensor.cpuBackend.Softmax"); !strings.Contains(got, "tensor.TestSoftmaxOp") || !strings.Contains(got, "[INFERRED]") {
		t.Errorf("tests-for reaches the implementation through the interface call:\n%s", got)
	}
}
