package ogrenme

import (
	"fmt"
	"io"
	"os"
	"sync"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
)

// ----------------------------------------------------------------------------
// İşleçler (VM kancaları yurutucu_kancalari.go'dadır)
// ----------------------------------------------------------------------------

var islemAdi = map[string]string{"+": "toplama", "-": "çıkarma", "*": "çarpma", "/": "bölme"}

// sondanEksen tensörün sondan i. (1'den başlayarak) ekseninin uzunluğunu verir; o eksen
// yoksa yayınlama kuralı gereği 1 sayılır. Şekil kopyalanmaz.
func sondanEksen(t *tensor.Tensor, i int) int {
	if j := t.Dim() - i; j >= 0 {
		return t.Size(j)
	}
	return 1
}

// yayinDenetle iki tensörün şekillerinin yayınlanabilirliğini denetler (eksenler sağdan
// hizalanır; eşit ya da biri 1 olan eksenler uyumludur). Sık çağrıldığı için ayırma
// yapmaz; şekil metni yalnızca hata durumunda üretilir.
func yayinDenetle(op string, a, b *tensor.Tensor) error {
	nd := max(a.Dim(), b.Dim())
	for i := 1; i <= nd; i++ {
		da, db := sondanEksen(a, i), sondanEksen(b, i)
		if da != db && da != 1 && db != 1 {
			return fmt.Errorf("%s ile %s şekilleri %s için yayınlanamaz; sondan %d. eksende %d ≠ %d (eksenler sağdan hizalanır, eşit ya da 1 olmalı)",
				sekilMetni(a.Shape()), sekilMetni(b.Shape()), islemAdi[op], i, da, db)
		}
	}
	return nil
}

func cihazDenetle(islem string, a, b *tensor.Tensor) error {
	if a.Device() != b.Device() {
		return fmt.Errorf("%s: tensörler farklı cihazlarda (%s ve %s); önce .%s(...) ile aynı cihaza taşıyın", islem, a.Device(), b.Device(), kod.Ad(uyeCihaza))
	}
	return nil
}

// ikiTarafGenisler yayınlamada iki işlenenin de genişletilip genişletilmediğini söyler
// (ör. [4, 1] ile [4] → [4, 4]). Bu çoğunlukla bir şekil hatasıdır. Ayırma yapmaz.
func ikiTarafGenisler(a, b *tensor.Tensor) bool {
	nd := max(a.Dim(), b.Dim())
	var aGenis, bGenis bool
	for i := 1; i <= nd; i++ {
		da, db := sondanEksen(a, i), sondanEksen(b, i)
		boy := max(da, db)
		if da == 0 || db == 0 {
			boy = 0
		}
		// Eksik eksen de (ön tarafa eklenen) genişletme sayılır, ancak uzunluğu 1 ise
		// veri çoğalmaz.
		if da != boy || (i > a.Dim() && boy != 1) {
			aGenis = true
		}
		if db != boy || (i > b.Dim() && boy != 1) {
			bGenis = true
		}
	}
	return aGenis && bGenis
}

var (
	uyariMu         sync.Mutex
	uyariCikisi     io.Writer = os.Stderr
	verilenUyari              = map[string]bool{}
	uyariSusturuldu bool      // sınır bildirimi yapıldı mı
)

// enFazlaYayinUyarisi gösterilecek en çok farklı yayın uyarısı (işlem ve şekil çifti).
const enFazlaYayinUyarisi = 100

// yayinUyarisi iki tarafın da genişletildiği yayınlamayı standart hataya bir kez yazar.
// İşleçler satır bilgisini almadığından (yorumlayıcı işleç çağrısına satır geçirmez)
// tekrar, işlem ve şekil çiftine göre önlenir.
func yayinUyarisi(op string, a, b *tensor.Tensor) {
	as, bs := sekilMetni(a.Shape()), sekilMetni(b.Shape())
	anahtar := op + as + bs
	uyariMu.Lock()
	defer uyariMu.Unlock()
	if verilenUyari[anahtar] {
		return
	}
	// Harita sınırlıdır: değişken uzunluklu dizilerde her şekil çifti yeni bir anahtar
	// olurdu. Sınıra varılınca bir kez bildirilir, sonraki yeni uyarılar gösterilmez.
	if len(verilenUyari) >= enFazlaYayinUyarisi {
		if !uyariSusturuldu {
			uyariSusturuldu = true
			fmt.Fprintf(uyariCikisi, "Uyarı: %d farklı yayın uyarısı gösterildi; sonraki yayın uyarıları gösterilmeyecek.\n", enFazlaYayinUyarisi)
		}
		return
	}
	verilenUyari[anahtar] = true
	sonuc := make([]int, max(a.Dim(), b.Dim()))
	for i := 1; i <= len(sonuc); i++ {
		sonuc[len(sonuc)-i] = max(sondanEksen(a, i), sondanEksen(b, i))
	}
	fmt.Fprintf(uyariCikisi, "Uyarı: %s ile %s yayınlanarak %s üretildi (iki taraf da genişletildi); bilerek yapmadıysanız şekilleri kontrol edin.\n",
		as, bs, sekilMetni(sonuc))
}

func (d *Deger) ikili(op string, other yurut.Value) (yurut.Value, error) {
	if s, ok := yurut.ToFloat(other); ok {
		f := float32(s)
		r, err := guvenli(func() *tensor.Tensor {
			switch op {
			case "+":
				return d.t.AddScalar(f)
			case "-":
				return d.t.SubScalar(f)
			case "*":
				return d.t.MulScalar(f)
			default:
				// Bölme, 1/s ile çarpma yerine gerçek bölmeyle yapılır (yuvarlama farkı olmasın);
				// sonuç tensörün türündedir (* + - gibi).
				return d.t.DivScalar(f)
			}
		})
		if err != nil {
			return nil, err
		}
		return Yeni(r), nil
	}
	// Sık yol (iki tensör) metin üretmez; işlem adı yalnızca hata mesajında birleştirilir.
	var o *tensor.Tensor
	if od, ok := other.(*Deger); ok {
		o = od.t
	} else {
		var err error
		if o, err = tensoreCevirCihaz("tensör "+islemAdi[op], other, d.t); err != nil {
			return nil, err
		}
	}
	if err := yayinDenetle(op, d.t, o); err != nil {
		return nil, err
	}
	if d.t.Device() != o.Device() {
		return nil, cihazDenetle("tensör "+islemAdi[op], d.t, o)
	}
	if ikiTarafGenisler(d.t, o) {
		yayinUyarisi(op, d.t, o)
	}
	r, err := guvenli(func() *tensor.Tensor {
		switch op {
		case "+":
			return tensor.Add(d.t, o)
		case "-":
			return tensor.Sub(d.t, o)
		case "*":
			return tensor.Mul(d.t, o)
		default:
			return tensor.Div(d.t, o)
		}
	})
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// matCarpTur '@' işlecidir: matris çarpımı. 1 boyutlu işlenenler vektör sayılır.
// karisik Float32 değilse (karışık_hassasiyet bloğu) girdiler o türe çevrilir (türevlenebilir
// kopya; ağırlıklar değişmez) ve çarpım float32 biriktirmeyle float32 sonuç verir. Aksi
// halde sonuç girdilerin türündedir (türler farklıysa float32).
func matCarpTur(a, b *tensor.Tensor, karisik tensor.DType) (*tensor.Tensor, error) {
	as, bs := a.Shape(), b.Shape()
	if len(as) == 0 || len(bs) == 0 {
		return nil, fmt.Errorf("%s ile %s matris çarpımı yapılamaz; skaler için '*' kullanın", sekilMetni(as), sekilMetni(bs))
	}
	if err := cihazDenetle("matris çarpımı (@)", a, b); err != nil {
		return nil, err
	}
	K := as[len(as)-1]
	var bK int
	if len(bs) == 1 {
		bK = bs[0]
	} else {
		bK = bs[len(bs)-2]
	}
	if K != bK {
		return nil, fmt.Errorf("%s ile %s matris çarpımı yapılamaz; %d ≠ %d (soldakinin son ekseni sağdakinin %s eksenine eşit olmalı)",
			sekilMetni(as), sekilMetni(bs), K, bK, map[bool]string{true: "tek", false: "sondan ikinci"}[len(bs) == 1])
	}
	if len(as) >= 2 && len(bs) > 2 {
		onA, onB := as[:len(as)-2], bs[:len(bs)-2]
		if len(onA) != len(onB) || sekilMetni(onA) != sekilMetni(onB) {
			return nil, fmt.Errorf("%s ile %s toplu matris çarpımı yapılamaz; öndeki eksenler eşit olmalı (%s ≠ %s)",
				sekilMetni(as), sekilMetni(bs), sekilMetni(onA), sekilMetni(onB))
		}
	}
	if len(as) == 1 && len(bs) > 2 {
		return nil, fmt.Errorf("%s ile %s matris çarpımı yapılamaz; vektör toplu matrisle çarpılamaz", sekilMetni(as), sekilMetni(bs))
	}
	return guvenli(func() *tensor.Tensor {
		x, y := a, b
		if len(as) == 1 {
			x = x.Unsqueeze(0)
		}
		if len(bs) == 1 {
			y = y.Unsqueeze(1)
		}
		var r *tensor.Tensor
		if karisik != tensor.Float32 {
			r = tensor.MatMulOut(x.Cast(karisik), y.Cast(karisik), tensor.Float32)
		} else {
			r = tensor.MatMul(x, y)
		}
		if len(bs) == 1 {
			r = r.Squeeze(-1)
		}
		if len(as) == 1 {
			r = r.Squeeze(0)
		}
		return r
	})
}
