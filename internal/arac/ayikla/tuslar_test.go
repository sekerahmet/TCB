package ayikla

import (
	"bufio"
	"io"
	"os"
	"strings"
	"testing"

	"tr_dili/internal/dil/yurut"
)

// Hakem mimari 25: terminal hata ayıklayıcısında f5 adım, f8 devam idi; VS Code'da F5
// devam, F10 üstten geç, F11 içine gir, Shift+F11 dışarı çık. Artık tuş adları iki yerde de
// aynı anlamdadır.
func TestTerminalTuslariVSCodeIleAyni(t *testing.T) {
	eskiCikti := os.Stdout
	bos, _ := os.Open(os.DevNull)
	os.Stdout = bos
	defer func() { os.Stdout = eskiCikti; bos.Close() }()
	for girdi, bekle := range map[string]StepMode{
		"f5":        StepContinue,
		"F5":        StepContinue,
		"devam":     StepContinue,
		"f10":       StepOver,
		"geç":       StepOver,
		"f11":       StepInto,
		"adım":      StepInto,
		"":          StepInto,
		"shift+f11": StepOut,
		"dön":       StepOut,
	} {
		d := NewDebugger("t.tr", "x\n", false)
		d.Mode = -1
		d.Reader = bufio.NewReader(strings.NewReader(girdi + "\n"))
		d.promptCLI(yurut.New(), 1, yurut.NewEnvironment(nil))
		if d.Mode != bekle {
			t.Errorf("%q: kip %d bekleniyordu, %d", girdi, bekle, d.Mode)
		}
	}
	// Eski anlamlı tuşlar sessizce başka iş yapmaz; yeni tuşu söyler ve komut beklemeye devam eder
	for _, eski := range []string{"f6", "f7", "f8"} {
		d := NewDebugger("t.tr", "x\n", false)
		d.Mode = -1
		var b strings.Builder
		r, w, _ := os.Pipe()
		os.Stdout = w
		d.Reader = bufio.NewReader(strings.NewReader(eski + "\ndevam\n"))
		d.promptCLI(yurut.New(), 1, yurut.NewEnvironment(nil))
		w.Close()
		veri, _ := io.ReadAll(r)
		os.Stdout = bos
		b.Write(veri)
		if d.Mode != StepContinue || !strings.Contains(b.String(), "F5") || !strings.Contains(b.String(), "F10") {
			t.Errorf("%q: yeni tuşları gösteren uyarı bekleniyordu: %q", eski, b.String())
		}
	}
}

// Her komutun tek yazımı vardır: kısaltmalar, Türkçe karaktersiz ve İngilizce eşler
// çalışmaz; asıl komutu söyleyen bir ipucu verir ve komut beklemeye devam edilir.
func TestKomutlarinTekYazimi(t *testing.T) {
	eskiCikti := os.Stdout
	defer func() { os.Stdout = eskiCikti }()
	for eski, asil := range map[string]string{"step": "adım", "adim": "adım", "next": "geç", "o": "dön",
		"c": "devam", "set": "değiştir", "kalem": "değiştir", "goto": "zıpla", "break": "kesme",
		"watch": "izle", "stack": "yığın", "p": "hesapla", "help": "yardım", "quit": "çıkış"} {
		d := NewDebugger("t.tr", "x\n", false)
		d.Mode = -1
		r, w, _ := os.Pipe()
		os.Stdout = w
		d.Reader = bufio.NewReader(strings.NewReader(eski + "\ndevam\n"))
		d.promptCLI(yurut.New(), 1, yurut.NewEnvironment(nil))
		w.Close()
		veri, _ := io.ReadAll(r)
		os.Stdout = eskiCikti
		if want := "'" + eski + "' hata ayıklayıcıda bir komut değil; '" + asil + "' yazın"; d.Mode != StepContinue || !strings.Contains(string(veri), want) {
			t.Errorf("%q: %q ipucu ve devam bekleniyordu, kip %d: %q", eski, want, d.Mode, veri)
		}
	}
}
