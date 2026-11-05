package ml

import (
	"testing"

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
	fix := suggestedSpelling(t, err.Error())
	if got := mustRun(t, param+`TR_load_parameters(m, {"w": `+fix+`})`+"\n"+`TR_print(m["w"].TR_tolist)`); got != "[5.0, 6.0]\n" {
		t.Errorf("TR_load_parameters, %s: %q", fix, got)
	}
}
