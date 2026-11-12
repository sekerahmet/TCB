package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (15.45, 15.6; T1 dilim 2a): TR_replay kaydında tam sayıya dönüşüm kaydedilir (bugüne kadar
// konağa okuduğu için kayıt H4065 veriyordu); hatası oynatmanın sonunda okunan cihaz içi
// bayraktan gelir ve ileti kaydedilen satırı söyler, doğrudan çağrının iletisiyle aynı. Hatalı
// oynatmadan sonra kayıt kullanılır kalır. Tip çağrısı (TR_Tensor[TR_Int], ondalıktan) ve
// .TR_to_dtype (tam sayı daraltması) iki yerleşik yoldur.
func TestReplayConversionFaultNamesRecordedLine(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, conv := range []string{`TR_Tensor[TR_Int](y).TR_to_dtype("int8")`, `TR_Tensor[TR_Int](y, TR_dtype: "int8")`} {
		program := `TR_default_device("gpu")
TR_func g(x: TR_Tensor) -> TR_Tensor[TR_Int] {
    TR_var y = x * 2.0
    TR_return ` + conv + `
}
TR_var f = TR_replay(g)
TR_no_grad {
    TR_print(f(TR_Tensor([1.0, 2.0])).TR_to_device("cpu").TR_tolist)
    TR_print(f(TR_Tensor([3.0, -4.0])).TR_to_device("cpu").TR_tolist)
    TR_print(f(TR_Tensor([5.0, 6.0])).TR_to_device("cpu").TR_tolist)
    TR_try { f(TR_Tensor([7.0, 100.0])) } TR_catch h { TR_print("hata:", h.TR_code) }
    TR_print(f(TR_Tensor([8.0, 9.0])).TR_to_device("cpu").TR_tolist)
    f(TR_zeros([2]) / TR_zeros([2]))
}`
		out, err := run(t, program)
		want := "[2, 4]\n[6, -8]\n[10, 12]\nhata: H6964\n[16, 18]\n"
		if out != want {
			t.Fatalf("%s: çıktı\n%s\nbeklenen\n%s(hata %v)", conv, out, want, err)
		}
		line, _, ok := messages.Pos(err)
		if !messages.Has(err, messages.H6964) || !ok || line != 4 {
			t.Fatalf("%s: NaN'ın iletisi kaydedilen satırı (4) söylemeli: satır %d, %v", conv, line, err)
		}
		direct, derr := run(t, strings.Replace(program, "TR_var f = TR_replay(g)", "TR_var f = g", 1))
		if direct != want || derr == nil || derr.Error() != err.Error() {
			t.Fatalf("%s: doğrudan çağrı başka hata verdi:\n%s\n%v\nkayıtta %v", conv, direct, derr, err)
		}
	}
}

// Kural (15.41; T1 dilim 2a): kartta tam sayıya dönüşüm konaktan geçmez (eskiden tensör konağa
// kopyalanıp işlemcide çevriliyordu); hata denetimi cihaz içi bayrakladır.
func TestConvertDoesNotGoThroughHost(t *testing.T) {
	gpu.RegisterSimulator()
	x := tensor.NewOn(tensor.GPU(0), []float32{1.5, 2.5, 3}, 3)
	defer x.Drop()
	transfers := func() int64 {
		var n int64
		for _, r := range tensor.CounterSnapshot() {
			if r.OpKind == tensor.OpKindToHost || r.OpKind == tensor.OpKindToDevice {
				n += r.Calls
			}
		}
		return n
	}
	before := transfers()
	for _, dt := range []tensor.DType{tensor.Int64, tensor.Int8, tensor.UInt32} {
		r, err := convertKind(nil, "test", x, dt)
		if err != nil || r.Device() != tensor.GPU(0) {
			t.Fatalf("%s: %v, %v", dt, r, err)
		}
		r.Drop()
	}
	if got := transfers() - before; got != 0 {
		t.Fatalf("kartta dönüşüm %d konak aktarımı yaptı, 0 bekleniyordu", got)
	}
	_, err := convertKind(nil, "test", tensor.NewOn(tensor.GPU(0), []float32{1, 300}, 2), tensor.Int8)
	if !messages.Has(err, messages.H6964) {
		t.Fatalf("sığmama H6964 bekleniyordu: %v", err)
	}
}

// Kural (15.45, A12; T1 dilim 2a): kayıtta indeksli atamanın daraltan dönüşümü (`t[i] = x`, int64 →
// int8) de kaydedilir; hatanın iletisi atamanın kayıttaki satırını söyler (yorumlayıcı atama
// kancasına görevi ve satırı verir), doğrudan çağrının iletisiyle aynı.
func TestReplayIndexAssignFaultNamesRecordedLine(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	program := `TR_default_device("gpu")
TR_func g(t: TR_Tensor[TR_Int], x: TR_Tensor[TR_Int]) -> TR_Tensor[TR_Int] {
    t[0..2] = x
    TR_return t.TR_to_dtype("int16")
}
TR_var f = TR_replay(g)
TR_var t = TR_Tensor([0, 0], TR_dtype: "int8")
TR_no_grad {
    TR_print(f(t, TR_Tensor([1, 2])).TR_to_device("cpu").TR_tolist)
    TR_print(f(t, TR_Tensor([3, -4])).TR_to_device("cpu").TR_tolist)
    TR_print(f(t, TR_Tensor([5, 6])).TR_to_device("cpu").TR_tolist)
    f(t, TR_Tensor([7, 300]))
}`
	out, err := run(t, program)
	want := "[1, 2]\n[3, -4]\n[5, 6]\n"
	line, _, ok := messages.Pos(err)
	if out != want || !messages.Has(err, messages.H6964) || !ok || line != 3 {
		t.Fatalf("çıktı\n%s\nhata %v (satır %d); beklenen\n%sve satır 3'te H6964", out, err, line, want)
	}
	direct, derr := run(t, strings.Replace(program, "TR_var f = TR_replay(g)", "TR_var f = g", 1))
	if direct != want || derr == nil || derr.Error() != err.Error() {
		t.Fatalf("doğrudan çağrı başka hata verdi:\n%s\n%v\nkayıtta %v", direct, derr, err)
	}
}
