package logging_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Terminal değilken: ara satırlar en çok 10 saniyede bir, sonda özet satırı. Hız ve kalan
// süre ölçülür; günlüğe her güncelleme ve bitiş yazılır.
func TestProgressPlainLinesAndLog(t *testing.T) {
	s := setClock(t)
	k := t.TempDir()
	path := filepath.Join(k, "i.jsonl")
	m, w := newVM(t, k)
	if _, err := run(t, m, `değişken g = günlük("`+pathText(path)+`")
değişken ç = ilerleme(100, günlük: g)`); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 100; i++ {
		s.advance(200 * time.Millisecond) // saniyede 5 adım
		if _, err := run(t, m, `ç.güncelle({kayıp: 4.0 / `+strconv.Itoa(i)+`, oran: 0.0003})`); err != nil {
			t.Fatal(err)
		}
	}
	output := w.String()
	line := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	// 20 saniyede 100 adım: 10. ve 20. saniyede ara satır (20.'de çubuk bitti: yalnızca özet)
	if len(line) != 2 {
		t.Fatalf("1 ara satır ve özet bekleniyordu:\n%s", output)
	}
	if want := "adım 50/100 ▕████████░░░░░░░░▏ %50 · kayıp 0,08 · oran 3e-4 · 5 adım/sn · kalan 10 sn"; line[0] != want {
		t.Errorf("ara satır:\n%q\nbeklenen\n%q", line[0], want)
	}
	if want := "adım 100/100 · kayıp 0,04 · oran 3e-4 · 20 sn sürdü"; line[1] != want {
		t.Errorf("özet satırı:\n%q\nbeklenen\n%q", line[1], want)
	}
	records := lines(t, path)
	if len(records) != 102 || records[0]["total"] != 100.0 {
		t.Fatalf("başlangıç + 100 ölçü + bitiş bekleniyordu: %d kayıt, %v", len(records), records[0])
	}
	if o := records[50]; o["step"] != 50.0 || o["loss"] != 0.08 || o["rate"] != 5.0 {
		t.Errorf("ölçü kaydı: %v", o)
	}
	if b := records[101]; b["kind"] != "end" || b["step"] != 100.0 || b["seconds"] != 20.0 {
		t.Errorf("bitiş kaydı: %v", b)
	}
	// Bitmiş çubuk güncellenemez; durdur yanlış döner.
	if _, err := run(t, m, `ç.güncelle()`); !messages.Has(err, messages.H7316) {
		t.Errorf("bitmiş çubukta hata bekleniyordu: %v", err)
	}
	if v, _ := run(t, m, `ç.durdur()`); v != interp.False {
		t.Errorf("ikinci durdur yanlış dönmeli: %v", v)
	}
}

// adım: kaldığı yerden sürdürür; toplamdan büyük olamaz; ayrılmış ölçü adları reddedilir.
func TestProgressStepAndErrors(t *testing.T) {
	setClock(t)
	m, _ := newVM(t, t.TempDir())
	v, err := run(t, m, `TR_var ç = TR_progress(10)
ç.TR_update(TR_step: 7)
ç.TR_update()
ç`)
	if err != nil || v.Inspect() != "İlerleme(adım 8/10)" {
		t.Fatalf("adım: ile sürdürme: %v %v", v, err)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`ç.güncelle(adım: 11)`, messages.H7317},
		{`ç.güncelle(adım: -1)`, messages.H7315},
		{`ç.güncelle({adım: 3})`, messages.H7321},
		{`ç.güncelle({"rate": 3})`, messages.H7321},
		{`TR_progress(0)`, messages.H7313},
		{`TR_progress(10, TR_log: 5)`, messages.H7314},
		{`ç.durdur(1)`, messages.H2012},
	} {
		if _, err := run(t, m, d.code); !messages.Has(err, d.want) {
			t.Errorf("%s: %s hatası bekleniyordu: %v", d.code, d.want, err)
		}
	}
	// Argüman tipi hatası henüz kodlu değil (interp).
	if _, err := run(t, m, `ç.güncelle(5)`); err == nil || !strings.Contains(err.Error(), "1. argüman olarak TR_Dict bekler") {
		t.Errorf("argüman tipi hatası bekleniyordu: %v", err)
	}
	// Toplamsız sayaç: kalan süre yerine geçen süre.
	if v, err := run(t, m, `TR_var s = TR_progress()
s.TR_update(); s.TR_update(); s`); err != nil || v.Inspect() != "İlerleme(adım 2)" {
		t.Errorf("sayaç: %v %v", v, err)
	}
}

// Terminalde çubuk canlı satırdır: satır başından yeniden çizilir (en çok saniyede 10 kez),
// yazdır çıktısı çubuğu silip yazar ve çubuk yeniden çizilir; bitince satır özete dönüşür.
func TestProgressTerminal(t *testing.T) {
	t.Cleanup(logging.FakeTerminal(100))
	s := setClock(t)
	m, w := newVM(t, t.TempDir())
	if _, err := run(t, m, `TR_var ç = TR_progress(4)`); err != nil {
		t.Fatal(err)
	}
	step := func(code string) {
		t.Helper()
		if _, err := run(t, m, code); err != nil {
			t.Fatal(err)
		}
	}
	step(`ç.güncelle({kayıp: 2.0})`) // çizilir
	step(`ç.güncelle({kayıp: 1.5})`) // 100 ms dolmadı: çizilmez
	step(`TR_print("ara")`)          // sil, yaz, tazelenmiş satırı yeniden çiz
	s.advance(time.Second)
	step(`ç.güncelle({kayıp: 1.0})`) // çizilir; hız 1 saniyede 3 adım
	step(`ç.güncelle({kayıp: 0.5})`) // bitti: özet
	length := func(s string) int { return len([]rune(s)) }
	c1 := "adım 1/4 ▕████░░░░░░░░░░░░▏ %25 · kayıp 2"
	c2 := "adım 2/4 ▕████████░░░░░░░░▏ %50 · kayıp 1,5"
	c3 := "adım 3/4 ▕████████████░░░░▏ %75 · kayıp 1 · 3 adım/sn · kalan 1 sn"
	last := "adım 4/4 · kayıp 0,5 · 1 sn sürdü"
	want := "\r" + c1 + "\r" + strings.Repeat(" ", length(c1)) + "\r" + "ara\n" + "\r" + c2 +
		"\r" + c3 + "\r" + last + strings.Repeat(" ", length(c3)-length(last)) + "\n"
	if got := w.String(); got != want {
		t.Errorf("terminal çıktısı:\n%q\nbeklenen\n%q", got, want)
	}
	// Çubuk bitince canlı satır kalkar: yazdır doğrudan yazar.
	step(`TR_print("son")`)
	if !strings.HasSuffix(w.String(), "\nson\n") {
		t.Errorf("çubuk bittikten sonra yazdır doğrudan yazmalı: %q", w.String())
	}
}

// Adım sınırı kancası her güncellemede tamamlanan adımla çağrılır.
func TestStepHook(t *testing.T) {
	m, _ := newVM(t, t.TempDir())
	var steps []int64
	logging.SetStepHook(func(a logging.StepBoundary) {
		if a.VM == nil || a.Total != 3 {
			t.Errorf("kanca bilgisi: %+v", a)
		}
		steps = append(steps, a.Step)
	})
	t.Cleanup(func() { logging.SetStepHook(nil) })
	if _, err := run(t, m, `TR_var ç = TR_progress(3)
TR_for i TR_in 0..3 { ç.TR_update() }`); err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 || steps[0] != 1 || steps[2] != 3 {
		t.Errorf("kanca adımları: %v", steps)
	}
}

// Arka plan işinde (TR_IS_KLASORU) çubuk iş klasörüne progress.json yazar.
func TestJobStatus(t *testing.T) {
	job := t.TempDir()
	t.Setenv("TR_IS_KLASORU", job)
	t.Cleanup(logging.FakeTerminal(60)) // iş kipinde terminal kullanılmaz
	m, w := newVM(t, t.TempDir())
	if _, err := run(t, m, `TR_var ç = TR_progress(2)
ç.TR_update({TR_loss: 0.25})`); err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(mustRead(t, filepath.Join(job, "progress.json")), &d); err != nil {
		t.Fatal(err)
	}
	if d["step"] != 1.0 || d["total"] != 2.0 || d["done"] != false || d["metrics"].(map[string]any)["loss"] != 0.25 {
		t.Errorf("progress.json: %v", d)
	}
	if _, err := run(t, m, `ç.güncelle()`); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustRead(t, filepath.Join(job, "progress.json")), &d); err != nil || d["done"] != true || d["step"] != 2.0 {
		t.Errorf("son progress.json: %v %v", d, err)
	}
	if strings.Contains(w.String(), "\r") {
		t.Errorf("iş kipinde canlı satır olmamalı: %q", w.String())
	}
	if remaining, _ := reporoot.Glob(filepath.Join(job, ".progress.json.*")); len(remaining) > 0 {
		t.Errorf("geçici dosya kalmamalı: %v", remaining)
	}
	_ = os.Remove(filepath.Join(job, "progress.json"))
}

// Program bitince bitmemiş çubuk özet satırını yazar.
func TestCloseAllFinishesBar(t *testing.T) {
	setClock(t)
	m, w := newVM(t, t.TempDir())
	if _, err := run(t, m, `TR_var ç = TR_progress(10)
ç.TR_update({TR_loss: 3.25})`); err != nil {
		t.Fatal(err)
	}
	logging.CloseAll()
	if got := w.String(); got != "adım 1/10 · kayıp 3,25 · 0 sn sürdü\n" {
		t.Errorf("özet satırı: %q", got)
	}
}

// Her 10 adımda bir (adım: ile) güncellenen çubuğun hızı ölçülür; aynı adımın ikinci
// güncellemesi (doğrulama kaybı) hızı bozmaz. Eskiden her ileri atlama "sıçrama" sayılıp hız
// hep 0 kalıyordu.
func TestProgressRateWithStepGaps(t *testing.T) {
	s := setClock(t)
	k := t.TempDir()
	path := filepath.Join(k, "i.jsonl")
	m, _ := newVM(t, k)
	if _, err := run(t, m, `değişken g = günlük("`+pathText(path)+`")
değişken ç = ilerleme(100, günlük: g)`); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		s.advance(2 * time.Second) // 2 saniyede 10 adım: saniyede 5
		if _, err := run(t, m, `ç.güncelle({kayıp: 1.0}, adım: `+strconv.Itoa(10*i)+`)`); err != nil {
			t.Fatal(err)
		}
		if _, err := run(t, m, `ç.güncelle({doğrulama: 2.0}, adım: `+strconv.Itoa(10*i)+`)`); err != nil {
			t.Fatal(err)
		}
	}
	records := lines(t, path)
	last := records[len(records)-1]
	if last["step"] != 50.0 || last["rate"] != 5.0 {
		t.Fatalf("hız ölçülmedi: %v", last)
	}
}

// İç içe çubuklar: iç çubuk görünürken dış çubuk güncellenirse, iç çubuk bitince dış çubuk
// eski değil güncel satırıyla yeniden çizilir.
func TestNestedProgressRedrawsOuter(t *testing.T) {
	t.Cleanup(logging.FakeTerminal(100))
	s := setClock(t)
	m, w := newVM(t, t.TempDir())
	for _, code := range []string{
		`TR_var dış = TR_progress(3)`,
		`dış.güncelle({kayıp: 9.0})`,
		`TR_var iç = TR_progress(2)`,
		`iç.güncelle()`,
		`dış.güncelle({kayıp: 7.0})`, // iç çubuk görünürken: yalnızca işaretlenir
		`iç.güncelle()`,              // iç biter: dış yeniden çizilir
	} {
		s.advance(time.Second)
		if _, err := run(t, m, code); err != nil {
			t.Fatal(err)
		}
	}
	out := w.String()
	tail := out[strings.LastIndex(out, "\n")+1:]
	if !strings.Contains(tail, "adım 2/3") || !strings.Contains(tail, "kayıp 7") {
		t.Fatalf("dış çubuk güncel satırla çizilmeli; son satır %q\ntüm çıktı %q", tail, out)
	}
}
