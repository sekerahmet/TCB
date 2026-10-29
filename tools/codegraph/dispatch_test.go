package main

import (
	"slices"
	"testing"
)

// Kural: gönderim tablosu yorumlayıcının kodundan okunur (elle liste yok): `@` tensör kancasının matris çarpımına,
// `+` sayı ve tensör toplamasına, çağrı derleyicinin çağrı işlevine, metin yerleştirmesi (derleme yardımcısı
// compiler.interpolation üzerinden) değeri metne çeviren valueText'e ve Value.Inspect'e gider (deneme 9 H1, deneme 10,
// değer dili tasarımı 3. tur). Bir dal tanınmazsa test düşer.
func TestDispatchTableIsReadFromTheInterpreter(t *testing.T) {
	_, m, _ := repoGraph(t)
	d := readDispatch(m)
	const interp, ml = "go:func:github.com/sekerahmet/TCB/internal/lang/interp.", "go:func:github.com/sekerahmet/TCB/internal/stdlib/ml."
	for _, c := range []struct {
		table map[string][]string
		key   string
		want  string
	}{
		{d.ops, "@", ml + "TensorValue.matmulOp"},
		{d.ops, "+", ml + "TensorValue.binary"},
		{d.ops, "+", interp + "floatOp"},
		{d.kinds, "CallExpression", interp + "compiler.call"},
		{d.kinds, "InterpolatedString", interp + "Value.Inspect"},
		{d.kinds, "InterpolatedString", interp + "valueText"},
		{d.kinds, "IndexExpression", interp + "VM.indexValue"},
		{d.kinds, "MemberExpression", interp + "VM.memberValue"},
	} {
		if !slices.Contains(c.table[c.key], c.want) {
			t.Errorf("%s does not dispatch to %s: %v", c.key, c.want, c.table[c.key])
		}
	}
	for _, id := range d.ops["+"] {
		if p := d.pkg[id]; p != m.Path+"/internal/lang/interp" && !slices.Contains([]string{m.Path + "/internal/stdlib/ml"}, p) {
			t.Errorf("an operator implementation outside the interpreter and its hooks: %s (%s)", id, p)
		}
	}
	// İşlenen türü bilinince yalnız o türün uygulaması: Tensör kancaya, sayı yorumlayıcıya; bilinmez ikisine.
	if operandRules("type:TR_type_Tensor", true) || !operandRules("type:TR_type_Tensor", false) || operandRules("type:TR_type_Int", false) ||
		!operandRules("type:TR_type_Int", true) || !operandRules("", true) || !operandRules("", false) {
		t.Error("operand rules: Tensör → hooks, numbers → interpreter, unknown → both")
	}
}

// Kural: `implemented_by` tests-for zincirine yalnızca ölçüm programlarından (tools/bench) girer; her TR testi çağrı
// ve işleç kullandığı için, aksi hâlde yorumlayıcının her işlevi binlerce TR testiyle "sınanmış" görünürdü
// (kod-grafigi-40 onayı, 3. madde).
func TestImplementedByJoinsTheChainOnlyFromMeasurements(t *testing.T) {
	to := "go:func:example.com/x/interp.VM.callPrepared"
	if !reachEdge(edgeImplementedBy, inferred, "tr:func:tools/bench/tr/fib.tr:fib", to) {
		t.Error("a bench program reaches the interpreter")
	}
	if reachEdge(edgeImplementedBy, inferred, "tr:test:lib/layers_test.tr:12", to) || reachEdge(edgeImplementedBy, inferred, "tr:func:lib/layers.tr:f", to) {
		t.Error("a TR test or library function does not count as a test of the interpreter through implemented_by")
	}
}

// helperDispatchFiles bir dalın gövdesi derleme yardımcısına taşınmış küçük bir yorumlayıcıdır: A doğrudan, B yardımcı
// üzerinden, C yardımcının yardımcısı üzerinden; yardımcı genel derleme metodunu (expression) da çağırır.
var helperDispatchFiles = map[string]string{
	"internal/lang/ast/ast.go": "package ast\n\ntype A struct{}\ntype B struct{}\ntype C struct{}\n",
	"internal/lang/interp/interp.go": `package interp

import "example.com/audit/internal/lang/ast"

type compiler struct{}

type code func() int

func implA() int { return 1 }
func implB() int { return 2 }
func implC() int { return 3 }

func (c *compiler) expression(e any) code {
	switch e.(type) {
	case *ast.A:
		implA()
		return nil
	case *ast.B:
		return c.helperB()
	case *ast.C:
		return c.helperC()
	}
	return nil
}

func (c *compiler) helperB() code {
	inner := c.expression(nil)
	return func() int { return implB() + inner() }
}

func (c *compiler) helperC() code { return c.deeper() }

func (c *compiler) deeper() code { return func() int { return implC() } }
`,
}

// Kural: dil yapısı → uygulama bağı dalın çağırdığı derleme yardımcılarını da izler (zincir halinde): bir dalın
// gövdesi bir yardımcıya ya da yardımcının yardımcısına taşınınca bağ kopmaz; yardımcının çağırdığı genel derleme
// metodu (kendi AST switch'i olan expression) yapıların hepsini dağıttığı için o yapının uygulaması sayılmaz
// (değer dili adım 1-2: yerleştirme compiler.interpolation'a taşınınca Value.Inspect bağı kopmuştu).
func TestConstructLinksFollowCompileHelpers(t *testing.T) {
	root := auditModule(t, helperDispatchFiles)
	m, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	d := readDispatch(m)
	const p = "go:func:example.com/audit/internal/lang/interp."
	for kind, want := range map[string]string{"A": "implA", "B": "implB", "C": "implC"} {
		if !slices.Contains(d.kinds[kind], p+want) {
			t.Errorf("%s is implemented by %s: %v", kind, want, d.kinds[kind])
		}
	}
	for kind, list := range d.kinds {
		if slices.Contains(list, p+"compiler.expression") {
			t.Errorf("%s reaches the general compile method through a helper: %v", kind, list)
		}
	}
	if slices.Contains(d.kinds["B"], p+"implA") || slices.Contains(d.kinds["C"], p+"implB") {
		t.Errorf("a helper does not carry the other branches: B %v, C %v", d.kinds["B"], d.kinds["C"])
	}
}
