// Package ogrenme, n-boyutlu tensör çekirdeğini (tr_dili/internal/hesap/tensor) TR diline bağlar:
// tensör değeri, işleçleri (@ + - * /), özellik ve metotları, matematik ve sinir ağı
// fonksiyonları, türevsiz çalışma, iyileştiriciler ve hazır sinir ağı modeli.
//
// Dilin tek sayısal dizi türü tensördür: tek sayıdan tensör 0 boyutlu, matris adlı
// oluşturucuların (sıfırlar([2, 3]), birim_matris ...) sonucu 2 boyutlu tensördür.
//
// Aynı adı taşıyan çekirdek ve yapay zekâ yerleşikleri (toplam, ortalama, min, maks,
// karekök) silinmez; kayıt sırasında sarılır: argüman tensörse yeni yol, değilse eski
// yerleşik çalışır. Bu yüzden bu motor onlardan SONRA kaydedilmelidir (bkz. internal/motor).
package ogrenme

import (
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"tr_dili/internal/dil/kod"

	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
)

// Deger, TR'de bir tensörü taşıyan değerdir. Tip adı "Tensör"dür.
type Deger struct {
	t *tensor.Tensor
	// ara, tensörün türev izlenen bir işlemin sonucu (yaprak olmayan) olduğunu gösterir.
	// Çekirdek, geri yayılımdan sonra ara düğümlerin çizge bağlarını bıraktığı için bu
	// bilgi oluşturma anında saklanır; böylece ara tensörün .gradyan'ı sessizce 'yok'
	// dönmez ve ara tensör iyileştirici parametresi olarak kabul edilmez.
	ara bool
	// tamSayili, değerlerin tam sayı olduğunu (ör. maks_indeks sonucu) gösterir: .değer
	// Tam döndürür, yazdırma ondalıksız yapılır. İşlem sonuçlarına geçmez.
	tamSayili bool
}

// Yeni çekirdek tensörünü TR değeri olarak sarar.
func Yeni(t *tensor.Tensor) *Deger { return &Deger{t: t, ara: t.RequiresGrad() && !t.IsLeaf()} }

// YeniTam değerleri tam sayı olan (ör. token kimlikleri) bir tensörü sarar: .değer Tam
// verir ve tensör ondalıksız yazdırılır.
func YeniTam(t *tensor.Tensor) *Deger {
	d := Yeni(t)
	d.tamSayili = true
	return d
}

// Tensor sarılı çekirdek tensörünü verir.
func (d *Deger) Tensor() *tensor.Tensor { return d.t }

func (d *Deger) Type() yurut.ValueType { return yurut.VAL_TENSOR }

// IsTruthy tek elemanlı tensörde elemanın 0'dan farklı olup olmadığıdır; boş tensör
// yanlıştır. Çok elemanlı tensörün doğruluk değeri belirsizdir; yorumlayıcı bu arayüzden
// hata alamadığı için şimdilik "boş değil" (doğru) sayılır.
func (d *Deger) IsTruthy() bool {
	switch d.t.Numel() {
	case 0:
		return false
	case 1:
		return d.t.Item() != 0
	}
	return true
}

// ----------------------------------------------------------------------------
// Yazdırma
// ----------------------------------------------------------------------------

// yazdirmaSiniri bu sayıdan çok elemanı olan tensörlerde her eksenin yalnızca
// başı ve sonu yazılır.
const yazdirmaSiniri = 1000

// kenarEleman kısaltılmış yazdırmada eksen başına baştan ve sondan gösterilen eleman sayısı.
const kenarEleman = 3

func (d *Deger) Inspect() string {
	t := d.t
	sh := t.Shape()
	var ek []string
	// Tür yalnızca float32 dışındaki türlerde gösterilir: "Tensör[2×3] bf16".
	turEki := ""
	if t.DType() != tensor.Float32 {
		turEki = " " + turAdi(t.DType())
	}
	if t.Device() != tensor.CPU {
		ek = append(ek, t.Device().String())
	}
	if t.RequiresGrad() {
		ek = append(ek, "türev izleniyor")
	}
	if len(sh) == 0 {
		x := t.Item()
		s := sayiMetni(x, bilimselMi([]float32{x}))
		if d.tamSayili {
			s = strconv.FormatInt(int64(x), 10)
		}
		if len(ek) > 0 {
			s += ", " + strings.Join(ek, ", ")
		}
		return "Tensör(" + s + ")" + turEki
	}
	baslik := "Tensör[" + sekilCarpimi(sh) + "]" + turEki
	if len(ek) > 0 {
		baslik += " (" + strings.Join(ek, ", ") + ")"
	}
	kisalt := t.Numel() > yazdirmaSiniri
	// Gösterilecek hücreler: kısaltılmamış tensörde hepsi (tek kopya), kısaltılmışta
	// yalnızca eksen başı/sonu hücreleri tek tek okunur; tüm veri kopyalanıp
	// biçimlenmez. Hücreler gezinme sırasıyla toplanır, sonra aynı sırayla yazılır.
	var veri []float32
	if !kisalt {
		veri = t.Data()
	}
	adimlar := make([]int, len(sh))
	s := 1
	for i := len(sh) - 1; i >= 0; i-- {
		adimlar[i] = s
		s *= sh[i]
	}
	idx := make([]int, len(sh))
	var hucreler []float32
	var topla func(eksen, ofset int)
	topla = func(eksen, ofset int) {
		for _, i := range gosterilecek(sh[eksen], kisalt) {
			if i < 0 {
				continue
			}
			idx[eksen] = i
			if eksen < len(sh)-1 {
				topla(eksen+1, ofset+i*adimlar[eksen])
			} else if kisalt {
				hucreler = append(hucreler, t.At(idx...))
			} else {
				hucreler = append(hucreler, veri[ofset+i])
			}
		}
	}
	topla(0, 0)
	// Tüm hücreler aynı biçimde ve genişlikte yazılır.
	bilimsel := bilimselMi(hucreler)
	metinler := make([]string, len(hucreler))
	genislik := 0
	for i, x := range hucreler {
		if d.tamSayili {
			metinler[i] = strconv.FormatInt(int64(x), 10)
		} else {
			metinler[i] = sayiMetni(x, bilimsel)
		}
		genislik = max(genislik, len([]rune(metinler[i])))
	}
	sira := 0
	var sb strings.Builder
	sb.WriteString(baslik + " ")
	var yaz func(eksen int, girinti string)
	yaz = func(eksen int, girinti string) {
		n := sh[eksen]
		indeksler := gosterilecek(n, kisalt)
		if eksen == len(sh)-1 {
			sb.WriteString("[")
			for k, i := range indeksler {
				if k > 0 {
					sb.WriteString(", ")
				}
				if i < 0 {
					sb.WriteString("…")
					continue
				}
				m := metinler[sira]
				sira++
				for p := len([]rune(m)); p < genislik; p++ {
					sb.WriteByte(' ')
				}
				sb.WriteString(m)
			}
			sb.WriteString("]")
			return
		}
		ic := girinti + "  "
		sb.WriteString("[\n")
		for k, i := range indeksler {
			if k > 0 {
				sb.WriteString(",\n")
			}
			sb.WriteString(ic)
			if i < 0 {
				sb.WriteString("…")
				continue
			}
			yaz(eksen+1, ic)
		}
		sb.WriteString("\n" + girinti + "]")
	}
	yaz(0, "")
	return sb.String()
}

// bilimselMi sabit noktalı (%.4f) gösterimin bilgi kaybettireceği durumda bilimsel
// gösterime geçilip geçilmeyeceğini söyler: sıfır olmayan bir değer 0.0001'den küçükse
// (0.0000 görünürdü) ya da bir değer 1e8 ve üzerindeyse.
func bilimselMi(x []float32) bool {
	for _, v := range x {
		a := math.Abs(float64(v))
		if math.IsNaN(a) || math.IsInf(a, 0) {
			continue
		}
		if (a != 0 && a < 1e-4) || a >= 1e8 {
			return true
		}
	}
	return false
}

// gosterilecek bir eksende yazılacak indeksleri verir; -1 "…" yerini gösterir.
func gosterilecek(n int, kisalt bool) []int {
	if !kisalt || n <= 2*kenarEleman {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, 2*kenarEleman+1)
	for i := 0; i < kenarEleman; i++ {
		out = append(out, i)
	}
	out = append(out, -1)
	for i := n - kenarEleman; i < n; i++ {
		out = append(out, i)
	}
	return out
}

// sayiMetni bir hücreyi yazar. Sayı olmayan değerler Türkçe ve tutarlıdır:
// "Tanımsız" (NaN), "Sonsuz", "-Sonsuz".
func sayiMetni(x float32, bilimsel bool) string {
	f := float64(x)
	switch {
	case math.IsNaN(f):
		return "Tanımsız"
	case math.IsInf(f, 1):
		return "Sonsuz"
	case math.IsInf(f, -1):
		return "-Sonsuz"
	}
	if bilimsel {
		return strconv.FormatFloat(f, 'e', 4, 64)
	}
	return strconv.FormatFloat(f, 'f', 4, 64)
}

// ----------------------------------------------------------------------------
// TR değerinden tensöre dönüşüm
// ----------------------------------------------------------------------------

// tensoreCevir sayı, iç içe liste ya da tensörü çekirdek tensörüne çevirir.
// Tensör verilirse kendisi döner (kopyalanmaz, hesap çizgesi korunur).
func tensoreCevir(fn string, v yurut.Value) (*tensor.Tensor, error) {
	switch x := v.(type) {
	case *Deger:
		return x.t, nil
	case *yurut.IntVal:
		return tensor.Scalar(float32(x.Val)), nil
	case *yurut.FloatVal:
		return tensor.Scalar(float32(x.Val)), nil
	case *yurut.ListVal:
		veri, sh, err := listeVerisi(fn, x)
		if err != nil {
			return nil, err
		}
		return tensor.New(veri, sh...), nil
	}
	ipucu := ""
	if b, ok := v.(*yurut.BuiltinVal); ok {
		ipucu = fmt.Sprintf("; metodu çağırmak için parantez ekleyin: %s()", b.Name)
	}
	return nil, fmt.Errorf("%s: %s tipi tensöre dönüştürülemez (sayı, iç içe liste ya da tensör bekleniyor)%s", fn, tipAdi(v), ipucu)
}

// listeVerisi iç içe listeyi düz veri ve şekle çevirir; düzensiz listeleri reddeder.
func listeVerisi(fn string, l *yurut.ListVal) ([]float32, []int, error) {
	var veri []float32
	sh, err := listeGez(fn, l, func(n int) { veri = make([]float32, 0, n) }, func(v yurut.Value) error {
		f, ok := yurut.ToFloat(v)
		if !ok {
			return fmt.Errorf("%s: tensör elemanları sayı olmalıdır, %s (%s) verildi", fn, v.Inspect(), tipAdi(v))
		}
		veri = append(veri, float32(f))
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return veri, sh, nil
}

// listeTamSayilari iç içe tam sayı listesini düz dizi ve şekle çevirir. Değerler doğrudan
// tam sayı olarak okunur (float32'den geçirilmez; 2^24 üstü indeksler yuvarlanmaz).
func listeTamSayilari(fn string, l *yurut.ListVal) ([]int, []int, error) {
	var veri []int
	sh, err := listeGez(fn, l, func(n int) { veri = make([]int, 0, n) }, func(v yurut.Value) error {
		n, ok := tamSayi(v)
		if !ok {
			if _, sayi := yurut.ToFloat(v); sayi {
				return fmt.Errorf("%s: indeksler tam sayı olmalıdır, %s verildi", fn, v.Inspect())
			}
			return fmt.Errorf("%s: indeksler tam sayı olmalıdır, %s (%s) verildi", fn, v.Inspect(), tipAdi(v))
		}
		veri = append(veri, n)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return veri, sh, nil
}

// listeGez iç içe listenin şeklini bulur (ilk elemanlar boyunca inilerek), düzensizliği
// reddeder ve yaprakları sırayla yaprak'a verir; hazirla eleman sayısıyla bir kez çağrılır.
func listeGez(fn string, l *yurut.ListVal, hazirla func(n int), yaprak func(v yurut.Value) error) ([]int, error) {
	var sh []int
	var cur yurut.Value = l
	for {
		ll, ok := cur.(*yurut.ListVal)
		if !ok {
			break
		}
		sh = append(sh, len(ll.Elements))
		if len(ll.Elements) == 0 {
			break
		}
		cur = ll.Elements[0]
	}
	n := 1
	for _, d := range sh {
		n *= d
		if n > yurut.MaxCollectionSize {
			return nil, fmt.Errorf("%s: liste çok büyük (en fazla %d eleman)", fn, yurut.MaxCollectionSize)
		}
	}
	hazirla(n)
	var gez func(v yurut.Value, eksen int) error
	gez = func(v yurut.Value, eksen int) error {
		if eksen == len(sh) {
			if _, liste := v.(*yurut.ListVal); liste {
				return fmt.Errorf("%s: iç içe liste düzensiz; %d. derinlikte sayı yerine liste var", fn, eksen+1)
			}
			return yaprak(v)
		}
		ll, ok := v.(*yurut.ListVal)
		if !ok {
			return fmt.Errorf("%s: iç içe liste düzensiz; %d. derinlikte liste yerine %s var (beklenen şekil %s)", fn, eksen+1, v.Inspect(), sekilMetni(sh))
		}
		if len(ll.Elements) != sh[eksen] {
			return fmt.Errorf("%s: iç içe liste düzensiz; %d. derinlikte %d ve %d uzunlukta listeler var (tüm satırlar eşit uzunlukta olmalı)", fn, eksen+1, sh[eksen], len(ll.Elements))
		}
		for _, el := range ll.Elements {
			if err := gez(el, eksen+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := gez(l, 0); err != nil {
		return nil, err
	}
	return sh, nil
}

// ----------------------------------------------------------------------------
// İşleçler (VM kancaları vm_kancalari.go'dadır)
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
		return fmt.Errorf("%s: tensörler farklı cihazlarda (%s ve %s); önce .cihaza(...) ile aynı cihaza taşıyın", islem, a.Device(), b.Device())
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
		if o, err = tensoreCevir("tensör "+islemAdi[op], other); err != nil {
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

// ----------------------------------------------------------------------------
// Özellikler ve metotlar
// ----------------------------------------------------------------------------

// Tensör üyeleri (adları kod listesinde, internal/dil/kod Y4xx).
const (
	uyeSekil           = kod.Y400
	uyeCihaz           = kod.Y401
	uyeTur             = kod.Y402
	uyeGradyan         = kod.Y403
	uyeDeger           = kod.Y404
	uyeSekillendir     = kod.Y410
	uyeDevrik          = kod.Y411
	uyeEksenleriSirala = kod.Y412
	uyeDilim           = kod.Y413
	uyeCihaza          = kod.Y414
	uyeTure            = kod.Y415
)

// tensorUyeKodlari öneri listesinin sırasıdır.
var tensorUyeKodlari = []kod.Kod{uyeSekil, uyeCihaz, uyeTur, uyeGradyan, uyeDeger, uyeSekillendir, uyeDevrik, uyeEksenleriSirala, uyeDilim, uyeCihaza, uyeTure, uyeBol}

var tensorUyeleri = kod.Adlar(tensorUyeKodlari...)

func tensorUyesi(k kod.Kod) bool { return slices.Contains(tensorUyeKodlari, k) }

func (d *Deger) GetProperty(kd kod.Kod, prop string) (yurut.Value, error) {
	t := d.t
	switch kd {
	case uyeSekil:
		return intListesi(t.Shape()), nil
	case uyeCihaz:
		return &yurut.StringVal{Val: t.Device().String()}, nil
	case uyeTur:
		return &yurut.StringVal{Val: turAdi(t.DType())}, nil
	case uyeTure:
		return d.adliMetot(uyeTure, d.ture), nil
	case uyeGradyan:
		if d.ara {
			return nil, fmt.Errorf("bu tensör (%s) bir işlemin sonucu; gradyan yalnızca türev_izle: doğru ile oluşturulan tensörlerde tutulur. Ağırlığı güncellemek için bir iyileştirici (AdamW, SGD) kullanın ya da yeni değeri türevsiz { } içinde hesaplayıp tensör(..., türev_izle: doğru) ile yeniden oluşturun", sekilMetni(t.Shape()))
		}
		g := t.Grad()
		if g == nil {
			return yurut.NULL, nil
		}
		return Yeni(g), nil
	case uyeDeger:
		return d.degerListesi(), nil
	case uyeSekillendir:
		return d.metot(uyeSekillendir, d.sekillendir), nil
	case uyeDevrik:
		return d.metot(uyeDevrik, func(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("devrik() argüman almaz")
			}
			return devrik(t)
		}), nil
	case uyeEksenleriSirala:
		return d.metot(uyeEksenleriSirala, d.eksenleriSirala), nil
	case uyeDilim:
		return d.metot(uyeDilim, d.dilim), nil
	case uyeCihaza:
		return d.metot(uyeCihaza, d.cihaza), nil
	case uyeBol:
		return d.adliMetot(uyeBol, d.bol), nil
	}
	oneri := ""
	if k, _, eski := kod.EskiBul(kod.AlanUye, prop, tensorUyesi); eski {
		o := kod.Ad(k)
		if k == uyeDevrik {
			o += "()" // metot
		}
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	} else if o := turkce.EnYakin(prop, tensorUyeleri); o != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return nil, fmt.Errorf("Tensör üzerinde %q özelliği ya da metodu bulunamadı%s", prop, oneri)
}

// metot adlı argüman almayan bir tensör metodunun bağlı değeridir; adliMetot adlı
// argümanlarını kendisi denetleyen bir metodunkidir.
func (d *Deger) metot(kd kod.Kod, fn yurut.BuiltinFunc) *yurut.BuiltinVal {
	return &yurut.BuiltinVal{Name: kod.Ad(kd), Fn: fn, AdliAlmaz: true}
}

func (d *Deger) adliMetot(kd kod.Kod, fn yurut.BuiltinFunc) *yurut.BuiltinVal {
	return &yurut.BuiltinVal{Name: kod.Ad(kd), Fn: fn}
}

func intListesi(sh []int) *yurut.ListVal {
	el := make([]yurut.Value, len(sh))
	for i, x := range sh {
		el[i] = yurut.NewInt(int64(x))
	}
	return &yurut.ListVal{Elements: el}
}

// kisaFloat64 float32 değeri, onu yazan en kısa ondalık gösterime karşılık gelen
// float64'e çevirir: 0.1 (float32) → 0.1, 0.10000000149011612 değil. Böylece
// a.değer[0] == 0.1 doğru olur.
func kisaFloat64(x float32) float64 {
	f := float64(x)
	if f == math.Trunc(f) || math.IsInf(f, 0) || math.IsNaN(f) {
		return f
	}
	var tampon [32]byte
	s := strconv.AppendFloat(tampon[:0], f, 'g', -1, 32)
	if g, err := strconv.ParseFloat(string(s), 64); err == nil {
		return g
	}
	return f
}

// degerListesi tensörün değerlerini Ondalık (0 boyutlu) ya da iç içe liste olarak verir.
// Büyük tensörlerde her liste ve sayı için ayrı ayırma yapmamak için tüm listeler,
// eleman dilimleri ve sayılar önceden ayrılmış üç dilimden dağıtılır. Eleman dilimleri
// kapasiteyle sınırlıdır (a[i:j:j]); listeye eleman eklemek komşu listeyi bozmaz.
func (d *Deger) degerListesi() yurut.Value {
	veri := d.t.Data()
	sh := d.t.Shape()
	if len(sh) == 0 {
		if d.tamSayili {
			return yurut.NewInt(int64(veri[0]))
		}
		return &yurut.FloatVal{Val: kisaFloat64(veri[0])}
	}
	nd := len(sh)
	listeSay, degerSay, p := 0, 0, 1
	for k := 0; k < nd; k++ {
		listeSay += p
		p *= sh[k]
		degerSay += p
	}
	listeler := make([]yurut.ListVal, listeSay)
	degerler := make([]yurut.Value, degerSay)
	var sayilar []yurut.FloatVal
	if !d.tamSayili {
		sayilar = make([]yurut.FloatVal, len(veri))
		for i, x := range veri {
			sayilar[i].Val = kisaFloat64(x)
		}
	}
	li, di := 0, 0 // bu seviyenin ilk listesi ve ilk eleman yeri
	p = 1          // bu seviyedeki liste sayısı
	for k := 0; k < nd; k++ {
		n := sh[k]
		sonrakiListe := li + p
		for j := 0; j < p; j++ {
			bas := di + j*n
			el := degerler[bas : bas+n : bas+n]
			for i := range el {
				m := j*n + i // bu seviyedeki elemanın sırası
				if k == nd-1 && d.tamSayili {
					el[i] = yurut.NewInt(int64(veri[m]))
				} else if k == nd-1 {
					el[i] = &sayilar[m]
				} else {
					el[i] = &listeler[sonrakiListe+m]
				}
			}
			listeler[li+j].Elements = el
		}
		li = sonrakiListe
		di += p * n
		p *= n
	}
	return &listeler[0]
}

func (d *Deger) sekillendir(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
	var hedef []yurut.Value
	if len(args) == 1 {
		if l, ok := args[0].(*yurut.ListVal); ok {
			hedef = l.Elements
		}
	}
	if hedef == nil {
		hedef = args
	}
	if len(hedef) == 0 {
		return nil, fmt.Errorf("şekillendir([4, 6]) yeni şekli liste olarak bekler")
	}
	sh := make([]int, len(hedef))
	bilinmeyen := 0
	for i, v := range hedef {
		n, ok := tamSayi(v)
		if !ok || n < -1 {
			return nil, fmt.Errorf("şekillendir: şeklin %d. elemanı negatif olmayan bir tam sayı (ya da çıkarsanacak eksen için -1) olmalıdır, %s verildi", i+1, v.Inspect())
		}
		if n == -1 {
			bilinmeyen++
		}
		sh[i] = n
	}
	if bilinmeyen > 1 {
		return nil, fmt.Errorf("şekillendir: yalnızca bir eksen -1 olabilir, %s verildi", sekilMetni(sh))
	}
	n := d.t.Numel()
	carpim := 1
	for _, x := range sh {
		if x != -1 {
			carpim *= x
		}
	}
	if (bilinmeyen == 0 && carpim != n) || (bilinmeyen == 1 && (carpim == 0 || n%carpim != 0)) {
		return nil, fmt.Errorf("%s şeklindeki tensör (%d eleman) %s şekline sokulamaz", sekilMetni(d.t.Shape()), n, sekilMetni(sh))
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.Reshape(sh...) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func devrik(t *tensor.Tensor) (yurut.Value, error) {
	sh := t.Shape()
	if len(sh) < 2 {
		return nil, fmt.Errorf("devrik() en az 2 boyutlu tensör ister, %s verildi", sekilMetni(sh))
	}
	r, err := guvenli(func() *tensor.Tensor { return t.T() })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func (d *Deger) eksenleriSirala(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
	var liste []yurut.Value
	if len(args) == 1 {
		if l, ok := args[0].(*yurut.ListVal); ok {
			liste = l.Elements
		}
	}
	if liste == nil {
		liste = args
	}
	sh := d.t.Shape()
	if len(liste) != len(sh) {
		return nil, fmt.Errorf("eksenleri_sırala: %s şeklindeki %d boyutlu tensör için %d eksen verilmeli, %d verildi", sekilMetni(sh), len(sh), len(sh), len(liste))
	}
	dims := make([]int, len(liste))
	gorulen := make([]bool, len(sh))
	for i, v := range liste {
		n, ok := tamSayi(v)
		if !ok {
			return nil, fmt.Errorf("eksenleri_sırala: eksenler tam sayı olmalıdır, %s verildi", v.Inspect())
		}
		e, err := eksenCoz("eksenleri_sırala", n, sh)
		if err != nil {
			return nil, err
		}
		if gorulen[e] {
			return nil, fmt.Errorf("eksenleri_sırala: %d. eksen birden çok kez verildi; her eksen bir kez yer almalı", e)
		}
		gorulen[e] = true
		dims[i] = e
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.Permute(dims...) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func (d *Deger) dilim(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("dilim(eksen, baş, son) 3 parametre bekler, %d verildi", len(args))
	}
	var n [3]int
	for i, a := range args {
		x, ok := tamSayi(a)
		if !ok {
			return nil, fmt.Errorf("dilim(eksen, baş, son) tam sayılar bekler, %s verildi", a.Inspect())
		}
		n[i] = x
	}
	sh := d.t.Shape()
	e, err := eksenCoz("dilim", n[0], sh)
	if err != nil {
		return nil, err
	}
	bas, son := n[1], n[2]
	uz := sh[e]
	if bas < 0 {
		bas += uz
	}
	if son < 0 {
		son += uz
	}
	if bas < 0 || bas > uz || son < bas || son > uz {
		return nil, fmt.Errorf("dilim: %d. eksenin uzunluğu %d (%s); [%d, %d) aralığı geçersiz (0 ≤ baş ≤ son ≤ %d olmalı)", e, uz, sekilMetni(sh), n[1], n[2], uz)
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.Slice(e, bas, son) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// cihazCoz "cpu", "gpu", "gpu:1" gibi bir adı cihaza çevirir.
func cihazCoz(ad string) (tensor.Device, error) {
	a := strings.ToLower(strings.TrimSpace(ad))
	switch {
	case a == "cpu":
		return tensor.CPU, nil
	case a == "gpu":
		return tensor.GPU(0), nil
	case strings.HasPrefix(a, "gpu:"):
		var i int
		if _, err := fmt.Sscanf(a[4:], "%d", &i); err == nil && i >= 0 {
			return tensor.GPU(i), nil
		}
	}
	return tensor.Device{}, fmt.Errorf("bilinmeyen cihaz %q; \"cpu\", \"gpu\" ya da \"gpu:1\" gibi bir ad verin", ad)
}

func (d *Deger) cihaza(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("cihaza(\"cpu\") 1 parametre bekler")
	}
	s, ok := args[0].(*yurut.StringVal)
	if !ok {
		return nil, fmt.Errorf("cihaza() cihaz adını metin olarak bekler (\"cpu\" ya da \"gpu\"), %s verildi", tipAdi(args[0]))
	}
	dev, err := cihazCoz(s.Val)
	if err != nil {
		return nil, err
	}
	if !tensor.HasDevice(dev) {
		return nil, fmt.Errorf("%s cihazı kullanılamıyor: bu TR derlemesinde GPU arka ucu yok; tensörler şimdilik yalnızca \"cpu\" üzerinde çalışır", dev)
	}
	r, err := guvenli(func() *tensor.Tensor { return d.t.To(dev) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// ----------------------------------------------------------------------------
// Hata ayıklayıcı (İzleme Masası) desteği
// ----------------------------------------------------------------------------

// IzlemeTuru İzleme Masası'nda ikon seçimi için değerin türüdür.
func (d *Deger) IzlemeTuru() string { return "tensor" }

// IzlemeEtiketi tip sütununda gösterilen özettir (ör. "Tensör [2×3]"; float32 dışındaki
// türlerde tür de eklenir: "Tensör [2×3] bf16").
func (d *Deger) IzlemeEtiketi() string {
	sh := d.t.Shape()
	turEki := ""
	if dt := d.t.DType(); dt != tensor.Float32 {
		turEki = " " + turAdi(dt)
	}
	if len(sh) == 0 {
		return "Tensör (skaler)" + turEki
	}
	return "Tensör [" + sekilCarpimi(sh) + "]" + turEki
}

// TabloVerisi 1-2 boyutlu tensörü tabloya (sütunlar + satırlar) çevirir.
func (d *Deger) TabloVerisi() ([]string, [][]string, error) {
	const sinir = 10000
	sh := d.t.Shape()
	veri := d.t.Data()
	bicim := func(x float32) string { return fmt.Sprintf("%.6g", x) }
	switch len(sh) {
	case 1:
		var rows [][]string
		for i, x := range veri {
			if i >= sinir {
				break
			}
			rows = append(rows, []string{bicim(x)})
		}
		return []string{"Değer"}, rows, nil
	case 2:
		cols := make([]string, sh[1])
		for j := range cols {
			cols[j] = fmt.Sprintf("Sütun %d", j+1)
		}
		var rows [][]string
		for i := 0; i < sh[0] && i < sinir; i++ {
			row := make([]string, sh[1])
			for j := range row {
				row[j] = bicim(veri[i*sh[1]+j])
			}
			rows = append(rows, row)
		}
		return cols, rows, nil
	}
	return nil, nil, fmt.Errorf("tablo görünümü yalnızca 1 ve 2 boyutlu tensörler için kullanılabilir (bu tensör: %s)", sekilMetni(sh))
}
