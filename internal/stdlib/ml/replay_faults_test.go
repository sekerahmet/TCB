package ml

import (
	"fmt"
	"math"
	"strconv"
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

// Kural (15.45, 15.8, 15.23; T1 dilim 2c): kayıtta gömmenin tensör indeksi ve dilim yazmanın
// tensör başlangıcı kırpılmaz: tam sayı olmayan ya da sınır dışı değer cihaz içi bayrağa yazar,
// ileti oynatmanın sonunda, kaydedilen satırla ve doğrudan çağrınınkiyle aynı (H6012, H6305,
// H6441, H6442). Hatalı oynatmadan sonra hatasız oynatma doğrudur. float64 indeks kendi türünde
// denetlenir: 2,0000001 kayıtta da tam sayı değildir (davranış değişikliği; eskiden float32'ye
// inip 2. satırı sessizce verirdi).
func TestReplayIndexFaultsMatchDirectCall(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	lookup := `TR_default_device("gpu")
TR_var tablo = TR_Tensor([[1.0, 2.0], [3.0, 4.0], [5.0, 6.0], [7.0, 8.0]])
TR_func g(i: TR_Tensor) -> TR_Tensor {
    TR_return TR_embedding_lookup(tablo, i)
}
`
	write := `TR_default_device("gpu")
TR_var c = TR_zeros([2, 4])
TR_func g(k: TR_Tensor, s: TR_Tensor) -> TR_Tensor {
    TR_write_slice(c, s, TR_dim: 1, TR_start: k)
    TR_return c * 1.0
}
`
	ws := func(k string) string { return k + ", TR_ones([2, 2])" }
	for _, c := range []struct {
		head      string
		good, bad []string
		code      messages.Code
	}{
		{lookup, []string{"TR_Tensor([1.0, 2.0])", "TR_Tensor([3.0, 0.0])", "TR_Tensor([2.0, 1.0])"}, []string{"TR_Tensor([1.0, 2.5])"}, messages.H6012},
		{lookup, []string{"TR_Tensor([1.0, 2.0])", "TR_Tensor([3.0, 0.0])", "TR_Tensor([2.0, 1.0])"}, []string{"TR_Tensor([9.0, 2.5])", "TR_Tensor([0.0, -1.0])"}, messages.H6305},
		{lookup, []string{`TR_Tensor([1.0, 2.0], TR_dtype: "float64")`, `TR_Tensor([3.0, 0.0], TR_dtype: "float64")`, `TR_Tensor([2.0, 1.0], TR_dtype: "float64")`}, []string{`TR_Tensor([1.0, 2.0000001], TR_dtype: "float64")`}, messages.H6012},
		{lookup, []string{"TR_Tensor([1, 2])", "TR_Tensor([3, 0])", "TR_Tensor([2, 1])"}, []string{"TR_Tensor([1, 4])"}, messages.H6305},
		{write, []string{ws("TR_Tensor(0.0)"), ws("TR_Tensor(1.0)"), ws("TR_Tensor(2.0)")}, []string{ws("TR_Tensor(3.0)"), ws("TR_Tensor(-1.0)")}, messages.H6442},
		{write, []string{ws("TR_Tensor(0.0)"), ws("TR_Tensor(1.0)"), ws("TR_Tensor(2.0)")}, []string{ws("TR_Tensor(1.5)")}, messages.H6441},
		{write, []string{ws("TR_Tensor(0)"), ws("TR_Tensor(1)"), ws("TR_Tensor(2)")}, []string{ws("TR_Tensor(3)")}, messages.H6442},
	} {
		for _, bad := range c.bad {
			// İleti programın içinde yazılır: H6441 başlangıç tensörünü gösterir, program bitince
			// tensörler bırakılır (hata sonradan yazılamaz).
			show := func(arg string) string { return `    TR_print(f(` + arg + `).TR_to_device("cpu").TR_tolist)` + "\n" }
			program := c.head + "TR_var f = TR_replay(g)\nTR_no_grad {\n" + show(c.good[0]) + show(c.good[1]) +
				"    TR_try { f(" + bad + ") } TR_catch h { TR_print(\"hata:\", h.TR_code, h) }\n" + show(c.good[2]) +
				"    f(" + bad + ")\n}"
			out, err := run(t, program)
			direct, derr := run(t, strings.Replace(program, "TR_var f = TR_replay(g)", "TR_var f = g", 1))
			for name, e := range map[string]error{"kayıtta": err, "doğrudan": derr} {
				if line, _, ok := messages.Pos(e); !messages.Has(e, c.code) || !ok || line != 4 {
					t.Fatalf("%s %s: satır 4'te %s bekleniyordu, satır %d (%v)", name, bad, c.code, line, ok)
				}
			}
			if !strings.Contains(out, "hata: "+c.code.String()+" ") || direct != out {
				t.Fatalf("%s: kayıtta ve doğrudan çağrıda çıktı ve ileti aynı olmalı:\nkayıtta\n%s\ndoğrudan\n%s", bad, out, direct)
			}
		}
	}
}

// Kural (15.8, 15.45; DENETİM 499 N2, T1 dilim 2b): dilim yazmanın sınır denetimi taşmaz. 2⁶³'e
// yakın başlangıçta başlangıç + n toplamı eksiye dönüyordu: doğrudan çağrı hiçbir şey yazmadan
// hatasız geçiyordu, kayıttaki kart H6442 veriyordu. Başlangıç sayı ya da 0 boyutlu int64 tensör,
// n = 1 ve 2; boyut − n yazar, boyut − n + 1 ve 2⁶³ − n, 2⁶³ − 1 H6442 verir, kayıtta ve
// doğrudan çağrıda aynı iletiyle ve satırla.
func TestWriteSliceStartNearMaxInt64IsOutOfRange(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	const size = 4
	for _, n := range []int{1, 2} {
		src := fmt.Sprintf("TR_ones([2, %d])", n)
		for _, c := range []struct {
			start string
			bad   bool
		}{
			{strconv.Itoa(size - n), false},
			{strconv.Itoa(size - n + 1), true},
			{strconv.Itoa(math.MaxInt64 - n + 1), true},
			{strconv.Itoa(math.MaxInt64), true},
		} {
			number := `TR_default_device("gpu")
TR_var c = TR_zeros([2, 4])
TR_func g(k: TR_Tensor, s: TR_Tensor) -> TR_Tensor {
    TR_write_slice(c, s, TR_dim: 1, TR_start: ` + c.start + `)
    TR_return c * k
}
`
			tensorStart := strings.Replace(number, "TR_start: "+c.start+")", "TR_start: k)", 1)
			tensorStart = strings.Replace(tensorStart, "c * k", "c * 1.0", 1)
			for _, h := range []struct{ head, k string }{{number, "TR_Tensor(1.0)"}, {tensorStart, "TR_Tensor(" + c.start + ")"}} {
				program := h.head + "TR_var f = TR_replay(g)\nTR_no_grad {\n    TR_print(f(" + h.k + ", " + src + `).TR_to_device("cpu").TR_tolist)` + "\n}"
				out, err := run(t, program)
				direct, derr := run(t, strings.Replace(program, "TR_var f = TR_replay(g)", "TR_var f = g", 1))
				name := fmt.Sprintf("n=%d, başlangıç %s (%s)", n, c.start, h.k)
				if !c.bad {
					if err != nil || derr != nil || out == "" || out != direct {
						t.Fatalf("%s: yazmalıydı: kayıtta %q %v, doğrudan %q %v", name, out, err, direct, derr)
					}
					continue
				}
				for where, e := range map[string]error{"kayıtta": err, "doğrudan": derr} {
					if line, _, ok := messages.Pos(e); !messages.Has(e, messages.H6442) || !ok || line != 4 {
						t.Fatalf("%s %s: satır 4'te H6442 bekleniyordu: %v", name, where, e)
					}
				}
				if out != direct || err.Error() != derr.Error() {
					t.Fatalf("%s: kayıtta ve doğrudan çağrıda ileti aynı olmalı:\n%v\n%v", name, err, derr)
				}
			}
		}
	}
}
