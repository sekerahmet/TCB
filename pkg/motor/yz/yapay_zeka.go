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
	"tr_dili/pkg/motor/web"
	"tr_dili/pkg/vm"
)

// yzSorZamanAsimi yz_sor isteğinin toplam süre sınırıdır (modelin yanıt üretmesi dahil).
var yzSorZamanAsimi = 120 * time.Second

// Kaydet istatistik, vektör benzerliği ve yz_sor yerleşiklerini VM'ye kaydeder. Tensör,
// matris, otomatik türev ve sinir ağı yerleşikleri pkg/motor/tensor, JSON/CSV yerleşikleri
// pkg/motor/veribicim, web_al/web_gönder ise pkg/motor/web paketindedir.
func Kaydet(v *vm.VM) {
	// --- Veri Bilimi, İstatistik ve Vektör Benzerlik ---
	v.RegisterBuiltin("ortalama", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

	v.RegisterBuiltin("standart_sapma", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

	v.RegisterBuiltin("ortanca", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

	v.RegisterBuiltin("normalize_et", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

	v.RegisterBuiltin("nokta_çarpım", "nokta_carpim", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

	v.RegisterBuiltin("kosinüs_benzerliği", "kosinus_benzerligi", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

	v.RegisterBuiltin("öklid_uzaklığı", "oklid_uzakligi", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

	// --- Büyük Dil Modeli (LLM: yz_sor) ---
	v.RegisterBuiltin("yz_sor", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("yz_sor(istem, [model: 'llama3']) en az 1 parametre bekler")
		}
		prompt := args[0].Inspect()
		model := "llama3"
		endpoint := "http://localhost:11434/api/generate"
		if named != nil {
			if m, ok := named["model"]; ok {
				model = m.Inspect()
			}
			if u, ok := named["adres"]; ok {
				endpoint = u.Inspect()
			}
		}

		// Yerel Ollama sunucusuna istek. Sunucu kapalıysa bağlantı hemen reddedilir;
		// zaman aşımı yalnızca modelin yanıt üretme süresini sınırlar. HTTP kısmı (gövde
		// boyutu sınırı, zaman aşımı, kilidi bırakma) web paketinin ortak istemcisindedir.
		payload, _ := json.Marshal(map[string]interface{}{
			"model":  model,
			"prompt": prompt,
			"stream": false,
		})
		yanit, err := web.Iste(makine, http.MethodPost, endpoint, payload, "application/json", yzSorZamanAsimi)
		// Sunucuya ulaşılamazsa sahte bir "yanıt" döndürmek yerine açık hata verilir;
		// aksi hâlde program uydurma bir metni gerçek model çıktısı sanabilir.
		if err != nil {
			var ih *web.IstekHatasi
			if errors.As(err, &ih) && (ih.ZamanAsim || ih.Buyuk) {
				return nil, fmt.Errorf("yz_sor: YZ sunucusu (%s, model: %s): %v", endpoint, model, err)
			}
			return nil, fmt.Errorf("yz_sor: yerel YZ sunucusuna (%s) ulaşılamadı: %v (Ollama'yı 'ollama serve' ile başlatın ve modeli 'ollama pull %s' ile indirin)", endpoint, err, model)
		}
		if yanit.Durum != http.StatusOK {
			return nil, fmt.Errorf("yz_sor: YZ sunucusu %d durum kodu döndürdü (model: %s): %s", yanit.Durum, model, yanit.GovdeOzeti(300))
		}
		var parsed map[string]interface{}
		if json.Unmarshal(yanit.Govde, &parsed) == nil {
			if ans, ok := parsed["response"].(string); ok {
				return &vm.StringVal{Val: strings.TrimSpace(ans)}, nil
			}
		}
		return nil, fmt.Errorf("yz_sor: YZ sunucusunun (%s) yanıtı çözümlenemedi; Ollama /api/generate biçiminde 'response' alanı bekleniyordu", endpoint)
	})
}
