package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Öğrenme yolu: examples/tr/NN_ad.tr. Örnekler kodlu saklanır ve her gösterim dilinde aynı programdır
// (17.7; `tr göster --dil en` onu İngilizce gösterir): ayrı bir İngilizce yazım klasörü yoktur.

// errorsDir hata örneklerinin klasör adıdır (behavior_example_errors_test.go).
var errorsDir = map[string]string{"tr": "hatalar"}

var exampleNumber = regexp.MustCompile(`^(\d\d)_[a-z0-9_]+\.tr$`)

// exampleFiles bir yazım klasöründeki örnekleri numarasıyla verir ("01" → "01_hello.tr").
func exampleFiles(t *testing.T, lang string) map[string]string {
	t.Helper()
	entries, err := reporoot.ReadDir(reporoot.Path("examples", lang))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			if e.Name() != "helpers" && e.Name() != errorsDir[lang] {
				t.Errorf("examples/%s/%s: yalnızca helpers/ ve %s/ klasörleri olabilir", lang, e.Name(), errorsDir[lang])
			}
			continue
		}
		m := exampleNumber.FindStringSubmatch(e.Name())
		if m == nil {
			t.Errorf("examples/%s/%s: ad NN_ad.tr biçiminde değil", lang, e.Name())
			continue
		}
		if prev, dup := out[m[1]]; dup {
			t.Errorf("examples/%s: %s numarasını %s ve %s paylaşıyor", lang, m[1], prev, e.Name())
		}
		out[m[1]] = e.Name()
	}
	return out
}

func readExample(t *testing.T, lang, name string) string {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path("examples", lang, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Örneklerin numaraları 01'den boşluksuz gider, dosyanın ilk yorum satırı kendi numarasını taşır ve
// her örnek kodlu saklanır (17.1).
func TestExamplesNumbered(t *testing.T) {
	turkish := exampleFiles(t, "tr")
	if len(turkish) < 20 {
		t.Fatalf("yalnızca %d örnek var", len(turkish))
	}
	var numbers []string
	for n := range turkish {
		numbers = append(numbers, n)
	}
	sort.Strings(numbers)
	for i, n := range numbers {
		if want := fmt.Sprintf("%02d", i+1); n != want {
			t.Errorf("numaralar boşluksuz değil: %d. örnek %s olmalı, %s var", i+1, want, n)
			break
		}
	}
	for n, name := range turkish {
		text := readExample(t, "tr", name)
		if !strings.HasPrefix(text, source.Header) {
			t.Errorf("examples/tr/%s kodlu saklanmalı (%q ile başlamalı)", name, source.Header)
		}
		first := strings.SplitN(text, "\n", 3)
		if title := first[min(1, len(first)-1)]; !strings.HasPrefix(title, "# "+n+" —") {
			t.Errorf("examples/tr/%s: ilk yorum satırı %q ile başlamalı", name, "# "+n+" —")
		}
	}
}

// structure bir kodlu dosyadan programın iskeletini çıkarır: yorumlar, metin sabitlerinin
// düz yazısı ve kullanıcı adları atılır (ad → ID); kodlar, sayılar, işleçler ve metin içi
// yerleştirmelerdeki kod kalır. Aynı programın iki yazımının iskeleti aynıdır.
func structure(coded string) []string {
	// Başlık (dosyanın dili dahil) iskelete girmez: iki yazım ayrı dillerde kodludur.
	if b, err := source.Split(coded); err == nil {
		coded = b.Body
	}
	var out []string
	s := &scanner{src: []rune(coded), out: &out}
	s.code(false)
	return out
}

type scanner struct {
	src []rune
	pos int
	out *[]string
}

func (s *scanner) emit(tok string) { *s.out = append(*s.out, tok) }

// code kodu tarar; inInterpolation ise eşleşmeyen '}' yerleştirmeyi bitirir.
func (s *scanner) code(inInterpolation bool) {
	depth := 0
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case unicode.IsSpace(c):
			s.pos++
		case c == '/' && s.peek(1) == '/':
			for s.pos < len(s.src) && s.src[s.pos] != '\n' {
				s.pos++
			}
		case c == '"':
			s.str()
		case unicode.IsLetter(c) || c == '_':
			s.word()
		case unicode.IsDigit(c):
			s.number()
		case c == '}' && inInterpolation && depth == 0:
			s.pos++
			return
		default:
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
			}
			s.emit(string(c))
			s.pos++
		}
	}
}

func (s *scanner) peek(n int) rune {
	if s.pos+n < len(s.src) {
		return s.src[s.pos+n]
	}
	return 0
}

func (s *scanner) word() {
	start := s.pos
	for s.pos < len(s.src) && (unicode.IsLetter(s.src[s.pos]) || unicode.IsDigit(s.src[s.pos]) || s.src[s.pos] == '_') {
		s.pos++
	}
	if w := string(s.src[start:s.pos]); codes.IsReservedName(w) { // kimlik (TR_fn_print): dilin sözcüğü
		s.emit(w)
	} else {
		s.emit("ID")
	}
}

func (s *scanner) number() {
	start := s.pos
	for s.pos < len(s.src) && (unicode.IsDigit(s.src[s.pos]) || (s.src[s.pos] == '.' && unicode.IsDigit(s.peek(1)))) {
		s.pos++
	}
	s.emit(string(s.src[start:s.pos]))
}

// str bir metin sabitini tarar: düz yazı atılır, {...} yerleştirmeleri kod olarak taranır.
func (s *scanner) str() {
	s.emit("STR")
	s.pos++
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case '\\':
			s.pos += 2
		case '"':
			s.pos++
			return
		case '{':
			s.pos++
			s.emit("{")
			s.code(true)
			s.emit("}")
		default:
			s.pos++
		}
	}
}

func firstDifference(a, b []string) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		if i >= len(a) || i >= len(b) || a[i] != b[i] {
			return i
		}
	}
	return -1
}

var (
	exampleNumbers = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)
	exampleRunPath = regexp.MustCompile(`examples/(?:tr|en)/\S*`)
	exampleRunDir  = regexp.MustCompile(`runs/[a-z_]+`)
	// İlerleme çubuğunun kapanış satırındaki süre ("· 3 sn sürdü", İngilizcede "· took 3 s")
	// çalıştırmadan çalıştırmaya değişir.
	exampleElapsed = regexp.MustCompile(`\d+(?:[.,]\d+)? sn sürdü|took \d+(?:[.,]\d+)? s\b`)
)

// numberBag bir çıktının sayılarını sıralı verir: metinler dile göre değişir, sayılar aynı
// programda aynıdır (ilerleme çubuğu satırları, kök klasörü ve örnek yolları çıkarılır).
// Görüntüdeki ondalık ayırıcı kişinin dilindedir (TR_DIL: "2,31" / "2.31"); karşılaştırma için
// noktaya çevrilir.
func numberBag(out, root string) []string {
	out = strings.ReplaceAll(strings.ReplaceAll(out, filepath.ToSlash(root), ""), root, "")
	out = progressLine.ReplaceAllString(out, "")
	out = exampleElapsed.ReplaceAllString(out, "")
	out = exampleRunPath.ReplaceAllString(out, "")
	out = exampleRunDir.ReplaceAllString(out, "")
	bag := exampleNumbers.FindAllString(out, -1)
	for i, n := range bag {
		bag[i] = strings.Replace(n, ",", ".", 1)
	}
	sort.Strings(bag)
	return bag
}

// runExample örneği kendi (temiz) kök klasöründe display gösterim diliyle çalıştırır: çıkış kodu 0,
// uyarı yok.
func runExample(t *testing.T, root, display, name string) []string {
	t.Helper()
	s := testkit.RunBinary(t, trBinary, root, "", []string{"TR_DIL=" + display}, filepath.ToSlash(filepath.Join("examples", "tr", name)))
	if s.Code != 0 {
		t.Fatalf("examples/tr/%s (TR_DIL=%s) çıkış kodu %d\n%s%s", name, display, s.Code, s.Stdout, s.Stderr)
	}
	if strings.Contains(s.Stderr, "Uyar") || strings.Contains(s.Stderr, "Warning") {
		t.Errorf("examples/tr/%s (TR_DIL=%s) uyarı verdi: %s", name, display, s.Stderr)
	}
	return numberBag(s.Stdout+s.Stderr, root)
}

// Her örnek iki gösterim dilinde (TR_DIL=tr ve TR_DIL=en) aynı programdır: ikisi de 0 koduyla biter,
// uyarı vermez ve çıktılarının sayıları aynıdır (metinler gösterim diline göre değişir). Her dil kendi
// temiz kopyasında koşar (koşu klasörleri çakışmaz).
func TestExamplesSameOutputInEveryDisplayLanguage(t *testing.T) {
	trRoot, enRoot := t.TempDir(), t.TempDir()
	for _, k := range []string{"examples", "tests", "docs", "lib"} {
		testkit.CopyDir(t, reporoot.Path(k), filepath.Join(trRoot, k))
		testkit.CopyDir(t, reporoot.Path(k), filepath.Join(enRoot, k))
	}
	for n, name := range exampleFiles(t, "tr") {
		t.Run(n, func(t *testing.T) {
			a, b := pair(t,
				func(t *testing.T) []string { return runExample(t, trRoot, "tr", name) },
				func(t *testing.T) []string { return runExample(t, enRoot, "en", name) })
			if i := firstDifference(a, b); i >= 0 {
				t.Errorf("%s iki gösterim dilinde farklı sayılar üretti (%d. sayı)\ntr: %v\nen: %v", name, i, a, b)
			}
		})
	}
}

// examples/README.md'deki sıralı liste her örneği ve her hata örneğini listeler.
func TestExamplesListedInReadme(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("examples/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	for _, name := range exampleFiles(t, "tr") {
		if !strings.Contains(readme, "(tr/"+name+")") {
			t.Errorf("examples/README.md sıralı listesinde tr/%s TR_none", name)
		}
	}
	for lang, dir := range errorsDir {
		for _, p := range errorExamples(t, lang, dir) {
			if !strings.Contains(readme, "("+lang+"/"+dir+"/"+filepath.Base(p)+")") {
				t.Errorf("examples/README.md hata örnekleri listesinde %s/%s/%s TR_none", lang, dir, filepath.Base(p))
			}
		}
	}
}
