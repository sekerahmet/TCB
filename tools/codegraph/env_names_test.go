package main

import (
	"strings"
	"testing"
)

var envFiles = map[string]string{
	"internal/lang/codes/tr.json": `{
  "A001": { "ad": "eger", "aile": "keyword" },
  "K402": { "ad": "TR_GPU_ESZAMANLI", "aile": "ortam değişkeni" },
  "K404": { "ad": "TR_CPU_AVX2", "aile": "ortam değişkeni" }
}
`,
	"internal/lang/codes/en.json": `{
  "K402": { "ad": "TR_GPU_SYNC" }
}
`,
	"internal/lang/messages/tr.json": `{
"H4009": "kernel başarısız; bulmak için TR_GPU_ESZAMANLI=1 ile çalıştırın",
"H0002": "ortam adı yok"
}
`,
	"internal/lang/messages/en.json": `{
"H4009": "kernel failed; run with TR_GPU_SYNC=1 to locate it"
}
`,
	"app/app.go": `package app

import (
	"os"
	"testing"
)

const avx = "TR_CPU_AVX2"

func Sync() bool { return os.Getenv("TR_GPU_ESZAMANLI") == "1" }

func Avx() bool { v, ok := os.LookupEnv(avx); return ok && v != "0" }

func Other() string { return os.Getenv("HOME") }

func Alias() string { return os.Getenv("TR_GPU_SYNC") }

func hook(t *testing.T) { t.Setenv("TR_CPU_AVX2", "0") }
`,
}

// Kural: os.Getenv/LookupEnv/Setenv'e verilen ve bir K4xx adına (herhangi bir dildeki) eşit dize sabiti o ad koduna
// `uses_env` kenarıyla bağlanır; sabit adlar çözülür, başka değişkenler bağlanmaz. Bir ileti şablonu K4xx adı
// taşıyorsa ileti koda `uses_name` kenarıyla bağlanır (her dilin kendi adı); who-uses ve query bunları gösterir.
func TestEnvironmentNamesLinkToTheirCodes(t *testing.T) {
	root := auditModule(t, envFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	mustEdge(t, g, "go:func:example.com/audit/app.Sync", "name:K402", edgeUsesEnv, extracted)
	mustEdge(t, g, "go:func:example.com/audit/app.Alias", "name:K402", edgeUsesEnv, extracted) // İngilizce ad
	mustEdge(t, g, "go:func:example.com/audit/app.Avx", "name:K404", edgeUsesEnv, extracted)   // sabit ad
	mustEdge(t, g, "go:func:example.com/audit/app.hook", "name:K404", edgeUsesEnv, extracted)  // testing.T.Setenv
	mustEdge(t, g, "msg:H4009", "name:K402", edgeUsesName, extracted)
	if e := edge(g, "msg:H4009", "name:K402", edgeUsesName); e.Count != 2 || e.File != "internal/lang/messages/tr.json" {
		t.Errorf("both languages' templates name K402, first at tr.json: %+v", e)
	}
	for _, e := range g.Edges {
		if e.Kind == edgeUsesEnv && e.From == "go:func:example.com/audit/app.Other" {
			t.Errorf("HOME is not a name code: %+v", e)
		}
		if e.From == "msg:H0002" && e.Kind == edgeUsesName {
			t.Errorf("H0002 names no variable: %+v", e)
		}
	}
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	for _, query := range []string{"K402", "TR_GPU_SYNC"} {
		out.Reset()
		if code := run([]string{"who-uses", query, "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
			t.Fatalf("who-uses %s: %d %s", query, code, errb.String())
		}
		for _, want := range []string{"app.Sync", "app.Alias", "H4009"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("who-uses %s lacks %s:\n%s", query, want, out.String())
			}
		}
	}
}
