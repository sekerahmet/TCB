package tensor

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"tr_dili/pkg/sozluk"

	"tr_dili/pkg/motor/internal/baglama"
	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

// Bu dosya hazır sinir ağı modelini (sinir_ağı, .eğit, .tahmin_et, .ağırlıklar, .kaydet,
// model_yükle) ve kayıp_mse'yi tensör çekirdeği üzerinde uygular. Model tam bağlantılı
// katmanlardan oluşur; eğitim, tensörün otomatik türeviyle hesaplanan gradyanlarla düz
// gradyan inişidir (SGD). Kaydedilen JSON model biçimi eski matris tabanlı modelinkiyle
// aynıdır; eski dosyalar yüklenebilir.

// SinirAgi sinir_ağı(...) ile kurulan ya da model_yükle(...) ile okunan modeldir.
type SinirAgi struct {
	boyutlar   []int
	aktivasyon string
	agirliklar []*ct.Tensor // l. katman: [giriş, çıkış], türev izler
	sapmalar   []*ct.Tensor // l. katman: [1, çıkış], türev izler
}

func (nn *SinirAgi) Type() vm.ValueType { return vm.ValueType("SinirAğı") }
func (nn *SinirAgi) IsTruthy() bool     { return true }
func (nn *SinirAgi) Inspect() string {
	var dims []string
	for _, s := range nn.boyutlar {
		dims = append(dims, strconv.Itoa(s))
	}
	return fmt.Sprintf("<SinirAğı mimari=[%s] aktivasyon=%q>", strings.Join(dims, " -> "), nn.aktivasyon)
}

// yeniSinirAgi ağırlıkları Xavier (Glorot) normal dağılımıyla, sapmaları sıfırla kurar.
// Rastgele sayılar tohumdan satır öncelikli sırayla üretilir (eski modelle aynı başlangıç).
func yeniSinirAgi(boyutlar []int, aktivasyon string, tohum int64) *SinirAgi {
	rng := rand.New(rand.NewSource(tohum))
	nn := &SinirAgi{boyutlar: boyutlar, aktivasyon: strings.ToLower(aktivasyon)}
	for l := 0; l+1 < len(boyutlar); l++ {
		giris, cikis := boyutlar[l], boyutlar[l+1]
		olcek := math.Sqrt(2.0 / float64(giris+cikis))
		w := make([]float32, giris*cikis)
		for i := range w {
			w[i] = float32(rng.NormFloat64() * olcek)
		}
		nn.agirliklar = append(nn.agirliklar, ct.New(w, giris, cikis).SetRequiresGrad(true))
		nn.sapmalar = append(nn.sapmalar, ct.Zeros(1, cikis).SetRequiresGrad(true))
	}
	return nn
}

// etkinlestir katman çıktısına aktivasyonu uygular. Gizli katmanlarda "relu" sızıntılı
// ReLU'dur (eğim 0.01; ölü nöronları önler), "sigmoid" sigmoid, diğer her ad tanh'tır. Çıkış
// katmanı aktivasyon "sigmoid" ise sigmoid, değilse doğrusaldır.
func (nn *SinirAgi) etkinlestir(z *ct.Tensor, cikis bool) *ct.Tensor {
	if cikis {
		if nn.aktivasyon == "sigmoid" {
			return z.Sigmoid()
		}
		return z
	}
	switch nn.aktivasyon {
	case "relu":
		// max(x, 0)·0.99 + 0.01·x: x > 0 için x, x ≤ 0 için 0.01·x.
		return ct.Add(z.Relu().MulScalar(0.99), z.MulScalar(0.01))
	case "sigmoid":
		return z.Sigmoid()
	}
	return z.Tanh()
}

// ileri girdiyi ([örnek, giriş]) tüm katmanlardan geçirir.
func (nn *SinirAgi) ileri(x *ct.Tensor) *ct.Tensor {
	son := len(nn.agirliklar) - 1
	for l, w := range nn.agirliklar {
		x = nn.etkinlestir(ct.Add(ct.MatMul(x, w), nn.sapmalar[l]), l == son)
	}
	return x
}

// egit modeli ortalama kare hata kaybıyla devir kez eğitir ve her devirdeki kaybı verir.
// Gradyan, toplam kare hatanın örnek sayısına bölümünden alınır (çok çıkışlı ağlarda
// çıkışlar toplanır); bildirilen kayıp ise tüm elemanların ortalamasıdır.
func (nn *SinirAgi) egit(x, y *ct.Tensor, devir int, oran float64) ([]float64, error) {
	xs, ys := x.Shape(), y.Shape()
	if xs[0] != ys[0] {
		return nil, fmt.Errorf("Eğitim girdi satır sayısı (%d) ile hedef satır sayısı (%d) eşit olmalıdır", xs[0], ys[0])
	}
	if devir <= 0 {
		return nil, fmt.Errorf("devir (epoch) sayısı pozitif olmalıdır (%d verildi)", devir)
	}
	if len(nn.agirliklar) == 0 {
		return nil, fmt.Errorf("sinir ağında eğitilecek katman yok")
	}
	if cikis := nn.agirliklar[len(nn.agirliklar)-1].Size(1); ys[1] != cikis {
		return nil, fmt.Errorf("Hedef sütun sayısı (%d) çıkış katmanı boyutuyla (%d) eşit olmalıdır", ys[1], cikis)
	}
	if giris := nn.agirliklar[0].Size(0); xs[1] != giris {
		return nil, fmt.Errorf("Girdi sütun sayısı (%d) giriş katmanı boyutuyla (%d) eşit olmalıdır", xs[1], giris)
	}
	params := append(append([]*ct.Tensor{}, nn.agirliklar...), nn.sapmalar...)
	kayiplar := make([]float64, 0, devir)
	_, err := guvenli(func() bool {
		opt := ct.NewSGD(params, float32(oran), 0)
		olcek := float32(1 / float64(xs[0]))
		ortalama := float64(ys[1])
		for ep := 0; ep < devir; ep++ {
			opt.ZeroGrad()
			fark := ct.Sub(nn.ileri(x), y)
			kayip := fark.Square().SumAll().MulScalar(olcek)
			kayiplar = append(kayiplar, kisaFloat64(float32(float64(kayip.Item())/ortalama)))
			kayip.Backward()
			opt.Step()
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return kayiplar, nil
}

// tahmin girdiyi türev izlemeden modelden geçirir.
func (nn *SinirAgi) tahmin(x *ct.Tensor) (*ct.Tensor, error) {
	if giris := nn.agirliklar[0].Size(0); x.Size(1) != giris {
		return nil, fmt.Errorf("Girdi sütun sayısı (%d) giriş katmanı boyutuyla (%d) eşit olmalıdır", x.Size(1), giris)
	}
	return guvenli(func() *ct.Tensor {
		var out *ct.Tensor
		ct.NoGrad(func() { out = nn.ileri(x) })
		return out
	})
}

// matrisVerisi 2 boyutlu tensörü JSON'a yazılacak iç içe float64 dilimlere çevirir; her
// float32 değer en kısa ondalık gösterimiyle yazılır.
func matrisVerisi(t *ct.Tensor) [][]float64 {
	sh, veri := t.Shape(), t.Data()
	out := make([][]float64, sh[0])
	for i := range out {
		out[i] = make([]float64, sh[1])
		for j := range out[i] {
			out[i][j] = kisaFloat64(veri[i*sh[1]+j])
		}
	}
	return out
}

// kaydet modeli JSON olarak yazar. Göreli yol TR'nin tek yol kuralıyla çözülür (bkz.
// vm.(*VM).YolCoz); dosya hataları Türkçe anlatılır.
func (nn *SinirAgi) kaydet(m *vm.VM, yol string) error {
	agirliklar := make([][][]float64, len(nn.agirliklar))
	sapmalar := make([][][]float64, len(nn.sapmalar))
	for i := range nn.agirliklar {
		agirliklar[i] = matrisVerisi(nn.agirliklar[i])
		sapmalar[i] = matrisVerisi(nn.sapmalar[i])
	}
	veri, err := json.MarshalIndent(map[string]interface{}{
		"katman_boyutları": nn.boyutlar,
		"aktivasyon":       nn.aktivasyon,
		"ağırlıklar":       agirliklar,
		"sapmalar":         sapmalar,
	}, "", "  ")
	if err != nil {
		return err
	}
	cozulen := baglama.YolCoz(m, yol)
	var yazHata error
	m.Blocking(func() { yazHata = os.WriteFile(cozulen, veri, 0644) })
	return turkce.DosyaHatasi("model.kaydet", yol, "kaydedilemedi", yazHata)
}

// SinirAğı üyeleri (adları sözlükte, pkg/sozluk Y51x, Y005).
const (
	uyeEgit       = sozluk.Y + 510
	uyeTahminEt   = sozluk.Y + 511 // eş yazımı: tahmin
	uyeAgirliklar = sozluk.Y + 512
	uyeKaydet     = sozluk.Y + 5
)

func (nn *SinirAgi) GetProperty(kod sozluk.Kod, prop string) (vm.Value, error) {
	switch kod {
	case uyeEgit:
		return &vm.BuiltinVal{Name: "model." + sozluk.Ad(uyeEgit), Fn: nn.egitYerlesik}, nil

	case uyeTahminEt:
		return &vm.BuiltinVal{Name: "model." + sozluk.Ad(uyeTahminEt), Fn: func(_ *vm.VM, args []vm.Value, _ map[string]vm.Value) (vm.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("model.tahmin_et(Girdi) 1 parametre bekler")
			}
			x, err := matrisCevir(args[0])
			if err != nil {
				return nil, err
			}
			r, err := nn.tahmin(x)
			if err != nil {
				return nil, err
			}
			return Yeni(r), nil
		}}, nil

	case uyeAgirliklar:
		// Ağırlıklar canlı görünümlerdir (veriyi paylaşır) ama hesap çizgesine bağlanmaz.
		elems := make([]vm.Value, len(nn.agirliklar))
		for i, w := range nn.agirliklar {
			elems[i] = Yeni(w.Detach())
		}
		return &vm.ListVal{Elements: elems}, nil

	case uyeKaydet:
		return &vm.BuiltinVal{Name: "model." + sozluk.Ad(uyeKaydet), Fn: func(m *vm.VM, args []vm.Value, _ map[string]vm.Value) (vm.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("model.kaydet(dosya_yolu) 1 parametre bekler")
			}
			yol, err := yolArg("model.kaydet", args[0])
			if err != nil {
				return nil, err
			}
			if err := nn.kaydet(m, yol); err != nil {
				return nil, err
			}
			return vm.TRUE, nil
		}}, nil
	}
	return nil, fmt.Errorf("SinirAğı üzerinde %q özelliği veya metodu bulunamadı", prop)
}

// egitYerlesik model.eğit(Girdi, Hedef, [devir: 500], [öğrenme_hızı: 0.1]) çağrısıdır.
func (nn *SinirAgi) egitYerlesik(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("model.eğit(Girdi, Hedef, [devir: 500], [öğrenme_hızı: 0.1]) en az 2 parametre bekler")
	}
	x, err := matrisCevir(args[0])
	if err != nil {
		return nil, fmt.Errorf("Girdi matrise çevrilemedi: %v", err)
	}
	y, err := matrisCevir(args[1])
	if err != nil {
		return nil, fmt.Errorf("Hedef matrise çevrilemedi: %v", err)
	}
	devir, oran := 500, 0.1
	if v, ok := baglama.AdliAl(named, adDevir); ok {
		if iv, ok := v.(*vm.IntVal); ok {
			devir = int(iv.Val)
		}
	}
	if v, ok := baglama.AdliAl(named, adOgrenmeHizi); ok {
		if f, ok := vm.ToFloat(v); ok {
			oran = f
		}
	}
	if len(args) >= 3 {
		if iv, ok := args[2].(*vm.IntVal); ok {
			devir = int(iv.Val)
		}
	}
	if len(args) >= 4 {
		if f, ok := vm.ToFloat(args[3]); ok {
			oran = f
		}
	}
	gecmis, err := nn.egit(x, y, devir, oran)
	if err != nil {
		return nil, err
	}
	res := vm.NewDictVal()
	res.Set("ilk_kayıp", &vm.FloatVal{Val: gecmis[0]})
	res.Set("son_kayıp", &vm.FloatVal{Val: gecmis[len(gecmis)-1]})
	res.Set("devir", &vm.IntVal{Val: int64(devir)})
	return res, nil
}

// sinirAgiKur sinir_ağı(katman_boyutları, [aktivasyon: "tanh"], [tohum: 42]) yerleşiğidir.
func sinirAgiKur(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("sinir_ağı(katman_boyutları, [aktivasyon: 'tanh'], [tohum: 42]) en az 1 parametre bekler")
	}
	list, ok := args[0].(*vm.ListVal)
	if !ok || len(list.Elements) < 2 {
		return nil, fmt.Errorf("sinir_ağı() en az 2 katman boyutu içeren bir Liste bekler (Örn: [2, 8, 1])")
	}
	boyutlar := make([]int, len(list.Elements))
	carpim := 0
	for i, el := range list.Elements {
		iv, ok := el.(*vm.IntVal)
		if !ok || iv.Val <= 0 || iv.Val > vm.MaxCollectionSize {
			return nil, fmt.Errorf("Katman boyutları pozitif Tam sayı olmalıdır")
		}
		boyutlar[i] = int(iv.Val)
		if i > 0 {
			carpim += boyutlar[i-1] * boyutlar[i]
			if carpim > vm.MaxCollectionSize {
				return nil, fmt.Errorf("sinir_ağı: model çok büyük (en fazla %d ağırlık)", vm.MaxCollectionSize)
			}
		}
	}
	aktivasyon := "tanh"
	tohum := int64(42)
	if len(args) >= 2 {
		aktivasyon = args[1].Inspect()
	}
	if v, ok := baglama.AdliAl(named, adAktivasyon); ok {
		aktivasyon = v.Inspect()
	}
	if v, ok := baglama.AdliAl(named, adTohum); ok {
		if iv, ok := v.(*vm.IntVal); ok {
			tohum = iv.Val
		}
	}
	return yeniSinirAgi(boyutlar, aktivasyon, tohum), nil
}

// yuklenenMatris dosyadan okunan iç içe diziyi boş ya da düzensiz olmadığını denetleyerek
// türev izleyen bir parametre tensörüne çevirir.
func yuklenenMatris(veri [][]float64) (*ct.Tensor, error) {
	if len(veri) == 0 || len(veri[0]) == 0 {
		return nil, fmt.Errorf("boş matris")
	}
	duz := make([]float32, 0, len(veri)*len(veri[0]))
	for i, satir := range veri {
		if len(satir) != len(veri[0]) {
			return nil, fmt.Errorf("%d. satırın uzunluğu %d, %d bekleniyordu", i+1, len(satir), len(veri[0]))
		}
		for _, x := range satir {
			duz = append(duz, float32(x))
		}
	}
	return ct.New(duz, len(veri), len(veri[0])).SetRequiresGrad(true), nil
}

// modelYukle model_yükle(dosya_yolu) yerleşiğidir: model.kaydet ile yazılmış (ya da eski
// matris tabanlı modelin kaydettiği) JSON dosyasını okur.
func modelYukle(m *vm.VM, args []vm.Value, _ map[string]vm.Value) (vm.Value, error) {
	const fn = "model_yükle"
	if len(args) != 1 {
		return nil, fmt.Errorf("model_yükle(dosya_yolu) 1 parametre bekler")
	}
	yol, err := yolArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	// Göreli yol TR'nin tek yol kuralıyla çözülür (bkz. vm.(*VM).YolCoz).
	cozulen, bakilanlar := baglama.YolCozBakilan(m, yol)
	var veri []byte
	m.Blocking(func() { veri, err = os.ReadFile(cozulen) })
	if err != nil {
		return nil, vm.OkumaHatasi(fn, yol, "okunamadı", err, bakilanlar)
	}
	var kayitli struct {
		Boyutlar   []int         `json:"katman_boyutları"`
		Aktivasyon string        `json:"aktivasyon"`
		Agirliklar [][][]float64 `json:"ağırlıklar"`
		Sapmalar   [][][]float64 `json:"sapmalar"`
	}
	if err := json.Unmarshal(veri, &kayitli); err != nil {
		return nil, fmt.Errorf("Model dosyası geçersiz formatta: %v", err)
	}
	if len(kayitli.Agirliklar) == 0 || len(kayitli.Agirliklar) != len(kayitli.Sapmalar) {
		return nil, fmt.Errorf("Model dosyası bozuk (%s): %d ağırlık ve %d sapma katmanı var", yol, len(kayitli.Agirliklar), len(kayitli.Sapmalar))
	}
	nn := &SinirAgi{boyutlar: kayitli.Boyutlar, aktivasyon: kayitli.Aktivasyon}
	for i := range kayitli.Agirliklar {
		w, err := yuklenenMatris(kayitli.Agirliklar[i])
		if err != nil {
			return nil, fmt.Errorf("Model dosyası bozuk (%s): %d. katman ağırlıkları: %v", yol, i+1, err)
		}
		b, err := yuklenenMatris(kayitli.Sapmalar[i])
		if err != nil {
			return nil, fmt.Errorf("Model dosyası bozuk (%s): %d. katman sapmaları: %v", yol, i+1, err)
		}
		if i > 0 && nn.agirliklar[i-1].Size(1) != w.Size(0) {
			return nil, fmt.Errorf("Model dosyası bozuk (%s): %d. katman boyutları uyuşmuyor", yol, i+1)
		}
		if b.Size(0) != 1 || b.Size(1) != w.Size(1) {
			return nil, fmt.Errorf("Model dosyası bozuk (%s): %d. katman sapma boyutu %dx%d, 1x%d bekleniyordu", yol, i+1, b.Size(0), b.Size(1), w.Size(1))
		}
		nn.agirliklar = append(nn.agirliklar, w)
		nn.sapmalar = append(nn.sapmalar, b)
	}
	return nn, nil
}

// kayipMSE kayıp_mse(tahmin, gerçek) yerleşiğidir: iki matrisin (liste ya da tensör)
// elemanları arasındaki kare farkların ortalamasını Ondalık olarak verir.
func kayipMSE(_ *vm.VM, args []vm.Value, _ map[string]vm.Value) (vm.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("kayıp_mse(tahmin, gerçek) 2 parametre bekler")
	}
	a, err1 := matrisCevir(args[0])
	b, err2 := matrisCevir(args[1])
	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("kayıp_mse() Liste ya da Tensör bekler")
	}
	if sekilMetni(a.Shape()) != sekilMetni(b.Shape()) {
		return nil, fmt.Errorf("kayıp_mse boyut uyuşmazlığı: %s ile %s", sekilMetni(a.Shape()), sekilMetni(b.Shape()))
	}
	x, y := a.Float64s(), b.Float64s()
	toplam := 0.0
	for i := range x {
		d := x[i] - y[i]
		toplam += d * d
	}
	return &vm.FloatVal{Val: toplam / float64(len(x))}, nil
}
