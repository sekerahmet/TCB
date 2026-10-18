package tensor

import (
	"fmt"
	"strconv"
	"strings"

	"tr_dili/pkg/motor/internal/baglama"
	"tr_dili/pkg/sozluk"
	"tr_dili/pkg/vm"
)

// Çekirdek tensör paketi (tr_dili/pkg/tensor) hataları panik ile bildirir. Bu dosyadaki
// yardımcılar her çekirdek çağrısını sarar ve paniği, TR'nin 'dene/yakala' ile
// yakalanabilen sıradan bir hataya çevirir. Yerleşik çağrılarında VM mesajın başına
// "Satır N:" ekler. Adlı argüman denetimi ve paniği yakalama pkg/motor/internal/baglama'nın
// ortak yardımcılarıyla yapılır.

// Adlı argümanlar. Adları sözlüktedir (pkg/sozluk, T9xx). Başka dillerden alışılmış
// adlar (lr, axis...) kabul edilmez; baglama.AdliDenetle hata mesajında Türkçesini önerir.
var (
	adTurevIzle       = baglama.KodAd(sozluk.T900)
	adEksen           = baglama.KodAd(sozluk.T902)
	adTohum           = baglama.KodAd(sozluk.T903)
	adEnFazla         = baglama.KodAd(sozluk.T904)
	adEpsilon         = baglama.KodAd(sozluk.T905)
	adOgrenmeOrani    = baglama.KodAd(sozluk.T910)
	adAgirlikCurumesi = baglama.KodAd(sozluk.T911)
	adBeta1           = baglama.KodAd(sozluk.T912)
	adBeta2           = baglama.KodAd(sozluk.T913)
	adMomentum        = baglama.KodAd(sozluk.T914)
	adAgirlik         = baglama.KodAd(sozluk.T920)
	adSapma           = baglama.KodAd(sozluk.T921)
	adTur             = baglama.KodAd(sozluk.T901)
	adAnaKopya        = baglama.KodAd(sozluk.T915)
)

// guvenli fn'i çalıştırır; çekirdekten gelen paniği Türkçe bir hataya dönüştürür.
func guvenli[T any](fn func() T) (T, error) { return baglama.Kurtar(fn, panikHatasi) }

// panikHatasi çekirdeğin panik değerini kullanıcıya gösterilecek hataya çevirir.
func panikHatasi(r any) error {
	if e, ok := r.(error); ok {
		return fmt.Errorf("tensör işlemi başarısız: %v", e)
	}
	msg := fmt.Sprint(r)
	msg = strings.TrimPrefix(msg, "tensor: ")
	return fmt.Errorf("tensör işlemi başarısız: %s", msg)
}

// tamSayi bir TR değerini tam sayıya çevirir (tam değerli ondalıklar da kabul edilir).
func tamSayi(v vm.Value) (int, bool) {
	n, ok := baglama.TamSayi(v)
	return int(n), ok
}

// sekilMetni bir şekli "[2, 3]" biçiminde yazar.
func sekilMetni(sh []int) string {
	parcalar := make([]string, len(sh))
	for i, d := range sh {
		parcalar[i] = strconv.Itoa(d)
	}
	return "[" + strings.Join(parcalar, ", ") + "]"
}

// sekilCarpimi bir şekli "2×3" biçiminde yazar (izleme masası ve yazdırma başlığı için).
func sekilCarpimi(sh []int) string {
	parcalar := make([]string, len(sh))
	for i, d := range sh {
		parcalar[i] = strconv.Itoa(d)
	}
	return strings.Join(parcalar, "×")
}

// eksenCoz negatif ekseni (sondan sayma) pozitife çevirir ve sınırı denetler.
func eksenCoz(fn string, eksen int, sh []int) (int, error) {
	nd := len(sh)
	if nd == 0 {
		return 0, fmt.Errorf("%s: skaler (0 boyutlu) tensörde eksen seçilemez", fn)
	}
	e := eksen
	if e < 0 {
		e += nd
	}
	if e < 0 || e >= nd {
		return 0, fmt.Errorf("%s: eksen %d, %s şeklindeki %d boyutlu tensör için geçersiz; %d ile %d arasında olmalı", fn, eksen, sekilMetni(sh), nd, -nd, nd-1)
	}
	return e, nil
}

// tipAdi hata mesajlarında değerin tipini adlandırır.
func tipAdi(v vm.Value) string { return string(v.Type()) }

// argMetni argümanları virgülle ayrılmış biçimde yazar.
func argMetni(args []vm.Value) string {
	p := make([]string, len(args))
	for i, a := range args {
		p[i] = a.Inspect()
	}
	return strings.Join(p, ", ")
}

// sekilListesi [2, 3] gibi bir TR listesini şekle çevirir.
func sekilListesi(fn string, v vm.Value) ([]int, error) {
	l, ok := v.(*vm.ListVal)
	if !ok {
		return nil, fmt.Errorf("%s: şekil [2, 3] gibi bir tam sayı listesi olmalıdır, %s verildi", fn, tipAdi(v))
	}
	sh := make([]int, len(l.Elements))
	toplamEleman := 1
	for i, el := range l.Elements {
		d, ok := tamSayi(el)
		if !ok || d < 0 {
			return nil, fmt.Errorf("%s: şeklin %d. elemanı negatif olmayan bir tam sayı olmalıdır, %s verildi", fn, i+1, el.Inspect())
		}
		sh[i] = d
		if d > 0 && toplamEleman > vm.MaxCollectionSize/d {
			return nil, fmt.Errorf("%s: %s şekli çok büyük (en fazla %d eleman)", fn, sekilMetni(sh[:i+1]), vm.MaxCollectionSize)
		}
		toplamEleman *= d
	}
	return sh, nil
}

// sinir_ağı ve yz_sor'un adlı argümanları.
var (
	adDevir      = baglama.KodAd(sozluk.T930)
	adAktivasyon = baglama.KodAd(sozluk.T932)
)
