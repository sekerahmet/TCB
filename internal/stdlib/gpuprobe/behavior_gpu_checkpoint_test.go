package gpuprobe_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/stdlib"
)

// GPU'da kontrol noktası: parametreler ve iyileştirici durumları (momentler, adım
// sayaçları) cihazdadır; kayıtta konağa indirilir, yüklemede aynı cihaza konur ve
// parametreleri_yükle iyileştiricinin tuttuğu cihaz tensörlerine yazar. Kesintisiz GPU eğitimi
// ile 3. adımda kaydedilip yeni nesnelerle sürdürülen GPU eğitimi bit düzeyinde aynıdır. Gerçek
// GPU yoksa PTX benzetimidir (yavaş; şekiller küçük).
const gpuCheckpointProgram = `
varsayılan_cihaz("gpu")
değişken KOŞU = %q
fonksiyon eğit(son) {
    değişken sayaç = 0
    fonksiyon ağırlık(şekil) {
        sayaç += 1
        sonuç tensör(türevsiz(rastgele_normal(şekil, tohum: sayaç) * 0.2), türev_izle: doğru)
    }
    değişken model = {E: ağırlık([16, 8]), W: ağırlık([8, 16])}
    değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.01)
    değişken izlenen = {kayıplar: []}
    değişken kn = kontrol_noktası(KOŞU + "/" + metin(son), {model: model, op: op, izlenen: izlenen}, her_dakika: 0)
    değişken girdiler = [[[3, 1, 4, 1], [5, 9, 2, 6]], [[5, 3, 5, 8], [9, 7, 9, 3]], [[2, 3, 8, 4], [6, 2, 6, 4]]]
    değişken hedefler = [[1, 4, 1, 5, 9, 2, 6, 5], [3, 5, 8, 9, 7, 9, 3, 2], [3, 8, 4, 6, 2, 6, 4, 3]]
    döngü adım içinde kn.kaldığı_adım..son {
        op.sıfırla()
        değişken kayıp = çapraz_entropi(gömme(model.E, tensör(girdiler[adım %% 3])).şekillendir([8, 8]) @ model.W, tensör(hedefler[adım %% 3]))
        geri_yayılım(kayıp)
        op.adım()
        izlenen.kayıplar.ekle(kayıp.değer)
        kn.güncelle(adım + 1)
    }
    sonuç [model, op, izlenen, kn]
}
`

func runOnGPU(t *testing.T, code string) string {
	t.Helper()
	p := parser.New(source.TextLexer(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	var out bytes.Buffer
	m.Out, m.Err = &out, &out
	if _, err := m.Run(prog); err != nil {
		t.Fatalf("program hatası: %v\nçıktı:\n%s", err, out.String())
	}
	m.WaitAsync()
	stdlib.Close()
	return out.String()
}

func TestGPUCheckpoint(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	run := filepath.ToSlash(t.TempDir())
	code := fmt.Sprintf(gpuCheckpointProgram, run) + `
TR_var a = eğit(6)
# "kesilen" koşu: 3 adım; sonra aynı koşu klasöründe başka tohumlu yeni nesnelerle 6. adıma
# kadar sürdürülür (yükleme yeni nesnelerin değerlerinin üzerine yazar).
TR_var b = eğit(3)
TR_func sürdür() {
    TR_var sayaç = 0
    TR_func ağırlık(şekil) {
        sayaç += 1
        TR_return TR_tensor(TR_no_grad(TR_randn(şekil, TR_seed: sayaç + 100) * 0.2), TR_requires_grad: TR_true)
    }
    TR_var model = {E: ağırlık([16, 8]), W: ağırlık([8, 16])}
    TR_var op = TR_AdamW(TR_parameters(model), TR_lr: 0.01)
    TR_var izlenen = {TR_losses: []}
    TR_var kn = TR_checkpoint(KOŞU + "/3", {model: model, op: op, izlenen: izlenen}, TR_every_minutes: 0)
    TR_var girdiler = [[[3, 1, 4, 1], [5, 9, 2, 6]], [[5, 3, 5, 8], [9, 7, 9, 3]], [[2, 3, 8, 4], [6, 2, 6, 4]]]
    TR_var hedefler = [[1, 4, 1, 5, 9, 2, 6, 5], [3, 5, 8, 9, 7, 9, 3, 2], [3, 8, 4, 6, 2, 6, 4, 3]]
    TR_print("cihaz:", model.E.TR_device, op.TR_state["m.0"].TR_device, kn.TR_resume_step)
    TR_for adım TR_in kn.TR_resume_step..6 {
        op.TR_zero_grad()
        TR_var kayıp = TR_cross_entropy(TR_embedding(model.E, TR_tensor(girdiler[adım % 3])).TR_reshape([8, 8]) @ model.W, TR_tensor(hedefler[adım % 3]))
        TR_backward(kayıp)
        op.TR_step()
        izlenen.TR_losses.TR_append(kayıp.TR_tolist)
        kn.TR_update(adım + 1)
    }
    TR_return [model, op, izlenen]
}
TR_var c = sürdür()
TR_print("parametre:", a[0].E.TR_tolist == c[0].E.TR_tolist, a[0].W.TR_tolist == c[0].W.TR_tolist)
TR_var ayni = TR_true
TR_for ad TR_in a[1].TR_state.TR_keys() {
    TR_if a[1].TR_state[ad].TR_tolist != c[1].TR_state[ad].TR_tolist { ayni = TR_false }
}
TR_print("durum:", ayni, a[1].TR_state.TR_keys() == c[1].TR_state.TR_keys())
TR_print("kayıp:", a[2].TR_losses == c[2].TR_losses, c[2].TR_losses.TR_length)
# parametreleri_yükle CPU'daki tensörleri iyileştiricinin tuttuğu GPU tensörüne yazar.
TR_var p = {w: TR_tensor(TR_ones([4]), TR_requires_grad: TR_true)}
TR_var sgd = TR_SGD(TR_parameters(p), TR_lr: 0.5)
TR_load_parameters(p, {w: TR_tensor([1.0, 2.0, 3.0, 4.0]).TR_to_device("cpu")})
TR_backward(TR_sum(p.w))
sgd.TR_step()
TR_print("yükle:", p.w.TR_device, p.w.TR_to_device("cpu").TR_tolist)
`
	output := runOnGPU(t, code)
	for _, want := range []string{"cihaz: gpu gpu 3", "parametre: doğru doğru", "durum: doğru doğru", "kayıp: doğru 6", "yükle: gpu [0.5, 1.5, 2.5, 3.5]"} {
		if !strings.Contains(output, want) {
			t.Fatalf("%q bekleniyordu:\n%s", want, output)
		}
	}
}
