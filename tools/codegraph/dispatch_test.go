package main

import (
	"slices"
	"testing"
)

// Kural: gönderim tablosu yorumlayıcının kodundan okunur (elle liste yok): `@` tensör kancasının matris çarpımına,
// `+` sayı ve tensör toplamasına, çağrı derleyicinin çağrı işlevine, metin yerleştirmesi değeri metne çeviren
// Value.Inspect'e gider (deneme 9 H1, deneme 10, değer dili tasarımı 3. tur). Bir dal tanınmazsa test düşer.
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
	if operandRules("type:A212", true) || !operandRules("type:A212", false) || operandRules("type:A200", false) ||
		!operandRules("type:A200", true) || !operandRules("", true) || !operandRules("", false) {
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
	if reachEdge(edgeImplementedBy, inferred, "tr:test:lib/katman_test.tr:12", to) || reachEdge(edgeImplementedBy, inferred, "tr:func:lib/katman.tr:f", to) {
		t.Error("a TR test or library function does not count as a test of the interpreter through implemented_by")
	}
}
