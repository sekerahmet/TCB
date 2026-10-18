// Package parcalayici, genel tokenizer.json parçalayıcısını (tr_dili/internal/hesap/parca)
// TR diline bağlar: parçalayıcı_yükle(yol) ve parçalayıcı nesnesinin .kodla, .çöz,
// .dağarcık_boyutu ve .parça_kimliği üyeleri.
//
// Kodlama/çözme hata döndüren Guvenli... işlevleriyle yapılır: düzenli ifade adım
// sınırı aşılırsa yarım sonuç yerine yol gösterici bir TR hatası oluşur. Uzun işler
// (dosya yükleme, liste kodlama/çözme) yorumlayıcı kilidi bırakılarak yapılır.
package parcalayici

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"tr_dili/internal/dil/kod"

	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/parca"
	"tr_dili/internal/motor/ogrenme"
	"tr_dili/internal/motor/ortak/baglama"
)

// TipAdi parçalayıcı nesnesinin TR'deki tip adıdır (kod listesinde A234).
var TipAdi = string(yurut.TipAdi(kod.A234))

// Nesne TR'de yüklenmiş bir parçalayıcıdır. Salt okunurdur; dosya tutamağı ya da
// başka bir kaynak tutmaz (dosya yüklemede tümüyle okunup kapatılır).
type Nesne struct {
	p   *parca.Parcalayici
	yol string
}

// Parcalayici sarılı Go parçalayıcısını verir.
func (n *Nesne) Parcalayici() *parca.Parcalayici { return n.p }

func (n *Nesne) Type() yurut.ValueType { return yurut.ValueType(TipAdi) }
func (n *Nesne) IsTruthy() bool        { return true }

func (n *Nesne) Inspect() string {
	return fmt.Sprintf("Parçalayıcı(dağarcık %d, %s)", n.p.SozlukBoyutu(), filepath.Base(n.yol))
}

// IzlemeTuru İzleme Masası'nda ikon seçimi için türdür.
func (n *Nesne) IzlemeTuru() string { return "diger" }

// IzlemeEtiketi tip sütununda gösterilen özettir.
func (n *Nesne) IzlemeEtiketi() string {
	return fmt.Sprintf("Parçalayıcı (dağarcık %d)", n.p.SozlukBoyutu())
}

// TabloVerisi dağarcığa eklenmiş parçaları (özel parçalar dahil) tablo olarak verir.
func (n *Nesne) TabloVerisi() ([]string, [][]string, error) {
	ekler := n.p.EkTokenlar()
	if len(ekler) == 0 {
		return nil, nil, fmt.Errorf("bu parçalayıcıda eklenmiş (özel) parça yok; dağarcık boyutu %d", n.p.SozlukBoyutu())
	}
	var satirlar [][]string
	for _, e := range ekler {
		ozel := "hayır"
		if e.Ozel {
			ozel = "evet"
		}
		satirlar = append(satirlar, []string{fmt.Sprint(e.ID), e.Icerik, ozel})
	}
	return []string{"Kimlik", "Parça", "Özel"}, satirlar, nil
}

// Parçalayıcı üyeleri (adları kod listesinde, internal/dil/kod Y61x).
const (
	uyeKodla        = kod.Y610
	uyeCoz          = kod.Y611
	uyeSozlukBoyutu = kod.Y612
	uyeTokenKimligi = kod.Y613
)

var uyeler = kod.Adlar(uyeKodla, uyeCoz, uyeSozlukBoyutu, uyeTokenKimligi)

func (n *Nesne) GetProperty(kd kod.Kod, prop string) (yurut.Value, error) {
	switch kd {
	case uyeKodla:
		return n.metot(kod.Ad(uyeKodla), n.kodla), nil
	case uyeCoz:
		return n.metot(kod.Ad(uyeCoz), n.coz), nil
	case uyeSozlukBoyutu:
		return yurut.NewInt(int64(n.p.SozlukBoyutu())), nil
	case uyeTokenKimligi:
		return n.metot(kod.Ad(uyeTokenKimligi), n.tokenKimligi), nil
	}
	oneri := ""
	if o := turkce.EnYakin(prop, uyeler); o != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	} else {
		oneri = fmt.Sprintf(" (üyeler: %s)", strings.Join(uyeler, ", "))
	}
	return nil, fmt.Errorf("Parçalayıcı üzerinde %q özelliği ya da metodu bulunamadı%s", prop, oneri)
}

func (n *Nesne) metot(ad string, fn yurut.BuiltinFunc) *yurut.BuiltinVal {
	return &yurut.BuiltinVal{Name: ad, Fn: func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
		return baglama.Guvenli(ad, func() (yurut.Value, error) { return fn(m, args, named) })
	}}
}

// ----------------------------------------------------------------------------
// Kayıt ve yükleme
// ----------------------------------------------------------------------------

// Kaydet parçalayıcı yerleşiklerini VM'ye kaydeder.
func Kaydet(v *yurut.VM) {
	v.RegisterBuiltinKod(kod.V200, func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
		return baglama.Guvenli("parçalayıcı_yükle", func() (yurut.Value, error) { return yukle(m, args, named) })
	})
}

// uyarilanlar aynı dosyanın yükleme uyarılarının süreçte bir kez yazılmasını sağlar.
var uyarilanlar sync.Map

func yukle(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	const fn = "parçalayıcı_yükle"
	if err := baglama.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(yol) 1 parametre bekler, %d verildi", fn, len(args))
	}
	s, ok := args[0].(*yurut.StringVal)
	if !ok {
		return nil, fmt.Errorf("%s: yol bir metin olmalıdır (ör. \"tokenizer.json\"), %s verildi", fn, args[0].Type())
	}
	yol, bakilanlar := baglama.YolCozBakilan(m, s.Val)
	var p *parca.Parcalayici
	var err error
	var bilgi os.FileInfo
	m.Blocking(func() {
		bilgi, err = os.Stat(yol)
		if err == nil && !bilgi.IsDir() {
			p, err = parca.Yukle(yol)
		}
	})
	if err != nil {
		return nil, yuklemeHatasi(s.Val, err, bakilanlar)
	}
	if bilgi.IsDir() {
		aday := filepath.Join(yol, "tokenizer.json")
		ek := ""
		if _, e := os.Stat(aday); e == nil {
			ek = fmt.Sprintf("; bu klasörde tokenizer.json var: parçalayıcı_yükle(%q) deneyin", filepath.ToSlash(filepath.Join(s.Val, "tokenizer.json")))
		}
		return nil, fmt.Errorf("%s: %q bir klasör; tokenizer.json biçimindeki parçalayıcı dosyasının yolunu verin%s", fn, s.Val, ek)
	}
	uyarilariYaz(yol, p)
	return &Nesne{p: p, yol: yol}, nil
}

func yuklemeHatasi(yol string, err error, bakilanlar []string) error {
	const fn = "parçalayıcı_yükle"
	if baglama.YokMu(err) {
		return turkce.DosyaHatasi(fn, yol, "yüklenemedi", err,
			"tokenizer.json biçimindeki parçalayıcı dosyasının yolunu verin", yurut.BakilanYerler(bakilanlar))
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "desteklenmeyen"):
		msg = strings.Replace(msg, "desteklenmeyen", "parçalayıcı türü desteklenmiyor:", 1)
		return fmt.Errorf("%s: %q yüklenemedi, %s", fn, yol, msg)
	case strings.Contains(msg, "reddedildi"):
		msg = strings.Replace(msg, " (izin için DuzIfadeAyarlari.RiskliDesenIzni)", "", 1)
		return fmt.Errorf("%s: %q yüklenemedi: %s; bu desen kötü niyetli bir metinde üstel süre harcayabileceği için güvenlik gereği kabul edilmiyor (desenin iç içe sınırsız niceleyici içermeyen bir biçimini kullanın)", fn, yol, msg)
	case strings.Contains(msg, "JSON"):
		ayrinti := "dosya JSON olarak okunamadı"
		var se *json.SyntaxError
		var te *json.UnmarshalTypeError
		if errors.As(err, &se) {
			ayrinti = fmt.Sprintf("%d. baytta JSON sözdizimi hatası", se.Offset)
		} else if errors.As(err, &te) {
			ayrinti = fmt.Sprintf("JSON yapısı beklenen biçimde değil (%q alanı)", te.Field)
		}
		return fmt.Errorf("%s: %q geçerli bir tokenizer.json dosyası değil: %s", fn, yol, ayrinti)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		// İzin, kilitli dosya, disk hatası...: işletim sisteminin metni yerine Türkçe neden
		return turkce.DosyaHatasi(fn, yol, "yüklenemedi", err)
	}
	return fmt.Errorf("%s: %q yüklenemedi: %s", fn, yol, msg)
}

func uyarilariYaz(yol string, p *parca.Parcalayici) {
	u := p.Uyarilar()
	if len(u) == 0 {
		return
	}
	anahtar := yol
	if a, err := filepath.Abs(yol); err == nil {
		anahtar = a
	}
	if _, yazildi := uyarilanlar.LoadOrStore(anahtar, true); yazildi {
		return
	}
	fmt.Fprintf(os.Stderr, "Uyarı: parçalayıcı %q düzenli ifadelerinde riskli desen var (bazı metinlerde yavaşlayabilir; adım sınırı aşılırsa kodlama hata verir):\n", filepath.Base(yol))
	for _, x := range u {
		fmt.Fprintf(os.Stderr, "  - %s\n", x)
	}
}

// ----------------------------------------------------------------------------
// Kodlama
// ----------------------------------------------------------------------------

var (
	adOzel   = baglama.KodAd(kod.V910)
	adIkinci = baglama.KodAd(kod.V911)
)

// uzunMetin bu bayttan uzun tek metinler yorumlayıcı kilidi bırakılarak kodlanır.
const uzunMetin = 1 << 16

func adimHatasi(fn string, err error) error {
	var ah *parca.AdimSiniriHatasi
	if errors.As(err, &ah) && ah.Bellek {
		return fmt.Errorf("%s: parçalayıcının düzenli ifadesi %q bu metinde geri izleme belleği sınırını aştı (aramanın başladığı bayt: %d); desen çok uzun bir yinelemeyi geri izlenebilir tutuyor ve işlem güvenlik için durduruldu. Metni daha küçük parçalara bölmeyi ya da olağan dışı uzun tekrarları temizlemeyi deneyin",
			fn, ah.Desen, ah.Konum)
	}
	if ah != nil {
		return fmt.Errorf("%s: parçalayıcının düzenli ifadesi %q bu metinde adım sınırını (%d adım) aştı (aramanın başladığı bayt: %d); desen bu girdide aşırı geri izleme yapıyor ve işlem güvenlik için durduruldu. Metni daha küçük parçalara bölmeyi ya da olağan dışı uzun tekrarları (ör. binlerce aynı karakter) temizlemeyi deneyin",
			fn, ah.Desen, ah.Butce, ah.Konum)
	}
	return fmt.Errorf("%s: %v", fn, err)
}

func (n *Nesne) kodla(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	const fn = "kodla"
	if err := baglama.AdliDenetle(fn+"()", named, adOzel, adIkinci); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("kodla(metin) 1 parametre bekler (metin ya da metin listesi), %d verildi", len(args))
	}
	var sec parca.Secenekler
	if v, ok := baglama.AdliAl(named, adOzel); ok {
		b, err := baglama.Mantik(fn, kod.Ad(kod.V910), v)
		if err != nil {
			return nil, err
		}
		sec.OzelEklenmesin = !b
	}
	ikinciV, ikinciVar := baglama.AdliAl(named, adIkinci)

	switch x := args[0].(type) {
	case *yurut.StringVal:
		if ikinciVar {
			ik, ok := ikinciV.(*yurut.StringVal)
			if !ok {
				return nil, fmt.Errorf("kodla: tek metinle ikinci: de bir metin olmalıdır, %s verildi", ikinciV.Type())
			}
			var k *parca.Kodlama
			var err error
			n.belkiBirak(m, len(x.Val)+len(ik.Val), func() { k, err = n.p.GuvenliKodlaCift(x.Val, &ik.Val, sec) })
			if err != nil {
				return nil, adimHatasi(fn, err)
			}
			return kimlikListesi32(k.Idler), nil
		}
		var ids []int
		var err error
		n.belkiBirak(m, len(x.Val), func() { ids, err = n.p.GuvenliKodla(x.Val, sec) })
		if err != nil {
			return nil, adimHatasi(fn, err)
		}
		return kimlikListesi(ids), nil
	case *yurut.ListVal:
		metinler, err := metinListesi(fn, "metin listesi", x)
		if err != nil {
			return nil, err
		}
		var ikinciler []string
		if ikinciVar {
			il, ok := ikinciV.(*yurut.ListVal)
			if !ok {
				return nil, fmt.Errorf("kodla: metin listesiyle ikinci: de aynı uzunlukta bir metin listesi olmalıdır, %s verildi", ikinciV.Type())
			}
			if ikinciler, err = metinListesi(fn, "ikinci", il); err != nil {
				return nil, err
			}
			if len(ikinciler) != len(metinler) {
				return nil, fmt.Errorf("kodla: ikinci: listesi %d metin içeriyor, birinci liste %d; çiftler eşit uzunlukta olmalı", len(ikinciler), len(metinler))
			}
		}
		var sonuc [][]int
		var hatalar []error
		m.Blocking(func() {
			if ikinciler == nil {
				sonuc, hatalar = n.p.GuvenliTopluKodla(metinler, sec, 0)
				return
			}
			sonuc = make([][]int, len(metinler))
			hatalar = make([]error, len(metinler))
			paralel(len(metinler), func(i int) {
				k, err := n.p.GuvenliKodlaCift(metinler[i], &ikinciler[i], sec)
				hatalar[i] = err
				if err == nil {
					sonuc[i] = int32ToInt(k.Idler)
				}
			})
		})
		for i, h := range hatalar {
			if h != nil {
				return nil, adimHatasi(fmt.Sprintf("kodla (listenin %d. metni)", i+1), h)
			}
		}
		el := make([]yurut.Value, len(sonuc))
		for i, ids := range sonuc {
			el[i] = kimlikListesi(ids)
		}
		return &yurut.ListVal{Elements: el}, nil
	}
	return nil, fmt.Errorf("kodla: bir metin ya da metin listesi bekler, %s verildi", args[0].Type())
}

// belkiBirak uzun metinlerde işi yorumlayıcı kilidini bırakarak yapar.
func (n *Nesne) belkiBirak(m *yurut.VM, boyut int, f func()) {
	if boyut >= uzunMetin {
		m.Blocking(f)
		return
	}
	f()
}

func metinListesi(fn, ne string, l *yurut.ListVal) ([]string, error) {
	out := make([]string, len(l.Elements))
	for i, e := range l.Elements {
		s, ok := e.(*yurut.StringVal)
		if !ok {
			return nil, fmt.Errorf("%s: %s yalnızca metin içermeli; %d. eleman %s", fn, ne, i+1, e.Type())
		}
		out[i] = s.Val
	}
	return out, nil
}

// paralel f'yi 0..n-1 için işlemci sayısı kadar gorutinde çalıştırır. Bir gorutindeki
// panik süreci çökertmez: iş bitince çağıranın gorutininde yeniden fırlatılır (ve
// oradaki recover ile TR hatasına çevrilir).
func paralel(n int, f func(i int)) {
	isci := min(runtime.GOMAXPROCS(0), n)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var panik any
	sira := 0
	defer func() {
		if panik != nil {
			panic(panik)
		}
	}()
	for w := 0; w < isci; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					if panik == nil {
						panik = r
					}
					sira = n
					mu.Unlock()
				}
			}()
			for {
				mu.Lock()
				i := sira
				sira++
				mu.Unlock()
				if i >= n {
					return
				}
				f(i)
			}
		}()
	}
	wg.Wait()
}

func int32ToInt(ids []int32) []int {
	out := make([]int, len(ids))
	for i, x := range ids {
		out[i] = int(x)
	}
	return out
}

func kimlikListesi(ids []int) *yurut.ListVal {
	el := make([]yurut.Value, len(ids))
	for i, id := range ids {
		el[i] = yurut.NewInt(int64(id))
	}
	return &yurut.ListVal{Elements: el}
}

func kimlikListesi32(ids []int32) *yurut.ListVal {
	el := make([]yurut.Value, len(ids))
	for i, id := range ids {
		el[i] = yurut.NewInt(int64(id))
	}
	return &yurut.ListVal{Elements: el}
}

// ----------------------------------------------------------------------------
// Çözme
// ----------------------------------------------------------------------------

func (n *Nesne) coz(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	const fn = "çöz"
	if err := baglama.AdliDenetle(fn+"()", named, adOzel); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("çöz(kimlikler) 1 parametre bekler (kimlik listesi, listelerin listesi ya da tensör), %d verildi", len(args))
	}
	// Varsayılan, başvuru kütüphanesininki gibi özel parçaları ATLAMAKTIR;
	// özel_parçalar: doğru onları da çıktıya yazar.
	atla := true
	if v, ok := baglama.AdliAl(named, adOzel); ok {
		b, err := baglama.Mantik(fn, kod.Ad(kod.V910), v)
		if err != nil {
			return nil, err
		}
		atla = !b
	}
	diziler, tek, err := kimlikDizileri(args[0])
	if err != nil {
		return nil, err
	}
	sonuc := make([]string, len(diziler))
	hatalar := make([]error, len(diziler))
	is := func() {
		if len(diziler) == 1 {
			sonuc[0], hatalar[0] = n.p.GuvenliCoz(diziler[0], atla)
			return
		}
		paralel(len(diziler), func(i int) { sonuc[i], hatalar[i] = n.p.GuvenliCoz(diziler[i], atla) })
	}
	toplam := 0
	for _, d := range diziler {
		toplam += len(d)
	}
	n.belkiBirak(m, toplam*4, is)
	for i, h := range hatalar {
		if h != nil {
			if tek {
				return nil, adimHatasi(fn, h)
			}
			return nil, adimHatasi(fmt.Sprintf("çöz (%d. dizi)", i+1), h)
		}
	}
	if tek {
		return &yurut.StringVal{Val: sonuc[0]}, nil
	}
	el := make([]yurut.Value, len(sonuc))
	for i, s := range sonuc {
		el[i] = &yurut.StringVal{Val: s}
	}
	return &yurut.ListVal{Elements: el}, nil
}

// kimlikDizileri çöz'ün argümanını kimlik dizilerine çevirir. tek doğruysa argüman tek
// bir dizidir (sonuç metin), değilse dizilerin listesidir (sonuç metin listesi).
func kimlikDizileri(v yurut.Value) (diziler [][]int, tek bool, err error) {
	const fn = "çöz"
	switch x := v.(type) {
	case *yurut.ListVal:
		if len(x.Elements) == 0 {
			return [][]int{nil}, true, nil
		}
		if _, ic := x.Elements[0].(*yurut.ListVal); ic {
			diziler = make([][]int, len(x.Elements))
			for i, e := range x.Elements {
				l, ok := e.(*yurut.ListVal)
				if !ok {
					return nil, false, fmt.Errorf("%s: listelerin listesinde %d. eleman liste değil (%s)", fn, i+1, e.Type())
				}
				if diziler[i], err = tamListe(fmt.Sprintf("%d. listenin", i+1), l.Elements); err != nil {
					return nil, false, err
				}
			}
			return diziler, false, nil
		}
		d, err := tamListe("listenin", x.Elements)
		if err != nil {
			return nil, false, err
		}
		return [][]int{d}, true, nil
	case *ogrenme.Deger:
		t := x.Tensor()
		sh := t.Shape()
		veri := t.Data()
		if len(sh) > 2 {
			return nil, false, fmt.Errorf("%s: tensör 1 boyutlu (tek dizi) ya da 2 boyutlu (dizi listesi) olmalıdır, %d boyutlu verildi", fn, len(sh))
		}
		ids := make([]int, len(veri))
		for i, f := range veri {
			if float32(int(f)) != f {
				return nil, false, fmt.Errorf("%s: tensördeki kimlikler tam sayı olmalıdır, %v bulundu", fn, f)
			}
			ids[i] = int(f)
		}
		if len(sh) < 2 {
			return [][]int{ids}, true, nil
		}
		diziler = make([][]int, sh[0])
		for i := range diziler {
			diziler[i] = ids[i*sh[1] : (i+1)*sh[1]]
		}
		return diziler, false, nil
	}
	return nil, false, fmt.Errorf("%s: kimlik listesi, listelerin listesi ya da tensör bekler, %s verildi", fn, v.Type())
}

func tamListe(ne string, el []yurut.Value) ([]int, error) {
	out := make([]int, len(el))
	for i, e := range el {
		n, ok := baglama.TamSayi(e)
		if !ok {
			return nil, fmt.Errorf("çöz: kimlikler tam sayı olmalıdır; %s %d. elemanı %s", ne, i+1, e.Inspect())
		}
		out[i] = int(n)
	}
	return out, nil
}

// ----------------------------------------------------------------------------
// Sözlük sorguları
// ----------------------------------------------------------------------------

func (n *Nesne) tokenKimligi(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	if err := baglama.AdliDenetle(kod.Ad(kod.Y613)+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("parça_kimliği(parça) 1 parametre bekler, %d verildi", len(args))
	}
	s, ok := args[0].(*yurut.StringVal)
	if !ok {
		return nil, fmt.Errorf("parça_kimliği: parça bir metin olmalıdır (ör. \"<|endoftext|>\"), %s verildi", args[0].Type())
	}
	id, ok := n.p.TokenKimligi(s.Val)
	if !ok {
		return yurut.NULL, nil
	}
	return yurut.NewInt(int64(id)), nil
}
