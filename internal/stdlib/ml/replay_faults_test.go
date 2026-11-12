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

// Kural (15.45, 15.6, 15.3; T1 dilim 2b): kartta "int64" aritmetiği TR_replay kaydında çalışır
// (eskiden H6969 veriyordu); taşmanın, sıfıra bölmenin ve tekli eksinin hatası oynatmanın sonunda
// bayraktan gelir, ileti kaydedilen satırı söyler ve doğrudan çağrınınkiyle aynıdır. Hatalı
// oynatmadan sonra kayıt kullanılır kalır. Tam sayı sayı işleneni (`x * 2`) kayıtta konaktan
// yükleme olduğu için işlenenler tensördür (k).
func TestReplayInt64ArithFaultNamesRecordedLine(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, c := range []struct {
		body, k, bad string
		code         messages.Code
	}{
		{"x * k", "2", "TR_Tensor([4611686018427387904, 0])", messages.H6961},
		{"x // (x - k)", "1", "TR_Tensor([1, 5])", messages.H6962},
		{"k - k - x", "1", "TR_Tensor([3, -9223372036854775807 - 1])", messages.H6961},
		{"TR_abs(x) + k", "1", "TR_Tensor([3, -9223372036854775807 - 1])", messages.H6961},
	} {
		k := "TR_Tensor(" + c.k + ")"
		program := `TR_default_device("gpu")
TR_func g(x: TR_Tensor[TR_Int], k: TR_Tensor[TR_Int]) -> TR_Tensor[TR_Int] {
    TR_var y = ` + c.body + `
    TR_return y + k
}
TR_var f = TR_replay(g)
TR_no_grad {
    TR_print(f(TR_Tensor([2, 3]), ` + k + `).TR_to_device("cpu").TR_tolist)
    TR_print(f(TR_Tensor([4, 5]), ` + k + `).TR_to_device("cpu").TR_tolist)
    TR_try { f(` + c.bad + `, ` + k + `) } TR_catch h { TR_print("hata:", h.TR_code) }
    TR_print(f(TR_Tensor([6, 7]), ` + k + `).TR_to_device("cpu").TR_tolist)
    f(` + c.bad + `, ` + k + `)
}`
		out, err := run(t, program)
		line, _, ok := messages.Pos(err)
		if !messages.Has(err, c.code) || !ok || line != 3 || !strings.Contains(out, "hata: "+c.code.String()) {
			t.Fatalf("%s: satır 3'te %s bekleniyordu: çıktı\n%s\nhata %v (satır %d)", c.body, c.code, out, err, line)
		}
		direct, derr := run(t, strings.Replace(program, "TR_var f = TR_replay(g)", "TR_var f = g", 1))
		if direct != out || derr == nil || derr.Error() != err.Error() {
			t.Fatalf("%s: doğrudan çağrı başka sonuç verdi:\n%s\n%v\nkayıtta\n%s\n%v", c.body, direct, derr, out, err)
		}
	}
	// Tekli eksinin kancası (interp.Negatable) görevi almaz: kayıtta yuvanın yeri yazılamaz, kayıt
	// açık hatayla biter (H4065), sessiz değil.
	_, err := run(t, `TR_default_device("gpu")
TR_func g(x: TR_Tensor[TR_Int]) -> TR_Tensor[TR_Int] {
    TR_return -x
}
TR_var f = TR_replay(g)
TR_no_grad {
    f(TR_Tensor([2, 3]))
    f(TR_Tensor([2, 3]))
}`)
	if !messages.Has(err, messages.H4065) {
		t.Fatalf("kayıtta tekli eksi H4065 bekleniyordu: %v", err)
	}
}

// Kural (15.45, 15.18, 15.6; T1 dilim 2b): kartta "int64" TR_sum kayıtta çalışır; sığmayan
// toplamın iletisi oynatmanın sonunda, kaydedilen satırla ve doğrudan çağrınınkiyle aynı (kesin
// değeri söyler). Hatalı oynatmadan sonra kayıt kullanılır kalır.
func TestReplayInt64SumFaultNamesRecordedLine(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	const bad = "TR_Tensor([[1, 2], [9223372036854775807, 1]])"
	program := `TR_default_device("gpu")
TR_func g(x: TR_Tensor[TR_Int]) -> TR_Tensor[TR_Int] {
    TR_var s = TR_sum(x, TR_dim: -1)
    TR_return s * x[0]
}
TR_var f = TR_replay(g)
TR_no_grad {
    TR_print(f(TR_Tensor([[1, 2], [3, 4]])).TR_to_device("cpu").TR_tolist)
    TR_print(f(TR_Tensor([[5, 6], [7, 8]])).TR_to_device("cpu").TR_tolist)
    TR_try { f(` + bad + `) } TR_catch h { TR_print("hata:", h.TR_code) }
    TR_print(f(TR_Tensor([[0, 1], [2, 3]])).TR_to_device("cpu").TR_tolist)
    f(` + bad + `)
}`
	replayMatchesDirect(t, program, "[3, 14]\n[55, 90]\nhata: H6961\n[0, 5]\n", messages.H6961, 3)
}

// replayMatchesDirect programı (içinde "TR_var f = TR_replay(g)") koşar: çıktı want olmalı,
// program code'lu hatayla line'da bitmeli; aynı program kayıtsız (f = g) aynı çıktıyı ve hatayı
// vermeli.
func replayMatchesDirect(t *testing.T, program, want string, code messages.Code, line int) {
	t.Helper()
	out, err := run(t, program)
	got, _, ok := messages.Pos(err)
	if out != want || !messages.Has(err, code) || !ok || got != line {
		t.Fatalf("çıktı\n%s\nhata %v (satır %d); beklenen\n%sve satır %d'te %s", out, err, got, want, line, code)
	}
	direct, derr := run(t, strings.Replace(program, "TR_var f = TR_replay(g)", "TR_var f = g", 1))
	if direct != out || derr == nil || derr.Error() != err.Error() {
		t.Fatalf("doğrudan çağrı başka sonuç verdi:\n%s\n%v\nkayıtta %v", direct, derr, err)
	}
}

// Kural (15.45, 15.41; T1 dilim 2b, A15): TR_topk'ın ve TR_argsort'un indeksi kartta kartta
// kurulur, TR_replay kaydında da çalışır (eskiden indeks konağa okunuyordu, kayıt H4065
// veriyordu; DENETİM 501 N2). Ardından hatalı bir dönüşüm kaydedilen satırı söyler, iletisi
// doğrudan çağrınınkiyle aynı.
func TestReplaySortIndexThenConversionFault(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	const big = "TR_Tensor([7.0, 10000000000000000000.0])"
	program := `TR_default_device("gpu")
TR_func g(x: TR_Tensor) -> TR_Tensor[TR_Int] {
    TR_var s = TR_topk(x, 2)
    TR_var o = TR_argsort(x)
    TR_var y = TR_Tensor[TR_Int](x * 2.0)
    TR_return s[1] + o + y
}
TR_var f = TR_replay(g)
TR_no_grad {
    TR_print(f(TR_Tensor([1.0, 2.0])).TR_to_device("cpu").TR_tolist)
    TR_print(f(TR_Tensor([3.0, -4.0])).TR_to_device("cpu").TR_tolist)
    TR_print(f(TR_Tensor([5.0, 6.0])).TR_to_device("cpu").TR_tolist)
    TR_try { f(` + big + `) } TR_catch h { TR_print("hata:", h.TR_code) }
    TR_print(f(TR_Tensor([8.0, 9.0])).TR_to_device("cpu").TR_tolist)
    f(` + big + `)
}`
	replayMatchesDirect(t, program, "[3, 5]\n[7, -7]\n[11, 13]\nhata: H6964\n[17, 19]\n", messages.H6964, 5)
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
