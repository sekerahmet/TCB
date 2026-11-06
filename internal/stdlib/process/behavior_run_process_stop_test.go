package process_test

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// helperStop durdurmaya bakan testlerin programlarıdır.
func helperStop(args []string) {
	switch args[0] {
	case "nice": // hazır dosyası, sonra isteği görünce yazılacak dosya
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		writeMark(args[1])
		select {
		case <-c:
			writeMark(args[2])
		case <-time.After(20 * time.Second):
		}
	case "ignore": // isteği yakalar, bir şey yapmaz (Windows'ta signal.Ignore olayı varsayılana bırakır)
		signal.Notify(make(chan os.Signal, 1), os.Interrupt, syscall.SIGTERM)
		writeMark(args[1])
		time.Sleep(30 * time.Second)
	case "beat": // dosyaya atım yazar: dosya, atım sayısı
		n, _ := strconv.Atoi(args[2])
		for range n {
			f, err := os.OpenFile(args[1], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err == nil {
				_, _ = f.WriteString("x")
				_ = f.Close()
			}
			time.Sleep(20 * time.Millisecond)
		}
	case "family": // torun başlatır, kendisi uyur
		exe, _ := os.Executable()
		_ = exec.Command(exe, "beat", args[1], "2000").Start()
		time.Sleep(30 * time.Second)
	case "orphan": // boruyu tutan torun bırakıp hemen biter: dosya, atım sayısı, kod
		exe, _ := os.Executable()
		c := exec.Command(exe, "beat", args[1], args[2])
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		_ = c.Start()
		_, _ = os.Stdout.WriteString("ebeveyn çıktısı")
		n, _ := strconv.Atoi(args[3])
		os.Exit(n)
	case "escape": // gruptan kaçan, boruyu tutan torun başlatır, kendisi uyur
		exe, _ := os.Executable()
		c := exec.Command(exe, "beat", args[1], args[2])
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		leaveGroup(c)
		_ = c.Start()
		time.Sleep(30 * time.Second)
	case "touch-after": // uyur, sonra dosya yazar: dosya, saniye
		n, _ := strconv.ParseFloat(args[2], 64)
		time.Sleep(time.Duration(n * float64(time.Second)))
		writeMark(args[1])
	default:
		panic("bilinmeyen yardımcı: " + args[0])
	}
}

func writeMark(p string) { _ = os.WriteFile(p, []byte("x"), 0o644) }

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// await koşul doğru olana kadar bekler.
func await(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(15 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("beklenen olmadı: %s", what)
}

func size(p string) int64 {
	info, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return info.Size()
}

// stillBeating atım dosyasının büyüdüğünü söyler.
func stillBeating(p string) bool {
	before := size(p)
	time.Sleep(300 * time.Millisecond)
	return size(p) != before
}

// startProgram programı arka planda koşar; yorumlayıcıyı ve bitiş kanalını verir.
func startProgram(t *testing.T, code string, prepare ...func(*interp.VM)) (*interp.VM, <-chan error) {
	t.Helper()
	m := interp.New()
	m.Out, m.Err = io.Discard, io.Discard
	for _, f := range prepare {
		f(m)
	}
	p := parser.New(lexer.NewBody(code, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi: %v", p.Errors())
	}
	end := make(chan error, 1)
	go func() {
		_, err := m.Run(prog)
		end <- err
	}()
	return m, end
}

// finished programın bitmesini bekler.
func finished(t *testing.T, end <-chan error) error {
	t.Helper()
	select {
	case err := <-end:
		return err
	case <-time.After(20 * time.Second):
		t.Fatal("program bitmedi")
		return nil
	}
}

func tr(p string) string { return strconv.Quote(p) }

// Süre dolunca program durur ve zaman aşımı hatası gelir; ileti çağrının satırını söyler, görevden söz
// etmez; TR_error_code aynı kodu görür (21.4, 21.5, 10.7).
func TestTimeoutStopsTheProgram(t *testing.T) {
	prog := helperProgram(t)
	start := time.Now()
	_, err := run(t, "TR_var x = 1\nTR_run_process("+prog+`, ["sleep", "30"], TR_timeout: 0.1)`)
	if !messages.Has(err, messages.H2921) {
		t.Fatalf("H2921 bekleniyordu: %v", err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("%v sürdü", d)
	}
	if !strings.Contains(err.Error(), "satır 2") || strings.Contains(err.Error(), "görev") {
		t.Errorf("ileti: %v", err)
	}
	got, err := run(t, `TR_error_code { TR_run_process(`+prog+`, ["sleep", "30"], TR_timeout: 0.1) }`)
	if err != nil || got != "H2921" {
		t.Errorf("%s (%v)", got, err)
	}
}

// Süre g.TR_wait'teki gibi doğrulanır: aynı girdi aynı kodu verir, 0 hemen doldurur (21.5, 10.7).
func TestTimeoutIsCheckedLikeTaskWait(t *testing.T) {
	prog := helperProgram(t)
	for _, d := range []struct {
		limit string
		want  messages.Code
	}{
		{"-1", messages.H2433},
		{`"a"`, messages.H2433},
		{`TR_Float("nan")`, messages.H2433},
		{"1e18", messages.H2434},
		{"0", messages.H2921},
	} {
		if _, err := run(t, `TR_run_process(`+prog+`, ["sleep", "30"], TR_timeout: `+d.limit+`)`); !messages.Has(err, d.want) {
			t.Errorf("program, %s: %s bekleniyordu: %v", d.limit, d.want, err)
		}
		if _, err := run(t, `TR_var g = TR_async { TR_sleep(30) }
g.TR_wait(TR_timeout: `+d.limit+`)`); !messages.Has(err, d.want) {
			t.Errorf("görev, %s: %s bekleniyordu: %v", d.limit, d.want, err)
		}
	}
}

// Süresinde biten program süre sınırından etkilenmez (21.5).
func TestProgramEndingInTimeIsNotATimeout(t *testing.T) {
	prog := helperProgram(t)
	if _, err := run(t, `TR_run_process(`+prog+`, ["exit", "0"], TR_timeout: 30)`); err != nil {
		t.Error(err)
	}
	if _, err := run(t, `TR_run_process(`+prog+`, ["exit", "4"], TR_timeout: 30)`); !messages.Has(err, messages.H7503) {
		t.Errorf("H7503 bekleniyordu: %v", err)
	}
}

// Önce nazik istek gider: program onu görüp kendi bitirir (21.5). Konsolsuz Windows'ta istek gitmez,
// program öldürülür.
func TestNiceRequestComesFirst(t *testing.T) {
	prog := helperProgram(t)
	dir := t.TempDir()
	ready, got := filepath.Join(dir, "hazır"), filepath.Join(dir, "gördü")
	m, end := startProgram(t, `TR_run_process(`+prog+`, ["nice", `+tr(ready)+`, `+tr(got)+`])`)
	await(t, "program hazır", func() bool { return exists(ready) })
	start := time.Now()
	m.RequestStop(2)
	if err := finished(t, end); !errors.Is(err, interp.ErrInterrupted) {
		t.Errorf("durdurma bekleniyordu: %v", err)
	}
	d := time.Since(start)
	t.Logf("istek görüldü: %v, süre %v", exists(got), d)
	if consoleAttached() && !exists(got) {
		t.Error("program isteği görmedi")
	}
	if d > 1900*time.Millisecond {
		t.Errorf("program hemen durmalıydı: %v", d)
	}
}

// İsteği yok sayan program 2 saniye sonra öldürülür (21.5).
func TestStubbornProgramIsKilled(t *testing.T) {
	prog := helperProgram(t)
	ready := filepath.Join(t.TempDir(), "hazır")
	m, end := startProgram(t, `TR_run_process(`+prog+`, ["ignore", `+tr(ready)+`])`)
	await(t, "program hazır", func() bool { return exists(ready) })
	start := time.Now()
	m.RequestStop(2)
	if err := finished(t, end); !errors.Is(err, interp.ErrInterrupted) {
		t.Errorf("durdurma bekleniyordu: %v", err)
	}
	d := time.Since(start)
	t.Logf("süre %v", d)
	if d > 8*time.Second {
		t.Errorf("%v sürdü", d)
	}
	switch {
	case consoleAttached() && d < 1900*time.Millisecond:
		t.Errorf("öldürmeden önce 2 saniye verilmeliydi: %v", d)
	case !consoleAttached() && d > 1900*time.Millisecond:
		t.Errorf("konsolsuz istek gitmez, hemen öldürülmeliydi: %v", d)
	}
}

// Konsolsuz TR'de Ctrl+Break gitmez: öldürme yolu hemen çalışır, 2 saniye boşa beklenmez (21.5). Test
// kendini konsolu olmayan bir süreçte yeniden koşar.
func TestNoConsoleUsesTheKillPath(t *testing.T) {
	if !windows {
		t.Skip("Ctrl+Break Windows'ta")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command(exe, "-test.run=^TestStubbornProgramIsKilled$", "-test.v")
	withoutConsole(c)
	out, err := c.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Errorf("konsolsuz koşu: %v", err)
	}
}

// Torun süreç de durur (21.5).
func TestGrandchildStopsWithTheProgram(t *testing.T) {
	prog := helperProgram(t)
	beat := filepath.Join(t.TempDir(), "atım")
	m, end := startProgram(t, `TR_run_process(`+prog+`, ["family", `+tr(beat)+`])`)
	await(t, "torun atıyor", func() bool { return size(beat) > 0 })
	m.RequestStop(2)
	if err := finished(t, end); !errors.Is(err, interp.ErrInterrupted) {
		t.Errorf("durdurma bekleniyordu: %v", err)
	}
	if stillBeating(beat) {
		t.Error("torun durmadı")
	}
}

// Durdurma Go defer'inde değil Blocking'in içinde olur: yığın panikle çözülse de program durmuştur,
// çağrıdan sonraki satır çalışmaz, ertelenen işler çalışır, TR_catch yakalamaz (1.5, 21.5).
func TestStopUnwindsAfterTheProgramStopped(t *testing.T) {
	prog := helperProgram(t)
	dir := t.TempDir()
	beat, after, deferred, caught := filepath.Join(dir, "atım"), filepath.Join(dir, "sonra"), filepath.Join(dir, "ertelenen"), filepath.Join(dir, "yakalandı")
	m, end := startProgram(t, `TR_defer { TR_write_file(`+tr(deferred)+`, "x") }
TR_try {
    TR_run_process(`+prog+`, ["beat", `+tr(beat)+`, "2000"])
    TR_write_file(`+tr(after)+`, "x")
} TR_catch e {
    TR_write_file(`+tr(caught)+`, "x")
}`)
	await(t, "program atıyor", func() bool { return size(beat) > 0 })
	m.RequestStop(2)
	if err := finished(t, end); !errors.Is(err, interp.ErrInterrupted) {
		t.Errorf("durdurma bekleniyordu: %v", err)
	}
	if stillBeating(beat) {
		t.Error("program durmadı")
	}
	if exists(after) || exists(caught) || !exists(deferred) {
		t.Errorf("sonraki satır %v, yakala %v, ertelenen %v", exists(after), exists(caught), exists(deferred))
	}
}

// TR_handle_stop altında çağrı sürer: istek programı durdurmaz, sonuç gelir (21.5, 1.6).
func TestHandledStopLetsTheCallFinish(t *testing.T) {
	prog := helperProgram(t)
	dir := t.TempDir()
	beat, after := filepath.Join(dir, "atım"), filepath.Join(dir, "sonra")
	m, end := startProgram(t, `TR_handle_stop()
TR_run_process(`+prog+`, ["beat", `+tr(beat)+`, "40"])
TR_write_file(`+tr(after)+`, "x")`)
	await(t, "program atıyor", func() bool { return size(beat) > 0 })
	m.RequestStop(2)
	if err := finished(t, end); err != nil {
		t.Errorf("çağrı sürmeliydi: %v", err)
	}
	if size(beat) != 40 || !exists(after) {
		t.Errorf("atım %d, sonraki satır %v", size(beat), exists(after))
	}
	// İstekten sonra başlayan çağrı da süren bir işi bitirir, "durdurma istendi" hatası vermez.
	got, err := run(t, `TR_handle_stop()
TR_run_process(`+prog+`, ["exit", "0"]).TR_length`)
	if err != nil || got != "0" {
		t.Errorf("%s (%v)", got, err)
	}
}

// Kontrol noktasının ertelemesi bekleme sürerken dolarsa 1.5 o anda uygulanır: program durdurulur
// (1.6, 21.5). Çağrı erteleme süresince sürer.
func TestDeferralExpiringDuringTheCallStopsIt(t *testing.T) {
	prog := helperProgram(t)
	beat := filepath.Join(t.TempDir(), "atım")
	window := 300 * time.Millisecond
	m, end := startProgram(t, `TR_run_process(`+prog+`, ["beat", `+tr(beat)+`, "2000"])`, func(m *interp.VM) {
		m.DeferStops(func() (time.Duration, bool) { return window, true })
	})
	await(t, "program atıyor", func() bool { return size(beat) > 0 })
	start := time.Now()
	if got := m.RequestStop(2); got != interp.StopDeferred {
		t.Fatalf("istek ertelenmeliydi: %v", got)
	}
	if err := finished(t, end); !errors.Is(err, interp.ErrInterrupted) {
		t.Errorf("durdurma bekleniyordu: %v", err)
	}
	if d := time.Since(start); d < window {
		t.Errorf("erteleme dolmadan durdu: %v", d)
	}
	if stillBeating(beat) {
		t.Error("program durmadı")
	}
}

// Görev iptali çağrıyı durdurur; TR_handle_stop iptali etkilemez, bekleyen iptal hatasını alır (10.9, 21.5).
func TestCancelStopsTheCallEvenWhenStopIsHandled(t *testing.T) {
	prog := helperProgram(t)
	for _, first := range []string{"", "TR_handle_stop()\n"} {
		beat := filepath.Join(t.TempDir(), "atım")
		got, err := run(t, first+`TR_var g = TR_async { TR_run_process(`+prog+`, ["beat", `+tr(beat)+`, "2000"]) }
TR_for i TR_in 0..500 {
    TR_sleep(0.01)
    TR_if TR_Path(`+tr(beat)+`).TR_exists { TR_break }
}
g.TR_cancel()
TR_error_code { g.TR_wait() }`)
		if err != nil || got != "H2920" {
			t.Errorf("%q: %s (%v)", first, got, err)
		}
		if stillBeating(beat) {
			t.Errorf("%q: program durmadı", first)
		}
	}
}

// TR_defer içindeki bekleme bırakılmaz: durdurma sürerken ertelenen çağrı biter (1.5, 21.5).
func TestCallInADeferIsNotAbandoned(t *testing.T) {
	prog := helperProgram(t)
	dir := t.TempDir()
	mark, ready := filepath.Join(dir, "işaret"), filepath.Join(dir, "hazır")
	m, end := startProgram(t, `TR_defer {
    TR_run_process(`+prog+`, ["touch-after", `+tr(mark)+`, "0.4"])
}
TR_write_file(`+tr(ready)+`, "x")
TR_sleep(30)`)
	await(t, "program hazır", func() bool { return exists(ready) })
	m.RequestStop(2)
	if err := finished(t, end); !errors.Is(err, interp.ErrInterrupted) {
		t.Errorf("durdurma bekleniyordu: %v", err)
	}
	if !exists(mark) {
		t.Error("ertelenen çağrı bırakıldı")
	}
}

// Ertelenen işte de süre sınırı geçerlidir: bekleme kesilmez ama TR_timeout: kendi süresini tutar.
func TestTimeoutWorksInADefer(t *testing.T) {
	prog := helperProgram(t)
	start := time.Now()
	got, err := run(t, `TR_var kod = "yok"
TR_func f() {
    TR_defer {
        kod = TR_error_code { TR_run_process(`+prog+`, ["sleep", "30"], TR_timeout: 0.1) } ?? "yok"
    }
}
f()
kod`)
	if err != nil || got != "H2921" {
		t.Errorf("%s (%v)", got, err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("%v sürdü", d)
	}
}

// Durdurma kilit tutmaz: inatçı program 2 saniye beklenirken öbür görevler ilerler (21.5, 10.3).
func TestStoppingDoesNotHoldTheLock(t *testing.T) {
	prog := helperProgram(t)
	ready := filepath.Join(t.TempDir(), "hazır")
	got, err := run(t, `TR_var kod = "yok"
TR_var g = TR_async {
    kod = TR_error_code { TR_run_process(`+prog+`, ["ignore", `+tr(ready)+`], TR_timeout: 0.3) } ?? "yok"
}
TR_var n = 0
TR_for TR_not g.TR_done() {
    TR_sleep(0.01)
    n += 1
}
[kod, n > 5]`)
	if err != nil || !strings.Contains(got, "H2921") || !strings.Contains(got, "true") {
		t.Errorf("%s (%v)", got, err)
	}
}

// Program normal bitip torun boruyu tutarsa çağrı en çok 2 saniye bekler, okunan çıktı verilir, sonucu
// programın çıkış kodu belirler, torun öldürülmez; iki işletim sisteminde aynı (TCB kararı, 09:02).
func TestGrandchildHoldingThePipeDoesNotBlock(t *testing.T) {
	prog := helperProgram(t)
	beat := filepath.Join(t.TempDir(), "atım")
	start := time.Now()
	got, err := run(t, `TR_String(TR_run_process(`+prog+`, ["orphan", `+tr(beat)+`, "250", "0"]))`)
	d := time.Since(start)
	if err != nil || got != "ebeveyn çıktısı" {
		t.Errorf("%s (%v)", got, err)
	}
	if d < 1900*time.Millisecond || d > 4500*time.Millisecond {
		t.Errorf("%v sürdü, 2 saniye beklenirdi", d)
	}
	if !stillBeating(beat) {
		t.Error("torun öldürülmemeliydi")
	}
	await(t, "torun bitti", func() bool { return size(beat) >= 250 })

	beat = filepath.Join(t.TempDir(), "atım")
	_, err = run(t, `TR_run_process(`+prog+`, ["orphan", `+tr(beat)+`, "150", "3"])`)
	if !messages.Has(err, messages.H7503) {
		t.Errorf("sonucu çıkış kodu belirler: %v", err)
	}
	await(t, "torun bitti", func() bool { return size(beat) >= 150 })
}

// Gruptan kaçan torun boruyu tutsa da durdurulan çağrı döner (21.5). Windows'ta program iş nesnesinden
// çıkamaz: o yol yok.
func TestEscapedGrandchildDoesNotBlockTheStop(t *testing.T) {
	if windows {
		t.Skip("iş nesnesinden kaçılmaz")
	}
	prog := helperProgram(t)
	beat := filepath.Join(t.TempDir(), "atım")
	start := time.Now()
	_, err := run(t, `TR_run_process(`+prog+`, ["escape", `+tr(beat)+`, "300"], TR_timeout: 0.3)`)
	if !messages.Has(err, messages.H2921) {
		t.Errorf("H2921 bekleniyordu: %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("%v sürdü", d)
	}
	await(t, "torun bitti", func() bool { return size(beat) >= 300 })
}
