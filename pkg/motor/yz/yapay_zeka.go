package yz

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"tr_dili/pkg/vm"
)

// httpFetch bir HTTP yanıtının gövdesini okuyup kapatır.
func httpFetch(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// loadedMatrix dosyadan okunan iç içe diziyi boş veya düzensiz (satır uzunlukları farklı) olmadığını denetleyerek matrise çevirir.
func loadedMatrix(data [][]float64) (*vm.MatrixVal, error) {
	if len(data) == 0 || len(data[0]) == 0 {
		return nil, fmt.Errorf("boş matris")
	}
	for i, row := range data {
		if len(row) != len(data[0]) {
			return nil, fmt.Errorf("%d. satırın uzunluğu %d, %d bekleniyordu", i+1, len(row), len(data[0]))
		}
	}
	return &vm.MatrixVal{Rows: len(data), Cols: len(data[0]), Data: data}, nil
}

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
// 3. YÜKSEK SEVİYE SİNİR AĞI MODELİ (sinir_ağı, .eğit, .tahmin_et)
// ============================================================================

type NeuralNetVal struct {
	LayerSizes []int
	Activation string
	Weights    []*vm.MatrixVal
	Biases     []*vm.MatrixVal
}

func NewNeuralNet(sizes []int, activation string, seed int64) *NeuralNetVal {
	rng := rand.New(rand.NewSource(seed))
	numLayers := len(sizes) - 1
	weights := make([]*vm.MatrixVal, numLayers)
	biases := make([]*vm.MatrixVal, numLayers)

	for l := 0; l < numLayers; l++ {
		inDim := sizes[l]
		outDim := sizes[l+1]
		scale := math.Sqrt(2.0 / float64(inDim+outDim))
		w := vm.NewMatrix(inDim, outDim)
		for i := 0; i < inDim; i++ {
			for j := 0; j < outDim; j++ {
				w.Data[i][j] = rng.NormFloat64() * scale
			}
		}
		b := vm.NewMatrix(1, outDim)
		weights[l] = w
		biases[l] = b
	}

	return &NeuralNetVal{
		LayerSizes: sizes,
		Activation: strings.ToLower(activation),
		Weights:    weights,
		Biases:     biases,
	}
}

func (nn *NeuralNetVal) Type() vm.ValueType { return vm.ValueType("SinirAğı") }
func (nn *NeuralNetVal) IsTruthy() bool     { return true }
func (nn *NeuralNetVal) Inspect() string {
	var dims []string
	for _, s := range nn.LayerSizes {
		dims = append(dims, strconv.Itoa(s))
	}
	return fmt.Sprintf("<SinirAğı mimari=[%s] aktivasyon=%q>", strings.Join(dims, " -> "), nn.Activation)
}

func (nn *NeuralNetVal) activate(x float64, isOutput bool) float64 {
	if isOutput {
		if nn.Activation == "sigmoid" {
			return 1.0 / (1.0 + math.Exp(-x))
		}
		return x
	}
	switch nn.Activation {
	case "relu":
		if x > 0 {
			return x
		}
		return 0.01 * x // Leaky ReLU ölü nöronları önler
	case "sigmoid":
		return 1.0 / (1.0 + math.Exp(-x))
	default: // tanh varsayılan olarak hem doğrusal olmayan hem sınıflandırma/regresyon için çok kararlıdır
		return math.Tanh(x)
	}
}

func (nn *NeuralNetVal) activateDeriv(activatedVal float64, rawVal float64, isOutput bool) float64 {
	if isOutput {
		if nn.Activation == "sigmoid" {
			return activatedVal * (1.0 - activatedVal)
		}
		return 1.0
	}
	switch nn.Activation {
	case "relu":
		if rawVal > 0 {
			return 1.0
		}
		return 0.01
	case "sigmoid":
		return activatedVal * (1.0 - activatedVal)
	default: // tanh
		return 1.0 - activatedVal*activatedVal
	}
}

func (nn *NeuralNetVal) Forward(X *vm.MatrixVal) ([]*vm.MatrixVal, []*vm.MatrixVal, error) {
	activations := []*vm.MatrixVal{X}
	zs := []*vm.MatrixVal{}
	cur := X

	for l := 0; l < len(nn.Weights); l++ {
		isOut := l == len(nn.Weights)-1
		mmVal, err := cur.MatMul(nn.Weights[l])
		if err != nil {
			return nil, nil, err
		}
		zMat, err := mmVal.(*vm.MatrixVal).Add(nn.Biases[l])
		if err != nil {
			return nil, nil, err
		}
		z := zMat.(*vm.MatrixVal)
		zs = append(zs, z)

		a := vm.NewMatrix(z.Rows, z.Cols)
		for i := 0; i < z.Rows; i++ {
			for j := 0; j < z.Cols; j++ {
				a.Data[i][j] = nn.activate(z.Data[i][j], isOut)
			}
		}
		activations = append(activations, a)
		cur = a
	}

	return activations, zs, nil
}

func (nn *NeuralNetVal) Train(X, Y *vm.MatrixVal, epochs int, lr float64) ([]float64, error) {
	if X.Rows != Y.Rows {
		return nil, fmt.Errorf("Eğitim girdi satır sayısı (%d) ile hedef satır sayısı (%d) eşit olmalıdır", X.Rows, Y.Rows)
	}
	if epochs <= 0 {
		return nil, fmt.Errorf("devir (epoch) sayısı pozitif olmalıdır (%d verildi)", epochs)
	}
	if len(nn.Weights) == 0 {
		return nil, fmt.Errorf("sinir ağında eğitilecek katman yok")
	}
	if outCols := nn.Weights[len(nn.Weights)-1].Cols; Y.Cols != outCols {
		return nil, fmt.Errorf("Hedef sütun sayısı (%d) çıkış katmanı boyutuyla (%d) eşit olmalıdır", Y.Cols, outCols)
	}
	if inRows := nn.Weights[0].Rows; X.Cols != inRows {
		return nil, fmt.Errorf("Girdi sütun sayısı (%d) giriş katmanı boyutuyla (%d) eşit olmalıdır", X.Cols, inRows)
	}
	nSamples := float64(X.Rows)
	numLayers := len(nn.Weights)
	var lossHistory []float64

	for ep := 0; ep < epochs; ep++ {
		activations, zs, err := nn.Forward(X)
		if err != nil {
			return nil, err
		}
		pred := activations[len(activations)-1]

		// MSE Kaybı ve son katman deltası
		loss := 0.0
		delta := vm.NewMatrix(pred.Rows, pred.Cols)
		for i := 0; i < pred.Rows; i++ {
			for j := 0; j < pred.Cols; j++ {
				diff := pred.Data[i][j] - Y.Data[i][j]
				loss += diff * diff
				dAct := nn.activateDeriv(pred.Data[i][j], zs[numLayers-1].Data[i][j], true)
				delta.Data[i][j] = (2.0 * diff / nSamples) * dAct
			}
		}
		loss /= float64(pred.Rows * pred.Cols)
		lossHistory = append(lossHistory, loss)

		// Geri yayılım (Backpropagation)
		gradW := make([]*vm.MatrixVal, numLayers)
		gradB := make([]*vm.MatrixVal, numLayers)

		for l := numLayers - 1; l >= 0; l-- {
			prevActT := activations[l].Transpose()
			gwVal, err := prevActT.MatMul(delta)
			if err != nil {
				return nil, err
			}
			gradW[l] = gwVal.(*vm.MatrixVal)

			gb := vm.NewMatrix(1, delta.Cols)
			for i := 0; i < delta.Rows; i++ {
				for j := 0; j < delta.Cols; j++ {
					gb.Data[0][j] += delta.Data[i][j]
				}
			}
			gradB[l] = gb

			if l > 0 {
				wT := nn.Weights[l].Transpose()
				nextDeltaVal, err := delta.MatMul(wT)
				if err != nil {
					return nil, err
				}
				nextDelta := nextDeltaVal.(*vm.MatrixVal)
				for i := 0; i < nextDelta.Rows; i++ {
					for j := 0; j < nextDelta.Cols; j++ {
						nextDelta.Data[i][j] *= nn.activateDeriv(activations[l].Data[i][j], zs[l-1].Data[i][j], false)
					}
				}
				delta = nextDelta
			}
		}

		// Ağırlık ve sapma (bias) güncellemesi
		for l := 0; l < numLayers; l++ {
			for i := 0; i < nn.Weights[l].Rows; i++ {
				for j := 0; j < nn.Weights[l].Cols; j++ {
					nn.Weights[l].Data[i][j] -= lr * gradW[l].Data[i][j]
				}
			}
			for j := 0; j < nn.Biases[l].Cols; j++ {
				nn.Biases[l].Data[0][j] -= lr * gradB[l].Data[0][j]
			}
		}
	}

	return lossHistory, nil
}

// NeuralNetVal ayrıca MatrixOperator üzerinden .eğit, .tahmin_et, .ağırlıklar metodlarını sunar
func (nn *NeuralNetVal) Add(other vm.Value) (vm.Value, error) {
	return nil, fmt.Errorf("SinirAğı toplanamaz")
}
func (nn *NeuralNetVal) Sub(other vm.Value) (vm.Value, error) {
	return nil, fmt.Errorf("SinirAğı çıkarılamaz")
}
func (nn *NeuralNetVal) Mul(other vm.Value) (vm.Value, error) {
	return nil, fmt.Errorf("SinirAğı çarpılamaz")
}
func (nn *NeuralNetVal) Div(other vm.Value) (vm.Value, error) {
	return nil, fmt.Errorf("SinirAğı bölünemez")
}
func (nn *NeuralNetVal) MatMul(other vm.Value) (vm.Value, error) {
	return nil, fmt.Errorf("SinirAğı @ ile çarpılamaz")
}

func (nn *NeuralNetVal) GetProperty(prop string) (vm.Value, error) {
	switch prop {
	case "eğit", "egit":
		return &vm.BuiltinVal{
			Name: "model.eğit",
			Fn: func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("model.eğit(Girdi, Hedef, [devir: 500], [öğrenme_hızı: 0.1]) en az 2 parametre bekler")
				}
				X, err := valueToMatrix(args[0])
				if err != nil {
					return nil, fmt.Errorf("Girdi matrise çevrilemedi: %v", err)
				}
				Y, err := valueToMatrix(args[1])
				if err != nil {
					return nil, fmt.Errorf("Hedef matrise çevrilemedi: %v", err)
				}
				epochs := 500
				lr := 0.1
				if named != nil {
					if v, ok := named["devir"]; ok {
						if iv, ok := v.(*vm.IntVal); ok {
							epochs = int(iv.Val)
						}
					}
					if v, ok := named["öğrenme_hızı"]; ok {
						if fv, ok := vm.ToFloat(v); ok {
							lr = fv
						}
					} else if v, ok := named["ogrenme_hizi"]; ok {
						if fv, ok := vm.ToFloat(v); ok {
							lr = fv
						}
					}
				}
				if len(args) >= 3 {
					if iv, ok := args[2].(*vm.IntVal); ok {
						epochs = int(iv.Val)
					}
				}
				if len(args) >= 4 {
					if fv, ok := vm.ToFloat(args[3]); ok {
						lr = fv
					}
				}
				history, err := nn.Train(X, Y, epochs, lr)
				if err != nil {
					return nil, err
				}
				res := vm.NewDictVal()
				res.Set("ilk_kayıp", &vm.FloatVal{Val: history[0]})
				res.Set("son_kayıp", &vm.FloatVal{Val: history[len(history)-1]})
				res.Set("devir", &vm.IntVal{Val: int64(epochs)})
				return res, nil
			},
		}, nil

	case "tahmin_et", "tahmin":
		return &vm.BuiltinVal{
			Name: "model.tahmin_et",
			Fn: func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("model.tahmin_et(Girdi) 1 parametre bekler")
				}
				X, err := valueToMatrix(args[0])
				if err != nil {
					return nil, err
				}
				acts, _, err := nn.Forward(X)
				if err != nil {
					return nil, err
				}
				return acts[len(acts)-1], nil
			},
		}, nil

	case "ağırlıklar", "agirliklar":
		elems := make([]vm.Value, len(nn.Weights))
		for i, w := range nn.Weights {
			elems[i] = w
		}
		return &vm.ListVal{Elements: elems}, nil

	case "kaydet":
		return &vm.BuiltinVal{
			Name: "model.kaydet",
			Fn: func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("model.kaydet(dosya_yolu) 1 parametre bekler")
				}
				path := args[0].Inspect()
				weightsData := make([][][]float64, len(nn.Weights))
				for i, w := range nn.Weights {
					weightsData[i] = w.Data
				}
				biasesData := make([][][]float64, len(nn.Biases))
				for i, b := range nn.Biases {
					biasesData[i] = b.Data
				}
				payload := map[string]interface{}{
					"katman_boyutları": nn.LayerSizes,
					"aktivasyon":       nn.Activation,
					"ağırlıklar":       weightsData,
					"sapmalar":         biasesData,
				}
				data, err := json.MarshalIndent(payload, "", "  ")
				if err != nil {
					return nil, err
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					return nil, fmt.Errorf("Model kaydedilemedi: %v", err)
				}
				return vm.TRUE, nil
			},
		}, nil
	}
	return nil, fmt.Errorf("SinirAğı üzerinde %q özelliği veya metodu bulunamadı", prop)
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

// Kaydet matris, tensör, sinir ağı, istatistik, JSON/CSV, model ve web isteği
// yerleşiklerini VM'ye kaydeder.
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

	// --- Kayıp (Loss) ve Yüksek Seviye Sinir Ağı ---
	v.RegisterBuiltin("kayıp_mse", "kayip_mse", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("kayıp_mse(tahmin, gerçek) 2 parametre bekler")
		}
		m1, err1 := valueToMatrix(args[0])
		m2, err2 := valueToMatrix(args[1])
		if err1 == nil && err2 == nil {
			if m1.Rows != m2.Rows || m1.Cols != m2.Cols {
				return nil, fmt.Errorf("kayıp_mse boyut uyuşmazlığı")
			}
			sum := 0.0
			for i := 0; i < m1.Rows; i++ {
				for j := 0; j < m1.Cols; j++ {
					d := m1.Data[i][j] - m2.Data[i][j]
					sum += d * d
				}
			}
			return &vm.FloatVal{Val: sum / float64(m1.Rows*m1.Cols)}, nil
		}
		return nil, fmt.Errorf("kayıp_mse() Liste veya Matris bekler")
	})

	v.RegisterBuiltin("sinir_ağı", "sinir_agi", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("sinir_ağı(katman_boyutları, [aktivasyon: 'tanh'], [tohum: 42]) en az 1 parametre bekler")
		}
		list, ok := args[0].(*vm.ListVal)
		if !ok || len(list.Elements) < 2 {
			return nil, fmt.Errorf("sinir_ağı() en az 2 katman boyutu içeren bir Liste bekler (Örn: [2, 8, 1])")
		}
		sizes := make([]int, len(list.Elements))
		for i, el := range list.Elements {
			iv, ok := el.(*vm.IntVal)
			if !ok || iv.Val <= 0 {
				return nil, fmt.Errorf("Katman boyutları pozitif Tam sayı olmalıdır")
			}
			sizes[i] = int(iv.Val)
		}
		act := "tanh"
		seed := int64(42)
		if len(args) >= 2 {
			act = args[1].Inspect()
		}
		if named != nil {
			if v, ok := named["aktivasyon"]; ok {
				act = v.Inspect()
			}
			if v, ok := named["tohum"]; ok {
				if iv, ok := v.(*vm.IntVal); ok {
					seed = iv.Val
				}
			}
		}
		return NewNeuralNet(sizes, act, seed), nil
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

	// --- JSON ve CSV Veri Araçları ---
	v.RegisterBuiltin("json_çöz", "json_coz", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("json_çöz(metin) 1 parametre bekler")
		}
		val, err := vm.JSONToTR([]byte(args[0].Inspect()))
		if err != nil {
			return nil, fmt.Errorf("JSON çözümleme hatası: %v", err)
		}
		return val, nil
	})

	v.RegisterBuiltin("json_metin", "json_yaz", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("json_metin(değer) 1 parametre bekler")
		}
		b, err := json.Marshal(vm.TRToGoValue(args[0]))
		if err != nil {
			return nil, err
		}
		return &vm.StringVal{Val: string(b)}, nil
	})

	v.RegisterBuiltin("csv_oku", "tablo_oku", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("csv_oku(dosya_veya_metin) 1 parametre bekler")
		}
		input := args[0].Inspect()
		if data, err := os.ReadFile(input); err == nil {
			input = string(data)
		}
		r := csv.NewReader(strings.NewReader(strings.TrimSpace(input)))
		records, err := r.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("CSV okuma hatası: %v", err)
		}
		var rows []vm.Value
		for _, rec := range records {
			var cols []vm.Value
			for _, cell := range rec {
				cell = strings.TrimSpace(cell)
				if iv, err := strconv.ParseInt(cell, 10, 64); err == nil {
					cols = append(cols, &vm.IntVal{Val: iv})
				} else if fv, err := strconv.ParseFloat(cell, 64); err == nil {
					cols = append(cols, &vm.FloatVal{Val: fv})
				} else {
					cols = append(cols, &vm.StringVal{Val: cell})
				}
			}
			rows = append(rows, &vm.ListVal{Elements: cols})
		}
		return &vm.ListVal{Elements: rows}, nil
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
		// zaman aşımı yalnızca modelin yanıt üretme süresini sınırlar.
		payload, _ := json.Marshal(map[string]interface{}{
			"model":  model,
			"prompt": prompt,
			"stream": false,
		})
		client := &http.Client{Timeout: 120 * time.Second}
		var resp *http.Response
		var err error
		var body []byte
		makine.Blocking(func() {
			resp, err = client.Post(endpoint, "application/json", bytes.NewReader(payload))
			if err == nil {
				defer resp.Body.Close()
				body, _ = io.ReadAll(resp.Body)
			}
		})
		// Sunucuya ulaşılamazsa sahte bir "yanıt" döndürmek yerine açık hata verilir;
		// aksi hâlde program uydurma bir metni gerçek model çıktısı sanabilir.
		if err != nil {
			return nil, fmt.Errorf("yz_sor: yerel YZ sunucusuna (%s) ulaşılamadı: %v (Ollama'yı 'ollama serve' ile başlatın ve modeli 'ollama pull %s' ile indirin)", endpoint, err, model)
		}
		if resp.StatusCode != http.StatusOK {
			detay := strings.TrimSpace(string(body))
			if len(detay) > 300 {
				detay = detay[:300] + "..."
			}
			return nil, fmt.Errorf("yz_sor: YZ sunucusu %d durum kodu döndürdü (model: %s): %s", resp.StatusCode, model, detay)
		}
		var parsed map[string]interface{}
		if json.Unmarshal(body, &parsed) == nil {
			if ans, ok := parsed["response"].(string); ok {
				return &vm.StringVal{Val: strings.TrimSpace(ans)}, nil
			}
		}
		return nil, fmt.Errorf("yz_sor: YZ sunucusunun (%s) yanıtı çözümlenemedi; Ollama /api/generate biçiminde 'response' alanı bekleniyordu", endpoint)
	})

	// --- Model Yükleme (model_yükle) ---
	v.RegisterBuiltin("model_yükle", "model_yukle", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("model_yükle(dosya_yolu) 1 parametre bekler")
		}
		path := args[0].Inspect()
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("Model dosyası okunamadı (%s): %v", path, err)
		}
		var saved struct {
			LayerSizes []int         `json:"katman_boyutları"`
			Activation string        `json:"aktivasyon"`
			Weights    [][][]float64 `json:"ağırlıklar"`
			Biases     [][][]float64 `json:"sapmalar"`
		}
		if err := json.Unmarshal(data, &saved); err != nil {
			return nil, fmt.Errorf("Model dosyası geçersiz formatta: %v", err)
		}
		nn := &NeuralNetVal{
			LayerSizes: saved.LayerSizes,
			Activation: saved.Activation,
			Weights:    make([]*vm.MatrixVal, len(saved.Weights)),
			Biases:     make([]*vm.MatrixVal, len(saved.Biases)),
		}
		if len(saved.Weights) == 0 || len(saved.Weights) != len(saved.Biases) {
			return nil, fmt.Errorf("Model dosyası bozuk (%s): %d ağırlık ve %d sapma katmanı var", path, len(saved.Weights), len(saved.Biases))
		}
		for i := range saved.Weights {
			w, err := loadedMatrix(saved.Weights[i])
			if err != nil {
				return nil, fmt.Errorf("Model dosyası bozuk (%s): %d. katman ağırlıkları: %v", path, i+1, err)
			}
			b, err := loadedMatrix(saved.Biases[i])
			if err != nil {
				return nil, fmt.Errorf("Model dosyası bozuk (%s): %d. katman sapmaları: %v", path, i+1, err)
			}
			if i > 0 && nn.Weights[i-1].Cols != w.Rows {
				return nil, fmt.Errorf("Model dosyası bozuk (%s): %d. katman boyutları uyuşmuyor", path, i+1)
			}
			if b.Rows != 1 || b.Cols != w.Cols {
				return nil, fmt.Errorf("Model dosyası bozuk (%s): %d. katman sapma boyutu %dx%d, 1x%d bekleniyordu", path, i+1, b.Rows, b.Cols, w.Cols)
			}
			nn.Weights[i], nn.Biases[i] = w, b
		}
		return nn, nil
	})

	// --- Yerleşik Web/API İstemcisi (web_al, web_gönder) ---
	v.RegisterBuiltin("web_al", "http_al", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("web_al(url) 1 parametre bekler")
		}
		url := args[0].Inspect()
		client := &http.Client{Timeout: 10 * time.Second}
		var body []byte
		var netErr error
		makine.Blocking(func() { body, netErr = httpFetch(client.Get(url)) })
		if netErr != nil {
			return nil, fmt.Errorf("Web bağlantı hatası (%s): %v", url, netErr)
		}
		if val, err := vm.JSONToTR(body); err == nil {
			return val, nil
		}
		return &vm.StringVal{Val: string(body)}, nil
	})

	v.RegisterBuiltin("web_gönder", "web_gonder", func(makine *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("web_gönder(url, veri) 2 parametre bekler")
		}
		url := args[0].Inspect()
		payloadBytes, err := json.Marshal(vm.TRToGoValue(args[1]))
		if err != nil {
			return nil, err
		}
		client := &http.Client{Timeout: 10 * time.Second}
		var body []byte
		var netErr error
		// Ağ beklemesi sırasında yorumlayıcı kilidini bırak (aynı programın kendi sunucusuna istek atabilmesi için)
		makine.Blocking(func() { body, netErr = httpFetch(client.Post(url, "application/json", bytes.NewReader(payloadBytes))) })
		if netErr != nil {
			return nil, fmt.Errorf("Web gönderim hatası (%s): %v", url, netErr)
		}
		if val, err := vm.JSONToTR(body); err == nil {
			return val, nil
		}
		return &vm.StringVal{Val: string(body)}, nil
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
