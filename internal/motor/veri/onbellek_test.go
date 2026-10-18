package veri_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/veri"
)

// Çok büyük kota_gb taşıp "pozitif olmalı" demek yerine açık hata verir.
func TestKotaCokBuyuk(t *testing.T) {
	d := kumeKur(t)
	ob := filepath.Join(t.TempDir(), "onb")
	_, _, err := calistir(t, `veri_kümesi("`+tr(d)+`", önbellek: "`+tr(ob)+`", kota_gb: 1e10)`)
	if err == nil || !strings.Contains(err.Error(), "kota_gb çok büyük") {
		t.Fatalf("çok büyük kota hatası bekleniyordu: %v", err)
	}
}

// önbellek: yolu da tek yol kuralıyla çözülür (16.8): çalışma klasöründe
// yoksa programın klasörüne göre.
func TestOnbellekYoluProgramKlasorunden(t *testing.T) {
	d := kumeKur(t)
	program := t.TempDir()
	if err := os.Mkdir(filepath.Join(program, "onb_16_8"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := ayristir.New(sozcuk.New(`veri_kümesi("` + tr(d) + `", önbellek: "onb_16_8/kume", kota_gb: 0.001).örnekleyici(toplu: 2, bağlam: 8).sonraki()`))
	prog := p.ParseProgram()
	m := yurut.New()
	m.SourceFile = filepath.Join(program, "egit.tr")
	m.Out = &bytes.Buffer{}
	if _, err := m.Run(prog); err != nil {
		t.Fatal(err)
	}
	m.WaitAsync()
	veri.TumunuKapat(time.Second)
	if _, err := os.Stat("onb_16_8"); err == nil {
		_ = os.RemoveAll("onb_16_8")
		t.Fatal("önbellek çalışma klasörüne açıldı")
	}
	girdiler, err := os.ReadDir(filepath.Join(program, "onb_16_8", "kume"))
	if err != nil || len(girdiler) == 0 {
		t.Fatalf("önbellek programın klasöründe açılmadı: %v %d", err, len(girdiler))
	}
}

// Aynı önbellek klasörünü paylaşan iki küme tek kotayı paylaşır: klasör
// kotayı aşmaz; ikisine birden yetmeyen kota açık hatayla bildirilir
// (eskiden her küme kotayı ayrı sayıyor, klasör kotanın iki katına çıkıyordu).
func TestPaylasilanOnbellekKotasi(t *testing.T) {
	a, b := kumeKur(t), kumeKur(t)
	program := func(ob string, kota int64) string {
		return `
değişken K = ` + strconv.FormatInt(kota, 10) + ` / 1073741824
değişken a = veri_kümesi("` + tr(a) + `", önbellek: "` + tr(ob) + `", kota_gb: K)
değişken b = veri_kümesi("` + tr(b) + `", önbellek: "` + tr(ob) + `", kota_gb: K)
değişken öa = a.örnekleyici(toplu: 1, bağlam: 8, tohum: 1)
değişken öb = b.örnekleyici(toplu: 1, bağlam: 8, tohum: 1)
döngü i içinde 0..50 {
    öa.sonraki()
    öb.sonraki()
}
yazdır("bitti")
`
	}
	klasorBoyu := func(ob string) int64 {
		var n int64
		girdiler, _ := os.ReadDir(ob)
		for _, g := range girdiler {
			if bilgi, err := g.Info(); err == nil && strings.Contains(g.Name(), ".onb") {
				n += bilgi.Size()
			}
		}
		return n
	}
	// Bir küme 2 parça × 2128 bayt tutar: 6000 bayt birine yeter, ikisine yetmez.
	ob := filepath.Join(t.TempDir(), "onb")
	_, _, err := calistir(t, program(ob, 6000))
	if n := klasorBoyu(ob); n > 6000 {
		t.Fatalf("paylaşılan önbellek kotayı aştı: %d > 6000 bayt", n)
	}
	if err == nil || !strings.Contains(err.Error(), "paylaşan") {
		t.Fatalf("paylaşılan kota hatası bekleniyordu: %v", err)
	}
	veri.TumunuKapat(time.Second)
	// İkisine yeten kotayla sorunsuz.
	ob2 := filepath.Join(t.TempDir(), "onb")
	out, _ := mustCalistir(t, program(ob2, 9000))
	if strings.TrimSpace(out) != "bitti" {
		t.Fatalf("çıktı %q", out)
	}
	if n := klasorBoyu(ob2); n > 9000 {
		t.Fatalf("paylaşılan önbellek kotayı aştı: %d > 9000 bayt", n)
	}
	veri.TumunuKapat(time.Second)
}
