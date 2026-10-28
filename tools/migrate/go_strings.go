package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"

	trast "github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	trparser "github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Go testlerindeki TR metni. Metin Go'nun kendi çözümleyicisiyle (go/ast) bulunur: her metin değişmezi
// bir aday; kodlu biçimdeki (sınıflı kimlikler) ya da Türkçe gösterimde yazılmış bir TR programı ise
// çevrilir. Türkçe gösterimdeki metin bir program sayılmak için ayrıştırılabilmeli ve bir çağrı ya da
// deyim içermelidir (beklenen çıktılar "doğru", "Tam", "[1, 2]" program değildir). Gösterimi ya da
// kodlamayı sınayan test dosyalarında (viewTests) metin gösterim olarak kalır, yalnız yazımı çevrilir;
// öbürlerinde kimliğe çevrilir (başlıksız kodlu gövde: yardımcılar onu source.Options.Body ile okur).

// viewTests gösterim metnini sınayan test dosyalarının ya da klasörlerinin önekleridir.
var viewTests = []string{
	"internal/lang/lexer/", "internal/lang/parser/", "internal/lang/source/", "internal/lang/codes/",
	"internal/lang/hint/", "internal/devtools/formatter/",
	"cmd/tr/e2e/behavior_display_language_test.go", "cmd/tr/e2e/behavior_coded_files_test.go",
}

// convertGoTest Go test dosyasındaki TR metnini çevirir.
func convertGoTest(rel, in string) (string, []problem) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, in, parser.ParseComments)
	if err != nil {
		return in, []problem{{Line: 1, Reason: "Go parse: " + err.Error()}}
	}
	view := false
	for _, p := range viewTests {
		view = view || strings.HasPrefix(rel, p)
	}
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	var probs []problem
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		line := fset.Position(lit.Pos()).Line
		conv, ok, p := convertSnippet(s, view, line)
		probs = append(probs, p...)
		if !ok || conv == s {
			return true
		}
		q := quoteLike(lit.Value, conv)
		edits = append(edits, edit{fset.Position(lit.Pos()).Offset, fset.Position(lit.End()).Offset, q})
		return true
	})
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var b strings.Builder
	prev := 0
	for _, e := range edits {
		b.WriteString(in[prev:e.start])
		b.WriteString(e.text)
		prev = e.end
	}
	b.WriteString(in[prev:])
	return b.String(), probs
}

// quoteLike çevrilen metni özgün değişmezin tırnak biçimiyle yazar (ters tırnak, içinde ters tırnak
// yoksa).
func quoteLike(orig, s string) string {
	if strings.HasPrefix(orig, "`") && !strings.Contains(s, "`") && !strings.Contains(s, "\r") {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

// convertSnippet bir metin değişmezini çevirir; ok, metnin bir TR metni olduğudur.
func convertSnippet(s string, view bool, line int) (string, bool, []problem) {
	if looksNew(s) {
		return s, false, nil
	}
	if strings.HasPrefix(strings.TrimPrefix(s, bom), oldHeaderPrefix) {
		out, probs := convertTRFile("", s)
		return out, true, relocate(probs, line-1)
	}
	if hasOldIdentity(s) {
		out, probs := convertCode(s, nil, 0)
		return out, true, relocate(probs, line-1)
	}
	syntax, probs := convertCode(s, viewOf(codes.SourceLanguage), 0)
	if !isTurkishProgram(syntax) {
		return s, false, nil
	}
	if view {
		return syntax, true, relocate(probs, line-1)
	}
	coded, err := encodeSnippet(syntax)
	if err != nil {
		return s, false, []problem{{Line: line, Reason: "cannot encode the Turkish snippet: " + err.Error()}}
	}
	return coded, true, relocate(probs, line-1)
}

// relocate sorunların satırını Go dosyasındaki satıra taşır.
func relocate(probs []problem, base int) []problem {
	for i := range probs {
		probs[i].Line += base
	}
	return probs
}

// looksNew metin zaten yeni biçimde mi: yeni başlıkla başlıyor, yeni yazımda bir kimlik (TR_print) ya
// da '\(' yerleştirmesi içeriyor (eski yazımda TR_print ayrılmış ad, '\(' bilinmeyen kaçış hatasıydı).
// Yeni biçimdeki girdiye dokunulmaz: iki kez koşmak bir kez koşmakla aynıdır.
func looksNew(s string) bool {
	if strings.HasPrefix(strings.TrimPrefix(s, bom), newHeaderPrefix) || strings.Contains(s, `\(`) {
		return true
	}
	for w := range wordsOf(s) {
		if !strings.HasPrefix(w, codes.IdentityPrefix) || knownClass(codes.IdentityClass(w)) {
			continue
		}
		for _, p := range []codes.Place{codes.PlaceOther, codes.PlaceMember, codes.PlaceArg} {
			if _, ok := codes.FromSpelling(p, w); ok {
				return true
			}
		}
	}
	return false
}

// hasOldIdentity metinde eski sınıflı bir kimlik (TR_fn_print) var mı?
func hasOldIdentity(s string) bool {
	for w := range wordsOf(s) {
		if _, ok := removedIdentities[w]; ok {
			return true
		}
		if codes.IdentityClass(w) != "" {
			if _, err := codes.Parse(w); err == nil {
				return true
			}
		}
	}
	return false
}

// isTurkishProgram metin Türkçe gösterimde yazılmış bir program mı: Türkçe okununca sözdizimi hatası
// yok, bir dil sözcüğü (anahtar kelime ya da yerleşik adı) ve bir çağrı ya da ifade olmayan bir
// deyim içeriyor.
func isTurkishProgram(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	l := lexer.NewBodyIn(s, false, codes.Source())
	l.StartRecording()
	p := trparser.New(l)
	prog := p.ParseProgram()
	if len(p.Errs()) > 0 || len(prog.Statements) == 0 {
		return false
	}
	word := false
	for _, r := range l.Records() {
		if r.Code != codes.None || lexer.KeywordCode(r.Type) != codes.None {
			word = true
		}
		if r.Type == lexer.TokIdent {
			if _, ok := codes.Source().Find(codes.NamespaceGlobal, r.Literal); ok {
				word = true
			}
		}
	}
	structure := false
	trast.Walk(prog, func(n any) bool {
		switch n.(type) {
		case *trast.CallExpression, *trast.VarStatement, *trast.FunctionLiteral, *trast.AssignStatement,
			*trast.ForInStatement, *trast.WhileStatement, *trast.IfExpression, *trast.StructStatement,
			*trast.TestStatement, *trast.TryCatchStatement, *trast.ImportStatement, *trast.MemberExpression:
			structure = true
		}
		return !structure
	})
	return word && structure
}

// encodeSnippet Türkçe gösterimdeki parçayı başlıksız kodlu gövdeye çevirir (doğrulanmış).
func encodeSnippet(s string) (string, error) {
	coded, err := source.EncodeIn(s, codes.Source())
	if err != nil {
		return "", err
	}
	_, body, _ := firstLine(coded)
	return body, nil
}
