package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (15.3): yazma cinsli türler arasında dönüştürmez; TR_write_slice ve TR_load_parameters
// kodlu hata verir (iç hata değil, DENETİM 171). İletinin önerdiği yazım, kopyalanınca, kaynağı
// hedefin türüne getirir.
func TestTypedWriteSuggestionRuns(t *testing.T) {
	const (
		f64  = `TR_Tensor([0.0, 0.0], TR_dtype: "float64")`
		i64  = `TR_Tensor([0, 0])`
		i32  = `TR_Tensor([0, 0], TR_dtype: "int32")`
		mask = `TR_Tensor([TR_false, TR_false])`
	)
	cases := []struct {
		dst, src string
		code     messages.Code
		want     string
	}{
		{mask, `TR_Tensor([5, 0])`, messages.H3112, "[true, false]"},
		{mask, `TR_Tensor([1.5, 0.0], TR_dtype: "float64")`, messages.H3112, "[true, false]"},
		{i64, `TR_Tensor([TR_true, TR_false])`, messages.H3113, "[1, 0]"},
		{i32, `TR_Tensor([TR_true, TR_false])`, messages.H3113, "[1, 0]"},
		{f64, `TR_Tensor([TR_true, TR_false])`, messages.H3113, "[1.0, 0.0]"},
		{i32, `TR_Tensor([5, 6])`, messages.H3114, "[5, 6]"},
		{i64, `TR_Tensor([5, 6], TR_dtype: "int32")`, messages.H3114, "[5, 6]"},
		{i64, `TR_Tensor([5.0, 6.0], TR_dtype: "float64")`, messages.H3114, "[5, 6]"},
		{f64, `TR_Tensor([5, 6])`, messages.H3115, "[5.0, 6.0]"},
	}
	for _, c := range cases {
		prelude := "TR_var t = " + c.dst + "\nTR_var x = " + c.src + "\n"
		_, err := run(t, prelude+"TR_write_slice(t, x)")
		if !messages.Has(err, c.code) {
			t.Errorf("%s <- %s: %v, %s bekleniyordu", c.dst, c.src, err, c.code)
			continue
		}
		fix := suggestedSpelling(t, err.Error())
		if got := mustRun(t, prelude+"TR_write_slice(t, "+fix+")\nTR_print(t.TR_tolist)"); got != c.want+"\n" {
			t.Errorf("%s <- %s, %s: %q, beklenen %s", c.dst, c.src, fix, got, c.want)
		}
	}
	const param = `TR_var m = {"w": TR_Tensor([0.0, 0.0], TR_dtype: "float64", TR_requires_grad: TR_true)}
TR_var x = TR_Tensor([5, 6])
`
	_, err := run(t, param+`TR_load_parameters(m, {"w": x})`)
	if !messages.Has(err, messages.H3115) {
		t.Fatalf("TR_load_parameters: %v, H3115 bekleniyordu", err)
	}
	// messages.Has ve TR_error_code sarılmış iletinin içindeki kodu da bulur: sarma ayrıca sınanır.
	if messages.Has(err, messages.H2000) {
		t.Errorf("TR_load_parameters kodlu hatayı H2000'e sarıyor: %v", err)
	}
	fix := suggestedSpelling(t, err.Error())
	if got := mustRun(t, param+`TR_load_parameters(m, {"w": `+fix+`})`+"\n"+`TR_print(m["w"].TR_tolist)`); got != "[5.0, 6.0]\n" {
		t.Errorf("TR_load_parameters, %s: %q", fix, got)
	}
}

// Kural (20.11): TR_load_parameters'ın yazmadan önceki denetimi kopyanın kendisiyle aynı karardır:
// yazılabilen tür reddedilmez, yazılamayan denetimden geçip modeli yarım bırakmaz.
func TestCopyTypeErrorIsTheErrorOfTheCopy(t *testing.T) {
	types := []tensor.DType{
		tensor.Float32, tensor.BFloat16, tensor.Float16, tensor.Float64,
		tensor.Int64, tensor.Int32, tensor.Int16, tensor.Int8, tensor.UInt32, tensor.UInt16, tensor.UInt8,
		tensor.Bool, tensor.Q8, tensor.Q4K, tensor.Q6K,
	}
	for _, dst := range types {
		for _, src := range types {
			d, s := tensor.ZerosOf(dst, 256), tensor.ZerosOf(src, 256)
			_, err := safe(func() bool { d.CopyFrom(s); return true })
			want, _ := panicReason(err).(*messages.Error)
			got := tensor.CopyTypeError(dst, src, tensor.CPU)
			switch {
			case err == nil && got == nil:
			case err != nil && want == nil:
				t.Errorf("%s <- %s: kopya kodsuz düştü: %v", dst.Name(), src.Name(), err)
			case got == nil || want == nil || got.Error() != want.Error():
				t.Errorf("%s <- %s: denetim %v, kopya %v", dst.Name(), src.Name(), got, err)
			}
		}
	}
}

// Kural (20.11): tür hatası artık yazmadan önce çıkar; yazma sırasındaki panik (cihaz belleği)
// kodluysa kodunu korur, H2000 yalnız kodsuz nedeni taşır.
func TestLoadFailureKeepsTheCodeOfThePanic(t *testing.T) {
	fn := codes.NameRef(codes.FnLoadParameters)
	_, err := safe(func() bool { panic(messages.New(messages.H3115, "float64", "int64")) })
	if e, ok := loadFailure(fn, err).(*messages.Error); !ok || e.Code != messages.H3115 {
		t.Errorf("kodlu panik: %v", loadFailure(fn, err))
	}
	_, err = safe(func() bool { panic("kodsuz") })
	if e, ok := loadFailure(fn, err).(*messages.Error); !ok || e.Code != messages.H2000 {
		t.Errorf("kodsuz panik: %v", loadFailure(fn, err))
	}
}
