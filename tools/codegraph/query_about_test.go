package main

import (
	"sort"
	"strings"
	"testing"
)

// Kural (README, tests-for'un ilk grubu): bir testin "expects Hxxxx" ile hedefi konu edinmesi için iletiyi hedef ve
// yalnız onun çağırdıkları üretir. Hedef bir ad koduysa hedefin işlevleri onu uygulayan (test dosyası dışındaki) Go
// işlevleridir; birden çok hedefin çağırdığı işlevin iletisi hiçbirinin değildir. Grafiğin düz kenarlarından, tests-for
// kodundan bağımsız olarak denetlenir: her (test olmayan) üretici ya hedefin işlevidir ya da onların çağırdığı ve test
// dışında yalnız onların çağırdığı bir işlevdir. Hedefler planlama sınavınınkiler: her TR işlevi, kullanılan her ad kodu,
// internal/stdlib'in her işlevi (eksikler listesi, kritik, ana oturum 2026-10-26: `tests-for TR_fn_sum` H6126'yı bekleyen testi
// ilk grupta veriyordu; H6126'yı yalnız ml.matmulDType üretir ve toplam'dan oraya yol yok).
func TestAnExpectedMessageIsTheTargetsOwn(t *testing.T) {
	_, _, g := repoGraph(t)
	ix := newIndex(g) // kök yok: TR testinin satırını okuyan yedek yol kapalı, yalnız kenarla bekleme sınanır
	var candidates []*Node
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if !isTestNode(n) {
			continue
		}
		for _, e := range ix.out[n.ID] {
			if e.Kind == edgeUsesMessage {
				candidates = append(candidates, n)
				break
			}
		}
	}
	var bad []string
	checked := 0
	for i := range g.Nodes {
		n := &g.Nodes[i]
		switch {
		case n.Kind == kindTRFunc || n.Kind == kindTRMethod:
		case n.Kind == kindName && len(ix.in[n.ID]) > 0:
		case n.Kind == kindFunc && strings.HasPrefix(n.File, "internal/stdlib/") && !inTestFile(n):
		default:
			continue
		}
		own := ownFunctions(ix, n)
		for _, a := range ix.aboutTests(n, candidates) {
			code, ok := strings.CutPrefix(a.reason, "expects ")
			if !ok {
				continue
			}
			checked++
			for _, e := range ix.in["msg:"+code] {
				if p := ix.byID[e.From]; e.Kind == edgeUsesMessage && p != nil && !testCode(p) && !own[p.ID] {
					bad = append(bad, n.Name+": "+a.test.Name+" expects "+code+", which "+p.Name+" produces")
				}
			}
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d expected messages are not the target's own:\n  %s", len(bad), strings.Join(bad[:min(len(bad), 20)], "\n  "))
	}
	if checked < 20 {
		t.Errorf("only %d tests expect a message of their target; the check is broken", checked)
	}
}

// testCode test ya da test dosyasındaki (Go ya da TR) yardımcı mı (isTestCode'dan bağımsız yazım).
func testCode(n *Node) bool {
	return n.Kind == kindTRTest || strings.HasSuffix(n.File, "_test.go") || strings.HasSuffix(n.File, "_test.tr")
}

// ownFunctions hedefin işlevleri ve yalnız onların çağırdığı işlevlerdir (README kuralının düz kenarlarla yazımı).
func ownFunctions(ix *index, n *Node) map[string]bool {
	own := map[string]bool{n.ID: true}
	if n.Kind == kindName {
		for _, e := range ix.in[n.ID] {
			if p := ix.byID[e.From]; e.Kind == edgeUsesName && p != nil && strings.HasPrefix(p.ID, "go:") && !testCode(p) {
				own[p.ID] = true
			}
		}
	}
	callees := map[string]bool{}
	for id := range own {
		for _, e := range ix.out[id] {
			if e.Kind == edgeCalls && e.Confidence == extracted {
				callees[e.To] = true
			}
		}
	}
	for c := range callees {
		only := true
		for _, e := range ix.in[c] {
			if p := ix.byID[e.From]; e.Kind == edgeCalls && p != nil && !testCode(p) && !own[p.ID] {
				only = false
			}
		}
		if only {
			own[c] = true
		}
	}
	return own
}
