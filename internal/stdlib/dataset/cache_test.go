package dataset_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/stdlib/dataset"
)

// Çok büyük kota_gb taşıp "pozitif olmalı" demek yerine açık hata verir.
func TestQuotaTooLarge(t *testing.T) {
	d := buildDataset(t)
	cache := filepath.Join(t.TempDir(), "onb")
	_, _, err := run(t, `veri_kümesi("`+trPath(d)+`", önbellek: "`+trPath(cache)+`", kota_gb: 1e10)`)
	if !messages.Has(err, messages.H5512) {
		t.Fatalf("çok büyük kota hatası bekleniyordu: %v", err)
	}
}

// önbellek: yolu da tek yol kuralıyla çözülür (16.8): çalışma klasöründe
// yoksa programın klasörüne göre.
func TestCachePathFromProgramDir(t *testing.T) {
	d := buildDataset(t)
	program := t.TempDir()
	if err := os.Mkdir(filepath.Join(program, "onb_16_8"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(`veri_kümesi("` + trPath(d) + `", önbellek: "onb_16_8/kume", kota_gb: 0.001).örnekleyici(toplu: 2, bağlam: 8).sonraki()`))
	prog := p.ParseProgram()
	m := interp.New()
	m.SourceFile = filepath.Join(program, "egit.tr")
	m.Out = &bytes.Buffer{}
	if _, err := m.Run(prog); err != nil {
		t.Fatal(err)
	}
	m.WaitAsync()
	dataset.CloseAll(time.Second)
	if _, err := os.Stat("onb_16_8"); err == nil {
		_ = os.RemoveAll("onb_16_8")
		t.Fatal("önbellek çalışma klasörüne açıldı")
	}
	inputs, err := os.ReadDir(filepath.Join(program, "onb_16_8", "kume"))
	if err != nil || len(inputs) == 0 {
		t.Fatalf("önbellek programın klasöründe açılmadı: %v %d", err, len(inputs))
	}
}

// Aynı önbellek klasörünü paylaşan iki küme tek kotayı paylaşır: klasör
// kotayı aşmaz; ikisine birden yetmeyen kota açık hatayla bildirilir
// (eskiden her küme kotayı ayrı sayıyor, klasör kotanın iki katına çıkıyordu).
func TestSharedCacheQuota(t *testing.T) {
	a, b := buildDataset(t), buildDataset(t)
	program := func(cache string, quota int64) string {
		return `
değişken K = ` + strconv.FormatInt(quota, 10) + ` / 1073741824
değişken a = veri_kümesi("` + trPath(a) + `", önbellek: "` + trPath(cache) + `", kota_gb: K)
değişken b = veri_kümesi("` + trPath(b) + `", önbellek: "` + trPath(cache) + `", kota_gb: K)
değişken öa = a.örnekleyici(toplu: 1, bağlam: 8, tohum: 1)
değişken öb = b.örnekleyici(toplu: 1, bağlam: 8, tohum: 1)
döngü i içinde 0..50 {
    öa.sonraki()
    öb.sonraki()
}
yazdır("bitti")
`
	}
	dirSize := func(cache string) int64 {
		var n int64
		inputs, _ := os.ReadDir(cache)
		for _, g := range inputs {
			if info, err := g.Info(); err == nil && strings.Contains(g.Name(), ".onb") {
				n += info.Size()
			}
		}
		return n
	}
	// Bir küme 2 parça × 2128 bayt tutar: 6000 bayt birine yeter, ikisine yetmez.
	cache := filepath.Join(t.TempDir(), "onb")
	_, _, err := run(t, program(cache, 6000))
	if n := dirSize(cache); n > 6000 {
		t.Fatalf("paylaşılan önbellek kotayı aştı: %d > 6000 bayt", n)
	}
	if !messages.Has(err, messages.H5515) || !strings.Contains(err.Error(), "paylaşan") {
		t.Fatalf("paylaşılan kota hatası bekleniyordu: %v", err)
	}
	dataset.CloseAll(time.Second)
	// İkisine yeten kotayla sorunsuz.
	cache2 := filepath.Join(t.TempDir(), "onb")
	out, _ := mustRun(t, program(cache2, 9000))
	if strings.TrimSpace(out) != "bitti" {
		t.Fatalf("çıktı %q", out)
	}
	if n := dirSize(cache2); n > 9000 {
		t.Fatalf("paylaşılan önbellek kotayı aştı: %d > 9000 bayt", n)
	}
	dataset.CloseAll(time.Second)
}
