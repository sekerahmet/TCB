package ml

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

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
// [2, 0], [1048576, 1, 0] ve [1048576, 0, 8192] şekilli tensörler bildirebiliyor, examples/05
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
	_, err = run(t, `değişken p = tensör_yükle("`+filepath.ToSlash(fpath)+`")`)
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
	output, err := run(t, `değişken p = tensör_yükle("`+filepath.ToSlash(fpath)+`")
dene {
    p["A"] @ p["B"]
    yazdır("hata bekleniyordu")
} yakala h {
    yazdır(h)
}`)
	if err != nil {
		t.Fatalf("hata yakalanmalıydı: %v", err)
	}
	for _, p := range []string{"matris çarpımı: sonuç tensörü [46000000, 46000000] şeklinde", "PB bellek ister", "bu makinede"} {
		if !strings.Contains(output, p) {
			t.Fatalf("mesajda %q yok: %s", p, output)
		}
	}
}
