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
	"tests/kod_test.tr": "test \"17.7 kodsuz dosya Türkçedir\" {\n    yazdır(1)\n}\n",
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
