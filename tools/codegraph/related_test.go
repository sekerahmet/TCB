package main

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: dil tanımının "related" bağı (internal/lang/codes/definitions.json) ad kodları arasında `related` kenarıdır;
// `query <ad>` akraba adları gösterir (deneme 8 S2: yazdır ↔ girdi, birleştir ↔ böl grafikte yoktu).
func TestRelatedNamesAreLinked(t *testing.T) {
	root := auditModule(t, map[string]string{
		"internal/lang/codes/tr.json":          "{\n  \"TR_mem_split\": { \"ad\": \"böl\", \"aile\": \"x\" },\n  \"TR_mem_join\": { \"ad\": \"birleştir\", \"aile\": \"x\" }\n}\n",
		"internal/lang/codes/definitions.json": "{\n  \"TR_mem_split\": { \"kind\": \"method\" },\n  \"TR_mem_join\": { \"kind\": \"method\", \"related\": [\"TR_mem_split\"] }\n}\n",
		"internal/lang/codes/codes.go":         "package codes\n\ntype Code int\n",
	})
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	mustEdge(t, g, "name:TR_mem_join", "name:TR_mem_split", edgeRelated, extracted)
	if e := edge(g, "name:TR_mem_split", "name:TR_mem_join", edgeRelated); e != nil {
		t.Errorf("only the direction the definition writes: %+v", e)
	}
	if code, _, errOut := runCLI("build", "--root", root); code != exitOK {
		t.Fatalf("build: %d %s", code, errOut)
	}
	_, out, _ := runCLI("query", "birleştir", "--root", root, "--no-rebuild")
	testkit.MustContain(t, out, "-> related (1)", "TR_mem_split")
}
