package main

import (
	"slices"
	"testing"
)

// Kural: gönderim tablosu yorumlayıcının kodundan okunur (elle liste yok): çağrı derleyicinin çağrı işlevine, metin
// yerleştirmesi değeri metne çeviren Value.Inspect'e, indeks VM.indexValue'ya, üye erişimi VM.memberValue'ya gider
// (deneme 9 H1, deneme 10, değer dili tasarımı 3. tur). Bir dal tanınmazsa test düşer.
func TestDispatchTableIsReadFromTheInterpreter(t *testing.T) {
	_, m, _ := repoGraph(t)
	d := readDispatch(m)
	const interp = "go:func:github.com/sekerahmet/TCB/internal/lang/interp."
	for kind, want := range map[string]string{
		"CallExpression":     interp + "compiler.call",
		"InterpolatedString": interp + "Value.Inspect",
		"IndexExpression":    interp + "VM.indexValue",
		"MemberExpression":   interp + "VM.memberValue",
	} {
		if !slices.Contains(d.kinds[kind], want) {
			t.Errorf("%s does not dispatch to %s: %v", kind, want, d.kinds[kind])
		}
	}
}

// Kural: `implemented_by` tests-for zincirine yalnızca ölçüm programlarından (tools/bench) girer; her TR testi çağrı
// kullandığı için, aksi hâlde yorumlayıcının her işlevi binlerce TR testiyle "sınanmış" görünürdü (kod-grafigi-40
// onayı, 3. madde).
func TestImplementedByJoinsTheChainOnlyFromMeasurements(t *testing.T) {
	to := "go:func:example.com/x/interp.VM.callPrepared"
	if !reachEdge(edgeImplementedBy, inferred, "tr:func:tools/bench/tr/fib.tr:fib", to) {
		t.Error("a bench program reaches the interpreter")
	}
	if reachEdge(edgeImplementedBy, inferred, "tr:test:lib/katman_test.tr:12", to) || reachEdge(edgeImplementedBy, inferred, "tr:func:lib/katman.tr:f", to) {
		t.Error("a TR test or library function does not count as a test of the interpreter through implemented_by")
	}
}
