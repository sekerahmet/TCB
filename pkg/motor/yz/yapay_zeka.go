package yz

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"tr_dili/pkg/kod"
	"tr_dili/pkg/motor/internal/baglama"
	"tr_dili/pkg/motor/web"
	"tr_dili/pkg/vm"
)

// yz_sor'un adlı argümanları; adları kod listesindedir (pkg/kod, T94x).
var (
	adModel = baglama.KodAd(kod.T940)
	adAdres = baglama.KodAd(kod.T941)
)

// yzSorZamanAsimi yz_sor isteğinin toplam süre sınırıdır (modelin yanıt üretmesi dahil).
var yzSorZamanAsimi = 120 * time.Second

// Kaydet istatistik, vektör benzerliği ve yz_sor yerleşiklerini VM'ye kaydeder. Tensör,
// matris, otomatik türev ve sinir ağı yerleşikleri pkg/motor/tensor, JSON/CSV yerleşikleri
// pkg/motor/veribicim, web_al/web_gönder ise pkg/motor/web paketindedir.
func Kaydet(v *vm.VM) {
	// --- Veri Bilimi, İstatistik ve Vektör Benzerlik ---
	v.RegisterBuiltinKod(kod.G410, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		nums := vm.ExtractNumericSlice(args)
		if len(nums) == 0 {
			return &vm.FloatVal{Val: 0}, nil
		}
		sum := 0.0
		for _, x := range nums {
			sum += x
		}
		return &vm.FloatVal{Val: sum / float64(len(nums))}, nil
	})

	v.RegisterBuiltinKod(kod.G411, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		nums := vm.ExtractNumericSlice(args)
		if len(nums) == 0 {
			return &vm.FloatVal{Val: 0}, nil
		}
		mean := 0.0
		for _, x := range nums {
			mean += x
		}
		mean /= float64(len(nums))
		variance := 0.0
		for _, x := range nums {
			d := x - mean
			variance += d * d
		}
		variance /= float64(len(nums))
		return &vm.FloatVal{Val: math.Sqrt(variance)}, nil
	})

	v.RegisterBuiltinKod(kod.G412, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		nums := vm.ExtractNumericSlice(args)
		if len(nums) == 0 {
			return &vm.FloatVal{Val: 0}, nil
		}
		sorted := make([]float64, len(nums))
		copy(sorted, nums)
		sort.Float64s(sorted)
		mid := len(sorted) / 2
		if len(sorted)%2 == 1 {
			return &vm.FloatVal{Val: sorted[mid]}, nil
		}
		return &vm.FloatVal{Val: (sorted[mid-1] + sorted[mid]) / 2.0}, nil
	})

	v.RegisterBuiltinKod(kod.T600, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		nums := vm.ExtractNumericSlice(args)
		if len(nums) == 0 {
			return &vm.ListVal{Elements: []vm.Value{}}, nil
		}
		minV, maxV := nums[0], nums[0]
		for _, x := range nums[1:] {
			if x < minV {
				minV = x
			}
			if x > maxV {
				maxV = x
			}
		}
		span := maxV - minV
		elems := make([]vm.Value, len(nums))
		for i, x := range nums {
			if span == 0 {
				elems[i] = &vm.FloatVal{Val: 0.0}
			} else {
				elems[i] = &vm.FloatVal{Val: (x - minV) / span}
			}
		}
		return &vm.ListVal{Elements: elems}, nil
	})

	v.RegisterBuiltinKod(kod.T601, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("nokta_çarpım(v1, v2) 2 parametre bekler")
		}
		v1 := vm.ExtractNumericSlice([]vm.Value{args[0]})
		v2 := vm.ExtractNumericSlice([]vm.Value{args[1]})
		if len(v1) != len(v2) || len(v1) == 0 {
			return nil, fmt.Errorf("nokta_çarpım() eşit uzunlukta iki sayısal liste bekler")
		}
		dot := 0.0
		for i := range v1 {
			dot += v1[i] * v2[i]
		}
		return &vm.FloatVal{Val: dot}, nil
	})

	v.RegisterBuiltinKod(kod.T602, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("kosinüs_benzerliği(v1, v2) 2 parametre bekler")
		}
		v1 := vm.ExtractNumericSlice([]vm.Value{args[0]})
		v2 := vm.ExtractNumericSlice([]vm.Value{args[1]})
		if len(v1) != len(v2) || len(v1) == 0 {
			return nil, fmt.Errorf("kosinüs_benzerliği() eşit uzunlukta iki vektör bekler")
		}
		dot, n1, n2 := 0.0, 0.0, 0.0
		for i := range v1 {
			dot += v1[i] * v2[i]
			n1 += v1[i] * v1[i]
			n2 += v2[i] * v2[i]
		}
		denom := math.Sqrt(n1) * math.Sqrt(n2)
		if denom == 0 {
			return &vm.FloatVal{Val: 0}, nil
		}
		return &vm.FloatVal{Val: dot / denom}, nil
	})

	v.RegisterBuiltinKod(kod.T603, func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("öklid_uzaklığı(v1, v2) 2 parametre bekler")
		}
		v1 := vm.ExtractNumericSlice([]vm.Value{args[0]})
		v2 := vm.ExtractNumericSlice([]vm.Value{args[1]})
		if len(v1) != len(v2) || len(v1) == 0 {
			return nil, fmt.Errorf("öklid_uzaklığı() eşit uzunlukta iki vektör bekler")
		}
		sum := 0.0
		for i := range v1 {
			d := v1[i] - v2[i]
			sum += d * d
		}
		return &vm.FloatVal{Val: math.Sqrt(sum)}, nil
	})

	// --- Yerel dil modeline soru (yz_sor) ---
	v.RegisterBuiltinKod(kod.T610, yzSor)
}

// yzVarsayilanAdres yz_sor'un varsayılan adresidir: bu bilgisayarda 11434 portunda çalışan
// yerel dil modeli sunucusunun metin üretme uç noktası.
const yzVarsayilanAdres = "http://localhost:11434/api/generate"

const yzSorKullanim = `yz_sor(istem, model: "model_adı", adres: "http://localhost:11434/api/generate")`

// yzSor yerel dil modeli sunucusuna bir istem gönderir ve üretilen metni döndürür. Model
// verilmezse sunucunun yüklü modellerinin ilki kullanılır (ürüne bir model adı gömülmez).
// Sunucu kapalıysa bağlantı hemen reddedilir; zaman aşımı yalnızca modelin yanıt üretme
// süresini sınırlar. HTTP kısmı (gövde boyutu sınırı, zaman aşımı, kilidi bırakma) web
// paketinin ortak istemcisindedir.
func yzSor(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 1 {
		return nil, vm.ArgumanSayisiHatasi("yz_sor", "1", len(args), yzSorKullanim)
	}
	istem, ok := args[0].(*vm.StringVal)
	if !ok {
		return nil, vm.ArgumanTipHatasi("yz_sor", 1, "Metin (istem)", args[0])
	}
	if err := baglama.AdliDenetle("yz_sor", named, adModel, adAdres); err != nil {
		return nil, err
	}
	adres := yzVarsayilanAdres
	if u, ok := baglama.AdliAl(named, adAdres); ok {
		adres = u.Inspect()
	}
	model := ""
	if m, ok := baglama.AdliAl(named, adModel); ok {
		model = m.Inspect()
	}
	if model == "" {
		var err error
		if model, err = yzIlkModel(makine, adres); err != nil {
			return nil, err
		}
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"model":  model,
		"prompt": istem.Val,
		"stream": false,
	})
	yanit, err := web.Iste(makine, http.MethodPost, adres, payload, "application/json", yzSorZamanAsimi)
	if err != nil {
		return nil, yzAgHatasi(adres, model, err)
	}
	if yanit.Durum != http.StatusOK {
		return nil, fmt.Errorf("yz_sor: yerel dil modeli sunucusu %d durum kodu döndürdü (model: %s): %s", yanit.Durum, model, yanit.GovdeOzeti(300))
	}
	var parsed map[string]interface{}
	if json.Unmarshal(yanit.Govde, &parsed) == nil {
		if ans, ok := parsed["response"].(string); ok {
			return &vm.StringVal{Val: strings.TrimSpace(ans)}, nil
		}
	}
	return nil, fmt.Errorf("yz_sor: yerel dil modeli sunucusunun (%s) yanıtı çözümlenemedi; metin üretme yanıtında 'response' alanı bekleniyordu", adres)
}

// yzAgHatasi ağ hatasını açıklar. Sunucuya ulaşılamazsa sahte bir "yanıt" döndürmek yerine
// açık hata verilir; aksi hâlde program uydurma bir metni gerçek model çıktısı sanabilir.
func yzAgHatasi(adres, model string, err error) error {
	var ih *web.IstekHatasi
	if errors.As(err, &ih) && (ih.ZamanAsim || ih.Buyuk) {
		if model != "" {
			return fmt.Errorf("yz_sor: yerel dil modeli sunucusu (%s, model: %s): %v", adres, model, err)
		}
		return fmt.Errorf("yz_sor: yerel dil modeli sunucusu (%s): %v", adres, err)
	}
	return fmt.Errorf("yz_sor: yerel dil modeli sunucusuna (%s) ulaşılamadı: %v (yz_sor, bu bilgisayarda 11434 portunda çalışan bir yerel dil modeli sunucusu ister; sunucuyu başlatıp bir model indirin ya da adres: ile başka bir sunucu verin)", adres, err)
}

// yzIlkModel model verilmediğinde sunucunun yüklü modellerinin listesinden (aynı sunucunun
// /api/tags uç noktası) ilkini seçer.
func yzIlkModel(makine *vm.VM, adres string) (string, error) {
	liste := adres
	if i := strings.Index(liste, "/api/"); i >= 0 {
		liste = liste[:i]
	}
	liste = strings.TrimSuffix(liste, "/") + "/api/tags"
	yanit, err := web.Iste(makine, http.MethodGet, liste, nil, "", yzSorZamanAsimi)
	if err != nil {
		return "", yzAgHatasi(adres, "", err)
	}
	var govde struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if yanit.Durum != http.StatusOK || json.Unmarshal(yanit.Govde, &govde) != nil {
		return "", fmt.Errorf("yz_sor: yerel dil modeli sunucusundan (%s) yüklü modellerin listesi alınamadı; modeli model: ile verin", liste)
	}
	for _, m := range govde.Models {
		if m.Name != "" {
			return m.Name, nil
		}
	}
	return "", fmt.Errorf("yz_sor: yerel dil modeli sunucusunda (%s) yüklü model yok; bir model indirip adını model: ile verin", liste)
}
