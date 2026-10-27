package interp

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// fakeLiveLine silinip çizildiğini ortak çıktıya işaretleyen bir canlı satırdır.
type fakeLiveLine struct{ w *bytes.Buffer }

func (s *fakeLiveLine) Clear() { s.w.WriteString("[sil]") }
func (s *fakeLiveLine) Draw()  { s.w.WriteString("[çiz]") }

func execCode(t *testing.T, vmx *VM, codeVal string) {
	t.Helper()
	prog := mustParseProgram(t, codeVal)
	if _, err := vmx.Run(prog); err != nil {
		t.Fatal(err)
	}
}

// Etkin bir canlı satır (ilerleme çubuğu) varken yazdır, girdi istemi ve hata çıktısı
// önce satırı siler, metni yazar, satırı yeniden çizer; canlı satır kaldırılınca düz yazılır.
func TestLiveLineHook(t *testing.T) {
	var output bytes.Buffer
	vmx := New()
	vmx.Out, vmx.Err = &output, &output
	vmx.Stdin = bytes.NewBufferString("ali\n")
	vmx.SetLiveLine(&fakeLiveLine{w: &output})
	execCode(t, vmx, "TR_print(\"a\", 1)\nTR_var ad = TR_input(\"ad: \")\n")
	vmx.reportAsyncError(3, errors.New("bozuk"))
	expected := "[sil]a 1\n[çiz][sil]ad: [çiz][sil]⚠️  Eşzamanlı görev hatası (satır 3'te başlatılan): bozuk\n[çiz]"
	if output.String() != expected {
		t.Errorf("çıktı\n%q\nbeklenen\n%q", output.String(), expected)
	}
	output.Reset()
	vmx.SetLiveLine(nil)
	execCode(t, vmx, "TR_print(\"b\")\n")
	if output.String() != "b\n" {
		t.Errorf("canlı satır kaldırılınca düz yazılmalı: %q", output.String())
	}
	// Canlı satırın kendi çizimi de aynı kilit altında
	vmx.UpdateLiveLine(func() { output.WriteString("çubuk") })
	if output.String() != "b\nçubuk" {
		t.Errorf("CanliSatirGuncelle: %q", output.String())
	}
}

// Gizli değer süzgeci: kayıtlı değerler çıktı noktalarında (yazdır, hata çıktısı, argüman
// uyarısı) *** ile maskelenir; değerin kendisi (yazdır'ın döndürdüğü, değişkenler) değişmez.
// Kısa değerler kaydedilmez; biri ötekini içeren değerlerde uzunu bütünüyle maskelenir.
func TestSecretFilter(t *testing.T) {
	var output bytes.Buffer
	vmx := New()
	vmx.Out, vmx.Err = &output, &output
	if vmx.Mask("hf_gizli123") != "hf_gizli123" {
		t.Error("kayıt yokken metin değişmemeli")
	}
	if vmx.AddSecret("abc") {
		t.Error("kısa değer kaydedilmemeli")
	}
	if !vmx.AddSecret("hf_gizli123") || !vmx.AddSecret("hf_gizli123_uzun") || !vmx.AddSecret("hf_gizli123") {
		t.Fatal("gizli değer kaydedilemedi")
	}
	vmx.GlobalEnv.Define("anahtar", &StringVal{Val: "hf_gizli123"}, false)
	execCode(t, vmx, "TR_var d = TR_print(\"anahtar: \\(anahtar), uzun: hf_gizli123_uzun, abc\")\n")
	if got := output.String(); got != "anahtar: ***, uzun: ***, abc\n" {
		t.Errorf("yazdır çıktısı maskelenmeli: %q", got)
	}
	if d, _ := vmx.GlobalEnv.Get("d"); d.Inspect() != "anahtar: hf_gizli123, uzun: hf_gizli123_uzun, abc" {
		t.Errorf("yazdır'ın döndürdüğü değer (veri) değişmemeli: %s", d.Inspect())
	}
	output.Reset()
	vmx.reportAsyncError(1, errors.New("istek reddedildi: hf_gizli123"))
	if got := output.String(); !strings.Contains(got, "istek reddedildi: ***") {
		t.Errorf("hata çıktısı maskelenmeli: %q", got)
	}
	vmx.SetArgs([]string{"--anahtar", "hf_gizli123"})
	if u := vmx.UnusedArgsWarning(); !strings.Contains(u, "(--anahtar ***)") {
		t.Errorf("argüman uyarısı maskelenmeli: %q", u)
	}
}
