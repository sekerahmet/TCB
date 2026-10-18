package tensor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/vm"
)

// programdan kodu, programın klasörü klasor olacak biçimde (çalışma klasörü değişmeden)
// çalıştırır.
func programdan(t *testing.T, klasor, kod string) (string, error) {
	t.Helper()
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := vm.New()
	m.SourceFile = filepath.Join(klasor, "program.tr")
	var out bytes.Buffer
	m.Out = &out
	_, err := m.Run(prog)
	return out.String(), err
}

// model_yükle, model.kaydet, tensör_kaydet ve tensör_yükle göreli yolları TR'nin tek yol
// kuralıyla (vm.YolCoz) çözer: klasörü çalışma klasöründe olmayan bir yol programın
// klasörüne göre yazılır ve okunur.
func TestDosyaIslemleriGoreliYol(t *testing.T) {
	klasor := t.TempDir()
	const alt = "tr_yol_testi_alt_klasoru"
	if err := os.Mkdir(filepath.Join(klasor, alt), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(alt); err == nil {
		t.Fatalf("%s çalışma klasöründe olmamalı", alt)
	}
	out, err := programdan(t, klasor, `
değişken model = sinir_ağı([2, 3, 1], tohum: 1)
model.kaydet("`+alt+`/model.json")
yazdır(model_yükle("`+alt+`/model.json").tahmin_et([[0, 1]]).şekil)
tensör_kaydet("`+alt+`/t.trt", {"w": tensör([1.5, 2.5])})
yazdır(tensör_yükle("`+alt+`/t.trt")["w"].değer)
`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[1, 1]\n[1.5, 2.5]" {
		t.Fatalf("beklenmeyen çıktı: %q", out)
	}
	for _, ad := range []string{"model.json", "t.trt"} {
		if _, err := os.Stat(filepath.Join(klasor, alt, ad)); err != nil {
			t.Errorf("%s programın klasörüne yazılmalıydı: %v", ad, err)
		}
	}
}

// Dosya hataları diğer motorlardaki gibi Türkçe anlatılır; işletim sisteminin İngilizce
// metni sızmaz, okumada bakılan yerler gösterilir.
func TestDosyaIslemleriTurkceHata(t *testing.T) {
	klasor := t.TempDir()
	for _, d := range []struct{ kod, beklenen string }{
		{`model_yükle("yok_12345.json")`, `model_yükle: "yok_12345.json" okunamadı: böyle bir dosya ya da klasör yok`},
		{`tensör_yükle("yok_12345.trt")`, `tensör_yükle: "yok_12345.trt" yüklenemedi: böyle bir dosya ya da klasör yok`},
		{`sinir_ağı([2, 1]).kaydet("olmayan_klasör_12345/m.json")`, `model.kaydet: "olmayan_klasör_12345/m.json" kaydedilemedi: "olmayan_klasör_12345" klasörü yok`},
		{`tensör_kaydet("olmayan_klasör_12345/t.trt", {"w": tensör([1])})`, `tensör_kaydet: "olmayan_klasör_12345/t.trt" yazılamadı: "olmayan_klasör_12345" klasörü yok`},
		{`model_yükle(5)`, `model_yükle: dosya yolu metin olmalıdır`},
	} {
		_, err := programdan(t, klasor, d.kod)
		if err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s: %v (içermeli: %s)", d.kod, err, d.beklenen)
			continue
		}
		if strings.Contains(d.kod, "yok_12345") && !strings.Contains(err.Error(), "bakılan yerler: ") {
			t.Errorf("%s: bakılan yerler gösterilmeli: %v", d.kod, err)
		}
		if strings.Contains(err.Error(), "cannot") || strings.Contains(err.Error(), "system") || strings.Contains(err.Error(), "no such") {
			t.Errorf("%s: işletim sisteminin İngilizce metni sızmamalı: %v", d.kod, err)
		}
	}
}
