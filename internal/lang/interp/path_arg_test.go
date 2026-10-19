package interp

import (
	"strings"
	"testing"
)

// denemeYolu YolDegerli'yi uygulayan bir dış değerdir (ileride standart kitaplığın Yol tipi).
type denemeYolu struct{ yol string }

func (d *denemeYolu) Type() ValueType  { return "DenemeYolu" }
func (d *denemeYolu) Inspect() string  { return d.yol }
func (d *denemeYolu) IsTruthy() bool   { return true }
func (d *denemeYolu) FilePath() string { return d.yol }

func TestYolArgumani(t *testing.T) {
	for _, d := range []Value{&StringVal{Val: "veri/a.txt"}, &denemeYolu{yol: "veri/a.txt"}} {
		yol, ok, err := PathArg("dosya_oku()", d)
		if !ok || err != nil || yol != "veri/a.txt" {
			t.Errorf("%T: (%q, %v, %v)", d, yol, ok, err)
		}
	}
	if _, ok, _ := PathArg("dosya_oku()", NewInt(5)); ok {
		t.Error("Tam bir yol sayılmamalı")
	}
	testler := map[string]string{
		"C:\tokenizer.json":     "dosya yolunda sekme karakteri var; metindeki ters bölü bir kaçış dizisi sayıldı",
		"D:\new\x0dapor.txt":    "dosya yolunda satır sonu karakteri var",
		"a\tb\nc":               "ham metin kullanın: `a\\tb\\nc`",
		"C:\\veri\\yeni\tx.trt": "`C:\\veri\\yeni\\tx.trt`",
	}
	for yol, beklenen := range testler {
		_, ok, err := PathArg("dosya_oku()", &denemeYolu{yol: yol})
		if !ok || err == nil || !strings.Contains(err.Error(), beklenen) || !strings.HasPrefix(err.Error(), "dosya_oku(): ") {
			t.Errorf("%q: %q içeren hata bekleniyordu, alınan: %v", yol, beklenen, err)
		}
	}
}
