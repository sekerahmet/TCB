package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural: iletideki kod örneği (listeyi tensöre çevir, türev izleyen tensör, özel_türev kullanımı)
// iletinin dilinde yazılır: aynı hata İngilizce iletide İngilizce örnek verir, Türkçe sözcük taşımaz.
func TestCodeExamplesFollowTheMessageLanguage(t *testing.T) {
	for code, want := range map[string]string{
		"exp([1.0, 2.0])":               "exp(tensor(...))",
		"softmax([1.0, 2.0])":           "softmax(tensor(...), dim: -1)",
		"geri_yayılım(tensör(3.0) * 2)": "tensor(..., requires_grad: true)",
		"özel_türev(1)":                 "custom_grad(forward: func(x) { return [x * x, [x]] }, backward: func(s, g) { return [g * 2 * s[0]] })",
	} {
		_, err := run(t, code)
		if err == nil {
			t.Fatalf("%s: hata bekleniyordu", code)
		}
		if en := messages.ErrorIn(err, "en"); !strings.Contains(en, want) || strings.ContainsAny(en, "çğıöşüÇĞİÖŞÜ") {
			t.Errorf("%s: İngilizce iletide %q yok ya da Türkçe harf var:\n%s", code, want, en)
		}
	}
}

func TestMessageConsistency(t *testing.T) {
	cases := []struct {
		code string
		want messages.Code
	}{
		{"AdamW([tensör(3.0)])", messages.H6513},
		{"AdamW([3.0])", messages.H6512},
		{"gömme(tensör(3.0), [0])", messages.H6303},
		{"gömme(3.0, [0])", messages.H6302},
		{"geri_yayılım(3.0)", messages.H6030},
		{"geri_yayılım(tensör(3.0) * 2)", messages.H6402},
		{"çapraz_entropi(sıfırlar([3, 4]), [[0, 1, 0, 0], [1, 0, 0, 0], [0, 0, 0, 1]])", messages.H6312},
		{"log_softmax([1, 2], eksen: 0)", messages.H6104},
		{"relu([[1, 2]])", messages.H6104},
	}
	for _, d := range cases {
		_, err := run(t, d.code)
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", d.code, err, d.want)
		}
		if err != nil && strings.Contains(err.Error(), "Tensör verildi") {
			t.Errorf("%s: çelişkili mesaj: %v", d.code, err)
		}
	}
}

// Kural (dil tanımı 15.46): veri taşıyan tensörün sınırı 2³² eleman (hazır modellerin tabloları
// liste sınırını aşar); sıfır eksenli (verisiz) şeklin sınırı listelerinki, sıfır eksenler 1
// sayılır.
func TestTensorLimit(t *testing.T) {
	for _, c := range []struct {
		dims    []uint64
		hasZero bool
		failAt  int
		bound   int64
	}{
		{[]uint64{151936, 1024}, false, -1, maxTensorElements},     // Qwen3-0.6B gömmesi
		{[]uint64{128256, 8192}, false, -1, maxTensorElements},     // 70B ölçeğinde gömme
		{[]uint64{1 << 19, 1 << 13}, false, -1, maxTensorElements}, // tam sınırda (2³²)
		{[]uint64{1 << 19, 1 << 13, 2}, false, 2, maxTensorElements},
		{[]uint64{1 << 20, 1, 1}, true, -1, interp.MaxCollectionSize},
		{[]uint64{1 << 20, 1, 8192}, true, 2, interp.MaxCollectionSize}, // [1048576, 0, 8192]
		{[]uint64{1 << 63, 4}, false, 0, maxTensorElements},             // taşma yok
	} {
		i, bound := tensorLimit(c.dims, c.hasZero)
		if i != c.failAt || bound != c.bound {
			t.Errorf("tensorLimit(%v, %v) = %d, %d; beklenen %d, %d", c.dims, c.hasZero, i, bound, c.failAt, c.bound)
		}
	}
	if err := checkShapeLimit("sıfırlar", []int{151936, 1024}); err != nil {
		t.Errorf("hazır model tablosu oluşturulabilmeli: %v", err)
	}
	if err := checkShapeLimit("sıfırlar", []int{1 << 20, 0, 8192}); err == nil {
		t.Error("verisiz dev şekil reddedilmeli")
	}
}
