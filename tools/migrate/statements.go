package main

import (
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
//   - fonksiyon ifadesinin tek yazımı ok (6.6): adsız TR_func(x) { ... } → (x) => { ... }, parantezsiz
//     x => e → (x) => e, ifade yerindeki adlı TR_func → ok, son argümandaki parametresiz TR_func →
//     sondaki blok: eba83ef5'te koştu.

// statementEdits gövdenin deyim ve fonksiyon kurallarıyla değişen aralıklarıdır; records sözcük
// birimlerinin kaydı, at onların (satır, sütun) dizinidir.
func statementEdits(prog *trast.Program, body string, records []lexer.Record, at map[[2]int]lexer.Record, lineBase int) ([]valueEdit, []problem) {
	return nil, nil
}
