package debugger

import (
	"bufio"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Hakem mimari 25: terminal hata ayıklayıcısında f5 adım, f8 devam idi; düzenleyicide F5
// devam, F10 üstten geç, F11 içine gir, Shift+F11 dışarı çık. Artık tuş adları iki yerde de
// aynı anlamdadır.
func TestTerminalKeysMatchVSCode(t *testing.T) {
	oldOutput := os.Stdout
	empty, _ := os.Open(os.DevNull)
	os.Stdout = empty
	defer func() { os.Stdout = oldOutput; empty.Close() }()
	for input, await := range map[string]StepMode{
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
		d.Reader = bufio.NewReader(strings.NewReader(input + "\n"))
		d.promptCLI(interp.New(), 1, interp.NewEnvironment(nil))
		if d.Mode != await {
			t.Errorf("%q: kip %d bekleniyordu, %d", input, await, d.Mode)
		}
	}
	// Eski anlamlı tuşlar sessizce başka iş yapmaz; yeni tuşu söyler ve komut beklemeye devam eder
	for _, old := range []string{"f6", "f7", "f8"} {
		d := NewDebugger("t.tr", "x\n", false)
		d.Mode = -1
		var b strings.Builder
		r, w, _ := os.Pipe()
		os.Stdout = w
		d.Reader = bufio.NewReader(strings.NewReader(old + "\ndevam\n"))
		d.promptCLI(interp.New(), 1, interp.NewEnvironment(nil))
		w.Close()
		data, _ := io.ReadAll(r)
		os.Stdout = empty
		b.Write(data)
		if d.Mode != StepContinue || !strings.Contains(b.String(), "F5") || !strings.Contains(b.String(), "F10") {
			t.Errorf("%q: yeni tuşları gösteren uyarı bekleniyordu: %q", old, b.String())
		}
	}
}

// Her komutun tek yazımı vardır: kısaltmalar, Türkçe karaktersiz ve İngilizce eşler
// çalışmaz; asıl komutu söyleyen bir ipucu verir ve komut beklemeye devam edilir.
func TestCommandsSingleSpelling(t *testing.T) {
	oldOutput := os.Stdout
	defer func() { os.Stdout = oldOutput }()
	for old, actual := range map[string]string{"step": "adım", "adim": "adım", "next": "geç", "o": "dön",
		"c": "devam", "set": "değiştir", "kalem": "değiştir", "goto": "zıpla", "break": "kesme",
		"watch": "izle", "stack": "yığın", "p": "hesapla", "help": "yardım", "quit": "çıkış"} {
		d := NewDebugger("t.tr", "x\n", false)
		d.Mode = -1
		r, w, _ := os.Pipe()
		os.Stdout = w
		d.Reader = bufio.NewReader(strings.NewReader(old + "\ndevam\n"))
		d.promptCLI(interp.New(), 1, interp.NewEnvironment(nil))
		w.Close()
		data, _ := io.ReadAll(r)
		os.Stdout = oldOutput
		if want := "'" + old + "' hata ayıklayıcıda bir komut değil; '" + actual + "' yazın"; d.Mode != StepContinue || !strings.Contains(string(data), want) {
			t.Errorf("%q: %q ipucu ve devam bekleniyordu, kip %d: %q", old, want, d.Mode, data)
		}
	}
}
