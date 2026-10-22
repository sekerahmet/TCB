package trrun

import "testing"

// Kural: Exec programı yeni bir VM'de çalıştırır; yazdır() çıktısını, VM'i ve programın hatasını
// verir (hata testi düşürmez); MustExec hatada düşürür; MustExecChecked tip denetiminden geçer.
func TestExecCapturesOutputVMAndError(t *testing.T) {
	o := Exec(t, "değişken x = 41\nyazdır(x + 1)\n")
	if o.Err != nil || o.Out != "42\n" {
		t.Fatalf("çıktı %q, hata %v", o.Out, o.Err)
	}
	if v, ok := o.VM.GlobalEnv.Get("x"); !ok || v.Inspect() != "41" {
		t.Fatalf("genel değişken x = %v (%v)", v, ok)
	}
	if bad := Exec(t, "fırlat \"x\"\n"); bad.Err == nil {
		t.Fatal("fırlatılan hata dönmedi")
	}
	if got := MustExecChecked(t, "yazdır(2)\n").Out; got != "2\n" {
		t.Fatalf("çıktı %q", got)
	}
}
