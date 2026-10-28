package interp

import (
	"strings"
	"testing"
)

// testPath YolDegerli'yi uygulayan bir dış değerdir (ileride standart kitaplığın Yol tipi).
type testPath struct{ filePath string }

func (d *testPath) Type() ValueType  { return "DenemeYolu" }
func (d *testPath) Inspect() string  { return d.filePath }
func (d *testPath) IsTruthy() bool   { return true }
func (d *testPath) FilePath() string { return d.filePath }

func TestPathArg(t *testing.T) {
	for _, d := range []Value{&StringVal{Val: "veri/a.txt"}, &testPath{filePath: "veri/a.txt"}} {
		filePath, ok, err := PathArg("dosya_oku()", d)
		if !ok || err != nil || filePath != "veri/a.txt" {
			t.Errorf("%T: (%q, %v, %v)", d, filePath, ok, err)
		}
	}
	if _, ok, _ := PathArg("dosya_oku()", NewInt(5)); ok {
		t.Error("Tam bir yol sayılmamalı")
	}
	tests := map[string]string{
		"C:\tokenizer.json":     "dosya yolunda sekme karakteri var; metindeki ters bölü bir kaçış dizisi sayıldı",
		"D:\new\x0dapor.txt":    "dosya yolunda satır sonu karakteri var",
		"a\tb\nc":               "ham metin kullanın: `a\\tb\\nc`",
		"C:\\veri\\yeni\tx.trt": "`C:\\veri\\yeni\\tx.trt`",
	}
	for filePath, expected := range tests {
		_, ok, err := PathArg("dosya_oku()", &testPath{filePath: filePath})
		if !ok || err == nil || !strings.Contains(err.Error(), expected) || !strings.HasPrefix(err.Error(), "dosya_oku(): ") {
			t.Errorf("%q: %q içeren hata bekleniyordu, alınan: %v", filePath, expected, err)
		}
	}
}
