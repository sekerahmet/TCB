package main

import (
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

var messageCodeName = regexp.MustCompile(`^H\d{4}$`)

// Kural: ileti kodunun her kullanımı grafikte bir `uses_message` kenarıdır; who-uses ve tests-for bir iletiyi üreten
// ve bekleyen yeri kaçırmaz. Kapsam, grafikten bağımsız iki yoldan: Go'da go/types'ın messages paketinin bir ileti
// sabitine (messages.H6932, paketin içinde H9000) gördüğü her kullanım; .tr kaynağında ileti kodu olan her metin
// (`TR_message("H6932", x)`, `TR_error_code(h) == "H6932"`). Her biri için, kullanımın bulunduğu dosyadaki
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
	for _, n := range g.Nodes {
		if n.Kind != kindFile || !strings.HasSuffix(n.File, ".tr") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.File)))
		if err != nil {
			t.Fatal(err)
		}
		b, err := source.Split(string(data))
		if err != nil {
			continue
		}
		l := b.IdentityLexer()
		for tok := l.NextToken(); tok.Type != lexer.TokEOF; tok = l.NextToken() {
			// İleti kodu bir metin olarak yazılır ("H6903").
			if tok.Type == lexer.TokString && messageCodeName.MatchString(tok.Literal) {
				want[tok.Literal+" "+n.File] = true
			}
		}
	}
	if goUses < 1000 || len(want)-goUses < 20 {
		t.Fatalf("%d Go and %d .tr message code uses; the scan is broken", goUses, len(want)-goUses)
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
