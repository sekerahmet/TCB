package process_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

const windows = runtime.GOOS == "windows"

// installTool test ikilisini dir klasörüne name adıyla kopyalar: Windows'ta .exe ekler, adı noktalı
// program için de öyle. Yolu verir.
func installTool(t *testing.T, dir, name string) string {
	t.Helper()
	t.Setenv(helperEnv, "1")
	if windows && filepath.Ext(name) != ".com" && filepath.Ext(name) != ".exe" {
		name += ".exe"
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// run program_çalıştır'ın kodunu koşar ve metin sonucu verir.
func run(t *testing.T, code string) (string, error) {
	t.Helper()
	got, _, err := commandLine(t, code)
	return got, err
}

// sameFile iki yolun aynı dosya olduğunu söyler (kısa ad ve bağlantı farkı sayılmaz).
func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	x, err1 := os.Stat(a)
	y, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(x, y)
}

// Ad PATH'te aranır, ilk bulunan çalışır (21.3).
func TestProgramNameIsSearchedInPath(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	want := installTool(t, first, "araç")
	installTool(t, second, "araç")
	t.Setenv("PATH", first+string(os.PathListSeparator)+second)
	got, err := run(t, `TR_String(TR_run_process("araç", ["self"]))`)
	if err != nil || !sameFile(t, got, want) {
		t.Errorf("%s (%v), beklenen %s", got, err, want)
	}
}

// Ad yol ayırıcısı içeremez; ileti yolu önerir, öneri çalışır (21.3).
func TestProgramNameHoldsNoSeparator(t *testing.T) {
	dir := t.TempDir()
	installTool(t, dir, "araç")
	t.Chdir(dir)
	for _, name := range []string{`./araç`, `.\\araç`, `alt/araç`} {
		_, err := run(t, `TR_run_process("`+name+`", [])`)
		if !messages.Has(err, messages.H7494) {
			t.Errorf("%s: H7494 bekleniyordu: %v", name, err)
		}
	}
	_, err := run(t, `TR_run_process("./araç", [])`)
	if !strings.Contains(err.Error(), `TR_run_process(TR_Path("./araç"), [])`) {
		t.Errorf("öneri yok: %v", err)
	}
	spelling := `TR_Path("./araç")`
	if windows {
		spelling = `TR_Path("./araç.exe")`
	}
	if got, err := run(t, `TR_run_process(`+spelling+`, ["args", "x"]).TR_length`); err != nil || got == "0" {
		t.Errorf("önerinin yazımı: %s (%v)", got, err)
	}
}

// Göreli yol çalışma klasörüne göredir, ayırıcısız yol PATH'e gitmez; program mutlak yolla başlar (21.3).
func TestProgramPathIsRelativeToTheWorkingDirectory(t *testing.T) {
	work, onPath := t.TempDir(), t.TempDir()
	want := installTool(t, work, "araç")
	installTool(t, onPath, "araç")
	t.Setenv("PATH", onPath)
	t.Chdir(work)
	spelling := `TR_Path("araç")`
	if windows {
		spelling = `TR_Path("araç.exe")`
	}
	got, err := run(t, `TR_String(TR_run_process(`+spelling+`, ["self"]))`)
	if err != nil || !sameFile(t, got, want) {
		t.Errorf("%s (%v), beklenen %s", got, err, want)
	}
	if _, err := run(t, `TR_run_process(TR_Path("olmayan.exe"), [])`); !messages.Has(err, messages.H7499) {
		t.Errorf("olmayan dosya: %v", err)
	}
}

// PATH'in göreli girdileri ve çalışma klasörü aranmaz; Unix'te boş girdi çalışma klasörüdür ve
// aranmaz: indirilen bir veri klasöründeki aynı adlı dosya çalışmaz (21.3).
func TestRelativePathEntriesAreNotSearched(t *testing.T) {
	work := t.TempDir()
	if err := os.Mkdir(filepath.Join(work, "alt"), 0o755); err != nil {
		t.Fatal(err)
	}
	installTool(t, work, "araç")
	installTool(t, filepath.Join(work, "alt"), "araç")
	t.Chdir(work)
	sep := string(os.PathListSeparator)
	for _, path := range []string{"", ".", "alt", sep, "." + sep + "alt", sep + "alt" + sep} {
		t.Setenv("PATH", path)
		if _, err := run(t, `TR_run_process("araç", [])`); !messages.Has(err, messages.H7497) {
			t.Errorf("PATH=%q: H7497 bekleniyordu: %v", path, err)
		}
	}
}

// Ad .exe, .com, .bat ya da .cmd ile bitemez, her işletim sisteminde; harf ayrılmaz, ileti uzantısızı
// önerir ve öneri çalışır (21.3). Windows'ta .bat ve .cmd'nin iletisi açık yazımdır.
func TestProgramNameCannotEndInAnExtension(t *testing.T) {
	dir := t.TempDir()
	installTool(t, dir, "ffmpeg")
	t.Setenv("PATH", dir)
	for _, name := range []string{"ffmpeg.exe", "ffmpeg.com", "FFMPEG.EXE", "ffmpeg.Com"} {
		_, err := run(t, `TR_run_process("`+name+`", [])`)
		if !messages.Has(err, messages.H7495) {
			t.Errorf("%s: H7495 bekleniyordu: %v", name, err)
		}
	}
	_, err := run(t, `TR_run_process("ffmpeg.exe", [])`)
	if !strings.Contains(err.Error(), `TR_run_process("ffmpeg", [])`) {
		t.Errorf("öneri yok: %v", err)
	}
	if _, err := run(t, `TR_run_process("ffmpeg", ["args"])`); err != nil {
		t.Errorf("öneri çalışmadı: %v", err)
	}
	want := messages.H7495
	if windows {
		want = messages.H7496
	}
	for _, name := range []string{"npm.bat", "npm.cmd", "NPM.CMD"} {
		if _, err := run(t, `TR_run_process("`+name+`", [])`); !messages.Has(err, want) {
			t.Errorf("%s: %s bekleniyordu: %v", name, want, err)
		}
	}
	if windows {
		for _, name := range []string{"ffmpeg.exe ", "ffmpeg.exe."} {
			if _, err := run(t, `TR_run_process("`+name+`", [])`); !messages.Has(err, messages.H7495) {
				t.Errorf("%q: H7495 bekleniyordu: %v", name, err)
			}
		}
	}
}

// Başka noktalı adlar program adıdır (21.3).
func TestDottedProgramNameIsAName(t *testing.T) {
	dir := t.TempDir()
	want := installTool(t, dir, "python3.11")
	t.Setenv("PATH", dir)
	got, err := run(t, `TR_String(TR_run_process("python3.11", ["self"]))`)
	if err != nil || !sameFile(t, got, want) {
		t.Errorf("%s (%v), beklenen %s", got, err, want)
	}
}

// Windows'ta yalnız .exe ve .com aranır ve çalışır; uzantısız dosya ve başka uzantı çalışmaz (21.3).
func TestWindowsRunsOnlyExeAndCom(t *testing.T) {
	if !windows {
		t.Skip("Windows kuralı")
	}
	dir := t.TempDir()
	want := installTool(t, dir, "araç.com")
	t.Setenv("PATH", dir)
	if got, err := run(t, `TR_String(TR_run_process("araç", ["self"]))`); err != nil || !sameFile(t, got, want) {
		t.Errorf(".com adla: %s (%v)", got, err)
	}
	if got, err := run(t, `TR_String(TR_run_process(TR_Path(`+strconv.Quote(want)+`), ["self"]))`); err != nil || !sameFile(t, got, want) {
		t.Errorf(".com yolla: %s (%v)", got, err)
	}
	upper := strings.TrimSuffix(want, ".com") + ".COM"
	if got, err := run(t, `TR_String(TR_run_process(TR_Path(`+strconv.Quote(upper)+`), ["self"]))`); err != nil || !sameFile(t, got, want) {
		t.Errorf(".COM yolla: %s (%v)", got, err)
	}
	for _, name := range []string{"veri", "betik.ps1", "betik.py"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := run(t, `TR_run_process(TR_Path(`+strconv.Quote(p)+`), [])`); !messages.Has(err, messages.H7500) {
			t.Errorf("%s: H7500 bekleniyordu: %v", name, err)
		}
		if _, err := run(t, `TR_run_process("`+strings.TrimSuffix(name, filepath.Ext(name))+`", [])`); !messages.Has(err, messages.H7497) {
			t.Errorf("%s adla: H7497 bekleniyordu: %v", name, err)
		}
	}
	stream := want + ":x.exe"
	if _, err := run(t, `TR_run_process(TR_Path(`+strconv.Quote(stream)+`), [])`); !messages.Has(err, messages.H7500) {
		t.Errorf("veri akışı: H7500 bekleniyordu: %v", err)
	}
	if _, err := run(t, `TR_run_process("C:araç", [])`); !messages.Has(err, messages.H7497) {
		t.Errorf("sürücüye göreli ad: H7497 bekleniyordu: %v", err)
	}
}

// Windows'ta .bat ve .cmd ad ya da yol olarak çalışmaz: DENETİM 217'nin tablosu, her satır bir sınama.
// Go bu yazımların hepsinde toplu iş dosyasını çalıştırıyordu. İşaret dosyası çalışmadığını gösterir.
func TestWindowsBatchFilesAreRefused(t *testing.T) {
	if !windows {
		t.Skip("Windows kuralı")
	}
	dir := t.TempDir()
	mark := filepath.Join(dir, "çalıştı.txt")
	for _, name := range []string{"hello.bat", "hi.cmd"} {
		text := "@echo x > \"" + mark + "\"\r\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	hello := filepath.Join(dir, "hello")
	for _, code := range []string{
		`"hello"`, `"hi"`,
		`TR_Path(` + strconv.Quote(hello) + `)`,
		`TR_Path(` + strconv.Quote(hello+".bat") + `)`,
		`TR_Path(` + strconv.Quote(hello+".BAT") + `)`,
		`TR_Path(` + strconv.Quote(hello+".bat ") + `)`,
		`TR_Path(` + strconv.Quote(hello+".bat.") + `)`,
		`TR_Path(` + strconv.Quote(hello+".bat. .") + `)`,
	} {
		if _, err := run(t, `TR_run_process(`+code+`, [])`); !messages.Has(err, messages.H7496) {
			t.Errorf("%s: H7496 bekleniyordu: %v", code, err)
		}
	}
	if _, err := os.Stat(mark); err == nil {
		t.Error("toplu iş dosyası çalıştı")
	}
	tool := installTool(t, dir, "araç")
	for _, p := range []string{tool + " ", tool + "."} {
		if _, err := run(t, `TR_run_process(TR_Path(`+strconv.Quote(p)+`), [])`); !messages.Has(err, messages.H7499) {
			t.Errorf("%q: dosya tam adıyla yok, H7499 bekleniyordu: %v", p, err)
		}
	}
}

// Toplu iş dosyasının iletisindeki açık yazım kopyalanınca çalışır (21.3).
func TestBatchSuggestionRuns(t *testing.T) {
	if !windows {
		t.Skip("Windows kuralı")
	}
	system := os.Getenv("SystemRoot")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "npm.cmd"), []byte("@echo merhaba\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+filepath.Join(system, "System32"))
	_, err := run(t, `TR_run_process("npm", [])`)
	want := `TR_run_process("cmd", ["/c", "npm.cmd"])`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("%s bekleniyordu: %v", want, err)
	}
	if !strings.Contains(err.Error(), "cmd.exe") {
		t.Errorf("ileti cmd.exe'nin kurallarını söylemeli: %v", err)
	}
	if got, err := run(t, `TR_String(`+want+`)`); err != nil || strings.TrimSpace(got) != "merhaba" {
		t.Errorf("%s (%v)", got, err)
	}
}

// Bulunamayan programın iletisi adı söyler; boşluklu adda argümanları listeye ayırmayı önerir, öneri
// çalışır (21.3). Boş ad ve boş bayt da hatadır.
func TestMissingProgramMessages(t *testing.T) {
	dir := t.TempDir()
	installTool(t, dir, "ffmpeg")
	t.Setenv("PATH", dir)
	_, err := run(t, `TR_run_process("olmayan", [])`)
	if !messages.Has(err, messages.H7497) || !strings.Contains(err.Error(), `"olmayan"`) {
		t.Errorf("ad söylenmeli: %v", err)
	}
	if _, err := run(t, `TR_run_process("", [])`); !messages.Has(err, messages.H7497) {
		t.Errorf("boş ad: %v", err)
	}
	_, err = run(t, `TR_run_process("ffmpeg args a.wav", [])`)
	want := `TR_run_process("ffmpeg", ["args", "a.wav"])`
	if !messages.Has(err, messages.H7498) || !strings.Contains(err.Error(), want) {
		t.Errorf("%s bekleniyordu: %v", want, err)
	}
	if _, err := run(t, want); err != nil {
		t.Errorf("öneri çalışmadı: %v", err)
	}
	for _, code := range []string{`"a\u{0}b"`, `TR_Path("a\u{0}b")`} {
		if _, err := run(t, `TR_run_process(`+code+`, [])`); !messages.Has(err, messages.H7502) {
			t.Errorf("%s: H7502 bekleniyordu: %v", code, err)
		}
	}
}
