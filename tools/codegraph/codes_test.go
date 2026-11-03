package main

import (
	"go/ast"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// commonNameFiles bir kodun adı yalnız definitions.json'daki ortak addır (madde 15 adım 2): dil dosyalarında satırı var
// (açıklama, örnek) ama adı yok; ortam değişkeninin ise dil dosyalarında satırı da yok.
var commonNameFiles = map[string]string{
	"p/p.go": "package p\n",
	"internal/lang/codes/definitions.json": `{
  "TR_fn_sin": { "kind": "function", "common_name": "sin" },
  "TR_fn_print": { "kind": "function" },
  "TR_env_CPU_AVX2": { "kind": "command", "common_name": "TR_CPU_AVX2" }
}
`,
	"internal/lang/codes/tr.json": `{
  "TR_fn_sin": { "family": "matematik" },
  "TR_fn_print": { "name": "yazdır", "family": "çıktı" }
}
`,
	"internal/lang/codes/en.json": `{
  "TR_fn_sin": { "family": "math" },
  "TR_fn_print": { "name": "print", "family": "output" }
}
`,
}

// Kural: dil dosyasında adı yazılmayan kodun adı definitions.json'daki ortak addır (tek kaynak): ad kodu düğümünün adı
// ve açıklaması ortak ad, `query <ortak ad>` kimliği bulur, ortak adlı ortam değişkeni koduna bağlanır.
func TestACommonNameComesFromTheDefinitions(t *testing.T) {
	root := auditModule(t, commonNameFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	sin := ix.byID["name:TR_fn_sin"]
	if sin == nil || !slices.Equal(sin.Aliases, []string{"sin"}) || !strings.HasPrefix(sin.Detail, "sin (") {
		t.Fatalf("TR_fn_sin: %+v", sin)
	}
	if found := ix.lookup("sin"); len(found) != 1 || found[0] != sin {
		t.Errorf("query sin: %v", found)
	}
	if p := ix.byID["name:TR_fn_print"]; p == nil || !slices.Equal(p.Aliases, []string{"yazdır", "print"}) {
		t.Errorf("a name in the language files stays: %+v", p)
	}
	if got := envCodes(root)["TR_CPU_AVX2"]; got != "TR_env_CPU_AVX2" {
		t.Errorf("the common name of an environment variable: %q", got)
	}
}

// Kural: codegraph'ın her ad kodu düğümünün adı codes paketinin adıdır (Türkçe ve İngilizce), ortam değişkeni kodlarının
// her dildeki adı koduna bağlanır. Örnekle değil, bütün kodlarla: adı başka bir kaynaktan (ortak ad) gelen bir kod
// okuyucu tarafından kaçırılırsa düşer.
func TestEveryNameCodeNodeHasTheNameOfTheCodesPackage(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	env := envCodes(root)
	for _, k := range codes.Codes() {
		id := k.String()
		n := ix.byID["name:"+id]
		if n == nil {
			t.Errorf("%s has no node", id)
			continue
		}
		want := []string{codes.Source().Name(k)}
		if e, ok := en.Entry(k); ok && e.Name != want[0] {
			want = append(want, e.Name)
		}
		if !slices.Equal(n.Aliases, want) {
			t.Errorf("%s: node names %v, the codes package %v", id, n.Aliases, want)
		}
		if k.Namespace() == codes.NamespaceEnvironment {
			for _, name := range want {
				if env[name] != id {
					t.Errorf("environment variable %s is not linked to %s (%q)", name, id, env[name])
				}
			}
		}
	}
}

// Kural (parti K2, C): bir ad kodu codes paketinin bir işlevine argüman olarak verilmişse (`codes.Spellings(codes.FnPrint)`)
// o satır kodun adını okur, yerleşiği uygulamaz: kenar `reads_name`dir, "uygulayıcının testi" kuralına girmez. Önceden
// kabuğun startsWithPrint'i yazdır'ın yazımlarını okuduğu için yazdır kullanan altı TR testi cmd/tr/main.go'nun testi
// görünüyordu. Okumalar codegraph'tan bağımsız, modülün tipleriyle bulunur; aynı satırda kodun başka bir kullanımı
// yoksa o satırda uses_name kenarı olmamalı, reads_name olmalı.
func TestANameCodeGivenToTheCodesPackageIsReadNotImplemented(t *testing.T) {
	root, m, g := repoGraph(t)
	keys := codeConstKeys(root)
	type site struct {
		file string
		line int
		code string
	}
	reads, other := map[site]bool{}, map[site]bool{}
	codesPkg := m.Path + "/" + codesDir
	for _, u := range m.Units {
		for _, f := range u.Files {
			inCall := map[*ast.Ident]bool{}
			ast.Inspect(f, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					if fn, ok := u.Info.Uses[nameIdent(c.Fun)].(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == codesPkg {
						for _, a := range c.Args {
							if sel, ok := a.(*ast.SelectorExpr); ok {
								inCall[sel.Sel] = true
							}
						}
					}
				}
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				k, isConst := u.Info.Uses[id].(*types.Const)
				if !isConst || k.Pkg() == nil || k.Pkg().Path() != codesPkg || keys[k.Name()] == "" || u.PkgPath == codesPkg {
					return true
				}
				pos := m.Fset.Position(id.Pos())
				s := site{m.Rel(pos.Filename), pos.Line, "name:" + keys[k.Name()]}
				if inCall[id] {
					reads[s] = true
				} else {
					other[s] = true
				}
				return true
			})
		}
	}
	kinds := map[site]map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == edgeUsesName || e.Kind == edgeReadsName {
			s := site{e.File, e.Line, e.To}
			if kinds[s] == nil {
				kinds[s] = map[string]bool{}
			}
			kinds[s][e.Kind] = true
		}
	}
	checked := 0
	for s := range reads {
		if other[s] || kinds[s] == nil {
			continue // aynı satırda başka kullanım ya da grafiğin görmediği yapılandırma
		}
		checked++
		if kinds[s][edgeUsesName] || !kinds[s][edgeReadsName] {
			t.Errorf("%s:%d gives %s to the codes package: want reads_name, the graph has %v", s.file, s.line, s.code, kinds[s])
		}
	}
	if checked < 20 {
		t.Errorf("only %d name reads checked", checked)
	}
}

// Kural: program metninde yazılan her kod yazımı (codeindex.SpelledNames: çıplak TR_print ve üye yazımı x.TR_map,
// x.TR_forward; ayrıca TR_self.TR_x, tek harf alıcı, uzun alıcı ve alıcısız .TR_x) ve nitelenmiş yazımlar
// (tip adıyla, kullanıcı tipiyle, harfsiz tiple ve modülle) iki araçta da (codegraph ve tr index) aynı düğüm kümesine
// çözülür; var olan bir tipin ya da modülün olmayan üyesi yerleşiğe düşmez, "bulunamadı" kalır; noktalı üye yazımı metin
// aramasına düşüp belge ya da yorum adaylarıyla belirsiz dönmez.
func TestEverySpelledNameResolvesToItsCodeOrLibraryDeclaration(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	idx, err := repoIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	reader := codeindex.NewReader(idx)
	checked := 0
	for _, k := range codes.Codes() {
		id := "name:" + k.String()
		want := []string{id}
		if k.IsLibrary() {
			want = nil
			for _, e := range ix.in[id] {
				if e.Kind == edgeUsesName && strings.HasPrefix(e.From, "tr:") && !slices.Contains(want, e.From) {
					want = append(want, e.From)
				}
			}
			slices.Sort(want)
		}
		for _, q := range codeindex.SpelledNames(k) {
			checked++
			var got []string
			for _, n := range ix.lookup(q) {
				got = append(got, n.ID)
			}
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("query %q (for %s): got %v, want %v", q, k, got, want)
			}
		}
	}
	if checked < 200 {
		t.Errorf("only %d spelled names checked", checked)
	}
	for _, q := range []string{
		"x.TR_map", "x.TR_forward",
		"model.TR_forward", "Model.TR_forward", "TR_Linear.TR_forward", "tr_linear.tr_forward",
		"Linear.TR_forward", "Doğrusal.TR_forward", "doğrusal.TR_forward", "GPT.TR_generate",
		"model.TR_generate", "a.TR_shape", "A.TR_shape", "A.TR_tolist", "model.TR_append", "tr_linear.TR_map",
		"Model.TR_append", "Linear.TR_map", "Doğrusal.TR_map", "GPT.TR_map",
		"layers.TR_Linear.TR_forward", "layers.TR_GPT", "katman.TR_GPT", "k.TR_Linear", "iyileştirici.TR_lr_schedule", "eğitim.TR_SpikeGuard",
		"l.TR_map", "m.TR_forward", "uzun_alici.TR_map", "uzun_alici.TR_forward",
		".TR_map", ".TR_forward", "TR_self.TR_map", "TR_self.TR_forward",
	} {
		var gotCG, gotTR []string
		for _, n := range ix.lookup(q) {
			gotCG = append(gotCG, n.ID)
		}
		for _, n := range reader.Resolve(q, "") {
			gotTR = append(gotTR, graphID(*n))
		}
		slices.Sort(gotCG)
		slices.Sort(gotTR)
		if len(gotCG) == 0 || !slices.Equal(gotCG, gotTR) {
			t.Errorf("query %q: codegraph %v, tr index %v", q, gotCG, gotTR)
		}
	}
	assertAmbiguousMemberCandidates(t, ix, reader, []string{
		"A.TR_shape", "A.TR_tolist", "Model.TR_append", "Linear.TR_map", "Doğrusal.TR_map", "GPT.TR_map",
	})
	for _, q := range []string{
		"TR_Linear.TR_map", "A.TR_GPT",
		"foo.TR_GPT", "model.TR_GPT", "TR_self.TR_GPT", ".TR_GPT", "x.TR_GPT",
		"layers.TR_map", "layers.TR_append", "layers.TR_Linear.TR_map",
	} {
		if got := ix.lookup(q); len(got) != 0 {
			t.Errorf("codegraph lookup %q: got %v, want not found", q, got)
		}
		if got := reader.Resolve(q, ""); len(got) != 0 {
			t.Errorf("tr index Resolve %q: got %v, want not found", q, got)
		}
	}
}

func assertAmbiguousMemberCandidates(t *testing.T, ix *index, reader *codeindex.Reader, queries []string) {
	t.Helper()
	typeCode, valCode := messages.H8877.String(), messages.H8878.String()
	for _, q := range queries {
		gotCG := ix.lookup(q)
		hasTypeCG := slices.ContainsFunc(gotCG, func(n *Node) bool { return n.NoteCode == typeCode && n.Note != "" })
		hasValCG := slices.ContainsFunc(gotCG, func(n *Node) bool { return n.NoteCode == valCode && n.Note != "" })
		if !hasTypeCG || !hasValCG {
			t.Errorf("codegraph lookup %q: got %d candidates (hasType=%v, hasVal=%v), want >=1 %s and >=1 %s", q, len(gotCG), hasTypeCG, hasValCG, typeCode, valCode)
		}
		gotTR := reader.Resolve(q, "")
		hasTypeTR := slices.ContainsFunc(gotTR, func(n *codeindex.Node) bool { return n.NoteCode == typeCode && n.Note != "" })
		hasValTR := slices.ContainsFunc(gotTR, func(n *codeindex.Node) bool { return n.NoteCode == valCode && n.Note != "" })
		if !hasTypeTR || !hasValTR {
			t.Errorf("tr index Resolve %q: got %d candidates (hasType=%v, hasVal=%v), want >=1 %s and >=1 %s", q, len(gotTR), hasTypeTR, hasValTR, typeCode, valCode)
		}
	}
}

// memberSpellingExceptions depodaki .tr dosyalarında yazılmış ama dilde üye ya da kitaplık bildirimi karşılığı
// gerçekten bulunmayan `<alıcı>.TR_<ad>` yazımlarıdır; her istisna gerekçesiyle yazılır.
var memberSpellingExceptions = map[string]string{
	"l.TR_print": "tests/negative/kimlik_yeri.tr:4 (# ERROR H0029): TR_fn_print genel yerleşiktir, nokta sonrasında üye yeri hatasını sınar",
}

// Kural (DENETİM istek 48, 55): depodaki .tr dosyalarında (`lib`, `examples`, `tests`, `tools`) zincir başı ve rakamla
// başlamayan her `<alıcı>.TR_<üye>` yazımı (büyük harfli alıcılar dahil) iki araçta da (codegraph ve tr index) bulunur
// ya da aynı aday kümesini verir; gerçekten tanımı olmayan negatif fikstür yazımları gerekçeli istisna listesindedir.
func TestEveryMemberSpellingInRepoTRFilesResolves(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	idx, err := repoIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	reader := codeindex.NewReader(idx)
	re := regexp.MustCompile(`[\p{L}\p{N}_]+\.TR_[\p{L}\p{N}_]+`)
	seen := map[string]bool{}
	files := codeindex.ListFiles(root, []string{"lib", "examples", "tests", "tools"}, codeindex.DefaultSkipDirs, codeindex.TrackedFiles, func(rel string) bool {
		return strings.HasSuffix(rel, ".tr")
	})
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		for _, loc := range re.FindAllStringIndex(s, -1) {
			if loc[0] > 0 && (s[loc[0]-1] == '.' || s[loc[0]-1] == ']' || s[loc[0]-1] == ')') {
				continue
			}
			m := s[loc[0]:loc[1]]
			recv, _, _ := strings.Cut(m, ".")
			if r, _ := utf8.DecodeRuneInString(recv); !unicode.IsDigit(r) && !strings.HasPrefix(recv, "TR_mem_") {
				seen[m] = true
			}
		}
	}
	if len(seen) < 695 {
		t.Fatalf("only %d member spellings found in .tr files", len(seen))
	}
	for _, q := range slices.Sorted(maps.Keys(seen)) {
		var gotCG, gotTR []string
		for _, n := range ix.lookup(q) {
			gotCG = append(gotCG, n.ID)
		}
		for _, n := range reader.Resolve(q, "") {
			gotTR = append(gotTR, graphID(*n))
		}
		slices.Sort(gotCG)
		slices.Sort(gotTR)
		if memberSpellingExceptions[q] != "" {
			if len(gotCG) != 0 || len(gotTR) != 0 {
				t.Errorf("exception %q resolved (codegraph=%v, tr index=%v); remove from memberSpellingExceptions", q, gotCG, gotTR)
			}
			continue
		}
		if len(gotCG) == 0 || !slices.Equal(gotCG, gotTR) {
			t.Errorf("%q: codegraph=%v, tr index=%v", q, gotCG, gotTR)
		}
	}
	for q := range memberSpellingExceptions {
		if !seen[q] {
			t.Errorf("stale exception %q: no longer appears in .tr files", q)
		}
	}
}
