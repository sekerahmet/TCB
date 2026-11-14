package trrun

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/files"
)

// Kural: Exec programı yeni bir VM'de çalıştırır; yazdır() çıktısını, VM'i ve programın hatasını
// verir (hata testi düşürmez); MustExec hatada düşürür; MustExecChecked tip denetiminden geçer.
func TestExecCapturesOutputVMAndError(t *testing.T) {
	o := Exec(t, "TR_var x = 41\nTR_print(x + 1)\n")
	if o.Err != nil || o.Out != "42\n" {
		t.Fatalf("çıktı %q, hata %v", o.Out, o.Err)
	}
	if v, ok := o.VM.GlobalEnv.Get("x"); !ok || v.Inspect() != "41" {
		t.Fatalf("genel değişken x = %v (%v)", v, ok)
	}
	if bad := Exec(t, "TR_throw \"x\"\n"); bad.Err == nil {
		t.Fatal("fırlatılan hata dönmedi")
	}
	if got := MustExecChecked(t, "TR_print(2)\n").Out; got != "2\n" {
		t.Fatalf("çıktı %q", got)
	}
}

// Kural: Exec'in programının geçici_klasör() ile açtığı klasör test sürerken durur, test bitince
// silinir (18.14). Temizlik yokken trrun'u kullanan Go testleri her koşuda %TEMP%'te klasör
// bırakıyordu (TestGPUInPlaceCompoundAssign iki klasör, 2026-11-14).
func TestExecRemovesTempDirsWhenTestEnds(t *testing.T) {
	interp.AddEngine(files.Register) // geçici_klasör() files motorunun yerleşiğidir
	var dir string
	t.Run("program", func(t *testing.T) {
		dir = strings.TrimSpace(MustExec(t, "TR_print(TR_temp_dir())\n").Out)
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("klasör test sürerken durmalı: %v", err)
		}
	})
	if dir == "" {
		t.Fatal("programın klasörü alınamadı")
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("test bitince klasör kalmamalı: %s (%v)", dir, err)
	}
}
