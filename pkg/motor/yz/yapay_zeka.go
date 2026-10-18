package yz

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"
	"tr_dili/pkg/motor/web"
	"tr_dili/pkg/vm"
)

// checkMatrixDims negatif / sıfır veya aşırı büyük matris boyutlarını panik yerine hata olarak bildirir.
func checkMatrixDims(fn string, rows, cols int64) error {
	if rows <= 0 || cols <= 0 {
		return fmt.Errorf("%s: satır ve sütun sayısı pozitif olmalıdır (%d x %d verildi)", fn, rows, cols)
	}
	if rows > vm.MaxCollectionSize || cols > vm.MaxCollectionSize || rows*cols > vm.MaxCollectionSize {
		return fmt.Errorf("%s: matris çok büyük (%d x %d), en fazla %d eleman", fn, rows, cols, vm.MaxCollectionSize)
	}
	return nil
}

// ============================================================================
// 2. TENSÖR VE OTOMATİK TÜREV (AUTOGRAD) MOTORU
// ============================================================================

type TensorVal struct {
	Val          float64
	Grad         float64
	RequiresGrad bool
	Prev         []*TensorVal
	BackwardFn   func()
}

func NewTensor(val float64, reqGrad bool) *TensorVal {
	return &TensorVal{
		Val:          val,
		Grad:         0.0,
		RequiresGrad: reqGrad,
	}
}

func (t *TensorVal) Type() vm.ValueType { return vm.VAL_TENSOR }
func (t *TensorVal) IsTruthy() bool     { return t.Val != 0 }
func (t *TensorVal) Inspect() string {
	if t.RequiresGrad {
		return fmt.Sprintf("Tensör(değer: %.4f, gradyan: %.4f)", t.Val, t.Grad)
	}
	return fmt.Sprintf("Tensör(%.4f)", t.Val)
}

func asTensor(v vm.Value) (*TensorVal, error) {
	if tv, ok := v.(*TensorVal); ok {
		return tv, nil
	}
	if fv, ok := vm.ToFloat(v); ok {
		return NewTensor(fv, false), nil
	}
	if v.Type() == vm.VAL_TENSOR {
		// Çok boyutlu tensör (pkg/motor/tensor) de "Tensör" tipini taşır; çelişkili
		// "Tensör tipi Tensör işlemine dönüştürülemez" mesajı yerine ayrımı açıkla.
		return nil, fmt.Errorf("tek sayılık türev değeri (tensör(3.0)) çok boyutlu tensörle işleme giremez; ikisini de tensör([...]) ile oluşturun")
	}
	return nil, fmt.Errorf("%s tipi Tensör işlemine dönüştürülemez", v.Type())
}

func (t *TensorVal) Add(other vm.Value) (vm.Value, error) {
	ot, err := asTensor(other)
	if err != nil {
		return nil, err
	}
	out := &TensorVal{
		Val:          t.Val + ot.Val,
		RequiresGrad: t.RequiresGrad || ot.RequiresGrad,
		Prev:         []*TensorVal{t, ot},
	}
	out.BackwardFn = func() {
		if t.RequiresGrad {
			t.Grad += 1.0 * out.Grad
		}
		if ot.RequiresGrad {
			ot.Grad += 1.0 * out.Grad
		}
	}
	return out, nil
}

func (t *TensorVal) Sub(other vm.Value) (vm.Value, error) {
	ot, err := asTensor(other)
	if err != nil {
		return nil, err
	}
	out := &TensorVal{
		Val:          t.Val - ot.Val,
		RequiresGrad: t.RequiresGrad || ot.RequiresGrad,
		Prev:         []*TensorVal{t, ot},
	}
	out.BackwardFn = func() {
		if t.RequiresGrad {
			t.Grad += 1.0 * out.Grad
		}
		if ot.RequiresGrad {
			ot.Grad -= 1.0 * out.Grad
		}
	}
	return out, nil
}

func (t *TensorVal) Mul(other vm.Value) (vm.Value, error) {
	ot, err := asTensor(other)
	if err != nil {
		return nil, err
	}
	out := &TensorVal{
		Val:          t.Val * ot.Val,
		RequiresGrad: t.RequiresGrad || ot.RequiresGrad,
		Prev:         []*TensorVal{t, ot},
	}
	out.BackwardFn = func() {
		if t.RequiresGrad {
			t.Grad += ot.Val * out.Grad
		}
		if ot.RequiresGrad {
			ot.Grad += t.Val * out.Grad
		}
	}
	return out, nil
}

func (t *TensorVal) Div(other vm.Value) (vm.Value, error) {
	ot, err := asTensor(other)
	if err != nil {
		return nil, err
	}
	if ot.Val == 0 {
		return nil, fmt.Errorf("Tensör sıfıra bölünemez")
	}
	out := &TensorVal{
		Val:          t.Val / ot.Val,
		RequiresGrad: t.RequiresGrad || ot.RequiresGrad,
		Prev:         []*TensorVal{t, ot},
	}
	out.BackwardFn = func() {
		if t.RequiresGrad {
			t.Grad += (1.0 / ot.Val) * out.Grad
		}
		if ot.RequiresGrad {
			ot.Grad += (-t.Val / (ot.Val * ot.Val)) * out.Grad
		}
	}
	return out, nil
}

func (t *TensorVal) MatMul(other vm.Value) (vm.Value, error) {
	return t.Mul(other)
}

func (t *TensorVal) GetProperty(prop string) (vm.Value, error) {
	switch prop {
	case "gradyan", "türev", "turev":
		return &vm.FloatVal{Val: t.Grad}, nil
	case "değer", "deger":
		return &vm.FloatVal{Val: t.Val}, nil
	}
	return nil, fmt.Errorf("Tensör üzerinde %q özelliği bulunamadı", prop)
}

func (t *TensorVal) Backward() {
	var topo []*TensorVal
	visited := make(map[*TensorVal]bool)
	var buildTopo func(v *TensorVal)
	buildTopo = func(v *TensorVal) {
		if !visited[v] {
			visited[v] = true
			for _, child := range v.Prev {
				buildTopo(child)
			}
			topo = append(topo, v)
		}
	}
	buildTopo(t)

	// Önce tüm düğümlerin gradyanını sıfırla, kök düğüme 1.0 ata
	for _, node := range topo {
		node.Grad = 0.0
	}
	t.Grad = 1.0
	for i := len(topo) - 1; i >= 0; i-- {
		if topo[i].BackwardFn != nil {
			topo[i].BackwardFn()
		}
	}
}

// ============================================================================
// 4. YARDIMCI DÖNÜŞTÜRÜCÜLER VE FAZ 2 YERLEŞİK FONKSİYON KAYITLARI
// ============================================================================

func valueToMatrix(v vm.Value) (*vm.MatrixVal, error) {
	if m, ok := v.(*vm.MatrixVal); ok {
		return m, nil
	}
	list, ok := v.(*vm.ListVal)
	if !ok || len(list.Elements) == 0 {
		return nil, fmt.Errorf("Matris oluşturmak için boş olmayan bir Liste gereklidir")
	}

	// 2 boyutlu liste: [[1, 2], [3, 4]]
	if firstRow, ok := list.Elements[0].(*vm.ListVal); ok {
		rows := len(list.Elements)
		cols := len(firstRow.Elements)
		mat := vm.NewMatrix(rows, cols)
		for i, rVal := range list.Elements {
			rList, ok := rVal.(*vm.ListVal)
			if !ok || len(rList.Elements) != cols {
				return nil, fmt.Errorf("Matrisin tüm satırları eşit uzunlukta olmalıdır")
			}
			for j, cVal := range rList.Elements {
				f, ok := vm.ToFloat(cVal)
				if !ok {
					return nil, fmt.Errorf("Matris elemanları sayısal olmalıdır")
				}
				mat.Data[i][j] = f
			}
		}
		return mat, nil
	}

	// 1 boyutlu liste -> 1xN satır matrisi
	cols := len(list.Elements)
	mat := vm.NewMatrix(1, cols)
	for j, cVal := range list.Elements {
		f, ok := vm.ToFloat(cVal)
		if !ok {
			return nil, fmt.Errorf("Matris elemanları sayısal olmalıdır")
		}
		mat.Data[0][j] = f
	}
	return mat, nil
}

// yzSorZamanAsimi yz_sor isteğinin toplam süre sınırıdır (modelin yanıt üretmesi dahil).
var yzSorZamanAsimi = 120 * time.Second

// Kaydet matris, tensör, sinir ağı, istatistik, model ve yz_sor yerleşiklerini VM'ye
// kaydeder. JSON/CSV yerleşikleri pkg/motor/veribicim, web_al/web_gönder ise
// pkg/motor/web paketindedir.
func Kaydet(v *vm.VM) {
	// --- Matris Oluşturucular ---
	v.RegisterBuiltin("matris", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("matris(liste) 1 parametre bekler")
		}
		return valueToMatrix(args[0])
	})

	v.RegisterBuiltin("sıfırlar", "sifirlar", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) == 1 {
			n, ok := args[0].(*vm.IntVal)
			if !ok {
				return nil, fmt.Errorf("sıfırlar() Tam sayı bekler")
			}
			if err := checkMatrixDims("sıfırlar", 1, n.Val); err != nil {
				return nil, err
			}
			return vm.NewMatrix(1, int(n.Val)), nil
		}
		if len(args) >= 2 {
			r, ok1 := args[0].(*vm.IntVal)
			c, ok2 := args[1].(*vm.IntVal)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("sıfırlar(satır, sütun) Tam sayı bekler")
			}
			if err := checkMatrixDims("sıfırlar", r.Val, c.Val); err != nil {
				return nil, err
			}
			return vm.NewMatrix(int(r.Val), int(c.Val)), nil
		}
		return nil, fmt.Errorf("sıfırlar(satır, [sütun]) en az 1 parametre bekler")
	})

	v.RegisterBuiltin("birler", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("birler(satır, sütun) 2 parametre bekler")
		}
		r, ok1 := args[0].(*vm.IntVal)
		c, ok2 := args[1].(*vm.IntVal)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("birler(satır, sütun) Tam sayı bekler")
		}
		if err := checkMatrixDims("birler", r.Val, c.Val); err != nil {
			return nil, err
		}
		m := vm.NewMatrix(int(r.Val), int(c.Val))
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				m.Data[i][j] = 1.0
			}
		}
		return m, nil
	})

	v.RegisterBuiltin("birim_matris", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("birim_matris(n) 1 parametre bekler")
		}
		n, ok := args[0].(*vm.IntVal)
		if !ok {
			return nil, fmt.Errorf("birim_matris(n) Tam sayı bekler")
		}
		if err := checkMatrixDims("birim_matris", n.Val, n.Val); err != nil {
			return nil, err
		}
		m := vm.NewMatrix(int(n.Val), int(n.Val))
		for i := 0; i < m.Rows; i++ {
			m.Data[i][i] = 1.0
		}
		return m, nil
	})

	v.RegisterBuiltin("rastgele_matris", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("rastgele_matris(satır, sütun, [tohum]) en az 2 parametre bekler")
		}
		r, ok1 := args[0].(*vm.IntVal)
		c, ok2 := args[1].(*vm.IntVal)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("rastgele_matris(satır, sütun) Tam sayı bekler")
		}
		if err := checkMatrixDims("rastgele_matris", r.Val, c.Val); err != nil {
			return nil, err
		}
		seed := int64(42)
		if len(args) >= 3 {
			if s, ok := args[2].(*vm.IntVal); ok {
				seed = s.Val
			}
		}
		if named != nil {
			if s, ok := named["tohum"]; ok {
				if iv, ok := s.(*vm.IntVal); ok {
					seed = iv.Val
				}
			}
		}
		rng := rand.New(rand.NewSource(seed))
		m := vm.NewMatrix(int(r.Val), int(c.Val))
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				m.Data[i][j] = rng.NormFloat64() * 0.5
			}
		}
		return m, nil
	})

	v.RegisterBuiltin("devrik", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("devrik(matris) 1 parametre bekler")
		}
		m, err := valueToMatrix(args[0])
		if err != nil {
			return nil, err
		}
		return m.Transpose(), nil
	})

	// --- Tensör ve Otomatik Türev (Autograd) ---
	v.RegisterBuiltin("tensör", "tensor", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("tensör(değer, [türev_izle: doğru]) en az 1 parametre bekler")
		}
		f, ok := vm.ToFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("tensör() sayısal değer bekler")
		}
		reqGrad := true
		if len(args) >= 2 {
			reqGrad = args[1].IsTruthy()
		}
		if named != nil {
			if v, ok := named["türev_izle"]; ok {
				reqGrad = v.IsTruthy()
			} else if v, ok := named["turev_izle"]; ok {
				reqGrad = v.IsTruthy()
			}
		}
		return NewTensor(f, reqGrad), nil
	})

	v.RegisterBuiltin("geri_yayılım", "geri_yayilim", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("geri_yayılım(tensör) 1 parametre bekler")
		}
		tv, ok := args[0].(*TensorVal)
		if !ok {
			return nil, fmt.Errorf("geri_yayılım() bir Tensör bekler, %s verildi", args[0].Type())
		}
		tv.Backward()
		return vm.TRUE, nil
	})

	// --- Aktivasyon Fonksiyonları (relu, sigmoid, tanh, softmax) ---
	v.RegisterBuiltin("relu", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("relu(x) 1 parametre bekler")
		}
		return applyUnaryActivation(args[0], func(x float64) float64 {
			if x > 0 {
				return x
			}
			return 0
		}, func(x float64) float64 {
			if x > 0 {
				return 1
			}
			return 0
		})
	})

	v.RegisterBuiltin("sigmoid", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("sigmoid(x) 1 parametre bekler")
		}
		return applyUnaryActivation(args[0], func(x float64) float64 {
			return 1.0 / (1.0 + math.Exp(-x))
		}, func(x float64) float64 {
			s := 1.0 / (1.0 + math.Exp(-x))
			return s * (1.0 - s)
		})
	})

	v.RegisterBuiltin("tanh", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("tanh(x) 1 parametre bekler")
		}
		return applyUnaryActivation(args[0], math.Tanh, func(x float64) float64 {
			t := math.Tanh(x)
			return 1.0 - t*t
		})
	})

	v.RegisterBuiltin("softmax", "", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("softmax(x) 1 parametre bekler")
		}
		if mat, ok := args[0].(*vm.MatrixVal); ok {
			res := vm.NewMatrix(mat.Rows, mat.Cols)
			for i := 0; i < mat.Rows; i++ {
				maxVal := mat.Data[i][0]
				for j := 1; j < mat.Cols; j++ {
					if mat.Data[i][j] > maxVal {
						maxVal = mat.Data[i][j]
					}
				}
				sum := 0.0
				for j := 0; j < mat.Cols; j++ {
					res.Data[i][j] = math.Exp(mat.Data[i][j] - maxVal)
					sum += res.Data[i][j]
				}
				for j := 0; j < mat.Cols; j++ {
					res.Data[i][j] /= sum
				}
			}
			return res, nil
		}
		nums := vm.ExtractNumericSlice(args)
		if len(nums) == 0 {
			return nil, fmt.Errorf("softmax() sayısal Liste veya Matris bekler")
		}
		maxVal := nums[0]
		for _, v := range nums[1:] {
			if v > maxVal {
				maxVal = v
			}
		}
		exps := make([]float64, len(nums))
		sum := 0.0
		for i, v := range nums {
			exps[i] = math.Exp(v - maxVal)
			sum += exps[i]
		}
		elems := make([]vm.Value, len(nums))
		for i, e := range exps {
			elems[i] = &vm.FloatVal{Val: e / sum}
		}
		return &vm.ListVal{Elements: elems}, nil
	})

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

	v.RegisterBuiltin("standart_sapma", "std_sapma", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

	v.RegisterBuiltin("medyan", "ortanca", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
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

func applyUnaryActivation(val vm.Value, fn func(float64) float64, deriv func(float64) float64) (vm.Value, error) {
	switch v := val.(type) {
	case *TensorVal:
		out := &TensorVal{
			Val:          fn(v.Val),
			RequiresGrad: v.RequiresGrad,
			Prev:         []*TensorVal{v},
		}
		out.BackwardFn = func() {
			if v.RequiresGrad {
				v.Grad += deriv(v.Val) * out.Grad
			}
		}
		return out, nil
	case *vm.MatrixVal:
		res := vm.NewMatrix(v.Rows, v.Cols)
		for i := 0; i < v.Rows; i++ {
			for j := 0; j < v.Cols; j++ {
				res.Data[i][j] = fn(v.Data[i][j])
			}
		}
		return res, nil
	case *vm.ListVal:
		elems := make([]vm.Value, len(v.Elements))
		for i, el := range v.Elements {
			mapped, err := applyUnaryActivation(el, fn, deriv)
			if err != nil {
				return nil, err
			}
			elems[i] = mapped
		}
		return &vm.ListVal{Elements: elems}, nil
	default:
		if f, ok := vm.ToFloat(val); ok {
			return &vm.FloatVal{Val: fn(f)}, nil
		}
		return nil, fmt.Errorf("Aktivasyon fonksiyonu %s tipine uygulanamaz", val.Type())
	}
}
