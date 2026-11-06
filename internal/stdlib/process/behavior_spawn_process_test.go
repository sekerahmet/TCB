package process_test

import (
	"bytes"
	"errors"
	"io"
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
	"github.com/sekerahmet/TCB/internal/stdlib/process"
)

// helperSpawn akışlı çağrının testlerinin programlarıdır.
func helperSpawn(args []string) {
	switch args[0] {
	case "pattern": // n bayt desen yazar
		n, _ := strconv.Atoi(args[1])
		_, _ = os.Stdout.Write(pattern(n))
	case "endless": // durdurulana kadar yazar
		chunk := pattern(1 << 12)
		for {
			if _, err := os.Stdout.Write(chunk); err != nil {
				return
			}
		}
	case "dribble": // yazar, uyur, yazar: bayt sayısı, milisaniye, kod
		n, _ := strconv.Atoi(args[1])
		ms, _ := strconv.Atoi(args[2])
		_, _ = os.Stdout.Write(pattern(n))
		time.Sleep(time.Duration(ms) * time.Millisecond)
		_, _ = os.Stdout.Write(pattern(n))
		code, _ := strconv.Atoi(args[3])
		os.Exit(code)
	default:
		panic("bilinmeyen yardımcı: " + args[0])
	}
}

// pattern sırayı bozan ya da bayt kaybeden okumayı gösteren içeriktir: 251 baytın katları değildir.
func pattern(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

// program kodu koşar: son değer, hata çıktısı, çıkış kodu, hata.
func program(t *testing.T, code string) (interp.Value, string, int, error) {
	t.Helper()
	m := interp.New()
	var errs bytes.Buffer
	m.Out, m.Err = io.Discard, &errs
	p := parser.New(lexer.NewBody(code, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi: %v", p.Errors())
	}
	v, err := m.Run(prog)
	return v, errs.String(), m.End(err).Code, err
}

// joined TR listesindeki baytları birleştirir.
func joined(t *testing.T, v interp.Value) []byte {
	t.Helper()
	l, ok := v.(*interp.ListVal)
	if !ok {
		t.Fatalf("liste bekleniyordu: %v", v)
	}
	var out []byte
	for _, e := range l.Elements {
		out = append(out, e.(*interp.BytesVal).Data()...)
	}
	return out
}

const readAll = `
TR_var parçalar = []
TR_for {
    TR_var parça = p.TR_read(65536)
    TR_if parça.TR_length == 0 { TR_break }
    parçalar.TR_append(parça)
}
`

// Geri basınç: iç tampondan çok büyük çıktı yavaş okunur, hiçbir bayt kaybolmaz, sıra bozulmaz, hata
// olmaz (21.2).
func TestBackpressureKeepsEveryByte(t *testing.T) {
	n := 5 * process.OutputBuffer
	v, _, _, err := program(t, `TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["pattern", "`+strconv.Itoa(n)+`"])
TR_var parçalar = []
TR_for {
    TR_var parça = p.TR_read(262144)
    TR_if parça.TR_length == 0 { TR_break }
    parçalar.TR_append(parça)
    TR_sleep(0.005)
}
p.TR_wait()
parçalar`)
	if err != nil {
		t.Fatal(err)
	}
	if got := joined(t, v); !bytes.Equal(got, pattern(n)) {
		t.Errorf("%d bayt geldi, %d bekleniyordu ya da içerik bozuk", len(got), n)
	}
}

// Program iç tampon artı birkaç KiB yazıp biter: tampon doluyken sınır kalkar, boruda kalan alınır;
// TR stopGrace'ten uzun bekleyip okur, bütün veri gelir (21.2).
func TestOutputOutlivesTheProgram(t *testing.T) {
	for _, arg := range []string{"", ", TR_buffer_bytes: " + strconv.Itoa(2*process.OutputBuffer)} {
		n := process.OutputBuffer + 2048
		v, _, _, err := program(t, `TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["pattern", "`+strconv.Itoa(n)+`"]`+arg+`)
TR_for { TR_if p.TR_done() { TR_break }
    TR_sleep(0.01) }
TR_sleep(2.5)
`+readAll+`parçalar`)
		if err != nil {
			t.Fatalf("%q: %v", arg, err)
		}
		if got := joined(t, v); !bytes.Equal(got, pattern(n)) {
			t.Errorf("%q: %d bayt geldi, %d bekleniyordu ya da içerik bozuk", arg, len(got), n)
		}
	}
}

// TR hesap yaparken okuyucu sürer: boru dolmaz, program biter, bayt kaybolmaz (21.2).
func TestReaderRunsWhileTRComputes(t *testing.T) {
	n := 200000 // bu boy iki sistemin borusundan da büyüktür: okuyucu olmasa program bitmezdi
	v, _, _, err := program(t, `TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["pattern", "`+strconv.Itoa(n)+`"])
TR_var tur = 0
TR_for {
    tur = tur + 1
    TR_if p.TR_done() { TR_break }
}
`+readAll+`parçalar`)
	if err != nil {
		t.Fatal(err)
	}
	if got := joined(t, v); !bytes.Equal(got, pattern(n)) {
		t.Errorf("%d bayt geldi, %d bekleniyordu ya da içerik bozuk", len(got), n)
	}
}

// TR_buffer_bytes: taşınca program durur, baytlar atılır; hata yapışkandır, TR_cancel silmez,
// TR_try ile görülür, program 0 koduyla biter (21.2).
func TestOverflowStopsTheProgramAndSticks(t *testing.T) {
	prog := helperProgram(t)
	v, errs, code, err := program(t, `TR_var p = TR_spawn_process(`+prog+`, ["endless"], TR_buffer_bytes: 1000)
TR_for { TR_if p.TR_done() { TR_break }
    TR_sleep(0.01) }
TR_var görülen = TR_error_code { p.TR_read(1) }
p.TR_cancel()
[görülen, TR_error_code { p.TR_read(1) }, TR_error_code { p.TR_wait() }, p.TR_done()]`)
	if err != nil || strings.Count(v.Inspect(), "H7522") != 3 || !strings.Contains(v.Inspect(), "true") {
		t.Errorf("%s (%v)", v.Inspect(), err)
	}
	if errs != "" || code != 0 {
		t.Errorf("görülen taşma yazılmamalı: %q, kod %d", errs, code)
	}
}

// Hiç görülmeyen taşma program biterken yazılır, program 1 koduyla biter; iletide yazım vardır (21.2).
func TestUnseenOverflowIsWrittenAtTheEnd(t *testing.T) {
	_, errs, code, err := program(t, `TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["endless"], TR_buffer_bytes: 1000)
TR_for { TR_if p.TR_done() { TR_break }
    TR_sleep(0.01) }
p.TR_cancel()`)
	if err != nil || code != 1 || !strings.Contains(errs, "TR_buffer_bytes: 1000") {
		t.Errorf("hata %v, kod %d, çıktı %q", err, code, errs)
	}
}

// Taşma iletisinin önerisi çalışır (21.2).
func TestOverflowSuggestionRuns(t *testing.T) {
	dir := t.TempDir()
	installTool(t, dir, "araç")
	t.Setenv("PATH", dir)
	for _, program := range []string{`"araç"`, helperProgram(t)} {
		_, err := run(t, `TR_var p = TR_spawn_process(`+program+`, ["endless"], TR_buffer_bytes: 1000)
TR_for { TR_if p.TR_done() { TR_break }
    TR_sleep(0.01) }
p.TR_wait()`)
		if !messages.Has(err, messages.H7522) {
			t.Fatalf("%v", err)
		}
		const start = "`"
		text := err.Error()
		code := text[strings.LastIndex(text, "`TR_spawn_process(")+1 : strings.LastIndex(text, start)]
		got, err := run(t, `TR_var p = `+code+`
TR_var parça = p.TR_read(10)
p.TR_cancel()
parça.TR_length`)
		if err != nil || got != "10" {
			t.Errorf("%s: %s (%v)", code, got, err)
		}
	}
}

// Okuma tampondan büyük olamaz: hiç dolmaz (21.2).
func TestReadSizeAgainstTheBuffer(t *testing.T) {
	prog := helperProgram(t)
	if _, err := run(t, `TR_spawn_process(`+prog+`, ["pattern", "10"], TR_buffer_bytes: 8).TR_read(9)`); !messages.Has(err, messages.H7523) {
		t.Errorf("tampondan büyük: %v", err)
	}
	for _, n := range []string{"0", "-1"} {
		if _, err := run(t, `TR_spawn_process(`+prog+`, ["pattern", "10"]).TR_read(`+n+`)`); !messages.Has(err, messages.H7524) {
			t.Errorf("%s: %v", n, err)
		}
	}
}

// Okuma tam n bayt verir, gelmesini bekler; dosya sonunda kalanı, sonra boş verir; TR_done doğruyken
// veri kalabilir, TR_wait'ten sonra da okunur; çıkış kodu TR_wait'ten gelir (21.2, 21.4).
func TestReadGivesExactlyNThenTheRest(t *testing.T) {
	got, err := run(t, `TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["dribble", "100", "200", "3"], TR_exit_codes: [3])
TR_var a = p.TR_read(150)
TR_var kod = p.TR_wait()
[a.TR_length, kod, p.TR_done(), p.TR_read(1000).TR_length, p.TR_read(1).TR_length, p.TR_wait()]`)
	if want := "[150, 3, true, 50, 0, 3]"; err != nil || got != want {
		t.Errorf("%s (%v), beklenen %s", got, err, want)
	}
	_, err = run(t, `TR_spawn_process(`+helperProgram(t)+`, ["exit", "3"]).TR_wait()`)
	if !messages.Has(err, messages.H7503) {
		t.Errorf("kabul edilmeyen kod: %v", err)
	}
}

// Bitmiş programa yazmak hatadır; kapatılmış girdiye de. Girdi yazılır, kapatılınca program dosya sonu
// alır (21.2).
func TestWriteAndCloseStdin(t *testing.T) {
	prog := helperProgram(t)
	got, err := run(t, `TR_var p = TR_spawn_process(`+prog+`, ["cat"])
p.TR_write(TR_Bytes("merhaba"))
p.TR_close_stdin()
TR_var yanıt = TR_String(p.TR_read(100))
[yanıt, TR_error_code { p.TR_write(TR_Bytes("x")) }, p.TR_wait()]`)
	if err != nil || !strings.Contains(got, "merhaba") || !strings.Contains(got, "H75") {
		t.Errorf("%s (%v)", got, err)
	}
	_, err = run(t, `TR_var p = TR_spawn_process(`+prog+`, ["exit", "0"])
p.TR_wait()
p.TR_write(TR_Bytes("x"))`)
	if !messages.Has(err, messages.H7526) {
		t.Errorf("bitmiş programa yazma: %v", err)
	}
	_, err = run(t, `TR_var p = TR_spawn_process(`+prog+`, ["sleep", "5"])
TR_defer { p.TR_cancel() }
p.TR_close_stdin()
p.TR_write(TR_Bytes("x"))`)
	if !messages.Has(err, messages.H7528) {
		t.Errorf("kapatılmış girdi: %v", err)
	}
}

// İki görevin okuması sıraya girer, n bayt bölünmez (21.2).
func TestTwoTasksReadWholePieces(t *testing.T) {
	got, err := run(t, `TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["pattern", "2000"])
TR_func oku() -> TR_List[TR_Int] {
    TR_var boylar = []
    TR_for {
        TR_var parça = p.TR_read(100)
        TR_if parça.TR_length == 0 { TR_break }
        boylar.TR_append(parça.TR_length)
    }
    TR_return boylar
}
TR_var a = TR_async oku()
TR_var b = TR_async oku()
TR_var hepsi = a.TR_wait() + b.TR_wait()
[hepsi.TR_length, TR_sum(hepsi)]`)
	if err != nil || got != "[20, 2000]" {
		t.Errorf("%s (%v)", got, err)
	}
}

// Bekleyen okuma iptalde, süre sınırında ve başka görevden TR_cancel'de hemen döner; iki kipte (21.5).
func TestPendingReadEndsWithTheProgram(t *testing.T) {
	prog := helperProgram(t)
	for _, arg := range []string{"", ", TR_buffer_bytes: 1000"} {
		spawn := `TR_var p = TR_spawn_process(` + prog + `, ["sleep", "30"]` + arg + `)
TR_defer { p.TR_cancel() }
`
		start := time.Now()
		got, err := run(t, spawn+`TR_var g = TR_async { p.TR_read(10) }
TR_sleep(0.1)
g.TR_cancel()
[TR_error_code { g.TR_wait() }, p.TR_done()]`)
		if err != nil || !strings.Contains(got, "H2920") {
			t.Errorf("%q iptal: %s (%v)", arg, got, err)
		}
		reader := `TR_func oku() -> TR_String? { TR_return TR_error_code { p.TR_read(10) } }
TR_var g = TR_async oku()
`
		got, err = run(t, spawn+reader+`TR_sleep(0.1)
p.TR_cancel()
g.TR_wait()`)
		if err != nil || !strings.Contains(got, "H2920") {
			t.Errorf("%q başka görevden: %s (%v)", arg, got, err)
		}
		got, err = run(t, spawn+reader+`TR_var kod = TR_error_code { p.TR_wait(TR_timeout: 0.1) }
[kod, g.TR_wait()]`)
		if err != nil || strings.Count(got, "H2921") != 2 {
			t.Errorf("%q süre sınırı: %s (%v)", arg, got, err)
		}
		if d := time.Since(start); d > 9*time.Second {
			t.Errorf("%q: %v sürdü", arg, d)
		}
	}
}

// Okunmayan program tampon dolunca bekler, TR_wait de onu bekler: süre sınırı onu durdurur (21.2).
func TestUnreadProgramStopsWithAWaitLimit(t *testing.T) {
	got, err := run(t, `TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["endless"])
TR_error_code { p.TR_wait(TR_timeout: 0.3) }`)
	if err != nil || !strings.Contains(got, "H2921") {
		t.Errorf("%s (%v)", got, err)
	}
}

// TR_buffer_bytes: pozitif tam sayıdır; belleği aşan değer program başlamadan hatadır, büyük ama
// geçerli değer ön ayırma yapmadan başlar (21.2).
func TestBufferBytesIsChecked(t *testing.T) {
	prog := helperProgram(t)
	mark := t.TempDir() + "/başladı"
	for _, v := range []string{"0", "-1", "1.5", "2.0", `"10"`} {
		_, err := run(t, `TR_spawn_process(`+prog+`, ["touch-after", `+tr(mark)+`, "0"], TR_buffer_bytes: `+v+`)`)
		if !messages.Has(err, messages.H7520) {
			t.Errorf("%s: %v", v, err)
		}
	}
	restore := process.SetPhysicalMemory(1 << 20)
	defer restore()
	_, err := run(t, `TR_spawn_process(`+prog+`, ["touch-after", `+tr(mark)+`, "0"], TR_buffer_bytes: 2097152)`)
	if !messages.Has(err, messages.H7521) {
		t.Errorf("belleği aşan: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if exists(mark) {
		t.Error("program başlamamalıydı")
	}
	if _, err := run(t, `TR_spawn_process(`+prog+`, ["pattern", "1"]).TR_read(2097152)`); !messages.Has(err, messages.H7525) {
		t.Errorf("belleği aşan okuma: %v", err)
	}
	restore()
	got, err := run(t, `TR_var p = TR_spawn_process(`+prog+`, ["pattern", "5"], TR_buffer_bytes: 1073741824)
p.TR_read(5).TR_length`)
	if err != nil || got != "5" {
		t.Errorf("%s (%v)", got, err)
	}
}

// Torun boruyu tutsa da TR_wait ve TR_read asılmaz: okuma stopGrace sonunda kapanır (21.2).
func TestGrandchildDoesNotHangTheProcess(t *testing.T) {
	beat := t.TempDir() + "/atım"
	start := time.Now()
	got, err := run(t, `TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["orphan", `+tr(beat)+`, "250", "0"])
TR_var kod = p.TR_wait()
[kod, TR_String(p.TR_read(1000))]`)
	if err != nil || !strings.Contains(got, "ebeveyn çıktısı") {
		t.Errorf("%s (%v)", got, err)
	}
	if d := time.Since(start); d > 6*time.Second {
		t.Errorf("%v sürdü", d)
	}
	await(t, "torun bitti", func() bool { return size(beat) >= 250 })
}

// Program biterken süren süreç durdurulur, durması hata yazmaz; hiç beklenmemiş program kendiliğinden
// kabul edilmeyen kodla bittiyse hatası yazılır, kod 1 olur (21.2, 21.4).
func TestProgramEndStopsAndReports(t *testing.T) {
	prog := helperProgram(t)
	beat := t.TempDir() + "/atım"
	_, errs, code, err := program(t, `TR_spawn_process(`+prog+`, ["beat", `+tr(beat)+`, "100000"])
TR_sleep(0.2)`)
	if err != nil || errs != "" || code != 0 {
		t.Errorf("durdurulan yazılmamalı: %v, %q, kod %d", err, errs, code)
	}
	if stillBeating(beat) {
		t.Error("program durmalıydı")
	}
	for _, end := range []string{`TR_exit(4)`, `TR_throw "son"`} {
		beat = filepath.Join(t.TempDir(), "atım")
		_, _, _, _ = program(t, `TR_spawn_process(`+prog+`, ["beat", `+tr(beat)+`, "100000"])
TR_sleep(0.2)
`+end)
		if stillBeating(beat) {
			t.Errorf("%s: program durmalıydı", end)
		}
	}
	_, errs, code, err = program(t, `TR_var p = TR_spawn_process(`+prog+`, ["exit", "3"])
TR_for { TR_if p.TR_done() { TR_break }
    TR_sleep(0.01) }`)
	if err != nil || code != 1 || !strings.Contains(errs, "3") {
		t.Errorf("hata %v, kod %d, çıktı %q", err, code, errs)
	}
	_, errs, code, err = program(t, `TR_var p = TR_spawn_process(`+prog+`, ["exit", "3"])
TR_error_code { p.TR_wait() }`)
	if err != nil || errs != "" || code != 0 {
		t.Errorf("beklenen program yazılmamalı: %v, %q, kod %d", err, errs, code)
	}
}

// Üstlenilmiş programda istek gelince TR_read "durdurma istendi" ile döner, program ve baytlar kalır;
// TR_wait sürer (21.5).
func TestHandledStopLeavesTheReadBytes(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sonuç")
	m, end := startProgram(t, `TR_handle_stop()
TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["dribble", "10", "600", "0"])
TR_try {
    p.TR_read(15)
} TR_catch e {
    TR_write_file(`+tr(out)+`, TR_String([TR_error_code { TR_throw e }, p.TR_read(100).TR_length, p.TR_wait()]))
}`)
	time.Sleep(300 * time.Millisecond)
	m.RequestStop(2)
	if err := finished(t, end); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(got), "H2530") || !strings.Contains(string(got), "20, 0") {
		t.Errorf("%s (%v)", got, err)
	}
}

// Durdurma iki kipte de bekleyen okumayı bitirir, program durmadan yığın çözülmez (1.5, 21.5).
func TestStopEndsThePendingRead(t *testing.T) {
	for _, arg := range []string{"", ", TR_buffer_bytes: 1000"} {
		beat := filepath.Join(t.TempDir(), "atım")
		m, end := startProgram(t, `TR_var p = TR_spawn_process(`+helperProgram(t)+`, ["beat", `+tr(beat)+`, "100000"]`+arg+`)
p.TR_read(10)`)
		await(t, "program başladı", func() bool { return size(beat) > 0 })
		m.RequestStop(2)
		if err := finished(t, end); !errors.Is(err, interp.ErrInterrupted) {
			t.Errorf("%q: %v", arg, err)
		}
		if stillBeating(beat) {
			t.Errorf("%q: program durmalıydı", arg)
		}
	}
}

// Süreç değeri tipini söyler; bilinmeyen üye önerilir.
func TestProcessValue(t *testing.T) {
	got, err := run(t, `TR_var p: TR_Process = TR_spawn_process(`+helperProgram(t)+`, ["exit", "0"])
TR_var metin = TR_String(p)
p.TR_wait()
metin`)
	if err != nil || got == "" {
		t.Errorf("%s (%v)", got, err)
	}
	if _, err := run(t, `TR_spawn_process(`+helperProgram(t)+`, ["exit", "0"]).TR_length`); err == nil {
		t.Error("bilinmeyen üye hata vermeli")
	}
}
