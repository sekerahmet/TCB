package main

import (
	"go/ast"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
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
// kabuğun startsWithPrint'i yazdır'ın yazımlarını okuduğu için yazdır kullanan altı TR testi cmd/trlang/main.go'nun testi
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
// (tip adıyla, kullanıcı tipiyle, harfsiz tiple ve modülle) iki araçta da (codegraph ve trlang index) aynı düğüm kümesine
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
		if enumStateCodes[k.String()] {
			continue
		}
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
		"layers.TR_Linear.TR_forward", "layers.TR_Linear", "layers.TR_GPT", "katman.TR_GPT", "k.TR_Linear", "iyileştirici.TR_lr_schedule", "eğitim.TR_SpikeGuard",
		"metin.TR_length", "layers.TR_map", "layers.TR_append",
		"l.TR_map", "k.TR_length", "m.TR_forward", "uzun_alici.TR_map", "uzun_alici.TR_forward",
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
			t.Errorf("query %q: codegraph %v, trlang index %v", q, gotCG, gotTR)
		}
		if q == "k.TR_length" && !slices.Equal(gotCG, []string{"name:TR_mem_length"}) {
			t.Errorf("query %q: got %v, want [name:TR_mem_length]", q, gotCG)
		}
	}
	assertAmbiguousMemberCandidates(t, ix, reader, messages.H8877, []string{
		"A.TR_shape", "A.TR_tolist", "Model.TR_append", "Linear.TR_map", "Doğrusal.TR_map", "GPT.TR_map",
	})
	assertAmbiguousMemberCandidates(t, ix, reader, messages.H8879, []string{
		"metin.TR_length", "layers.TR_map", "layers.TR_append",
	})
	for _, q := range []string{
		"TR_Linear.TR_map", "A.TR_GPT",
		"foo.TR_GPT", "model.TR_GPT", "TR_self.TR_GPT", ".TR_GPT", "x.TR_GPT",
		"geo.TR_Linear", "layers.TR_Linear.TR_map",
	} {
		if got := ix.lookup(q); len(got) != 0 {
			t.Errorf("codegraph lookup %q: got %v, want not found", q, got)
		}
		if got := reader.Resolve(q, ""); len(got) != 0 {
			t.Errorf("trlang index Resolve %q: got %v, want not found", q, got)
		}
	}
}

func assertAmbiguousMemberCandidates(t *testing.T, ix *index, reader *codeindex.Reader, ownerNote messages.Code, queries []string) {
	t.Helper()
	ownerCode, valCode := ownerNote.String(), messages.H8878.String()
	for _, q := range queries {
		gotCG := ix.lookup(q)
		hasOwnerCG := slices.ContainsFunc(gotCG, func(n *Node) bool { return n.NoteCode == ownerCode && n.Note != "" })
		hasValCG := slices.ContainsFunc(gotCG, func(n *Node) bool { return n.NoteCode == valCode && n.Note != "" })
		if !hasOwnerCG || !hasValCG {
			t.Errorf("codegraph lookup %q: got %d candidates (hasOwner=%v, hasVal=%v), want >=1 %s and >=1 %s", q, len(gotCG), hasOwnerCG, hasValCG, ownerCode, valCode)
		}
		gotTR := reader.Resolve(q, "")
		hasOwnerTR := slices.ContainsFunc(gotTR, func(n *codeindex.Node) bool { return n.NoteCode == ownerCode && n.Note != "" })
		hasValTR := slices.ContainsFunc(gotTR, func(n *codeindex.Node) bool { return n.NoteCode == valCode && n.Note != "" })
		if !hasOwnerTR || !hasValTR {
			t.Errorf("trlang index Resolve %q: got %d candidates (hasOwner=%v, hasVal=%v), want >=1 %s and >=1 %s", q, len(gotTR), hasOwnerTR, hasValTR, ownerCode, valCode)
		}
	}
}

// enumStateCodes kitaplıktaki durumlu tipin durumlarıdır: codeindex durumları indekslemiyor, kodun kullanım kenarı
// yok (T1 kök). İndeks durumları görünce bu liste silinir.
var enumStateCodes = map[string]bool{"TR_mem_Noun": true, "TR_mem_UnknownKind": true, "TR_mem_Verb": true,
	"TR_mem_Assistant": true, "TR_mem_System": true, "TR_mem_Tool": true, "TR_mem_User": true}

// memberSpellingExceptions depodaki .tr dosyalarında yazılmış ama dilde üye ya da kitaplık bildirimi karşılığı
// gerçekten bulunmayan `<alıcı>.TR_<ad>` yazımlarıdır; her istisna gerekçesiyle yazılır.
var memberSpellingExceptions = map[string]string{
	"l.TR_print":                 "tests/negative/kimlik_yeri.tr:4 (# ERROR H0029): TR_fn_print genel yerleşiktir, nokta sonrasında üye yeri hatasını sınar",
	"TR_WordKind.TR_Noun":        "tr/turkish: kitaplığın ilk durumlu tipi; codeindex durumları indekslemiyor (lib/turkish.tr, TR_WordKind)",
	"TR_WordKind.TR_UnknownKind": "tr/turkish: kitaplığın ilk durumlu tipi; codeindex durumları indekslemiyor (lib/turkish.tr, TR_WordKind)",
	"TR_WordKind.TR_Verb":        "tr/turkish: kitaplığın ilk durumlu tipi; codeindex durumları indekslemiyor (lib/turkish.tr, TR_WordKind)",
	"TR_Role.TR_Assistant":       "tr/chat: kitaplığın ikinci durumlu tipi; codeindex durumları indekslemiyor (lib/chat.tr, TR_Role)",
	"TR_Role.TR_System":          "tr/chat: kitaplığın ikinci durumlu tipi; codeindex durumları indekslemiyor (lib/chat.tr, TR_Role)",
	"TR_Role.TR_Tool":            "tr/chat: kitaplığın ikinci durumlu tipi; codeindex durumları indekslemiyor (lib/chat.tr, TR_Role)",
	"TR_Role.TR_User":            "tr/chat: kitaplığın ikinci durumlu tipi; codeindex durumları indekslemiyor (lib/chat.tr, TR_Role)",
}

// Kural (DENETİM istek 48, 55): depodaki .tr dosyalarında (`lib`, `examples`, `tests`, `tools`) zincir başı ve rakamla
// başlamayan her `<alıcı>.TR_<üye>` yazımı (büyük harfli alıcılar dahil) iki araçta da (codegraph ve trlang index) bulunur
// ya da aynı aday kümesini verir; gerçekten tanımı olmayan negatif fikstür yazımları gerekçeli istisna listesindedir.
func TestEveryMemberSpellingInRepoTRFilesResolves(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	idx, err := repoIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	reader := codeindex.NewReader(idx)
	seen := map[string]bool{}
	files := codeindex.ListFiles(root, []string{"lib", "examples", "tests", "tools"}, codeindex.DefaultSkipDirs, codeindex.TrackedFiles, func(rel string) bool {
		return strings.HasSuffix(rel, ".tr")
	})
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		addTRFileMemberSpellings(string(data), seen)
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
				t.Errorf("exception %q resolved (codegraph=%v, trlang index=%v); remove from memberSpellingExceptions", q, gotCG, gotTR)
			}
			continue
		}
		if len(gotCG) == 0 || !slices.Equal(gotCG, gotTR) {
			t.Errorf("%q: codegraph=%v, trlang index=%v", q, gotCG, gotTR)
		}
	}
	for q := range memberSpellingExceptions {
		if !seen[q] {
			t.Errorf("stale exception %q: no longer appears in .tr files", q)
		}
	}
}

var dottedMemberRe = regexp.MustCompile(`[\p{L}\p{N}_]+\.TR_[\p{L}\p{N}_]+`)

func trimEscapedReceiver(s string, start int, m string) string {
	bs := 0
	for i := start - 1; i >= 0 && s[i] == '\\'; i-- {
		bs++
	}
	if bs%2 == 1 && len(m) > 0 && strings.IndexByte("abfnrtv", m[0]) >= 0 {
		return m[1:]
	}
	return m
}

func addTRFileMemberSpellings(s string, seen map[string]bool) {
	for _, loc := range dottedMemberRe.FindAllStringIndex(s, -1) {
		if loc[0] > 0 && (s[loc[0]-1] == '.' || s[loc[0]-1] == ']' || s[loc[0]-1] == ')') {
			continue
		}
		m := trimEscapedReceiver(s, loc[0], s[loc[0]:loc[1]])
		recv, _, _ := strings.Cut(m, ".")
		// `TR_mem_` önekli alıcılar (`TR_mem_embedding.TR_mem_table` gibi 16 yazım) .tr dosyalarında kod olarak değil,
		// yalnız metin sabitlerinde (ağırlık sözlüğü anahtarları) ve yorumlarda geçer; bulunamadı ve Go'ya inmeme davranışı
		// testdata/planning_questions.txt ve testdata/member_spellings.txt içinde korunur.
		if r, _ := utf8.DecodeRuneInString(recv); recv != "" && !unicode.IsDigit(r) && !strings.HasPrefix(recv, "TR_mem_") {
			seen[m] = true
		}
	}
}

func addRepoMemberSpellings(s string, seen map[string]bool) {
	for _, loc := range dottedMemberRe.FindAllStringIndex(s, -1) {
		m := trimEscapedReceiver(s, loc[0], s[loc[0]:loc[1]])
		recv, _, _ := strings.Cut(m, ".")
		if r, _ := utf8.DecodeRuneInString(recv); recv != "" && !unicode.IsDigit(r) {
			seen[m] = true
		}
	}
}

func TestMemberSpellingExtractorsSkipLeadingEscape(t *testing.T) {
	src := `code := "TR_var x = [1]\nx.TR_length\n.TR_sort"`
	for name, fn := range map[string]func(string, map[string]bool){
		"trFiles":   addTRFileMemberSpellings,
		"repoFiles": addRepoMemberSpellings,
	} {
		seen := map[string]bool{}
		fn(src, seen)
		if got := slices.Sorted(maps.Keys(seen)); !slices.Equal(got, []string{"x.TR_length"}) {
			t.Errorf("%s: got %v, want [x.TR_length]", name, got)
		}
	}
}

const memberSpellingsHeader = "# İzlenen .tr, .go ve .md dosyalarındaki <alıcı>.TR_<ad> yazımlarının iki araçtaki cevabı.\n" +
	"# Biçim: <yazım>\\t<durum>\\t<çıkış>\\t<adaylar> (yalnız codegraph'ta spec: adayı varsa sonunda \\ttr_index:not_found:4).\n" +
	"# Yeniden yazmak için: go test ./tools/codegraph -run TestDottedMemberSpellingsGolden -update\n"

var goldenClassedIdentity = regexp.MustCompile(`(^|[^\p{L}\p{N}_])(TR_(?:kw|ctx|val|type|fn|mem|arg|const|cmd|sub|opt|repl|dbg|env|mod)_[A-Za-z0-9_]+)`)

type memberAnswer struct {
	status string
	exit   int
	cands  []string
}

func (a memberAnswer) equal(b memberAnswer) bool {
	return a.status == b.status && a.exit == b.exit && slices.Equal(a.cands, b.cands)
}

func (a memberAnswer) format() string {
	c := "-"
	if len(a.cands) > 0 {
		c = strings.Join(a.cands, ",")
	}
	return a.status + "\t" + strconv.Itoa(a.exit) + "\t" + c
}

func buildMemberAnswer(cands []string) memberAnswer {
	slices.Sort(cands)
	cands = slices.Compact(cands)
	switch len(cands) {
	case 0:
		return memberAnswer{status: string(codeindex.StatusNotFound), exit: exitNotFound}
	case 1:
		return memberAnswer{status: string(codeindex.StatusOK), exit: exitOK, cands: cands}
	default:
		return memberAnswer{status: string(codeindex.StatusAmbiguous), exit: exitAmbiguous, cands: cands}
	}
}

func candLabel(id, noteCode string) string {
	if noteCode != "" {
		return id + "[" + noteCode + "]"
	}
	return id
}

func collectRepoMemberSpellings(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	files := codeindex.ListFiles(root, nil, codeindex.DefaultSkipDirs, codeindex.TrackedFiles, func(rel string) bool {
		ext := filepath.Ext(rel)
		return ext == ".tr" || ext == ".go" || ext == ".md"
	})
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		addRepoMemberSpellings(string(data), seen)
	}
	return slices.Sorted(maps.Keys(seen))
}

func hasDeadClassedIdentity(s string) bool {
	for _, m := range goldenClassedIdentity.FindAllStringSubmatch(s, -1) {
		if _, err := codes.Parse(m[2]); err != nil {
			return true
		}
	}
	return false
}

func diffGoldenLines(wantText string, gotLines []string) []string {
	wantMap, gotMap := map[string]string{}, map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(wantText, "\r\n", "\n"), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			q, rest, _ := strings.Cut(line, "\t")
			wantMap[q] = rest
		}
	}
	for _, line := range gotLines {
		q, rest, _ := strings.Cut(line, "\t")
		gotMap[q] = rest
	}
	keys := map[string]bool{}
	maps.Copy(keys, mapKeysBool(wantMap))
	maps.Copy(keys, mapKeysBool(gotMap))
	var diffs []string
	for _, q := range slices.Sorted(maps.Keys(keys)) {
		w, okW := wantMap[q]
		g, okG := gotMap[q]
		switch {
		case !okW:
			diffs = append(diffs, "+ "+q+"\t"+g)
		case !okG:
			diffs = append(diffs, "- "+q+"\t"+w)
		case w != g:
			diffs = append(diffs, q+":\n    want: "+w+"\n     got: "+g)
		}
	}
	return diffs
}

func mapKeysBool(m map[string]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// Kural (DENETİM istek 58): depoda izlenen .tr, .go ve .md dosyalarındaki her `<alıcı>.TR_<ad>` yazımı iki araca da
// sorulur; spec: dil tanımı düğümleri dışındaki adaylarda iki araç birebir aynı cevabı (durum, çıkış, aday kimlikleri,
// etiket kodları) verir ve bütün cevaplar testdata/member_spellings.txt altın dosyasıyla eşleşir.
func TestDottedMemberSpellingsGolden(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	idx, err := repoIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	reader := codeindex.NewReader(idx)
	start := time.Now()
	spellings := collectRepoMemberSpellings(t, root)
	if len(spellings) < 1000 {
		t.Fatalf("only %d member spellings found across .tr, .go, .md files", len(spellings))
	}
	var gotLines, toolDiffs []string
	for _, q := range spellings {
		var cgAll, cgTR, trAll []string
		hasSpec := false
		for _, n := range ix.lookup(q) {
			lbl := candLabel(n.ID, n.NoteCode)
			cgAll = append(cgAll, lbl)
			if n.Kind == kindSpec {
				hasSpec = true
			} else {
				cgTR = append(cgTR, lbl)
			}
		}
		for _, n := range reader.Resolve(q, "") {
			trAll = append(trAll, candLabel(graphID(*n), n.NoteCode))
		}
		cgAns, cgTRAns, trAns := buildMemberAnswer(cgAll), buildMemberAnswer(cgTR), buildMemberAnswer(trAll)
		if !cgTRAns.equal(trAns) || (hasSpec && len(cgTR) > 0) {
			toolDiffs = append(toolDiffs, q+": codegraph="+cgAns.format()+" vs tr_index="+trAns.format())
		}
		if hasDeadClassedIdentity(q) {
			// Kural 17.4, 17.10 (TestClassedIdentitiesAreLive): bilerek uydurma sınıflı kimlikler (lib/optimizers.tr'deki
			// negatif desen örneği gibi) başka izlenen dosyaya kopyalanmaz; iki aracın da "bulunamadı" döndüğü denetlenir.
			if cgAns.exit != exitNotFound || trAns.exit != exitNotFound {
				toolDiffs = append(toolDiffs, q+": dead classed identity resolved: "+cgAns.format())
			}
			continue
		}
		line := q + "\t" + cgAns.format()
		if hasSpec {
			line += "\ttr_index:" + trAns.status + ":" + strconv.Itoa(trAns.exit)
		}
		gotLines = append(gotLines, line)
	}
	t.Logf("%d spellings checked in %s (%d in golden file)", len(spellings), time.Since(start).Round(time.Millisecond), len(gotLines))
	if len(toolDiffs) > 0 {
		t.Fatalf("%d spellings differ between codegraph and trlang index:\n  %s", len(toolDiffs), strings.Join(toolDiffs[:min(len(toolDiffs), 30)], "\n  "))
	}
	path := filepath.Join("testdata", "member_spellings.txt")
	content := memberSpellingsHeader + strings.Join(gotLines, "\n") + "\n"
	if *updateGolden {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if diffs := diffGoldenLines(string(wantBytes), gotLines); len(diffs) > 0 {
		t.Errorf("%d member spelling answers changed in %s (run with -update to rewrite):\n  %s", len(diffs), path, strings.Join(diffs[:min(len(diffs), 30)], "\n  "))
	}
}
