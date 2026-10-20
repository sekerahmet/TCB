package ml

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// fromProgram kodu, programın klasörü klasor olacak biçimde (çalışma klasörü değişmeden)
// çalıştırır.
func fromProgram(t *testing.T, dir, code string) (string, error) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	m.SourceFile = filepath.Join(dir, "program.tr")
	var out bytes.Buffer
	m.Out = &out
	_, err := m.Run(prog)
	return out.String(), err
}

// tensör_kaydet ve tensör_yükle göreli yolları TR'nin tek yol
// kuralıyla (interp.YolCoz) çözer: klasörü çalışma klasöründe olmayan bir yol programın
// klasörüne göre yazılır ve okunur.
func TestFileOpsRelativePath(t *testing.T) {
	dir := t.TempDir()
	const sub = "tr_yol_testi_alt_klasoru"
	if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sub); err == nil {
		t.Fatalf("%s çalışma klasöründe olmamalı", sub)
	}
	out, err := fromProgram(t, dir, `
tensör_kaydet("`+sub+`/t.trt", {"w": tensör([1.5, 2.5])})
yazdır(tensör_yükle("`+sub+`/t.trt")["w"].değer)
`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[1.5, 2.5]" {
		t.Fatalf("beklenmeyen çıktı: %q", out)
	}
	for _, ad := range []string{"t.trt"} {
		if _, err := os.Stat(filepath.Join(dir, sub, ad)); err != nil {
			t.Errorf("%s programın klasörüne yazılmalıydı: %v", ad, err)
		}
	}
}

// Dosya hataları diğer motorlardaki gibi Türkçe anlatılır; işletim sisteminin İngilizce
// metni sızmaz, okumada bakılan yerler gösterilir.
func TestFileOpsTurkishError(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`tensör_yükle("yok_12345.trt")`, messages.H2107},
		{`tensör_kaydet("olmayan_klasör_12345/t.trt", {"w": tensör([1])})`, messages.H2101},
	} {
		_, err := fromProgram(t, dir, d.code)
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
			continue
		}
		if strings.Contains(d.code, "yok_12345") && !strings.Contains(err.Error(), "bakılan yerler: ") {
			t.Errorf("%s: bakılan yerler gösterilmeli: %v", d.code, err)
		}
		if strings.Contains(err.Error(), "cannot") || strings.Contains(err.Error(), "system") || strings.Contains(err.Error(), "no such") {
			t.Errorf("%s: işletim sisteminin İngilizce metni sızmamalı: %v", d.code, err)
		}
	}
}
