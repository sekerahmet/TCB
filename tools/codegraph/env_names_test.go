package main

import (
	"strings"
	"testing"
)

var envFiles = map[string]string{
	"internal/lang/codes/tr.json": `{
  "TR_kw_if": { "ad": "eger", "aile": "keyword" },
  "TR_env_GPU_SYNC": { "ad": "TR_GPU_ESZAMANLI", "aile": "ortam değişkeni" },
  "TR_env_CPU_AVX2": { "ad": "TR_CPU_AVX2", "aile": "ortam değişkeni" }
}
`,
	"internal/lang/codes/en.json": `{
  "TR_env_GPU_SYNC": { "ad": "TR_GPU_SYNC" }
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
	mustEdge(t, g, "go:func:example.com/audit/app.Sync", "name:TR_env_GPU_SYNC", edgeUsesEnv, extracted)
	mustEdge(t, g, "go:func:example.com/audit/app.Alias", "name:TR_env_GPU_SYNC", edgeUsesEnv, extracted) // İngilizce ad
	mustEdge(t, g, "go:func:example.com/audit/app.Avx", "name:TR_env_CPU_AVX2", edgeUsesEnv, extracted)   // sabit ad
	mustEdge(t, g, "go:func:example.com/audit/app.hook", "name:TR_env_CPU_AVX2", edgeUsesEnv, extracted)  // testing.T.Setenv
	mustEdge(t, g, "msg:H4009", "name:TR_env_GPU_SYNC", edgeUsesName, extracted)
	if e := edge(g, "msg:H4009", "name:TR_env_GPU_SYNC", edgeUsesName); e.Count != 2 || e.File != "internal/lang/messages/tr.json" {
		t.Errorf("both languages' templates name TR_env_GPU_SYNC, first at tr.json: %+v", e)
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
	for _, query := range []string{"TR_env_GPU_SYNC", "TR_GPU_SYNC"} {
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

// envSetFiles ortam değişkenini koyan testler: Setenv, alt süreç ortamı dizesi (düz, sabitli, başı sabit toplama) ve
// test dosyasındaki bir yardımcı üzerinden; okuyan test ve HOME koyan test sınamaz.
var envSetFiles = map[string]string{
	"internal/lang/codes/tr.json": `{
  "TR_env_LANG": { "ad": "TR_DIL", "aile": "ortam değişkeni" },
  "TR_env_GPU_SYNC": { "ad": "TR_GPU_ESZAMANLI", "aile": "ortam değişkeni" },
  "TR_env_CPU_AVX2": { "ad": "TR_CPU_AVX2", "aile": "ortam değişkeni" }
}
`,
	"app/app.go": "package app\n\nfunc Run() {}\n",
	"kit/kit.go": `package kit

import (
	"os"
	"os/exec"
)

// WithLang yalnızca testlerden çağrılan ortak yardımcı.
func WithLang(lang string) { cmd := exec.Command("x"); cmd.Env = append(os.Environ(), "TR_DIL="+lang); _ = cmd }

// Var dilin ortam değişkeninin adıdır (başka paketten seçiciyle okunur).
const Var = "TR_GPU_ESZAMANLI"

// Serve üretim kodu: değişkeni kendi alt sürecine koyar, çağıran test onu sınamaz.
func Serve() { cmd := exec.Command("x"); cmd.Env = []string{"TR_CPU_AVX2=0"}; _ = cmd }

func Main() { Serve() }
`,
	"kit/kit_test.go": `package kit

import "testing"

func TestWithLang(t *testing.T) { WithLang("en") }

func TestServe(t *testing.T) { Serve() }
`,
	"other/other_test.go": `package other

import (
	"os"
	"os/exec"
	"testing"

	"example.com/audit/kit"
)

func TestSelectorConst(t *testing.T) {
	cmd := exec.Command("x")
	cmd.Env = append(os.Environ(), kit.Var+"=1")
	_ = cmd
}
`,
	"app/app_test.go": `package app

import (
	"os"
	"os/exec"
	"testing"
)

const langVar = "TR_DIL"

func TestSetenv(t *testing.T) { t.Setenv("TR_CPU_AVX2", "0"); Run() }

func TestChildEnv(t *testing.T) {
	cmd := exec.Command("x")
	cmd.Env = append(os.Environ(), "TR_DIL=en")
	_ = cmd
}

func TestConstEnv(t *testing.T) { run(t, []string{langVar + "=en"}) }

func TestPrefixEnv(t *testing.T) {
	for _, lang := range []string{"tr", "en"} {
		_ = []string{"TR_GPU_ESZAMANLI=" + lang}
	}
}

func TestReads(t *testing.T) { _ = os.Getenv("TR_DIL") }

func TestMessageOnly(t *testing.T) { t.Log("TR_DIL=en ile koşulmalı") }

func TestHome(t *testing.T) { t.Setenv("HOME", "/tmp") }

func run(t *testing.T, env []string) { _ = env }
`,
}

// Kural: bir ortam değişkenini koyan test (t.Setenv, os.Setenv, alt sürecin ortamındaki "AD=..." dizesi; kendisi ya
// da test dosyasındaki yardımcısı) o değişkenin ad kodunu sınar: `tests` kenarı; yalnızca okuyan test sınamaz (değer
// dili 3. tur: tests-for K400 TR_DIL'i koyan e2e testlerini göstermiyordu).
func TestTestsThatSetAnEnvironmentVariableTestItsCode(t *testing.T) {
	root := auditModule(t, envSetFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	for test, code := range map[string]string{
		"app.TestSetenv": "TR_env_CPU_AVX2", "app.TestChildEnv": "TR_env_LANG", "app.TestConstEnv": "TR_env_LANG", "app.TestPrefixEnv": "TR_env_GPU_SYNC",
		"kit.TestWithLang": "TR_env_LANG", "other.TestSelectorConst": "TR_env_GPU_SYNC",
	} {
		mustEdge(t, g, "go:func:example.com/audit/"+test, "name:"+code, edgeTests, extracted)
	}
	if e := edge(g, "go:func:example.com/audit/kit.TestServe", "name:TR_env_CPU_AVX2", edgeTests); e != nil {
		t.Errorf("Serve is production code (Main calls it); its child's variable is not TestServe's: %+v", e)
	}
	for _, e := range g.Edges {
		if e.Kind == edgeTests && strings.HasPrefix(e.To, "name:") && (strings.HasSuffix(e.From, ".TestReads") || strings.HasSuffix(e.From, ".TestHome") || strings.HasSuffix(e.From, ".TestMessageOnly")) {
			t.Errorf("reading TR_DIL or setting HOME tests no name code: %+v", e)
		}
	}
}

// Kural: dilin her ortam değişkeni ad kodunu (K4xx) koyan en az bir test vardır (kendisi, test dosyasındaki ya da
// yalnızca testlerin çağırdığı bir yardımcı üzerinden): değişkenin etkisi sınanır, yalnızca okunması değil. Yeni bir
// ortam değişkeni koyan testi olmadan eklenirse bu test düşer; istisna listesi yoktur (değer dili 3. tur).
func TestEveryEnvironmentVariableIsSetByATest(t *testing.T) {
	root, _, g := repoGraph(t)
	codes := map[string]bool{}
	for _, code := range envCodes(root) {
		codes[code] = true
	}
	if len(codes) < 5 {
		t.Fatalf("only %d environment-variable codes found; the code files were not read", len(codes))
	}
	set := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeTests && e.Confidence == extracted && strings.HasPrefix(e.From, "go:func:") {
			set[strings.TrimPrefix(e.To, "name:")] = true
		}
	}
	for code := range codes {
		if !set[code] {
			t.Errorf("%s: no test sets it (t.Setenv, os.Setenv or \"NAME=...\" in a child's environment)", code)
		}
	}
}
