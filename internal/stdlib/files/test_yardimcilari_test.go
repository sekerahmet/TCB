package files

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/stdlib/formats"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Testlerdeki VM'ler bu paketin ve yol alan öteki motorların yerleşiklerini içersin.
func init() {
	interp.MotorEkle(formats.Kaydet)
	interp.MotorEkle(ml.Kaydet)
	interp.MotorEkle(Kaydet)
}

// sonuc bir TR programının çalışmasıdır.
type sonuc struct {
	cikti, uyari string
	deger        interp.Value
	hata         error
}

// trCalistir kodu program klasörü programKlasoru olan bir program gibi çalıştırır (çalışma
// klasörü testin ayarladığıdır).
func trCalistir(t *testing.T, programKlasoru, kod string) sonuc {
	t.Helper()
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), kod)
	}
	var out, hata bytes.Buffer
	m := interp.New()
	m.Out, m.Err = &out, &hata
	if programKlasoru != "" {
		m.SourceFile = filepath.Join(programKlasoru, "program.tr")
	}
	d, err := m.Run(prog)
	m.WaitAsync()
	return sonuc{cikti: out.String(), uyari: hata.String(), deger: d, hata: err}
}

// tamam programı çalıştırır ve hatasız bitmesini bekler; son değerin görünüşünü verir.
func tamam(t *testing.T, kod string) string {
	t.Helper()
	s := trCalistir(t, "", kod)
	if s.hata != nil {
		t.Fatalf("beklenmeyen hata: %v\n%s", s.hata, kod)
	}
	return s.deger.Inspect()
}

// hataIcerir programın, verilen parçaları içeren bir hatayla durmasını bekler.
func hataIcerir(t *testing.T, kod string, parcalar ...string) {
	t.Helper()
	s := trCalistir(t, "", kod)
	if s.hata == nil {
		t.Fatalf("hata bekleniyordu: %s", kod)
	}
	for _, p := range parcalar {
		if !strings.Contains(s.hata.Error(), p) {
			t.Errorf("%s\n  hata %q içermeli: %v", kod, p, s.hata)
		}
	}
}

// geciciCalisma çalışma klasörünü test bitene kadar yeni bir geçici klasör yapar.
func geciciCalisma(t *testing.T) string {
	t.Helper()
	k := t.TempDir()
	t.Chdir(k)
	t.Cleanup(TumunuKapat) // açık kalan satır okuyucuları (Windows'ta klasör silinemezdi)
	return k
}

func yaz(t *testing.T, yol, icerik string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(yol), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yol, []byte(icerik), 0o644); err != nil {
		t.Fatal(err)
	}
}

func varMi(yol string) bool {
	_, err := os.Lstat(yol)
	return err == nil
}

// klasorBaglantisi hedef klasörü gösteren bir klasör bağlantısı kurar: önce sembolik
// bağlantı, olmazsa (Windows'ta yetki gerekir) dizin bağlantısı (junction). İkisi de
// kurulamazsa test atlanır.
func klasorBaglantisi(t *testing.T, hedef, ad string) {
	t.Helper()
	if err := os.Symlink(hedef, ad); err == nil {
		return
	}
	if runtime.GOOS != "windows" {
		t.Skip("sembolik bağlantı kurulamadı")
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", ad, hedef).CombinedOutput(); err != nil {
		t.Skipf("dizin bağlantısı kurulamadı: %v %s", err, out)
	}
}

// ileriYol yolun TR metninde yazılışıdır (/ ile).
func ileriYol(yol string) string { return filepath.ToSlash(yol) }
