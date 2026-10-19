// Package tokenizer, genel tokenizer.json parçalayıcısını (github.com/sekerahmet/TCB/internal/compute/tokenize)
// TR diline bağlar: parçalayıcı_yükle(yol) ve parçalayıcı nesnesinin .kodla, .çöz,
// .dağarcık_boyutu ve .parça_kimliği üyeleri.
//
// Kodlama/çözme hata döndüren Guvenli... işlevleriyle yapılır: düzenli ifade adım
// sınırı aşılırsa yarım sonuç yerine yol gösterici bir TR hatası oluşur. Uzun işler
// (dosya yükleme, liste kodlama/çözme) yorumlayıcı kilidi bırakılarak yapılır.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package tokenizer

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/compute/tokenize"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// TipAdi parçalayıcı nesnesinin TR'deki tip adıdır (kod listesinde A234).
var TipAdi = string(interp.TypeName(codes.A234))

// Nesne TR'de yüklenmiş bir parçalayıcıdır. Salt okunurdur; dosya tutamağı ya da
// başka bir kaynak tutmaz (dosya yüklemede tümüyle okunup kapatılır).
type Nesne struct {
	p   *tokenize.Tokenizer
	yol string
}

// Parcalayici sarılı Go parçalayıcısını verir.
func (n *Nesne) Parcalayici() *tokenize.Tokenizer { return n.p }

func (n *Nesne) Type() interp.ValueType { return interp.ValueType(TipAdi) }
func (n *Nesne) IsTruthy() bool         { return true }

func (n *Nesne) Inspect() string {
	return fmt.Sprintf("%s(dağarcık %d, %s)", TipAdi, n.p.VocabSize(), filepath.Base(n.yol))
}

// IzlemeTuru İzleme Masası'nda ikon seçimi için türdür.
func (n *Nesne) IzlemeTuru() string { return "diger" }

// IzlemeEtiketi tip sütununda gösterilen özettir.
func (n *Nesne) IzlemeEtiketi() string {
	return fmt.Sprintf("%s (dağarcık %d)", TipAdi, n.p.VocabSize())
}

// TabloVerisi dağarcığa eklenmiş parçaları (özel parçalar dahil) tablo olarak verir.
func (n *Nesne) TabloVerisi() ([]string, [][]string, error) {
	ekler := n.p.AddedTokens()
	if len(ekler) == 0 {
		return nil, nil, fmt.Errorf("bu parçalayıcıda eklenmiş (özel) parça yok; dağarcık boyutu %d", n.p.VocabSize())
	}
	var satirlar [][]string
	for _, e := range ekler {
		ozel := "hayır"
		if e.Special {
			ozel = "evet"
		}
		satirlar = append(satirlar, []string{fmt.Sprint(e.ID), e.Content, ozel})
	}
	return []string{"Kimlik", "Parça", "Özel"}, satirlar, nil
}

// Parçalayıcı üyeleri (adları kod listesinde, internal/lang/codes Y61x).
const (
	uyeKodla        = codes.Y610
	uyeCoz          = codes.Y611
	uyeSozlukBoyutu = codes.Y612
	uyeTokenKimligi = codes.Y613
)

var uyeler = codes.Names(uyeKodla, uyeCoz, uyeSozlukBoyutu, uyeTokenKimligi)

func (n *Nesne) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case uyeKodla:
		return n.metot(codes.Name(uyeKodla), n.kodla), nil
	case uyeCoz:
		return n.metot(codes.Name(uyeCoz), n.coz), nil
	case uyeSozlukBoyutu:
		return interp.NewInt(int64(n.p.VocabSize())), nil
	case uyeTokenKimligi:
		return n.metot(codes.Name(uyeTokenKimligi), n.tokenKimligi), nil
	}
	return nil, bind.UyeHatasi(TipAdi, prop, uyeler)
}

func (n *Nesne) metot(ad string, fn interp.BuiltinFunc) *interp.BuiltinVal {
	return &interp.BuiltinVal{Name: ad, Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(ad, func() (interp.Value, error) { return fn(m, args, named) })
	}}
}

// ----------------------------------------------------------------------------
// Kayıt ve yükleme
// ----------------------------------------------------------------------------

// Kaydet parçalayıcı yerleşiklerini VM'ye kaydeder.
func Kaydet(v *interp.VM) {
	v.RegisterBuiltinCode(codes.V200, func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(codes.Name(codes.V200), func() (interp.Value, error) { return yukle(m, args, named) })
	})
}

// uyarilanlar aynı dosyanın yükleme uyarılarının süreçte bir kez yazılmasını sağlar.
var uyarilanlar sync.Map

func yukle(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.V200)
	if err := bind.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(yol) 1 parametre bekler, %d verildi", fn, len(args))
	}
	ham, ok, yolHata := interp.PathArg(fn, args[0])
	if !ok {
		return nil, fmt.Errorf("%s: yol bir metin olmalıdır (ör. \"tokenizer.json\"), %s verildi", fn, args[0].Type())
	}
	if yolHata != nil {
		return nil, yolHata
	}
	yol, bakilanlar := bind.YolCozBakilan(m, ham)
	var p *tokenize.Tokenizer
	var err error
	var bilgi os.FileInfo
	m.Blocking(func() {
		bilgi, err = os.Stat(yol)
		if err == nil && !bilgi.IsDir() {
			p, err = tokenize.Load(yol)
		}
	})
	if err != nil {
		return nil, yuklemeHatasi(ham, err, bakilanlar)
	}
	if bilgi.IsDir() {
		aday := filepath.Join(yol, "tokenizer.json")
		ek := ""
		if _, e := os.Stat(aday); e == nil {
			ek = fmt.Sprintf("; bu klasörde tokenizer.json var: %s(%q) deneyin", fn, filepath.ToSlash(filepath.Join(ham, "tokenizer.json")))
		}
		return nil, fmt.Errorf("%s: %q bir klasör; tokenizer.json biçimindeki parçalayıcı dosyasının yolunu verin%s", fn, ham, ek)
	}
	uyarilariYaz(yol, p)
	return &Nesne{p: p, yol: yol}, nil
}

func yuklemeHatasi(yol string, err error, bakilanlar []string) error {
	fn := codes.Name(codes.V200)
	if bind.YokMu(err) {
		return turkish.FileError(fn, yol, "yüklenemedi", err,
			"tokenizer.json biçimindeki parçalayıcı dosyasının yolunu verin", interp.SearchedPlaces(bakilanlar))
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
		return turkish.FileError(fn, yol, "yüklenemedi", err)
	}
	return fmt.Errorf("%s: %q yüklenemedi: %s", fn, yol, msg)
}

func uyarilariYaz(yol string, p *tokenize.Tokenizer) {
	u := p.Warnings()
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
	adOzel   = bind.KodAd(codes.V910)
	adIkinci = bind.KodAd(codes.V911)
)

// uzunMetin bu bayttan uzun tek metinler yorumlayıcı kilidi bırakılarak kodlanır.
const uzunMetin = 1 << 16

func adimHatasi(fn string, err error) error {
	var ah *tokenize.StepLimitError
	if errors.As(err, &ah) && ah.Memory {
		return fmt.Errorf("%s: parçalayıcının düzenli ifadesi %q bu metinde geri izleme belleği sınırını aştı (aramanın başladığı bayt: %d); desen çok uzun bir yinelemeyi geri izlenebilir tutuyor ve işlem güvenlik için durduruldu. Metni daha küçük parçalara bölmeyi ya da olağan dışı uzun tekrarları temizlemeyi deneyin",
			fn, ah.Pattern, ah.Pos)
	}
	if ah != nil {
		return fmt.Errorf("%s: parçalayıcının düzenli ifadesi %q bu metinde adım sınırını (%d adım) aştı (aramanın başladığı bayt: %d); desen bu girdide aşırı geri izleme yapıyor ve işlem güvenlik için durduruldu. Metni daha küçük parçalara bölmeyi ya da olağan dışı uzun tekrarları (ör. binlerce aynı karakter) temizlemeyi deneyin",
			fn, ah.Pattern, ah.Budget, ah.Pos)
	}
	return fmt.Errorf("%s: %v", fn, err)
}

func (n *Nesne) kodla(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(uyeKodla)
	if err := bind.AdliDenetle(fn+"()", named, adOzel, adIkinci); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(metin) 1 parametre bekler (metin ya da metin listesi), %d verildi", fn, len(args))
	}
	var sec tokenize.Options
	if v, ok := bind.AdliAl(named, adOzel); ok {
		b, err := bind.Mantik(fn, codes.Name(codes.V910), v)
		if err != nil {
			return nil, err
		}
		sec.DontAddSpecial = !b
	}
	ikinciV, ikinciVar := bind.AdliAl(named, adIkinci)

	switch x := args[0].(type) {
	case *interp.StringVal:
		if ikinciVar {
			ik, ok := ikinciV.(*interp.StringVal)
			if !ok {
				return nil, fmt.Errorf("%s: tek metinle %s: de bir metin olmalıdır, %s verildi", fn, adIkinci.TR, ikinciV.Type())
			}
			var k *tokenize.Encoding
			var err error
			n.belkiBirak(m, len(x.Val)+len(ik.Val), func() { k, err = n.p.SafeEncodePair(x.Val, &ik.Val, sec) })
			if err != nil {
				return nil, adimHatasi(fn, err)
			}
			return kimlikListesi32(k.IDs), nil
		}
		var ids []int
		var err error
		n.belkiBirak(m, len(x.Val), func() { ids, err = n.p.SafeEncode(x.Val, sec) })
		if err != nil {
			return nil, adimHatasi(fn, err)
		}
		return kimlikListesi(ids), nil
	case *interp.ListVal:
		metinler, err := metinListesi(fn, "metin listesi", x)
		if err != nil {
			return nil, err
		}
		var ikinciler []string
		if ikinciVar {
			il, ok := ikinciV.(*interp.ListVal)
			if !ok {
				return nil, fmt.Errorf("%s: metin listesiyle %s: de aynı uzunlukta bir metin listesi olmalıdır, %s verildi", fn, adIkinci.TR, ikinciV.Type())
			}
			if ikinciler, err = metinListesi(fn, adIkinci.TR, il); err != nil {
				return nil, err
			}
			if len(ikinciler) != len(metinler) {
				return nil, fmt.Errorf("%s: %s: listesi %d metin içeriyor, birinci liste %d; çiftler eşit uzunlukta olmalı", fn, adIkinci.TR, len(ikinciler), len(metinler))
			}
		}
		var sonuc [][]int
		var hatalar []error
		m.Blocking(func() {
			if ikinciler == nil {
				sonuc, hatalar = n.p.SafeEncodeBatch(metinler, sec, 0)
				return
			}
			sonuc = make([][]int, len(metinler))
			hatalar = make([]error, len(metinler))
			paralel(len(metinler), func(i int) {
				k, err := n.p.SafeEncodePair(metinler[i], &ikinciler[i], sec)
				hatalar[i] = err
				if err == nil {
					sonuc[i] = int32ToInt(k.IDs)
				}
			})
		})
		for i, h := range hatalar {
			if h != nil {
				return nil, adimHatasi(fmt.Sprintf("kodla (listenin %d. metni)", i+1), h)
			}
		}
		el := make([]interp.Value, len(sonuc))
		for i, ids := range sonuc {
			el[i] = kimlikListesi(ids)
		}
		return &interp.ListVal{Elements: el}, nil
	}
	return nil, fmt.Errorf("kodla: bir metin ya da metin listesi bekler, %s verildi", args[0].Type())
}

// belkiBirak uzun metinlerde işi yorumlayıcı kilidini bırakarak yapar.
func (n *Nesne) belkiBirak(m *interp.VM, boyut int, f func()) {
	if boyut >= uzunMetin {
		m.Blocking(f)
		return
	}
	f()
}

func metinListesi(fn, ne string, l *interp.ListVal) ([]string, error) {
	out := make([]string, len(l.Elements))
	for i, e := range l.Elements {
		s, ok := e.(*interp.StringVal)
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

func kimlikListesi(ids []int) *interp.ListVal {
	el := make([]interp.Value, len(ids))
	for i, id := range ids {
		el[i] = interp.NewInt(int64(id))
	}
	return &interp.ListVal{Elements: el}
}

func kimlikListesi32(ids []int32) *interp.ListVal {
	el := make([]interp.Value, len(ids))
	for i, id := range ids {
		el[i] = interp.NewInt(int64(id))
	}
	return &interp.ListVal{Elements: el}
}

// ----------------------------------------------------------------------------
// Çözme
// ----------------------------------------------------------------------------

func (n *Nesne) coz(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(uyeCoz)
	if err := bind.AdliDenetle(fn+"()", named, adOzel); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(kimlikler) 1 parametre bekler (kimlik listesi, listelerin listesi ya da tensör), %d verildi", fn, len(args))
	}
	// Varsayılan, başvuru kütüphanesininki gibi özel parçaları ATLAMAKTIR;
	// özel_parçalar: doğru onları da çıktıya yazar.
	atla := true
	if v, ok := bind.AdliAl(named, adOzel); ok {
		b, err := bind.Mantik(fn, codes.Name(codes.V910), v)
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
			sonuc[0], hatalar[0] = n.p.SafeDecode(diziler[0], atla)
			return
		}
		paralel(len(diziler), func(i int) { sonuc[i], hatalar[i] = n.p.SafeDecode(diziler[i], atla) })
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
		return &interp.StringVal{Val: sonuc[0]}, nil
	}
	el := make([]interp.Value, len(sonuc))
	for i, s := range sonuc {
		el[i] = &interp.StringVal{Val: s}
	}
	return &interp.ListVal{Elements: el}, nil
}

// kimlikDizileri çöz'ün argümanını kimlik dizilerine çevirir. tek doğruysa argüman tek
// bir dizidir (sonuç metin), değilse dizilerin listesidir (sonuç metin listesi).
func kimlikDizileri(v interp.Value) (diziler [][]int, tek bool, err error) {
	fn := codes.Name(uyeCoz)
	switch x := v.(type) {
	case *interp.ListVal:
		if len(x.Elements) == 0 {
			return [][]int{nil}, true, nil
		}
		if _, ic := x.Elements[0].(*interp.ListVal); ic {
			diziler = make([][]int, len(x.Elements))
			for i, e := range x.Elements {
				l, ok := e.(*interp.ListVal)
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
	case *ml.Deger:
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

func tamListe(ne string, el []interp.Value) ([]int, error) {
	out := make([]int, len(el))
	for i, e := range el {
		n, ok := bind.TamSayi(e)
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

func (n *Nesne) tokenKimligi(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if err := bind.AdliDenetle(codes.Name(codes.Y613)+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parça) 1 parametre bekler, %d verildi", codes.Name(codes.Y613), len(args))
	}
	s, ok := args[0].(*interp.StringVal)
	if !ok {
		return nil, fmt.Errorf("%s: parça bir metin olmalıdır (ör. \"<|endoftext|>\"), %s verildi", codes.Name(codes.Y613), args[0].Type())
	}
	id, ok := n.p.TokenToID(s.Val)
	if !ok {
		return interp.NULL, nil
	}
	return interp.NewInt(int64(id)), nil
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*Nesne)(nil)
)
