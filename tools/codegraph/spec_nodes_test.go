package main

import (
	"strings"
	"testing"
)

var specNodeFiles = map[string]string{
	"docs/spec/17_kodlar.md": "**17.7** Yazım dilleri: gövde.\n**17.8** Ortam adları: gövde.\n",
	"docs/guide.md":          "# Rehber\n\nYazım dilleri için dil tanımı 17.7'ye bakın.\n",
	"app/app.go": `package app

// Compile yazım dilini okur (docs/spec/17_kodlar.md, 17.7).
func Compile() {}

func Plain() {
	// Bu satır kural 17.8 ile ilgilidir.
}
`,
	"app/app_test.go": `package app

import "testing"

// Kural (dil tanımı 17.7): yazım dili dosyanın dilidir.
func TestWritingLanguage(t *testing.T) { Compile() }
`,
	"tests/kod_test.tr": "TR_test \"17.7 kodsuz dosya Türkçedir\" {\n    TR_print(1)\n}\n",
}

// Kural: dil tanımı maddeleri düğümdür ve her atıf atıf yapan yerden (en iç bildirim, doc yorumunun bildirimi, başlık
// ya da dosya) maddeye `refs_spec` kenarıdır; atıf yapan bir test (Go ya da adı numarayla başlayan TR test bloğu) maddeyi
// sınar. `who-uses 17.7` etki listesini, `tests-for 17.7` testleri verir.
func TestSpecItemsAreNodesWithReferenceEdges(t *testing.T) {
	g, err := buildGraph(auditModule(t, specNodeFiles))
	if err != nil {
		t.Fatal(err)
	}
	const (
		item = "spec:17.7"
		app  = "go:func:example.com/audit/app."
	)
	if !hasNode(g, item) || !hasNode(g, "spec:17.8") {
		t.Fatal("spec items are nodes")
	}
	mustEdge(t, g, app+"Compile", item, edgeRefsSpec, extracted) // doc yorumundaki atıf bildirime gider
	mustEdge(t, g, app+"Plain", "spec:17.8", edgeRefsSpec, extracted)
	mustEdge(t, g, app+"TestWritingLanguage", item, edgeRefsSpec, extracted)
	mustEdge(t, g, app+"TestWritingLanguage", item, edgeTests, inferred)
	mustEdge(t, g, "heading:docs/guide.md#rehber", item, edgeRefsSpec, extracted)
	for _, e := range g.Edges {
		if e.Kind == edgeTests && e.To == item && strings.HasPrefix(e.From, "tr:test:tests/kod_test.tr") {
			return
		}
	}
	t.Error("a TR test block named after the item tests it")
}

// Kural (DENETİM istek 66): <takma ad>.TR_<ad> yalnız takma adın modülündeki bildirime çözülür; kodu olmayan
// üye yazımı ise maddenin gövdesinde (kod bloğu dahil) geçse bile alıcısından bağımsız olarak en öncelikli spec:
// maddesine çözülür.
func TestSpecMemberAliasesAndModuleScopedImportAliases(t *testing.T) {
	detachX := "x." + "TR_detach"
	detachY := "y." + "TR_detach"
	readP := "p." + "TR_read"
	readS := "akış." + "TR_read"
	katmanLin := "katman." + "TR_Linear"
	optNSA := "opt." + "TR_NS_A"
	optLin := "opt." + "TR_Linear"
	geoLin := "geo." + "TR_Linear"

	g, err := buildGraph(auditModule(t, map[string]string{
		"app/app.go":              "package app\n",
		"docs/spec/02_sozcuk.md":  "**2.9** Göç notu: eski metinde `" + detachX + "()` geçer.\n",
		"docs/spec/15_tensor.md":  "**15.27a** `" + detachX + "()` tensörü çizgeden ayırır.\n",
		"docs/spec/21_surec.md":   "**21.2** Akış okuma:\n\n```" + "tr\n# TR 4\n" + readP + "()\n```\n",
		"lib/layers.tr":           "# TR 4\nTR_struct TR_Linear {\n    TR_weight: TR_Int = 1\n}\n",
		"lib/optimizers.tr":       "# TR 4\nTR_const TR_NS_A = 3.4445\n",
		"examples/helpers/geo.tr": "# TR 4\nTR_struct Kare {\n    kenar: TR_Int = 1\n}\n",
		"examples/app.tr":         "# TR 4\nTR_import \"tr/layers\" TR_as katman\nTR_import \"tr/optimizers\" TR_as opt\nTR_import \"helpers/geo\" TR_as geo\nTR_var d = " + katmanLin + "()\nTR_print(" + optNSA + ", geo.Kare())\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	ids := func(ns []*Node) []string {
		var out []string
		for _, n := range ns {
			out = append(out, n.ID)
		}
		return out
	}
	for q, want := range map[string]string{
		katmanLin:    "tr:struct:lib/layers.tr:TR_type_Linear",
		optNSA:       "tr:const:lib/optimizers.tr:TR_const_NS_A",
		detachX:      "spec:15.27a",
		detachY:      "spec:15.27a",
		".TR_detach": "spec:15.27a",
		readP:        "spec:21.2",
		readS:        "spec:21.2",
		".TR_read":   "spec:21.2",
	} {
		got := ids(ix.lookup(q))
		if len(got) != 1 || got[0] != want {
			t.Errorf("lookup(%q) = %v, want [%s]", q, got, want)
		}
	}
	for _, q := range []string{optLin, geoLin} {
		if got := ids(ix.lookup(q)); len(got) != 0 {
			t.Errorf("lookup(%q) = %v, want empty", q, got)
		}
	}
}
