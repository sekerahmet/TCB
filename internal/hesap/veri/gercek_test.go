package veri

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestGercekVeriDuman, gerçek veri köküne karşı küçük bir duman testidir.
// Yalnızca TR_VERI_YOLU ortam değişkeni veri kökünü (meta.json, train/,
// _valid/ içeren dizin) gösteriyorsa çalışır. Yalnızca meta.json'u, birkaç
// parçanın başlığını ve birkaç küçük pencereyi okur; dosyaları kopyalamaz,
// eşleme yerine ReadAt kullanır (akışlı ağ sürücülerinde en az okuma).
func TestGercekVeriDuman(t *testing.T) {
	kok := os.Getenv("TR_VERI_YOLU")
	if kok == "" {
		t.Skip("TR_VERI_YOLU ayarlı değil; gerçek veri duman testi atlandı")
	}
	ctx := context.Background()
	sec := KumeSecenekleri{Parca: ParcaSecenekleri{EslemeKapali: true}}
	egitim, err := KumeAc(filepath.Join(kok, "train"), sec)
	if err != nil {
		t.Fatal(err)
	}
	m := egitim.Meta()
	if m == nil {
		t.Fatal("meta.json bulunamadı")
	}
	t.Logf("tokenizer: %s", m.TokenizerYolu())
	t.Logf("eğitim: %d parça, %d token (meta'da %d parça kaydı)", egitim.ParcaSayisi(), egitim.ToplamToken(), len(m.Parts))
	if egitim.ParcaSayisi() != len(m.Parts) {
		t.Errorf("dizindeki parça sayısı (%d) meta.json ile (%d) uyuşmuyor", egitim.ParcaSayisi(), len(m.Parts))
	}

	// İlk ve son parçanın başlığını gerçekten oku ve meta ile karşılaştır.
	ps := egitim.Parcalar()
	for _, i := range []int{0, len(ps) - 1} {
		n, err := baslikUzunlugu(ps[i].Yol)
		if err != nil {
			t.Fatal(err)
		}
		if n != ps[i].Token {
			t.Errorf("%s: başlık %d, meta %d", ps[i].Ad, n, ps[i].Token)
		}
		t.Logf("%s: başlık uzunluğu %d token", ps[i].Ad, n)
	}

	tokenDenetle := func(ad string, v []int32) {
		for _, x := range v {
			if x < 0 || x >= 40000 {
				t.Fatalf("%s: sözlük dışı token %d", ad, x)
			}
		}
	}

	// İlk parçadan 64 token.
	p, err := egitim.ParcaAc(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	ilk := make([]int32, 64)
	if err := p.Oku(ilk, 0); err != nil {
		t.Fatal(err)
	}
	_ = p.Kapat()
	tokenDenetle("ilk parça", ilk)
	t.Logf("%s ilk 16 token: %v (tür %s)", ps[0].Ad, ilk[:16], p.Tur())

	// Rastgele örnekleyiciden tek küçük toplu iş (2 × 33 token).
	o, err := egitim.Ornekleyici(OrnekleyiciSecenekleri{B: 2, T: 32, Tohum: 1, GrupBoyu: 1})
	if err != nil {
		t.Fatal(err)
	}
	var ti TopluIs
	if err := o.Sonraki(&ti); err != nil {
		t.Fatal(err)
	}
	_ = o.Kapat()
	tokenDenetle("örnekleyici", ti.Veri)
	t.Logf("rastgele toplu iş durumu %+v, satır 0 ilk 8: %v", ti.Durum, ti.Satir(0)[:8])

	// Doğrulama kümesinin başından sıralı tek toplu iş.
	dog, err := KumeAc(filepath.Join(kok, "_valid"), sec)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("doğrulama: %d parça, %d token", dog.ParcaSayisi(), dog.ToplamToken())
	if m.Valid != nil && m.Valid.Tokens != dog.ToplamToken() {
		t.Errorf("doğrulama token sayısı %d, meta %d", dog.ToplamToken(), m.Valid.Tokens)
	}
	so, err := dog.Sirali(SiraliSecenekleri{B: 2, T: 32})
	if err != nil {
		t.Fatal(err)
	}
	if err := so.Doldur(ctx, &ti); err != nil {
		t.Fatal(err)
	}
	_ = so.Kapat()
	tokenDenetle("doğrulama", ti.Veri)
	if ti.Satir(1)[0] != ti.Satir(0)[32] {
		t.Error("sıralı pencereler T adımıyla ardışık değil")
	}
}
