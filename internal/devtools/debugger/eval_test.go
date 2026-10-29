package debugger

import "testing"

// Kural: parseFirstExpression üç durumu ayırır: ilk deyim ifade, sözdizimi hatası ya da boş metin,
// ilk deyim ifade değil; sonraki deyimler yok sayılır (CLI kolları ilk deyimi kullanır).
// Neden: değiştir, izle ve incele komutları bu ayrımı üç yerde ayrı ayrı yazıyordu; H8043 iletisi
// yalnızca sözdizimi hatasında çıkar.
func TestParseFirstExpressionSeparatesOutcomes(t *testing.T) {
	for _, c := range []struct {
		text string
		want exprStatus
	}{
		{"1 + 2", exprParsed},
		{"1 + 2\nTR_print(3)", exprParsed},
		{"1 +", exprSyntaxError},
		{"", exprSyntaxError},
		{"TR_var a = 1", exprNotExpression},
	} {
		if _, got := parseFirstExpression(c.text); got != c.want {
			t.Errorf("parseFirstExpression(%q) = %d, beklenen %d", c.text, got, c.want)
		}
	}
}
