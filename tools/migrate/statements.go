package main

import (
	"sort"

	trast "github.com/sekerahmet/TCB/internal/lang/ast"
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
//   - TR_it ya da TR_acc okuyan sondaki blok son argümandaki oka döner (l.TR_map { TR_it * 2 } →
//     l.TR_map((x) => x * 2)), son argümandaki parametresiz ok sondaki blok olur
//     (f(() => { ... }) → f { ... }) (6.7): b0314e72'de koştu.
//   - fonksiyon ifadesinin tek yazımı ok (6.6): adsız TR_func(x) { ... } → (x) => { ... }; parantezsiz
//     x => e → (x) => e; ifade yerindeki adlı TR_func f(x) { ... } → (x) => { ... } (ad başka bir
//     yerde geçmiyorsa; geçiyorsa ya da dönüş tipi yazılmışsa elle çevrilir, bildirilir).

// statementEdits gövdenin deyim ve fonksiyon kurallarıyla değişen aralıklarıdır; records sözcük
// birimlerinin kaydı, at onların (satır, sütun) dizinidir.
func statementEdits(prog *trast.Program, body string, records []lexer.Record, at map[[2]int]lexer.Record, lineBase int) ([]valueEdit, []problem) {
	src := []rune(body)
	recs := append([]lexer.Record(nil), records...)
	sort.Slice(recs, func(i, j int) bool { return recs[i].Start < recs[j].Start })
	index := map[int]int{}
	for i, r := range recs {
		index[r.Start] = i
	}
	// Deyim olarak yazılmış fonksiyonlar ve yapı metotları tanımdır; öbür TR_func'lar ifade yerindedir.
	definitions := map[*trast.FunctionLiteral]bool{}
	lastArgs := map[*trast.FunctionLiteral]bool{} // adlı argümansız çağrının son argümanı
	uses := map[string]int{}
	trast.Walk(prog, func(n any) bool {
		switch x := n.(type) {
		case *trast.CallExpression:
			if n := len(x.Arguments); n > 0 && len(x.NamedArgs) == 0 {
				if f, ok := x.Arguments[n-1].(*trast.FunctionLiteral); ok {
					lastArgs[f] = true
				}
			}
		case *trast.ExpressionStatement:
			if f, ok := x.Expression.(*trast.FunctionLiteral); ok {
				definitions[f] = true
			}
		case *trast.StructStatement:
			for _, m := range x.Methods {
				definitions[m] = true
			}
		case *trast.Identifier:
			uses[x.Value]++
		}
		return true
	})
	var edits []valueEdit
	var probs []problem
	problem := func(line int, reason string) { probs = append(probs, problem{Line: lineBase + line, Reason: reason}) }
	trast.Walk(prog, func(n any) bool {
		f, ok := n.(*trast.FunctionLiteral)
		if !ok {
			return true
		}
		switch {
		case f.Token.Type == lexer.TokIdent && len(f.Parameters) == 1:
			// x => e → (x) => e
			r, ok := at[[2]int{f.Token.Line, f.Token.Column}]
			if !ok {
				problem(f.Line(), "arrow parameter position not found")
				return true
			}
			edits = append(edits, valueEdit{r.Start, r.Start, "("}, valueEdit{r.End, r.End, ")"})
		case f.Token.Type == lexer.TokFunction && !definitions[f] && lastArgs[f] && len(f.Parameters) == 0:
			// Son argümandaki parametresiz TR_func sondaki blok olur (6.7): f(a, TR_func() { TR_return x }) →
			// f(a) { x }. Gövdesi tek 'TR_return x' olmayan elle çevrilir.
			e, ok := parameterlessToBlock(f, recs, index, at, src)
			if !ok {
				problem(f.Line(), "parameterless function as the last argument: write a trailing block by hand")
				return true
			}
			edits = append(edits, e)
		case f.Token.Type == lexer.TokFunction && !definitions[f]:
			if f.ReturnToken.Line != 0 || len(f.ReturnTypes) > 0 {
				problem(f.Line(), "function expression with a return type: write it as an arrow by hand")
				return true
			}
			if f.Name != "" && uses[f.Name] > 0 {
				problem(f.Line(), "named function in an expression whose name is used: write it as an arrow by hand")
				return true
			}
			kw, ok := at[[2]int{f.Token.Line, f.Token.Column}]
			if !ok {
				problem(f.Line(), "function position not found")
				return true
			}
			// TR_func [ad](p) { → (p) => {: '(' bulunur, kapanışından sonra ' =>'
			i := index[kw.Start]
			for i < len(recs) && recs[i].Type != lexer.TokLParen {
				i++
			}
			depth, j := 0, i
			for ; j < len(recs); j++ {
				if recs[j].Type == lexer.TokLParen {
					depth++
				} else if recs[j].Type == lexer.TokRParen {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			if i >= len(recs) || j >= len(recs) {
				problem(f.Line(), "function parameters not found")
				return true
			}
			edits = append(edits, valueEdit{kw.Start, recs[i].Start, ""}, valueEdit{recs[j].End, recs[j].End, " =>"})
		}
		return true
	})
	return edits, probs
}

// parameterlessToBlock son argümandaki 'TR_func() { TR_return x }'ı sondaki bloğa çeviren düzenlemedir:
// argümanı açan '(' ya da önündeki ','den çağrının ')'sine kadarı ' { x }' ya da ') { x }' olur.
func parameterlessToBlock(f *trast.FunctionLiteral, recs []lexer.Record, index map[int]int, at map[[2]int]lexer.Record, src []rune) (valueEdit, bool) {
	if len(f.Body.Statements) != 1 {
		return valueEdit{}, false
	}
	ret, ok := f.Body.Statements[0].(*trast.ReturnStatement)
	kw, ok2 := at[[2]int{f.Token.Line, f.Token.Column}]
	retRec, ok3 := at[[2]int{ret.Token.Line, ret.Token.Column}]
	if !ok || !ok2 || !ok3 {
		return valueEdit{}, false
	}
	before := index[kw.Start] - 1 // '(' ya da ','
	if before < 0 || recs[before].Type != lexer.TokLParen && recs[before].Type != lexer.TokComma {
		return valueEdit{}, false
	}
	// Fonksiyonun gövdesinin '}'si ve çağrının ')'si: dönüşten sonraki ilk '}' (iç içe saymadan değil)
	depth, close := 0, -1
	for j := index[retRec.Start]; j < len(recs); j++ {
		switch recs[j].Type {
		case lexer.TokLBrace, lexer.TokLParen, lexer.TokLBracket:
			depth++
		case lexer.TokRParen, lexer.TokRBracket:
			depth--
		case lexer.TokRBrace:
			if depth == 0 {
				close = j
			}
			depth--
		}
		if close >= 0 {
			break
		}
	}
	if close < 0 || close+1 >= len(recs) || recs[close+1].Type != lexer.TokRParen || index[retRec.Start]+1 >= close {
		return valueEdit{}, false
	}
	value := string(src[recs[index[retRec.Start]+1].Start:recs[close-1].End])
	if recs[before].Type == lexer.TokLParen {
		return valueEdit{recs[before].Start, recs[close+1].End, " { " + value + " }"}, true
	}
	return valueEdit{recs[before].Start, recs[close+1].End, ") { " + value + " }"}, true
}
