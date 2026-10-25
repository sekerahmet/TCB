package main

import (
	"fmt"
	"go/ast"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: `dosya:satır` satırı içeren en dar düğüme çözülür: bildirimin ilk ve son satırı ve arası, kapanışın içi (kendi
// işlevi), belge yorumu (araya boş satır girmeden altındaki bildirim), tipin içindeki alan; hiçbir bildirimde olmayan
// satır dosyanın kendisidir ve bu söylenir. `.tr` dosyasında sütun verilince tr dizin'in kuralıyla imlecin altındaki adın
// başvurduğu bildirim; Markdown'da başlık bölümü. `query -` grep satırlarına içeren düğümü yazar (eksikler listesi:
// grep'in bulduğu yer grafiğe sorulamıyordu, içeren işlev elle aranıyordu).
func TestALineIsAnsweredByTheDeclarationThatContainsIt(t *testing.T) {
	root := gitFixture(t)
	g, err := readGraph(buildInto(t, root))
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	ix.root = root
	const shape = "shape/shape.go:"
	cases := map[string]string{
		shape + "20":       "shape.Describe", // belge yorumu
		shape + "21":       "shape.Describe", // ilk satır
		shape + "24":       "shape.Describe",
		shape + "27":       "shape.Describe", // kapanış satırı
		shape + "19":       "shape/shape.go", // iki bildirim arası
		shape + "4":        "shape/shape.go", // içe aktarma
		shape + "92":       "shape.Bump",     // func literal içinde
		shape + "73":       "shape.Counter.n",
		"lib/geo.tr:21":    "geo.yaz_alan",
		"lib/geo.tr:11":    "geo.Kare.alan",
		"lib/geo.tr:21:12": "geo.çevre_hesapla", // sütun: imlecin altındaki ad
		"lib/geo.tr:4":     "lib/geo.tr",
		"docs/guide.md:5":  "Usage",
	}
	for q, want := range cases {
		loc, ok := ix.locate(q, true)
		if !ok || loc.node.Name != want {
			t.Errorf("%s: got %+v (ok %v), want %s", q, loc.node, ok, want)
		}
		if strings.Contains(want, "/") != (loc.note != "" && !strings.HasPrefix(loc.note, "the name at")) {
			t.Errorf("%s: a line in no declaration says so (note %q)", q, loc.note)
		}
	}
	if _, ok := ix.locate(shape+"21:text", true); ok {
		t.Error("a query with text after the line is a name, not a location")
	}
	var out strings.Builder
	ix.printLocations(strings.NewReader(shape+"23:\t\treturn messages.New(messages.H0001)\nnot a hit\n"), &out)
	testkit.MustContain(t, out.String(), "    shape.Describe [func] shape/shape.go:21", "(not a location in a tracked file)")
}

// Kural (sınıf): depodaki her Go işlevinde ilk satır, son satır, belge yorumunun ilk satırı ve içindeki ilk func
// literal'in satırı o işleve çözülür; iki üst düzey bildirim arasındaki boş satır dosyaya. Beklenen go/ast'nin içeren
// bildirimidir (çıkarıcıdan bağımsız); rastgele satır değil, her koşuda aynı sınırlar sınanır.
func TestEveryFunctionBoundaryLocatesItsFunction(t *testing.T) {
	root, m, g := repoGraph(t)
	ix := newIndex(g)
	ix.root = root
	checked := 0
	for _, u := range m.Units {
		if !strings.HasPrefix(u.PkgPath, m.Path+"/tools/codegraph") && !strings.HasPrefix(u.PkgPath, m.Path+"/internal/lang/codes") {
			continue
		}
		for _, f := range u.Files {
			file := m.Rel(m.Fset.Position(f.Pos()).Filename)
			lineOf := func(n ast.Node) int { return m.Fset.Position(n.Pos()).Line }
			prevEnd := 0
			for _, d := range f.Decls {
				start := lineOf(d)
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Doc != nil {
					start = lineOf(fd.Doc)
				} else if gd, ok := d.(*ast.GenDecl); ok && gd.Doc != nil {
					start = lineOf(gd.Doc)
				}
				if start-prevEnd >= 2 && prevEnd > 0 { // arada boş satır var: dosyanın kendisi
					checked++
					if loc, _ := ix.locate(fmt.Sprintf("%s:%d", file, start-1), true); loc.node == nil || loc.node.Kind != kindFile {
						t.Errorf("%s:%d is between declarations, got %v", file, start-1, loc.node)
					}
				}
				prevEnd = m.Fset.Position(d.End()).Line
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Name.Name == "init" || fd.Name.Name == "_" {
					continue
				}
				recv := ""
				if fd.Recv != nil {
					recv = goload.RecvExprName(fd.Recv.List[0].Type)
				}
				want := memberID(u.PkgPath, recv, fd.Name.Name)
				lines := []int{start, lineOf(fd), prevEnd}
				ast.Inspect(fd, func(n ast.Node) bool {
					if lit, ok := n.(*ast.FuncLit); ok {
						lines = append(lines, lineOf(lit))
						return false
					}
					return true
				})
				for _, l := range lines {
					checked++
					if loc, _ := ix.locate(fmt.Sprintf("%s:%d", file, l), true); loc.node == nil || loc.node.ID != want {
						t.Errorf("%s:%d is in %s, got %v", file, l, want, loc.node)
					}
				}
			}
		}
	}
	if checked < 500 {
		t.Errorf("only %d boundaries checked", checked)
	}
}
