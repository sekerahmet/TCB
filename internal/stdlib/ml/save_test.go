package ml

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestTensorSaveLoad(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.ToSlash(filepath.Join(dir, "model.trt"))
	out := mustRun(t, `TR_var p = {"gömme": TR_tensor([[0.1, 0.2], [0.3, 0.4]]), "sapma": TR_tensor([1.5]), "ölçek": TR_sum(TR_tensor([2.0, 3.0])), "boş": TR_zeros([0, 3])}
TR_save_tensors("`+fpath+`", p)
TR_var y = TR_load_tensors("`+fpath+`")
TR_print(y.TR_keys())
TR_print(y["gömme"].TR_tolist, y["sapma"].TR_tolist, y["ölçek"].TR_tolist, y["boş"].TR_shape)
TR_print(y["gömme"].TR_tolist[0][0] == 0.1, y["sapma"].TR_grad)
TR_var z = TR_load_tensors("`+fpath+`", TR_requires_grad: TR_true)
TR_backward(TR_sum(z["sapma"] * 2))
TR_print(z["sapma"].TR_grad.TR_tolist)`)
	want := "[\"gömme\", \"sapma\", \"ölçek\", \"boş\"]\n" +
		"[[0.1, 0.2], [0.3, 0.4]] [1.5] 5.0 [0, 3]\n" +
		"true none\n" +
		"[2.0]\n"
	if out != want {
		t.Fatalf("kaydet/yükle:\n%q\nbeklenen:\n%q", out, want)
	}

	// Biçim: başlık, sürüm ve tür alanı yerinde.
	data, err := os.ReadFile(filepath.FromSlash(fpath))
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:8]) != "TRTENSOR" || binary.LittleEndian.Uint16(data[8:10]) != 2 || binary.LittleEndian.Uint32(data[10:14]) != 4 {
		t.Fatalf("başlık hatalı: % x", data[:14])
	}

	write := func(ad string, b []byte) string {
		p := filepath.Join(dir, ad)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(p)
	}
	broken := append([]byte{}, data...)
	broken[len(broken)-10] ^= 0xff
	version := append([]byte{}, data...)
	version[8] = 9
	dtype := append([]byte{}, data...)
	// İlk girdinin tür baytı: başlık (14) + ad uzunluğu (4) + "gömme" (6 bayt).
	dtype[14+4+len("gömme")] = 7
	cases := []struct {
		fpath string
		want  messages.Code
	}{
		{write("eksik.trt", data[:len(data)-20]), messages.H6601},
		{write("bozuk.trt", broken), messages.H6627},
		{write("baslik.trt", []byte("merhaba dünya, bu bir tensör dosyası değil")), messages.H6610},
		{write("kisa.trt", []byte("TRT")), messages.H6610},
		{write("surum.trt", version), messages.H6611},
		{write("tur.trt", dtype), messages.H6616},
		{write("fazla.trt", append(append([]byte{}, data...), 0)), messages.H6628},
		{filepath.ToSlash(filepath.Join(dir, "TR_none.trt")), messages.H2107},
		{filepath.ToSlash(dir), messages.H6629},
	}
	for _, d := range cases {
		_, err := run(t, `TR_load_tensors("`+d.fpath+`")`)
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", filepath.Base(d.fpath), err, d.want)
		}
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`TR_save_tensors("` + fpath + `", {"a": 1})`, messages.H6608},
		{`TR_save_tensors("` + fpath + `", {1: TR_tensor([1])})`, messages.H6606},
		{`TR_save_tensors("` + fpath + `", [TR_tensor([1])])`, messages.H6605},
		{`TR_save_tensors("", {})`, messages.H6603},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// fileBytes girdileri tensör dosyası baytlarına çevirir (bulanıklaştırma tohumları için).
func fileBytes(tb testing.TB, inputs []fileEntry) []byte {
	tb.Helper()
	var b bytes.Buffer
	if err := writeEntries(&b, inputs); err != nil {
		tb.Fatal(err)
	}
	return b.Bytes()
}

// Güvenlik denetimi (2026-10-18), bulgu: sıfır uzunluklu bir eksen tensör dosyasındaki
// eleman sınırını (interp.MaxCollectionSize) devre dışı bırakıyordu. 126 baytlık bir dosya
// [2, 0], [1048576, 1, 0] ve [1048576, 0, 8192] şekilli tensörler bildirebiliyor, examples/tr/30_xor_agi.tr
// tarzı bir ileri geçiş (X @ W1 + b1 ... @ W2) 137 GB ayırmaya çalışıp süreci
// kurtarılamaz biçimde ("fatal error: out of memory") çökertiyordu.
func TestSaveZeroAxisLimit(t *testing.T) {
	bad := fileBytes(t, []fileEntry{
		{ad: "W1", shape: []int{2, 0}, dtype: tensor.Float32, data: []float32{}},
		{ad: "b1", shape: []int{1 << 20, 1, 0}, dtype: tensor.Float32, data: []float32{}},
		{ad: "W2", shape: []int{1 << 20, 0, 8192}, dtype: tensor.Float32, data: []float32{}},
	})
	if _, err := decodeFile(bytes.NewReader(bad), int64(len(bad))); err == nil || !messages.Has(err, messages.H6621) {
		t.Fatalf("sıfır eksenle dev şekil reddedilmeliydi: %v", err)
	}

	// Sınırın altındaki sıfır eksenli tensörler (ör. boş tampon) okunmaya devam eder.
	good := fileBytes(t, []fileEntry{
		{ad: "boş", shape: []int{0, 3}, dtype: tensor.Float32, data: []float32{}},
		{ad: "sınırda", shape: []int{0, 5000, 10000}, dtype: tensor.BFloat16, data: []float32{}},
	})
	g, err := decodeFile(bytes.NewReader(good), int64(len(good)))
	if err != nil || len(g) != 2 || g[1].shape[2] != 10000 {
		t.Fatalf("sınır içindeki sıfır eksenli tensörler okunmalı: %v %+v", err, g)
	}

	// Kullanıcının gördüğü hata: tensör_yükle açık bir hata verir, süreç çökmez.
	fpath := filepath.Join(t.TempDir(), "kotu.trt")
	if err := os.WriteFile(fpath, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = run(t, `TR_var p = TR_load_tensors("`+filepath.ToSlash(fpath)+`")`)
	if err == nil || !messages.Has(err, messages.H6621) {
		t.Fatalf("tensör_yükle açık bir hata vermeli: %v", err)
	}
}

// Güvenlik denetimi Y2: dosyanın bildirdiği şekiller tek tek sınır içinde olsa da
// işlemlerin çıktısı fiziksel belleği aşabilir ([46000000, 0] @ [0, 46000000] verisiz iki
// tensörden yaklaşık 7,5 PB ister). Çekirdek ayırmadan önce denetler; hata dene/yakala ile
// yakalanır, süreç çökmez.
func TestSavedShapesOutputLimited(t *testing.T) {
	file := fileBytes(t, []fileEntry{
		{ad: "A", shape: []int{46000000, 0}, dtype: tensor.Float32, data: []float32{}},
		{ad: "B", shape: []int{0, 46000000}, dtype: tensor.Float32, data: []float32{}},
	})
	fpath := filepath.Join(t.TempDir(), "sekiller.trt")
	if err := os.WriteFile(fpath, file, 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := run(t, `TR_var p = TR_load_tensors("`+filepath.ToSlash(fpath)+`")
TR_try {
    p["A"] @ p["B"]
    TR_print("hata bekleniyordu")
} TR_catch h {
    TR_print(h)
}`)
	if err != nil {
		t.Fatalf("hata yakalanmalıydı: %v", err)
	}
	for _, p := range []string{"matris çarpımı: the result tensor of shape [46000000, 46000000]", "PB of memory", "this machine"} {
		if !strings.Contains(output, p) {
			t.Fatalf("mesajda %q yok: %s", p, output)
		}
	}
}
