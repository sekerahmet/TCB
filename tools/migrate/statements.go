package main

import (
	trast "github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Deyim ve fonksiyon kuralları (dil tanımı 5. ve 6. bölüm, katman 3A): yeni biçimdeki metnin
// çevirisi, değer kurallarıyla aynı geçişte (valueEdits) ve aynı yolla: metin bugünkü ayrıştırıcıyla
// okunur, yalnız değişen sözcükler yazılır.
//
//   - kısa tanım değişken tanımı olur: x := 5 → TR_var x = 5 (5.1).

// statementEdits gövdenin deyim ve fonksiyon kurallarıyla değişen aralıklarıdır; at sözcük
// biriminin kaydıdır (satır, sütun → aralık).
func statementEdits(prog *trast.Program, at map[[2]int]lexer.Record, lineBase int) ([]valueEdit, []problem) {
	var edits []valueEdit
	var probs []problem
	trast.Walk(prog, func(n any) bool {
		if s, ok := n.(*trast.VarStatement); ok && s.Token.Type == lexer.TokWalrus {
			name, okName := at[[2]int{s.Name.Token.Line, s.Name.Token.Column}]
			walrus, okWalrus := at[[2]int{s.Token.Line, s.Token.Column}]
			if !okName || !okWalrus {
				probs = append(probs, problem{Line: lineBase + s.Token.Line, Reason: "':=' position not found"})
				return true
			}
			edits = append(edits, valueEdit{name.Start, name.Start, "TR_var "}, valueEdit{walrus.Start, walrus.End, "="})
		}
		return true
	})
	return edits, probs
}
