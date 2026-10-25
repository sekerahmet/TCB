package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Kural: dilin her yerleşik, üye ve adlı argüman kodu (G/T/V/W/Y) için `tests-for <kod>` doğrudan en az bir test verir
// ya da kod gerekçesiyle testdata/untested_names.txt'te durur. "Doğrudan": bir TR test bloğunun ya da Go davranış
// testinin (TR kodunu dize olarak çalıştıran) o adı kullanması; yalnızca listeyi gezen testler (`walks`) sayılmaz, çünkü
// onlar her adı sayıp hiçbirini davranışıyla sınamaz. Yeni bir yerleşik eklenip testsiz bırakılırsa ya da bir testin
// silinmesiyle bir ad testsiz kalırsa bu test düşer; bayat bir istisna da düşürür; liste yalnızca küçülür (onlyShrinks).
func TestEveryBuiltinNameHasADirectTestOrAReason(t *testing.T) {
	const maxUntestedNames = 3 // testdata/untested_names.txt: yalnızca küçülür (onlyShrinks)
	_, _, g := repoGraph(t)    // sınavla paylaşılan yükleme
	direct := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeTests {
			direct[e.To] = true
		}
	}
	data, err := os.ReadFile(filepath.Join("testdata", "untested_names.txt"))
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		code, reason, ok := strings.Cut(line, "\t")
		if !ok || strings.TrimSpace(reason) == "" {
			t.Fatalf("testdata/untested_names.txt: %q: want a code, a tab and a reason", line)
		}
		reasons[code] = reason
	}
	onlyShrinks(t, "testdata/untested_names.txt", len(reasons), maxUntestedNames)
	var blind, stale []string
	live := map[string]bool{}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if !implementable(n) {
			continue
		}
		live[n.Name] = true
		_, excepted := reasons[n.Name]
		switch {
		case !direct[n.ID] && !excepted:
			blind = append(blind, n.Name+" "+n.Detail)
		case direct[n.ID] && excepted:
			stale = append(stale, n.Name)
		}
	}
	for code := range reasons {
		if !live[code] {
			stale = append(stale, code+" (not a live code)")
		}
	}
	sort.Strings(blind)
	sort.Strings(stale)
	if len(blind) > 0 {
		t.Errorf("%d builtin, member or named-argument code(s) no test uses (tests-for gives nothing direct): write a test (a test block or a Go test that runs TR code), or list the code with a reason in testdata/untested_names.txt:\n  %s",
			len(blind), strings.Join(blind, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("testdata/untested_names.txt lists code(s) that have a test now (or are not live): remove them:\n  %s", strings.Join(stale, "\n  "))
	}
}

var nameTestFiles = map[string]string{
	"internal/lang/codes/tr.json": "{\n  \"TR_mem_update\": { \"ad\": \"güncelle\", \"aile\": \"x\" }\n}\n",
	"internal/lang/codes/codes.go": `package codes

type Code int

const MemUpdate Code = 1 // TR_mem_update: update
`,
	"app/impl.go": `package app

import "example.com/audit/internal/lang/codes"

func record(c codes.Code) {}

// Save adımı kaydeder.
func Save() {}

// Update güncelle metodudur.
func Update() {
	record(codes.MemUpdate)
	Save()
}
`,
	"tests/x_test.tr": "test \"adım ilerlemesi bildirilir\" {\n    değişken kn = kontrol_noktası(1, {})\n    kn.güncelle(1)\n}\n",
}

// Kural: bir Go işlevi, uyguladığı (kodunu kullandığı) dil adının testleriyle ve onu çağıran işlevlerin uyguladığı adların
// testleriyle sınanmış sayılır: `tests-for app.Save`, Save'i çağıran Update'in uyguladığı güncelle'nin TR test bloğunu
// "tests through the name codes" altında verir.
func TestAGoFunctionIsTestedThroughTheNameCodesItsCallersImplement(t *testing.T) {
	root := auditModule(t, nameTestFiles)
	var out, errb strings.Builder
	if code := run([]string{"build", "--root", root}, &out, &errb); code != exitOK {
		t.Fatalf("build: %d %s", code, errb.String())
	}
	for _, fn := range []string{"app.Update", "app.Save"} {
		out.Reset()
		if code := run([]string{"tests-for", fn, "--root", root, "--no-rebuild"}, &out, &errb); code != exitOK {
			t.Fatalf("tests-for %s: %d %s", fn, code, errb.String())
		}
		if !strings.Contains(out.String(), "tests through the name codes") || !strings.Contains(out.String(), "adım ilerlemesi bildirilir") {
			t.Errorf("tests-for %s lacks the TR test block of TR_mem_update:\n%s", fn, out.String())
		}
	}
}

// Kural: ad kodu bağı bir Go testini ancak işlevin paketi o testin ikilisine giriyorsa (aynı paket ya da testin paketi onu
// dolaylı içe aktarıyor) işlevin testi sayar. `package main`'deki bir işlevi başka paketin testi hiç çalıştıramaz;
// kodun TR testi (tr ikilisinde koşar) sayılmaya devam eder (TCB 2026-10-25, 3. madde: tr komutunun ana dosyasının
// tests-for cevabının başında checker testleri vardı).
func TestNameCodeTestsMustLinkTheFunctionsPackage(t *testing.T) {
	const m = "example.com/m"
	g := &Graph{Module: m, Nodes: []Node{
		{ID: pkgID(m + "/cmd/app"), Kind: kindPackage},
		{ID: pkgID(m + "/checker"), Kind: kindPackage},
		{ID: pkgID(m + "/lib"), Kind: kindPackage},
		{ID: pkgID(m + "/core"), Kind: kindPackage},
		{ID: "go:func:" + m + "/cmd/app.startsWithPrint", Kind: kindFunc, Name: "app.startsWithPrint", File: "cmd/app/main.go"},
		{ID: "go:func:" + m + "/core.Print", Kind: kindFunc, Name: "core.Print", File: "core/print.go"},
		{ID: "name:TR_fn_print", Kind: kindName, Name: "TR_fn_print"},
		{ID: "go:func:" + m + "/checker.TestPrintWarning", Kind: kindFunc, Name: "checker.TestPrintWarning", File: "checker/x_test.go"},
		{ID: "go:func:" + m + "/cmd/app.TestShell", Kind: kindFunc, Name: "app.TestShell", File: "cmd/app/main_test.go"},
		{ID: "tr:test:tests/a_test.tr:1", Kind: kindTRTest, Name: "a_test: yazdır", File: "tests/a_test.tr"},
	}, Edges: []Edge{
		{From: pkgID(m + "/checker"), To: pkgID(m + "/lib"), Kind: edgeImports},
		{From: pkgID(m + "/lib"), To: pkgID(m + "/core"), Kind: edgeImports}, // checker → lib → core: dolaylı
		{From: "go:func:" + m + "/cmd/app.startsWithPrint", To: "name:TR_fn_print", Kind: edgeUsesName},
		{From: "go:func:" + m + "/core.Print", To: "name:TR_fn_print", Kind: edgeUsesName},
		{From: "go:func:" + m + "/checker.TestPrintWarning", To: "name:TR_fn_print", Kind: edgeTests},
		{From: "go:func:" + m + "/cmd/app.TestShell", To: "name:TR_fn_print", Kind: edgeTests},
		{From: "tr:test:tests/a_test.tr:1", To: "name:TR_fn_print", Kind: edgeTests},
	}}
	ix := newIndex(g)
	names := func(fn string) string {
		var out []string
		for _, nt := range ix.testsThroughNames(ix.byID["go:func:"+m+"/"+fn]) {
			out = append(out, nt.test.Name)
		}
		sort.Strings(out)
		return strings.Join(out, ", ")
	}
	if got, want := names("cmd/app.startsWithPrint"), "a_test: yazdır, app.TestShell"; got != want {
		t.Errorf("main package function: got %q, want %q (another package's test cannot run it)", got, want)
	}
	if got, want := names("core.Print"), "a_test: yazdır, checker.TestPrintWarning"; got != want {
		t.Errorf("imported (transitively) package: got %q, want %q", got, want)
	}
}
