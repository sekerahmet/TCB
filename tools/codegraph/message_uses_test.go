package main

import (
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

var messageCodeName = regexp.MustCompile(`^H\d{4}$`)

// Kural: ileti kodunun her kullanımı grafikte bir `uses_message` kenarıdır; who-uses ve tests-for bir iletiyi üreten
// ve bekleyen yeri kaçırmaz. Kapsam, grafikten bağımsız üç yoldan: Go'da go/types'ın messages paketinin bir ileti
// sabitine (messages.H6932, paketin içinde H9000) gördüğü her kullanım; .tr kaynağında ileti kodu olan her metin
// (`TR_Message("H6932", x)`, `TR_error_code(h) == "H6932"`); negatif fikstürlerdeki (`tests/negative/*.tr`) her satır
// sonu etiketi (`# ERROR H1026`, `# WARNING H1012`, `# RUN-ERROR H2478`). Her biri için, kullanımın bulunduğu dosyadaki
// bir düğümden o kodun ileti düğümüne `uses_message` kenarı olmalıdır.
func TestEveryMessageCodeUseIsAnEdge(t *testing.T) {
	root, m, g := repoGraph(t)
	want := map[string]bool{} // "kod dosya"
	for _, u := range m.Units {
		for id, obj := range u.Info.Uses {
			if c, ok := obj.(*types.Const); ok && c.Pkg() != nil && c.Pkg().Path() == m.Path+"/"+messagesDir && messageCodeName.MatchString(c.Name()) {
				want[c.Name()+" "+m.Rel(m.Fset.Position(id.Pos()).Filename)] = true
			}
		}
	}
	goUses := len(want)
	fixtureUses := 0
	for _, n := range g.Nodes {
		if n.Kind != kindFile || !strings.HasSuffix(n.File, ".tr") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.File)))
		if err != nil {
			t.Fatal(err)
		}
		if codeindex.IsNegativeFixture(n.File) {
			diags, _ := codeindex.FixtureLabels(string(data))
			for _, d := range diags {
				if messageCodeName.MatchString(d.Code) {
					want[d.Code+" "+n.File] = true
					fixtureUses++
				}
			}
		}
		b, err := source.Split(string(data))
		if err != nil {
			continue
		}
		l := b.Lexer()
		for tok := l.NextToken(); tok.Type != lexer.TokEOF; tok = l.NextToken() {
			// İleti kodu bir metin olarak yazılır ("H6903").
			if tok.Type == lexer.TokString && messageCodeName.MatchString(tok.Literal) {
				want[tok.Literal+" "+n.File] = true
			}
		}
	}
	if goUses < 1000 || len(want)-goUses < 20 || fixtureUses < 50 {
		t.Fatalf("%d Go, %d .tr and %d negative fixture message code uses; the scan is broken", goUses, len(want)-goUses, fixtureUses)
	}
	file := map[string]string{}
	for _, n := range g.Nodes {
		file[n.ID] = n.File
	}
	have := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeUsesMessage && strings.HasPrefix(e.To, "msg:") {
			have[strings.TrimPrefix(e.To, "msg:")+" "+file[e.From]] = true
		}
	}
	var missing []string
	for k := range want {
		if !have[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d message code uses have no uses_message edge from their file (of %d), e.g. %v", len(missing), len(want), missing[:min(len(missing), 10)])
	}
}

// Kural: negatif fikstürün üç etiket türü (`ERROR`, `WARNING`, `RUN-ERROR`) ve çok kodlu etiketleri (`# ERROR H1026 H1027`)
// ileti düğümüne `uses_message` kenarı kurar; etiket olmayan yorumdaki H-kodu bağ kurmaz; fikstür yalnız iletiyi gerçekten
// üreten dosyanın testidir (başka dosyanın da ürettiği ortak ileti onun değildir) ve koşu satırıyla `changed --go-tests`
// seçimine `TestNegativeFixtures` girer.
func TestNegativeFixtureLabelsAndTestsForProducerOnly(t *testing.T) {
	dir := t.TempDir()
	negDir := filepath.Join(dir, "tests", "negative")
	if err := os.MkdirAll(negDir, 0o755); err != nil {
		t.Fatal(err)
	}
	negSrc := "# TR 4\n" +
		"# TEST 2.6 etiket türleri ve yorumda H1028\n" +
		"TR_var a = 1 # sıradan yorum H1029\n" +
		"TR_var b = 2 # ERROR H1026 H1027\n" +
		"TR_var c = 3 # WARNING H1012\n" +
		"TR_var d = 4 # RUN-ERROR H2478\n"
	if err := os.WriteFile(filepath.Join(negDir, "etiketler.tr"), []byte(negSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	b := newBuilder("example.com/mini")
	for _, code := range []string{"H1026", "H1027", "H1012", "H2478", "H1028", "H1029", "H2000"} {
		b.addNode(Node{ID: "msg:" + code, Kind: kindMessage, Name: code})
	}
	extractTR(dir, b, nil)
	g := b.graph()
	for _, code := range []string{"H1026", "H1027", "H1012", "H2478"} {
		mustEdge(t, g, "tr:file:tests/negative/etiketler.tr", "msg:"+code, edgeUsesMessage, extracted)
	}
	for _, code := range []string{"H1028", "H1029"} {
		if e := edge(g, "tr:file:tests/negative/etiketler.tr", "msg:"+code, edgeUsesMessage); e != nil {
			t.Errorf("non-label comment code %s created an edge: %+v", code, e)
		}
	}

	// Kural: fikstür yalnız iletiyi gerçekten üreten dosyanın testidir; başka dosyanın ya da ortak iletinin testi değildir.
	g2 := &Graph{
		Nodes: []Node{
			{ID: "go:file:pkg/unpack.go", Kind: kindFile, Name: "pkg/unpack.go", File: "pkg/unpack.go", Line: 1},
			{ID: "go:func:pkg/unpack.go:checkUnpack", Kind: kindFunc, Name: "pkg.checkUnpack", File: "pkg/unpack.go", Line: 5},
			{ID: "go:file:pkg/other.go", Kind: kindFile, Name: "pkg/other.go", File: "pkg/other.go", Line: 1},
			{ID: "go:func:pkg/other.go:checkOther", Kind: kindFunc, Name: "pkg.checkOther", File: "pkg/other.go", Line: 5},
			{ID: "go:func:internal/devtools/testrunner/behavior_negative_fixtures_test.go:TestNegativeFixtures", Kind: kindFunc, Name: "testrunner.TestNegativeFixtures", File: "internal/devtools/testrunner/behavior_negative_fixtures_test.go", Line: 55},
			{ID: "go:func:internal/devtools/codeindex/resolve.go:FixtureLabels", Kind: kindFunc, Name: "codeindex.FixtureLabels", File: "internal/devtools/codeindex/resolve.go", Line: 239},
			{ID: "tr:file:tests/negative/demet_acma.tr", Kind: kindFile, Name: "tests/negative/demet_acma.tr", File: "tests/negative/demet_acma.tr", Line: 1},
			{ID: "tr:file:tests/negative/ortak.tr", Kind: kindFile, Name: "tests/negative/ortak.tr", File: "tests/negative/ortak.tr", Line: 1},
			{ID: "msg:H1026", Kind: kindMessage, Name: "H1026"},
			{ID: "msg:H2000", Kind: kindMessage, Name: "H2000"},
		},
		Edges: []Edge{
			{From: "go:file:pkg/unpack.go", To: "go:func:pkg/unpack.go:checkUnpack", Kind: edgeContains, Confidence: extracted},
			{From: "go:file:pkg/other.go", To: "go:func:pkg/other.go:checkOther", Kind: edgeContains, Confidence: extracted},
			{From: "go:func:pkg/unpack.go:checkUnpack", To: "msg:H1026", Kind: edgeUsesMessage, Confidence: extracted},
			{From: "go:func:pkg/unpack.go:checkUnpack", To: "msg:H2000", Kind: edgeUsesMessage, Confidence: extracted},
			{From: "go:func:pkg/other.go", To: "msg:H2000", Kind: edgeUsesMessage, Confidence: extracted},
			{From: "go:func:pkg/other.go:checkOther", To: "msg:H2000", Kind: edgeUsesMessage, Confidence: extracted},
			{From: "tr:file:tests/negative/demet_acma.tr", To: "msg:H1026", Kind: edgeUsesMessage, Confidence: extracted},
			{From: "tr:file:tests/negative/ortak.tr", To: "msg:H2000", Kind: edgeUsesMessage, Confidence: extracted},
			{From: "go:func:internal/devtools/testrunner/behavior_negative_fixtures_test.go:TestNegativeFixtures", To: "tr:file:tests/negative/demet_acma.tr", Kind: edgeTests, Confidence: inferred},
			{From: "go:func:internal/devtools/testrunner/behavior_negative_fixtures_test.go:TestNegativeFixtures", To: "tr:file:tests/negative/ortak.tr", Kind: edgeTests, Confidence: inferred},
			{From: "go:func:internal/devtools/testrunner/behavior_negative_fixtures_test.go:TestNegativeFixtures", To: "go:func:internal/devtools/codeindex/resolve.go:FixtureLabels", Kind: edgeCalls, Confidence: extracted},
		},
	}
	ix := newIndex(g2)
	var unpackOut, otherOut strings.Builder
	ix.printTestsFor(&unpackOut, ix.byID["go:file:pkg/unpack.go"], 50)
	ix.printTestsFor(&otherOut, ix.byID["go:file:pkg/other.go"], 50)
	if !strings.Contains(unpackOut.String(), "tests/negative/demet_acma.tr [file] tests/negative/demet_acma.tr:1  [expects H1026]") ||
		!strings.Contains(unpackOut.String(), "./internal/devtools/testrunner/") ||
		!strings.Contains(unpackOut.String(), "TestNegativeFixtures") {
		t.Errorf("tests-for pkg/unpack.go missing demet_acma.tr or TestNegativeFixtures:\n%s", unpackOut.String())
	}
	if strings.Contains(unpackOut.String(), "ortak.tr") {
		t.Errorf("tests-for pkg/unpack.go should not include ortak.tr for shared message H2000:\n%s", unpackOut.String())
	}
	if strings.Contains(otherOut.String(), "demet_acma.tr") || strings.Contains(otherOut.String(), "ortak.tr") || strings.Contains(otherOut.String(), "TestNegativeFixtures") {
		t.Errorf("tests-for pkg/other.go should not include negative fixtures it does not uniquely produce:\n%s", otherOut.String())
	}
}

// Kural: TR testi bir iletiyi çoğu zaman metniyle bekler (`h.içerir("'Nokta' yapısının 'yy' adında bir alanı yok")`).
// Test bloğundaki bir metin sabiti bir iletinin şablonunun sabit parçalarını sırayla taşıyorsa test o koda bağlanır
// (`uses_message`, çıkarım): tests-for H2418 o testi verir.
func TestATRTestExpectingAMessageTextUsesTheMessage(t *testing.T) {
	_, _, g := repoGraph(t)
	for _, c := range []struct{ code, file, title string }{
		{"H2418", "tests/yapilar_test.tr", "7.2 fazla konumlu ya da bilinmeyen adlı argüman hatadır"},
	} {
		found := false
		for _, e := range g.Edges {
			if e.Kind != edgeUsesMessage || e.To != "msg:"+c.code {
				continue
			}
			for _, n := range g.Nodes {
				if n.ID == e.From && n.Kind == kindTRTest && n.File == c.file && strings.HasSuffix(n.Name, c.title) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: the test %q in %s expects its text but has no uses_message edge", c.code, c.title, c.file)
		}
	}
}
