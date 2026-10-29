package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// trOutput tr'yi argümanlarla çalıştırır; birleşik çıktıyı ve başarılı bitip bitmediğini verir.
func trOutput(t *testing.T, args ...string) (string, bool) {
	t.Helper()
	out, err := exec.Command(trBuild(t), args...).CombinedOutput()
	return string(out), err == nil
}

// Kural (17.8): '-' ile başlayan ve komutun seçeneği olmayan argüman dosya adı sanılmaz (Python argparse
// ve Go flag gibi): her komut onu bilinmeyen seçenek (H8674) olarak reddeder, tr'nin kendisi
// H8604 ile; '--'dan sonraki argüman dosyadır. Komutlar commands tablosundan gezilir; tablo her
// komut kodunu (cmd sınıfı) içerir.
func TestUnknownOptionIsNotAFile(t *testing.T) {
	for _, k := range codes.Codes() {
		if _, ok := commands[k]; k.Class() == codes.ClassCommand && !ok {
			t.Errorf("%s komut tablosunda yok", k)
		}
	}
	for k := range commands {
		name := codes.DisplayName(k)
		// İletinin seçenek listesinden önceki kısmı komuttan bağımsızdır.
		want := messages.Text(messages.H8674, codes.NameRef(k), "--x", "", nil)
		want = want[:strings.Index(want, "--x")+len("--x")]
		if out, ok := trOutput(t, name, "--x"); ok || !strings.Contains(out, want) {
			t.Errorf("tr %s --x: bilinmeyen seçenek iletisi (%q) bekleniyordu, gelen (başarılı: %v):\n%s", name, want, ok, out)
		}
	}
	if out, ok := trOutput(t, "--x"); ok || !strings.Contains(out, messages.Text(messages.H8604, "--x", codes.NameRef(codes.CmdHelp))) {
		t.Errorf("tr --x: H8604 bekleniyordu, gelen:\n%s", out)
	}
	help := codes.DisplayName(codes.CmdHelp)
	if out, ok := trOutput(t, "--"+help); ok || !strings.Contains(out, messages.Text(messages.H8602, "--"+help, help)) {
		t.Errorf("tr --%s: 'tr %s' önerisi bekleniyordu, gelen:\n%s", help, help, out)
	}
	if out, _ := trOutput(t, codes.DisplayName(codes.CmdCheck), "--", "-TR_none.tr"); !strings.Contains(out, "-TR_none.tr") || strings.Contains(out, "--x") {
		t.Errorf("tr check -- -yok.tr: '-yok.tr' dosya olarak okunmalıydı, gelen:\n%s", out)
	}
}
