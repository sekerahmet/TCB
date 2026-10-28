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
//   - dönüş tipi ': T' → '-> T' (6.1).

// statementEdits gövdenin deyim ve fonksiyon kurallarıyla değişen aralıklarıdır; records sözcük
// birimlerinin kaydı, at onların (satır, sütun) dizinidir.
func statementEdits(prog *trast.Program, records []lexer.Record, at map[[2]int]lexer.Record, lineBase int) ([]valueEdit, []problem) {
	byStart := append([]lexer.Record(nil), records...)
	sort.Slice(byStart, func(i, j int) bool { return byStart[i].Start < byStart[j].Start })
	// before, aralığı start'ta başlayan belirteçten önceki belirteçtir.
	before := func(start int) (lexer.Record, bool) {
		i := sort.Search(len(byStart), func(i int) bool { return byStart[i].Start >= start })
		if i == 0 {
			return lexer.Record{}, false
		}
		return byStart[i-1], true
	}
	var edits []valueEdit
	var probs []problem
	trast.Walk(prog, func(n any) bool {
		f, ok := n.(*trast.FunctionLiteral)
		if !ok || f.ReturnToken.Line == 0 {
			return true
		}
		ret, okRet := at[[2]int{f.ReturnToken.Line, f.ReturnToken.Column}]
		if !okRet {
			probs = append(probs, problem{Line: lineBase + f.ReturnToken.Line, Reason: "return type position not found"})
			return true
		}
		if colon, ok := before(ret.Start); ok && colon.Type == lexer.TokColon {
			arrow := "->"
			if paren, ok := before(colon.Start); ok && paren.End == colon.Start {
				arrow = " ->" // f(): T → f() -> T
			}
			edits = append(edits, valueEdit{colon.Start, colon.End, arrow})
		}
		return true
	})
	return edits, probs
}
