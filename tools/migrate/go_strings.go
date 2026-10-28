package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
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
		// Go'nun '+' ile parçaladığı metin (`"...kaydet(\"" + yol + "\")"`) tek metin olarak
		// çevrilir: parçaların arasındaki ifade bir yer tutucudur.
		if bin, ok := n.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
			if leaves := concatLeaves(bin); hasLiteral(leaves) {
				line := fset.Position(bin.Pos()).Line
				pieces, ok, p := convertPieces(leaves, view, line)
				probs = append(probs, p...)
				if ok {
					for i, l := range leaves {
						if lit, isLit := l.(*ast.BasicLit); isLit && lit.Kind == token.STRING {
							edits = append(edits, edit{fset.Position(lit.Pos()).Offset, fset.Position(lit.End()).Offset, quoteLike(lit.Value, pieces[i])})
						}
					}
				}
				return false
			}
		}
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		line := fset.Position(lit.Pos()).Line
		conv, ok, p := convertFormatted(s, view, line)
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

// slotPrefix Go'nun metne kattığı parçanın (bir ifade ya da fmt biçim işareti) çeviri sırasındaki
// yer tutucusudur: TR'de bir kullanıcı adıdır, çeviri ona dokunmaz.
const slotPrefix = "migrateslot"

// concatLeaves '+' zincirinin yapraklarını soldan sağa verir.
func concatLeaves(e ast.Expr) []ast.Expr {
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.ADD {
		return append(concatLeaves(b.X), concatLeaves(b.Y)...)
	}
	if p, ok := e.(*ast.ParenExpr); ok {
		return concatLeaves(p.X)
	}
	return []ast.Expr{e}
}

// hasLiteral yapraklardan biri metin değişmezi mi?
func hasLiteral(leaves []ast.Expr) bool {
	for _, l := range leaves {
		if lit, ok := l.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			return true
		}
	}
	return false
}

// convertPieces '+' zincirini tek metin olarak çevirip parçalarına geri böler: pieces[i] i.
// yaprağın (metin değişmeziyse) yeni metnidir. Yer tutucular çeviriden sonra aynı sırayla bir kez
// geçmiyorsa ok false'tur.
func convertPieces(leaves []ast.Expr, view bool, line int) ([]string, bool, []problem) {
	var joined strings.Builder
	var slots []int
	for i, l := range leaves {
		lit, ok := l.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			joined.WriteString(slotPrefix + strconv.Itoa(len(slots)))
			slots = append(slots, i)
			continue
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return nil, false, nil
		}
		joined.WriteString(s)
	}
	conv, ok, probs := convertFormatted(joined.String(), view, line)
	if !ok {
		return nil, false, probs
	}
	moved := append(probs, problem{Line: line, Reason: "a concatenated Go string's parts moved in the conversion"})
	// Yer tutucular metni len(slots)+1 parçaya böler; k. parça (k-1). ile k. ifadenin arasındaki
	// değişmezlerin ilkine yazılır, öbürleri boş kalır. Arasında değişmez olmayan parça boş olmalı.
	segments := make([]string, 0, len(slots)+1)
	rest := conv
	for n := range slots {
		slot := slotPrefix + strconv.Itoa(n)
		at := strings.Index(rest, slot)
		if at < 0 || strings.Count(conv, slot) != 1 {
			return nil, false, moved
		}
		segments = append(segments, rest[:at])
		rest = rest[at+len(slot):]
	}
	segments = append(segments, rest)
	pieces := make([]string, len(leaves))
	from := 0
	for k, seg := range segments {
		to := len(leaves)
		if k < len(slots) {
			to = slots[k]
		}
		first := -1
		for i := from; i < to && first < 0; i++ {
			if lit, ok := leaves[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				first = i
			}
		}
		switch {
		case first >= 0:
			pieces[first] = seg
		case seg != "":
			return nil, false, moved
		}
		from = to + 1
	}
	return pieces, true, probs
}

// convertFormatted metni çevirir; içindeki fmt biçim işaretleri (%s, %d, %q, %v ...) çeviri boyunca
// yer tutucuyla korunur (TR'de '%' işleçtir, '(%d)' ayrıştırılamazdı).
func convertFormatted(s string, view bool, line int) (string, bool, []problem) {
	verbs := fmtVerb.FindAllString(s, -1)
	if len(verbs) == 0 {
		return convertSnippet(s, view, line)
	}
	n := 0
	masked := fmtVerb.ReplaceAllStringFunc(s, func(string) string {
		n++
		return slotPrefix + "f" + strconv.Itoa(n-1)
	})
	conv, ok, probs := convertSnippet(masked, view, line)
	if !ok {
		return s, false, probs
	}
	for i, v := range verbs {
		slot := slotPrefix + "f" + strconv.Itoa(i)
		if strings.Count(conv, slot) != 1 {
			return s, false, append(probs, problem{Line: line, Reason: "a format verb moved in the conversion"})
		}
		conv = strings.Replace(conv, slot, v, 1)
	}
	return conv, true, probs
}

// fmtVerb bir fmt biçim işaretidir ('%%' dahil değil).
var fmtVerb = regexp.MustCompile(`%[-+# 0]*[0-9]*(\.[0-9]+)?[sdqvgfxtc]`)

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
			// Dilin sözcüğü: genel ad, üye (ç.güncelle) ya da adlı argüman (adım: 7).
			for _, ns := range []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceMember, codes.NamespaceParam} {
				if _, ok := codes.Source().Find(ns, r.Literal); ok {
					word = true
				}
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
