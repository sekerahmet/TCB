package ml

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Bir modelin parametrelerini toplama (parametreler) ve var olan parametrelere yükleme
// (parametreleri_yükle). Kontrol noktası (internal/stdlib/checkpoint) aynı Go fonksiyonlarını
// kullanır: ParametreleriTopla, TensorleriTopla, ParametreUyumu, TensoreYaz.

// parametrelerYerlesik parametreler(model, adlarıyla: yanlış) yerleşiğidir: bir yapı
// nesnesini (iç içe yapılar, listeler ve sözlüklerle birlikte) gezip türev izleyen yaprak
// tensörleri sabit sırayla (yapı alanlarının tanım sırası, liste sırası, sözlük ekleme
// sırası) toplar. Aynı tensör bir kez yer alır. adlarıyla: doğru yol adı → tensör sözlüğü
// verir ("bloklar.0.dikkat.Wq"); bu sözlük tensör_kaydet ile kaydedilebilir.
func parametrelerYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(codes.T530)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(model, %s: %s) 1 konumlu parametre bekler, %d verildi", fn, adAdlariyla.TR, codes.Ad(codes.A041), len(args))
	}
	if err := bind.AdliDenetle(fn+"()", named, adAdlariyla); err != nil {
		return nil, err
	}
	adlariyla, err := adliMantik(fn, named, adAdlariyla, false)
	if err != nil {
		return nil, err
	}
	adlar, bulunan, err := ParametreleriTopla(fn, args[0])
	if err != nil {
		return nil, err
	}
	if !adlariyla {
		el := make([]interp.Value, len(bulunan))
		for i, d := range bulunan {
			el[i] = d
		}
		return &interp.ListVal{Elements: el}, nil
	}
	s := interp.NewDictVal()
	for i, ad := range adlar {
		s.Set(ad, bulunan[i])
	}
	return s, nil
}

// ParametreleriTopla bir modelin (yapı nesnesi; iç içe yapılar, listeler ve sözlüklerle; ya
// da tek bir tensör) türev izleyen yaprak tensörlerini parametreler(model, adlarıyla: doğru)
// ile aynı sırayla ve yol adlarıyla verir; aynı tensör bir kez yer alır, model tek bir
// tensörse adı "0"dır. fn hata mesajının başıdır. Model hiç parametre içermeyebilir.
func ParametreleriTopla(fn string, model interp.Value) (adlar []string, bulunan []*Deger, hata error) {
	return yapraklariTopla(fn, model, false)
}

// TensorleriTopla ParametreleriTopla gibidir, ama türev izlemeyen tensörleri de (durum:
// iyileştirici momentleri, sabit tablolar) aynı gezinti sırasıyla ve yol adlarıyla verir.
// Ara sonuçlar (türev izleyen işlem sonuçları) yine atlanır. Kontrol noktası nesnenin bütün
// tensörlerini bununla toplar.
func TensorleriTopla(fn string, model interp.Value) (adlar []string, bulunan []*Deger, hata error) {
	return yapraklariTopla(fn, model, true)
}

// yapraklariTopla modeli gezer; hepsi yanlışsa yalnızca türev izleyen yaprakları toplar.
func yapraklariTopla(fn string, model interp.Value, hepsi bool) (adlar []string, bulunan []*Deger, hata error) {
	gorulen := map[*tensor.Tensor]bool{}
	gezilen := map[any]bool{}
	adSahibi := map[string]*tensor.Tensor{}
	var gez func(v interp.Value, yol string)
	gez = func(v interp.Value, yol string) {
		if hata != nil {
			return
		}
		altYol := func(ad string) string {
			if yol == "" {
				return ad
			}
			return yol + "." + ad
		}
		switch x := v.(type) {
		case *Deger:
			if x.ara || (!hepsi && !x.t.RequiresGrad()) || gorulen[x.t] {
				return
			}
			if o, ok := adSahibi[yol]; ok && o != x.t {
				hata = fmt.Errorf("%s: iki farklı tensör aynı yol adına düşüyor (%q); sözlük anahtarlarında '.' kullanmayın", fn, yol)
				return
			}
			gorulen[x.t] = true
			adSahibi[yol] = x.t
			bulunan = append(bulunan, x)
			adlar = append(adlar, yol)
		case *interp.StructInstanceVal:
			if !gezilen[x] {
				gezilen[x] = true
				x.AlanlariGez(func(ad string, d interp.Value) { gez(d, altYol(ad)) })
			}
		case *interp.ListVal:
			if !gezilen[x] {
				gezilen[x] = true
				for i, el := range x.Elements {
					gez(el, altYol(strconv.Itoa(i)))
				}
			}
		case *interp.DictVal:
			if !gezilen[x] {
				gezilen[x] = true
				for _, k := range x.Order {
					gez(x.Pairs[k], altYol(x.DisplayKey(k)))
				}
			}
		}
	}
	gez(model, "")
	if hata != nil {
		return nil, nil, hata
	}
	for i, ad := range adlar {
		if ad == "" {
			adlar[i] = "0" // model tek bir tensörse
		}
	}
	return adlar, bulunan, nil
}

// ParametreUyumu modelin parametrelerini (ParametreleriTopla sonucu) yüklenecek kaynağın
// ad → şekil eşlemesiyle karşılaştırır: her parametre kaynakta aynı şekilde bulunmalı,
// kaynakta modelde olmayan bir ad olmamalıdır. kaynakAdi mesajda kaynağı anar ("sözlükte",
// "kayıtta"). İlk uyumsuzluk yol gösterici bir hatadır; hiçbir şey yazılmaz.
func ParametreUyumu(fn string, adlar []string, bulunan []*Deger, kaynak map[string][]int, kaynakAdi string) error {
	for i, ad := range adlar {
		sk, ok := kaynak[ad]
		if !ok {
			return fmt.Errorf("%s: modelin %q parametresi %s yok (model %d, %s %d parametre); model ile kayıt aynı yapıda olmalı",
				fn, ad, kaynakAdi, len(adlar), kaynakAdi, len(kaynak))
		}
		if sm := bulunan[i].t.Shape(); !slices.Equal(sm, sk) {
			return fmt.Errorf("%s: %q parametresinin şekli modelde %s, %s %s", fn, ad, sekilMetni(sm), kaynakAdi, sekilMetni(sk))
		}
	}
	if len(kaynak) == len(adlar) {
		return nil
	}
	var fazla []string
	for ad := range kaynak {
		if !slices.Contains(adlar, ad) {
			fazla = append(fazla, ad)
		}
	}
	slices.Sort(fazla)
	oneri := ""
	if o := turkish.EnYakin(fazla[0], adlar); o != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return fmt.Errorf("%s: %s %d fazla ad var, ilki %q modelde bir parametre değil%s", fn, kaynakAdi, len(fazla), fazla[0], oneri)
}

// TensoreYaz kaynağın değerlerini p'ye yerinde yazar: kaynak p'nin cihazına taşınır, türü
// farklıysa dönüştürülür (şekiller aynı olmalıdır). İşlem çizgeye kaydedilmez; p'nin sürüm
// sayacı artar (p'yi kullanmış eski bir çizgeden geri yayılım hata verir, 15.6). İyileştirici
// p'nin kendisini tuttuğu için güncellemeler yüklenen değerlerle sürer. Çekirdek panikleri
// (cihaz belleği) çağıranın kurtarıcısına gider.
func TensoreYaz(p, kaynak *tensor.Tensor) {
	if kaynak.RequiresGrad() {
		kaynak = kaynak.Detach()
	}
	p.CopyFrom(kaynak.To(p.Device()))
}

// parametreleriYukleYerlesik parametreleri_yükle(model, sözlük) yerleşiğidir: ad → tensör
// sözlüğünü (tensör_yükle sonucu) modelin var olan parametrelerine yerinde yazar ve yazılan
// parametre sayısını verir.
func parametreleriYukleYerlesik(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(codes.T703)
	if len(args) != 2 {
		return nil, interp.ArgumanSayisiHatasi(codes.T703, "2", len(args), "model", "{ad: tensör, ...}")
	}
	if err := bind.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	adlar, bulunan, err := ParametreleriTopla(fn, args[0])
	if err != nil {
		return nil, err
	}
	if len(adlar) == 0 {
		return nil, fmt.Errorf("%s: modelde türev izleyen parametre yok (%s); parametreler %s ile oluşturulmalı", fn, interp.TipVeOzet(args[0]), turevIzleyenOrnek("..."))
	}
	s, ok := args[1].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgumanTipHatasi(codes.T703, 2, args[1], codes.A206)
	}
	kaynak := make(map[string]*tensor.Tensor, len(s.Order))
	sekiller := make(map[string][]int, len(s.Order))
	for _, k := range s.Order {
		v, ok := s.Pairs[k]
		if !ok {
			continue
		}
		if _, metin := s.KeyValue(k).(*interp.StringVal); !metin {
			return nil, fmt.Errorf("%s: sözlük anahtarları parametre adları (%s) olmalıdır, %s verildi", fn, codes.Ad(codes.A202), s.DisplayKey(k))
		}
		d, ok := v.(*Deger)
		if !ok {
			return nil, fmt.Errorf("%s: %q değeri bir %s olmalıdır, %s verildi", fn, k, codes.Ad(codes.A212), tipAdi(v))
		}
		kaynak[k], sekiller[k] = d.t, d.t.Shape()
	}
	if err := ParametreUyumu(fn, adlar, bulunan, sekiller, "sözlükte"); err != nil {
		return nil, err
	}
	if _, err := guvenli(func() bool {
		for i, ad := range adlar {
			TensoreYaz(bulunan[i].t, kaynak[ad])
		}
		return true
	}); err != nil {
		return nil, fmt.Errorf("%s: %s", fn, strings.TrimPrefix(err.Error(), "tensör işlemi başarısız: "))
	}
	return interp.NewInt(int64(len(adlar))), nil
}
