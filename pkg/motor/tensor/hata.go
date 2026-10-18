package tensor

import (
	"fmt"
	"strconv"
	"strings"

	"tr_dili/pkg/motor/yz"
	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

// Çekirdek tensör paketi (tr_dili/pkg/tensor) hataları panik ile bildirir. Bu dosyadaki
// yardımcılar her çekirdek çağrısını sarar ve paniği, TR'nin 'dene/yakala' ile
// yakalanabilen sıradan bir hataya çevirir. Yerleşik çağrılarında VM mesajın başına
// "Satır N:" ekler.

// guvenli fn'i çalıştırır; çekirdekten gelen paniği Türkçe bir hataya dönüştürür.
func guvenli[T any](fn func() T) (sonuc T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = panikHatasi(r)
		}
	}()
	return fn(), nil
}

// panikHatasi çekirdeğin panik değerini kullanıcıya gösterilecek hataya çevirir.
func panikHatasi(r interface{}) error {
	if e, ok := r.(error); ok {
		return fmt.Errorf("tensör işlemi başarısız: %v", e)
	}
	msg := fmt.Sprint(r)
	msg = strings.TrimPrefix(msg, "tensor: ")
	return fmt.Errorf("tensör işlemi başarısız: %s", msg)
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

// ingilizceAdlar başka dillerden alışılmış parametre adlarının Türkçe karşılıklarıdır.
// Bu adlar KABUL EDİLMEZ; yalnızca hata mesajında doğru Türkçe ad önerilir.
var ingilizceAdlar = map[string]string{
	"lr":            "öğrenme_oranı",
	"learning_rate": "öğrenme_oranı",
	"axis":          "eksen",
	"dim":           "eksen",
	"requires_grad": "türev_izle",
	"weight_decay":  "ağırlık_çürümesi",
	"eps":           "epsilon",
	"seed":          "tohum",
	"weight":        "ağırlık",
	"bias":          "sapma",
	"betas":         "beta1",
}

// adliDenetle verilmiş adlı argümanların bilinen adlardan olduğunu denetler; yazım
// hatasında (ya da İngilizce bir adda) en yakın Türkçe adı önerir.
func adliDenetle(fn string, named map[string]vm.Value, izinli ...string) error {
	for ad := range named {
		bilinen := false
		for _, i := range izinli {
			if ad == i {
				bilinen = true
				break
			}
		}
		if !bilinen {
			oneri := ""
			o := ""
			if tr, ok := ingilizceAdlar[strings.ToLower(ad)]; ok {
				for _, i := range izinli {
					if i == tr {
						o = tr
						break
					}
				}
			}
			if o == "" {
				o = turkce.EnYakin(ad, izinli)
			}
			if o != "" {
				oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
			}
			return fmt.Errorf("%s '%s' adında bir parametre almaz%s", fn, ad, oneri)
		}
	}
	return nil
}

// tipAdi hata mesajlarında değerin tipini adlandırır. Eski tek sayılık türev değeri de
// "Tensör" tipini taşıdığı için çelişkili mesaj ("tensör olmalıdır, Tensör verildi")
// çıkmasın diye açıklayıcı biçimde yazılır.
func tipAdi(v vm.Value) string {
	switch v.(type) {
	case *yz.TensorVal:
		return "tek sayılık türev değeri (tensör(3.0))"
	case *Deger:
		return "çok boyutlu tensör"
	}
	return string(v.Type())
}

// argMetni argümanları virgülle ayrılmış biçimde yazar.
func argMetni(args []vm.Value) string {
	p := make([]string, len(args))
	for i, a := range args {
		p[i] = a.Inspect()
	}
	return strings.Join(p, ", ")
}

// adliAl adlardan ilk verileni döndürür (Türkçe ad ve ASCII karşılığı gibi).
func adliAl(named map[string]vm.Value, adlar ...string) (vm.Value, bool) {
	for _, a := range adlar {
		if v, ok := named[a]; ok {
			return v, true
		}
	}
	return nil, false
}

// tamSayi bir TR değerini tam sayıya çevirir (tam değerli ondalıklar da kabul edilir).
func tamSayi(v vm.Value) (int, bool) {
	switch n := v.(type) {
	case *vm.IntVal:
		return int(n.Val), true
	case *vm.FloatVal:
		if n.Val == float64(int64(n.Val)) {
			return int(n.Val), true
		}
	}
	return 0, false
}

// sekilListesi [2, 3] gibi bir TR listesini şekle çevirir.
func sekilListesi(fn string, v vm.Value) ([]int, error) {
	l, ok := v.(*vm.ListVal)
	if !ok {
		return nil, fmt.Errorf("%s: şekil [2, 3] gibi bir tam sayı listesi olmalıdır, %s verildi", fn, v.Type())
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
