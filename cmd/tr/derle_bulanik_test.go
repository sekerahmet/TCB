package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// FuzzGomuluPaket: 'tr derle' ile üretilmiş bir uygulama başlarken kendi ikilisinin
// sonundaki paketi çözer, modülleri geçici klasöre yazar ve 'aktar' yollarını yönlendirir.
// Paketi değiştirebilen biri ikiliyi de değiştirebileceği için bu bir güven sınırı değildir;
// yine de bozuk (yarım indirilmiş, kırpılmış) bir uygulama çökmemeli, açık bir hata vermeli
// ve modül dosyaları geçici klasörün dışına yazılmamalıdır. Derin arama:
//
//	go test ./cmd/tr -run '^$' -fuzz FuzzGomuluPaket -fuzztime 5m
func FuzzGomuluPaket(f *testing.F) {
	for _, pk := range []*paket{
		{Ana: paketDosyasi{Ad: "ana.tr", Kod: "yazdır(1)\n"}},
		{
			Ana:      paketDosyasi{Ad: "ana.tr", Kod: "aktar \"m\"\naktar \"alt/ç\" olarak c\n", Aktarimlar: []int{0, 1}},
			Moduller: []paketDosyasi{{Ad: "m.tr", Kod: "değişken a = 1\n"}, {Ad: "alt/ç.tr", Kod: "aktar \"m\"\n", Aktarimlar: []int{0}}},
		},
		{Ana: paketDosyasi{Kod: "\ufeffaktar \"..\"\n", Aktarimlar: []int{0}}, Moduller: []paketDosyasi{{Ad: "..", Kod: ""}}},
	} {
		kodlu, err := paketiKodla(pk)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(append([]byte("YORUMLAYICI"), kodlu...))
	}
	f.Add([]byte("yazdır(1)\n\x0a\x00\x00\x00\x00\x00\x00\x00TR_EXE_1"))

	f.Fuzz(func(t *testing.T, ikili []byte) {
		pk, sablonBoyu, bulundu, err := paketiAyikla(ikili)
		if sablonBoyu < 0 || sablonBoyu > len(ikili) {
			t.Fatalf("şablon boyu %d, ikili %d bayt", sablonBoyu, len(ikili))
		}
		if !bulundu || err != nil {
			return
		}
		if len(pk.Moduller) > 16 {
			return
		}
		klasor := t.TempDir()
		yollar, err := gomuluModulleriYaz(pk, klasor)
		if err != nil {
			return
		}
		for _, y := range yollar {
			if rel, err := filepath.Rel(klasor, y); err != nil || rel == "." || strings.HasPrefix(rel, "..") {
				t.Fatalf("modül geçici klasörün dışına yazıldı: %q (klasör %q)", y, klasor)
			}
		}
		_, _ = aktarlariYonlendir(pk.Ana, yollar)
	})
}
