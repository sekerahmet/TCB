package ml

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Çekirdek tensör paketi (github.com/sekerahmet/TCB/internal/compute/tensor) hataları panik ile bildirir. Bu dosyadaki
// yardımcılar her çekirdek çağrısını sarar ve paniği, TR'nin 'dene/yakala' ile
// yakalanabilen sıradan bir hataya çevirir. Yerleşik çağrılarında VM mesajın başına
// "Satır N:" ekler. Adlı argüman denetimi ve paniği yakalama internal/stdlib/common/bind'nın
// ortak yardımcılarıyla yapılır.

// Adlı argümanlar. Adları kod listesindedir (internal/lang/codes, T9xx). Başka dillerden alışılmış
// adlar (lr, axis...) kabul edilmez; baglama.AdliDenetle hata mesajında Türkçesini önerir.
var (
	adTurevIzle       = bind.KodAd(codes.T900)
	adEksen           = bind.KodAd(codes.T902)
	adTohum           = bind.KodAd(codes.T903)
	adEnFazla         = bind.KodAd(codes.T904)
	adEnAz            = bind.KodAd(codes.T907)
	adEpsilon         = bind.KodAd(codes.T905)
	adOgrenmeOrani    = bind.KodAd(codes.T910)
	adAgirlikCurumesi = bind.KodAd(codes.T911)
	adBeta1           = bind.KodAd(codes.T912)
	adBeta2           = bind.KodAd(codes.T913)
	adMomentum        = bind.KodAd(codes.T914)
	adAgirlik         = bind.KodAd(codes.T920)
	adSapma           = bind.KodAd(codes.T921)
	adTur             = bind.KodAd(codes.T901)
	adAnaKopya        = bind.KodAd(codes.T915)
)

// turevIzleyenOrnek hata mesajlarındaki türev izleyen tensör örneğidir:
// turevIzleyenOrnek("...") → "tensör(..., türev_izle: doğru)"; adlar koddan kurulur.
func turevIzleyenOrnek(veri string) string {
	return codes.Name(codes.T100) + "(" + veri + ", " + adTurevIzle.TR + ": " + codes.Name(codes.A040) + ")"
}

// guvenli fn'i çalıştırır; çekirdekten gelen paniği Türkçe bir hataya dönüştürür.
func guvenli[T any](fn func() T) (T, error) { return bind.Kurtar(fn, panikHatasi) }

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
func tamSayi(v interp.Value) (int, bool) {
	n, ok := bind.TamSayi(v)
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
func tipAdi(v interp.Value) string { return string(v.Type()) }

// argMetni argümanları virgülle ayrılmış biçimde yazar.
func argMetni(args []interp.Value) string {
	p := make([]string, len(args))
	for i, a := range args {
		p[i] = a.Inspect()
	}
	return strings.Join(p, ", ")
}

// sekilListesi [2, 3] gibi bir TR listesini şekle çevirir.
func sekilListesi(fn string, v interp.Value) ([]int, error) {
	l, ok := v.(*interp.ListVal)
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
		if d > 0 && toplamEleman > interp.MaxCollectionSize/d {
			return nil, fmt.Errorf("%s: %s şekli çok büyük (en fazla %d eleman)", fn, sekilMetni(sh[:i+1]), interp.MaxCollectionSize)
		}
		toplamEleman *= d
	}
	return sh, nil
}
