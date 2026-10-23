package main

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: dil tanımının "ilişkili" bağı (internal/lang/codes/definitions.json) ad kodları arasında `related` kenarıdır;
// `query <ad>` akraba adları gösterir (deneme 8 S2: yazdır ↔ girdi, birleştir ↔ böl grafikte yoktu).
func TestRelatedNamesAreLinked(t *testing.T) {
	root := auditModule(t, map[string]string{
		"internal/lang/codes/tr.json":          "{\n  \"Y103\": { \"ad\": \"böl\", \"aile\": \"x\" },\n  \"Y203\": { \"ad\": \"birleştir\", \"aile\": \"x\" }\n}\n",
		"internal/lang/codes/definitions.json": "{\n  \"Y103\": { \"tür\": \"metot\" },\n  \"Y203\": { \"tür\": \"metot\", \"ilişkili\": [\"Y103\"] }\n}\n",
		"internal/lang/codes/codes.go":         "package codes\n\ntype Code int\n",
	})
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	mustEdge(t, g, "name:Y203", "name:Y103", edgeRelated, extracted)
	if e := edge(g, "name:Y103", "name:Y203", edgeRelated); e != nil {
		t.Errorf("only the direction the definition writes: %+v", e)
	}
	if code, _, errOut := runCLI("build", "--root", root); code != exitOK {
		t.Fatalf("build: %d %s", code, errOut)
	}
	_, out, _ := runCLI("query", "birleştir", "--root", root, "--no-rebuild")
	testkit.MustContain(t, out, "-> related (1)", "Y103")
}
