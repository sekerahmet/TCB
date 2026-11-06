package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Dış programın TR programıyla bitişi (21.2 son cümlesi, 21.5, 1.7, 1.9). Dış program yine trlang'dır:
// test sh ve cmd varsaymaz. Atan program atım dosyasına sayısını yazar, durduğu buradan görülür.

// beatProgram durdurulana kadar atan programdır.
const beatProgram = `# TR 4
TR_var i = 0
TR_for i < 100000 {
    TR_write_file("atim.txt", TR_String(i))
    i += 1
    TR_sleep(0.02)
}
`

// stubbornProgram durdurma isteğini üstlenir ve atmayı sürdürür.
const stubbornProgram = `# TR 4
TR_handle_stop()
TR_var i = 0
TR_for i < 100000 {
    TR_write_file("atim.txt", TR_String(i))
    i += 1
    TR_try {
        TR_sleep(0.02)
    } TR_catch e {
    }
}
`

// shortProgram kısa sürer, bittiğini dosyaya yazar.
const shortProgram = `# TR 4
TR_sleep(0.5)
TR_write_file("bitti.txt", "x")
`

// callOf child.tr'yi dış program olarak çalıştıran TR yazımıdır.
func callOf() string {
	return `TR_run_process(TR_Path(` + strconv.Quote(trBinary) + `), ["child.tr"])`
}

// waitForBeat atım dosyası görünene kadar bekleyen TR kodudur.
const waitForBeat = `TR_for TR_not TR_Path("atim.txt").TR_exists {
    TR_sleep(0.01)
}
`

// beatStopped atım dosyasının artık değişmediğini söyler.
func beatStopped(dir string) bool {
	p := filepath.Join(dir, "atim.txt")
	var last []byte
	for range 3 {
		data, err := os.ReadFile(p)
		if err == nil && last != nil && string(data) != string(last) {
			return false
		}
		if err == nil {
			last = data
		}
		time.Sleep(150 * time.Millisecond)
	}
	return true
}

// Yakalanmayan hatayla biten programda görevdeki çağrı durur: öbür görevler 1.5'teki gibi durur (1.9).
func TestUncaughtErrorStopsTheExternalProgram(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "child.tr", beatProgram)
	writeTR(t, dir, "program.tr", "# TR 4\nTR_var g = TR_async { "+callOf()+" }\n"+waitForBeat+"TR_var s = 0\nTR_print(1 / s)\n")
	r := tr(t, dir, "program.tr")
	if r.Code != 1 {
		t.Errorf("çıkış %d, standart hata: %s", r.Code, r.Stderr)
	}
	if !beatStopped(dir) {
		t.Error("dış program durmadı")
	}
}

// TR_exit görevdeki çağrıyı durdurur, kod korunur (1.10).
func TestExitStopsTheExternalProgram(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "child.tr", beatProgram)
	writeTR(t, dir, "program.tr", "# TR 4\nTR_var g = TR_async { "+callOf()+" }\n"+waitForBeat+"TR_exit(4)\n")
	r := tr(t, dir, "program.tr")
	if r.Code != 4 {
		t.Errorf("çıkış %d, standart hata: %s", r.Code, r.Stderr)
	}
	if !beatStopped(dir) {
		t.Error("dış program durmadı")
	}
}

// Normal bitişte görevdeki çağrı durdurulmaz, beklenir (1.9).
func TestProgramEndWaitsForTheExternalProgram(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "child.tr", shortProgram)
	writeTR(t, dir, "program.tr", "# TR 4\nTR_var g = TR_async { "+callOf()+" }\n")
	r := tr(t, dir, "program.tr")
	if r.Code != 0 {
		t.Errorf("çıkış %d, standart hata: %s", r.Code, r.Stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "bitti.txt")); err != nil {
		t.Errorf("dış program bitmeden çıkıldı: %v", err)
	}
}

// Durdurma isteği dış programı da durdurur; süreç isteğin sinyaliyle biter (1.5, 21.5). Kesilen çağrının
// programı hata olarak yazılmaz (21.4).
func TestStopRequestStopsTheExternalProgram(t *testing.T) {
	s := startInterruptible(t, "# TR 4\n"+callOf()+"\n")
	writeTR(t, s.dir, "child.tr", beatProgram)
	s.waitCond("dış program atıyor", func() bool { return s.fileExists("atim.txt") })
	sendInterrupt(t, s.cmd)
	if !s.endedByInterrupt() {
		t.Fatalf("süreç isteğin sinyaliyle bitmeliydi; standart hata: %s", s.stderr.String())
	}
	if !beatStopped(s.dir) {
		t.Error("dış program durmadı")
	}
	if strings.Contains(s.stderr.String(), "koduyla bitti") {
		t.Errorf("durdurulan program hata yazmamalı: %s", s.stderr.String())
	}
}

// İkinci istek süreci hemen bitirir, önce süreç grupları öldürülür: isteği üstlenen dış program da
// durur, 2 saniye beklenmez (1.7, 21.5).
func TestSecondRequestKillsTheExternalProgram(t *testing.T) {
	s := startInterruptible(t, "# TR 4\n"+callOf()+"\n")
	writeTR(t, s.dir, "child.tr", stubbornProgram)
	s.waitCond("dış program atıyor", func() bool { return s.fileExists("atim.txt") })
	sendInterrupt(t, s.cmd)
	s.waitCond("ilk istek işlendi", func() bool { return strings.Contains(s.stderr.String(), "⏹️") })
	start := time.Now()
	sendInterrupt(t, s.cmd)
	if !s.endedByInterrupt() {
		t.Fatalf("süreç isteğin sinyaliyle hemen bitmeliydi; standart hata: %s", s.stderr.String())
	}
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Errorf("hemen bitiş %v sürdü", d)
	}
	if !beatStopped(s.dir) {
		t.Error("dış program durmadı")
	}
}

// TR_cmd_test altında TR_exit süreci bitirmez, testi düşürür: dış program durmaz, öbür test sürer (1.12).
func TestExitInATestLeavesTheExternalProgram(t *testing.T) {
	dir := t.TempDir()
	writeTR(t, dir, "child.tr", shortProgram)
	writeTR(t, dir, "deneme_test.tr", `# TR 4
TR_test "çıkan" {
    TR_var g = TR_async { `+callOf()+` }
    TR_exit(4)
}
TR_test "öbürü" {
    TR_assert(TR_true)
}
`)
	r := tr(t, dir, "test", "deneme_test.tr")
	if r.Code != 1 {
		t.Errorf("çıkış %d, standart çıktı: %s, standart hata: %s", r.Code, r.Stdout, r.Stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "bitti.txt")); err != nil {
		t.Errorf("dış program durdu: %v", err)
	}
}
