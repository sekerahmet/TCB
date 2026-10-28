package main

import (
	"fmt"
	"sort"
	"strings"

	trast "github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Deyim ve fonksiyon kuralları (dil tanımı 5. ve 6. bölüm, katman 3A): yeni biçimdeki metnin
// çevirisi, değer kurallarıyla aynı geçişte (valueEdits) ve aynı yolla: metin bugünkü ayrıştırıcıyla
// okunur, yalnız değişen sözcükler yazılır. Bir kural, eski yazımı ayrıştırıcı reddetmeden önceki
// göç commit'inde koşar; ayrıştırıcı reddedince eski yazımlı metin ayrıştırılamaz (elle çevrilir
// diye bildirilir) ve kural silinir:
//
//   - x := 5 → TR_var x = 5 (5.1): 2d36d5e8'de koştu.
//   - dönüş tipi ': T' → '-> T' (6.1): 23b5239e'de koştu.
//   - TR_it ya da TR_acc okuyan sondaki blok parametreli oka döner, çağrının son argümanı olur (6.7):
//     l.TR_map { TR_it * 2 } → l.TR_map((x) => x * 2), l.TR_reduce(0) { TR_acc + TR_it } →
//     l.TR_reduce(0, (acc, x) => acc + x). Parametrenin adı x (acc); bu ad blokla aynı fonksiyonda
//     başka bir yerde geçiyorsa x2 (acc2) ..., ve yer elle okunmak üzere bildirilir. Çok deyimli blok
//     elle çevrilir (bildirilir).
//   - son argümandaki parametresiz ok sondaki blok olur (6.7): f(() => { ... }) → f { ... },
//     f(a, () => x) → f(a) { x }. Gövdesinde TR_return olan ok ve kontrol yapısı başlığındaki çağrı
//     elle çevrilir (bildirilir).
//
// İç içe yerler bir geçişte yalnız en dıştakiyle çevrilir; convertValuesWith metni değişmeyene dek
// yeniden çevirir.

// srcText çevrilen gövdenin sözcük birimleridir: başlangıç sırasıyla, aralıklarıyla.
type srcText struct {
	text    []rune
	recs    []lexer.Record // başlangıca göre sıralı
	byStart map[int]int    // başlangıç → recs indeksi
	at      map[[2]int]lexer.Record
	ends    map[[2]int]int // bitiş (satır, sütun) → aralığın sonu
}

func newSource(body string, records []lexer.Record, at map[[2]int]lexer.Record) *srcText {
	s := &srcText{text: []rune(body), at: at, byStart: map[int]int{}, ends: map[[2]int]int{}}
	s.recs = append(s.recs, records...)
	sort.Slice(s.recs, func(i, j int) bool { return s.recs[i].Start < s.recs[j].Start })
	for i, r := range s.recs {
		s.byStart[r.Start] = i
		if r.EndLine > 0 {
			s.ends[[2]int{r.EndLine, r.EndColumn}] = r.End
		}
	}
	return s
}

// index t belirtecinin recs'teki yeridir.
func (s *srcText) index(t lexer.Token) (int, bool) {
	r, ok := s.at[[2]int{t.Line, t.Column}]
	if !ok {
		return 0, false
	}
	i, ok := s.byStart[r.Start]
	return i, ok
}

// closing open indeksindeki açılışı kapatan belirtecin indeksidir.
func (s *srcText) closing(open int) (int, bool) {
	o := s.recs[open].Type
	c := map[lexer.TokenType]lexer.TokenType{lexer.TokLParen: lexer.TokRParen, lexer.TokLBrace: lexer.TokRBrace, lexer.TokLBracket: lexer.TokRBracket}[o]
	depth := 0
	for i := open; i < len(s.recs); i++ {
		switch s.recs[i].Type {
		case o:
			depth++
		case c:
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

// span düğümün kaynaktaki aralığıdır (ilk belirtecinin başından son belirtecinin sonuna).
func (s *srcText) span(n any) (int, int, bool) {
	r := trast.RangeOf(n)
	first, ok1 := s.at[[2]int{r.Line, r.Col}]
	end, ok2 := s.ends[[2]int{r.EndLine, r.EndCol}]
	return first.Start, end, ok1 && ok2
}

func (s *srcText) slice(a, b int) string { return string(s.text[a:b]) }

// statementEdits gövdenin deyim ve fonksiyon kurallarıyla değişen aralıklarıdır; records sözcük
// birimlerinin kaydı, at onların (satır, sütun) dizinidir.
func statementEdits(prog *trast.Program, body string, records []lexer.Record, at map[[2]int]lexer.Record, lineBase int) ([]valueEdit, []problem) {
	s := newSource(body, records, at)
	var edits []valueEdit
	var probs []problem
	problem := func(line int, reason string) { probs = append(probs, problem{Line: lineBase + line, Reason: reason}) }
	headers := headerSpans(prog, s)
	funcs := functionSpans(prog, s)
	trast.Walk(prog, func(n any) bool {
		call, ok := n.(*trast.CallExpression)
		if !ok || len(call.Arguments) == 0 {
			return true
		}
		fn, ok := call.Arguments[len(call.Arguments)-1].(*trast.FunctionLiteral)
		if !ok {
			return true
		}
		var e valueEdit
		var err string
		switch {
		case fn.Token.Type == lexer.TokLBrace && usesBlockParams(fn):
			e, err = s.blockToArrow(call, fn, funcs)
		case fn.Token.Type == lexer.TokLParen && len(fn.Parameters) == 0 && fn.Name == "":
			if inside(headers, s, call) {
				err = "parameterless function in a control header: write the trailing block in parentheses by hand"
				break
			}
			e, err = s.arrowToBlock(call, fn)
		default:
			return true
		}
		if err != "" {
			problem(call.Line(), err)
			return true
		}
		if strings.HasPrefix(e.text, "\x00") { // ad değişti: yer elle okunur
			e.text = e.text[1:]
			problem(call.Line(), "trailing block converted with a renamed parameter: read it")
		}
		edits = append(edits, e)
		return true
	})
	return outermost(edits), probs
}

// outermost iç içe düzenlemelerden en dıştakileri bırakır (iç içe olanlar sonraki geçişte).
func outermost(edits []valueEdit) []valueEdit {
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].start < edits[j].start || edits[i].start == edits[j].start && edits[i].end > edits[j].end
	})
	var out []valueEdit
	end := -1
	for _, e := range edits {
		if e.start < end {
			continue
		}
		out = append(out, e)
		end = e.end
	}
	return out
}

// usesBlockParams sondaki bloğun gövdesi TR_it ya da TR_acc okuyor mu (iç içe sondaki bloklar adı
// yeniden bağlar, onlara inilmez).
func usesBlockParams(fn *trast.FunctionLiteral) bool {
	found := false
	trast.Walk(fn.Body, func(n any) bool {
		switch x := n.(type) {
		case *trast.FunctionLiteral:
			return x.Token.Type != lexer.TokLBrace
		case *trast.Identifier:
			found = found || x.Token.Code == codes.CtxIt || x.Token.Code == codes.CtxAcc
		}
		return !found
	})
	return found
}

// blockToArrow 'f(a) { TR_it ... }' ve 'l.f { TR_it ... }' çağrısının sondaki bloğunu son argümandaki oka
// çevirir. Düzenleme, çağrının ')'sinden (ya da hedefin sonundan) bloğun '}'sine kadardır.
func (s *srcText) blockToArrow(call *trast.CallExpression, fn *trast.FunctionLiteral, funcs [][2]int) (valueEdit, string) {
	open, ok := s.index(fn.Token)
	if !ok || open == 0 {
		return valueEdit{}, "trailing block position not found"
	}
	close, ok := s.closing(open)
	if !ok {
		return valueEdit{}, "trailing block end not found"
	}
	if len(fn.Body.Statements) != 1 {
		return valueEdit{}, "trailing block with several statements: convert to (x) => { ... TR_return ... } by hand"
	}
	es, ok := fn.Body.Statements[0].(*trast.ExpressionStatement)
	if !ok {
		return valueEdit{}, "trailing block whose statement is not an expression: convert by hand"
	}
	// İfade bloğun '{'sinden sonraki ilk belirteçten '}'sinden önceki son belirtece kadardır (ağaç kapanış
	// parantezlerini tutmaz; ifadenin aralığı iç içe bloğun '}'sinde bitmez).
	exprStart, exprEnd := s.recs[open+1].Start, s.recs[close-1].End
	// Adlar: x (acc); blokla aynı fonksiyonda geçen bir adla çakışırsa x2 (acc2) ...
	from, to := enclosing(funcs, s.recs[open].Start)
	if from < 0 {
		from, to = 0, len(s.text)
	}
	// Bloğun kendi içindeki her ad sayılır; bloğun dışında, aynı fonksiyonun içindeki başka
	// fonksiyonların (ve testlerin) adları sayılmaz: onlar ayrı kapsamdır.
	blockFrom, blockTo := s.recs[open].Start, s.recs[close].End
	used := map[string]bool{}
	for _, r := range s.recs {
		if r.Start < from || r.End > to || r.Type != lexer.TokIdent {
			continue
		}
		if (r.Start < blockFrom || r.End > blockTo) && nestedIn(funcs, from, to, r.Start) {
			continue
		}
		used[r.Literal] = true
	}
	renamed := false
	pick := func(base string) string {
		name := base
		for i := 2; used[name]; i++ {
			name, renamed = fmt.Sprintf("%s%d", base, i), true
		}
		used[name] = true
		return name
	}
	x := pick("x")
	names := map[codes.Code]string{codes.CtxIt: x}
	params := "(" + x + ")"
	if len(fn.Parameters) == 2 {
		acc := pick("acc")
		names[codes.CtxAcc] = acc
		params = "(" + acc + ", " + x + ")"
	}
	// Gövdenin ifadesi, TR_it ve TR_acc yeni adlarıyla (iç içe sondaki bloklarınki olduğu gibi).
	var repl []valueEdit
	trast.Walk(es.Expression, func(n any) bool {
		switch x := n.(type) {
		case *trast.FunctionLiteral:
			return x.Token.Type != lexer.TokLBrace
		case *trast.Identifier:
			if name, ok := names[x.Token.Code]; ok {
				if r, ok := s.at[[2]int{x.Token.Line, x.Token.Column}]; ok {
					repl = append(repl, valueEdit{r.Start, r.End, name})
				}
			}
		}
		return true
	})
	sort.Slice(repl, func(i, j int) bool { return repl[i].start < repl[j].start })
	var expr strings.Builder
	prev := exprStart
	for _, e := range repl {
		expr.WriteString(s.slice(prev, e.start))
		expr.WriteString(e.text)
		prev = e.end
	}
	expr.WriteString(s.slice(prev, exprEnd))
	arrow := params + " => " + expr.String()
	before := s.recs[open-1]
	var e valueEdit
	switch {
	case call.Token.Type == lexer.TokLParen && before.Type == lexer.TokRParen && len(call.Arguments) > 1:
		e = valueEdit{before.Start, s.recs[close].End, ", " + arrow + ")"} // f(a) { ... } → f(a, (x) => ...)
	case call.Token.Type == lexer.TokLParen && before.Type == lexer.TokRParen:
		e = valueEdit{before.Start, s.recs[close].End, arrow + ")"} // f() { ... } → f((x) => ...)
	case call.Token.Type == lexer.TokLBrace:
		e = valueEdit{before.End, s.recs[close].End, "(" + arrow + ")"} // l.f { ... } → l.f((x) => ...)
	default:
		return valueEdit{}, "call parentheses not found"
	}
	if renamed {
		e.text = "\x00" + e.text
	}
	return e, ""
}

// arrowToBlock 'f(() => X)' ve 'f(a, () => X)' çağrısının son argümanını sondaki bloğa çevirir.
func (s *srcText) arrowToBlock(call *trast.CallExpression, fn *trast.FunctionLiteral) (valueEdit, string) {
	if len(call.NamedArgs) > 0 {
		return valueEdit{}, "parameterless function before named arguments: convert to a trailing block by hand"
	}
	lp, ok := s.index(call.Token)
	if !ok || s.recs[lp].Type != lexer.TokLParen {
		return valueEdit{}, "call parentheses not found"
	}
	rp, ok := s.closing(lp)
	if !ok {
		return valueEdit{}, "call parentheses not found"
	}
	var blockText string
	if fn.Body.Token.Type == lexer.TokArrow {
		ret := fn.Body.Statements[0].(*trast.ReturnStatement)
		a, _, ok := s.span(ret.ReturnValue)
		if !ok || rp == 0 {
			return valueEdit{}, "function body position not found"
		}
		blockText = "{ " + s.slice(a, s.recs[rp-1].End) + " }" // son argümandır: ifade ')'den önce biter
	} else {
		if returns(fn.Body) {
			return valueEdit{}, "parameterless function whose body has TR_return: a trailing block has no TR_return, convert by hand"
		}
		open, ok := s.index(fn.Body.Token)
		if !ok {
			return valueEdit{}, "function body position not found"
		}
		close, ok := s.closing(open)
		if !ok {
			return valueEdit{}, "function body end not found"
		}
		blockText = s.slice(s.recs[open].Start, s.recs[close].End)
	}
	if len(call.Arguments) == 1 {
		switch call.Function.(type) {
		case *trast.Identifier, *trast.MemberExpression, *trast.CallExpression:
			return valueEdit{s.recs[lp].Start, s.recs[rp].End, " " + blockText}, ""
		}
		return valueEdit{s.recs[lp].Start, s.recs[rp].End, "() " + blockText}, ""
	}
	// f(a, () => X) → f(a) { X }: son argümandan önceki virgülden kapanışa kadar
	argStart, _, ok := s.span(fn)
	i, ok2 := s.byStart[argStart]
	if !ok || !ok2 || s.recs[i-1].Type != lexer.TokComma {
		return valueEdit{}, "argument position not found"
	}
	return valueEdit{s.recs[i-1].Start, s.recs[rp].End, ") " + blockText}, ""
}

// returns gövdede (iç içe fonksiyonlar dışında) TR_return var mı.
func returns(b *trast.BlockStatement) bool {
	found := false
	trast.Walk(b, func(n any) bool {
		switch n.(type) {
		case *trast.FunctionLiteral:
			return false
		case *trast.ReturnStatement:
			found = true
		}
		return !found
	})
	return found
}

// headerSpans kontrol yapılarının başlık ifadelerinin aralıklarıdır (orada '{' gövdeyi başlatır).
func headerSpans(prog *trast.Program, s *srcText) [][2]int {
	var out [][2]int
	add := func(e trast.Expression) {
		if a, b, ok := s.span(e); ok {
			out = append(out, [2]int{a, b})
		}
	}
	trast.Walk(prog, func(n any) bool {
		switch x := n.(type) {
		case *trast.IfExpression:
			add(x.Condition)
			for _, ei := range x.ElseIfs {
				add(ei.Condition)
			}
		case *trast.ForInStatement:
			add(x.Iterable)
		case *trast.WhileStatement:
			add(x.Condition)
		case *trast.MatchExpression:
			add(x.Subject)
		}
		return true
	})
	return out
}

// inside çağrı bir başlığın içinde mi.
func inside(spans [][2]int, s *srcText, n any) bool {
	a, b, ok := s.span(n)
	if !ok {
		return true
	}
	for _, h := range spans {
		if a >= h[0] && b <= h[1] {
			return true
		}
	}
	return false
}

// functionSpans sondaki blok olmayan fonksiyonların ve testlerin aralıklarıdır.
func functionSpans(prog *trast.Program, s *srcText) [][2]int {
	var out [][2]int
	add := func(n any) {
		if a, b, ok := s.span(n); ok {
			out = append(out, [2]int{a, b})
		}
	}
	trast.Walk(prog, func(n any) bool {
		switch x := n.(type) {
		case *trast.FunctionLiteral:
			if x.Token.Type != lexer.TokLBrace {
				add(x)
			}
		case *trast.TestStatement:
			add(x) // test gövdesi de ayrı bir kapsamdır
		}
		return true
	})
	return out
}

// nestedIn pos, [from, to) aralığının içindeki başka bir fonksiyonun içinde mi.
func nestedIn(spans [][2]int, from, to, pos int) bool {
	for _, f := range spans {
		if f[0] >= from && f[1] <= to && (f[0] != from || f[1] != to) && f[0] <= pos && pos < f[1] {
			return true
		}
	}
	return false
}

// enclosing pos'u içeren en küçük fonksiyon aralığıdır; yoksa (-1, -1).
func enclosing(spans [][2]int, pos int) (int, int) {
	from, to := -1, -1
	for _, f := range spans {
		if f[0] <= pos && pos < f[1] && (from < 0 || f[1]-f[0] < to-from) {
			from, to = f[0], f[1]
		}
	}
	return from, to
}
