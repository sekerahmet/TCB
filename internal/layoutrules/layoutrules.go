// Package layoutrules, kod düzeni standardının (docs/development.md) boyut sınırlarının tek kaynağıdır: el yazımı bir
// dosyanın ve test dışı bir fonksiyonun en çok satırı, bugünkü istisnalar ve istisnanın düzeltme payı. Düzen testleri
// (internal/layout_test.go) sınırı bunlarla denetler; kod grafiği (tools/codegraph) düzenleme anında aynı sınırı
// gösterir. Sınır bir yerde değişir, iki okuyucusu aynı sayıyı görür.
//
// Layer: layout rules (internal/layoutrules); yalnızca Go standart kitaplığını içe aktarır, düzen testleri ve
// tools/ kullanır.
package layoutrules

const (
	FileLimit = 600 // el yazımı bir dosyanın en fazla satırı
	FuncLimit = 80  // test dışı bir fonksiyonun en fazla satırı (func satırından kapanışa)
)

// FixMargin bir istisnanın tavanını aşmadan alabileceği küçük büyümedir (tavanın %10'u,
// en az 5 satır): hata düzeltmesi gibi küçük değişiklikler bölmeyi zorunlu kılmasın; kalıcı
// büyüme yine bölmeyi gerektirir.
func FixMargin(ceiling int) int {
	if p := ceiling / 10; p > 5 {
		return p
	}
	return 5
}
