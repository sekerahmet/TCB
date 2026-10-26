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

// Öğrenme yolu: examples/tr/NN_ad.tr ve examples/en/NN_name.tr aynı programın iki yazımıdır.
// Türkçe dosya düz Türkçe, İngilizce dosya kodlu saklanır (17.7: başka dilde yazılan metin
// yalnızca kodlanarak saklanır; `tr göster --dil en` onu İngilizce gösterir).

// errorsDir hata örneklerinin klasör adıdır (behavior_example_errors_test.go).
var errorsDir = map[string]string{"tr": "hatalar", "en": "errors"}

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

// Her Türkçe örneğin İngilizce eşi vardır ve tersi; numaralar 01'den boşluksuz gider, dosyanın
// ilk satırı kendi numarasını taşır, helpers/ klasörleri aynı sayıdadır.
func TestExamplesPaired(t *testing.T) {
	turkish, english := exampleFiles(t, "tr"), exampleFiles(t, "en")
	if len(turkish) < 20 {
		t.Fatalf("yalnızca %d Türkçe örnek var", len(turkish))
	}
	var numbers []string
	for n := range turkish {
		numbers = append(numbers, n)
		if _, ok := english[n]; !ok {
			t.Errorf("examples/tr/%s'in examples/en'de eşi yok", turkish[n])
		}
	}
	for n, name := range english {
		if _, ok := turkish[n]; !ok {
			t.Errorf("examples/en/%s'in examples/tr'de eşi yok", name)
		}
	}
	sort.Strings(numbers)
	for i, n := range numbers {
		if want := fmt.Sprintf("%02d", i+1); n != want {
			t.Errorf("numaralar boşluksuz değil: %d. örnek %s olmalı, %s var", i+1, want, n)
			break
		}
	}
	for _, lang := range []string{"tr", "en"} {
		files := exampleFiles(t, lang)
		for n, name := range files {
			first := strings.SplitN(readExample(t, lang, name), "\n", 3)
			title := first[0]
			if strings.HasPrefix(title, "//! tr kodlu") {
				title = first[1] // ilk satır kodlu başlık (iki dilde de dosya kodlu saklanır)
			}
			if !strings.HasPrefix(title, "// "+n+" —") {
				t.Errorf("examples/%s/%s: ilk yorum satırı %q ile başlamalı", lang, name, "// "+n+" —")
			}
		}
	}
	trHelpers, _ := reporoot.Glob(reporoot.Path("examples", "tr", "helpers", "*.tr"))
	enHelpers, _ := reporoot.Glob(reporoot.Path("examples", "en", "helpers", "*.tr"))
	if len(trHelpers) != len(enHelpers) {
		t.Errorf("helpers/ dosya sayıları farklı: tr %d, en %d", len(trHelpers), len(enHelpers))
	}
	// Türkçe örnekler de Türkçe yazılmış kodlu dosyalardır (17.1: dil=tr); disk her dilde kodludur.
	if header := source.Header + "tr\n"; !strings.HasPrefix(readExample(t, "tr", turkish["01"]), header) {
		t.Errorf("Türkçe örnekler %q başlığıyla kodlu saklanır", header)
	}
	for n, name := range english {
		// İngilizce örnekler İngilizce yazılmış kodlu dosyalardır (17.1: dil=en).
		if header := source.Header + "en\n"; !strings.HasPrefix(readExample(t, "en", name), header) {
			t.Errorf("examples/en/%s kodlu saklanmalı (%q ile başlamalı); %s numaralı örnek", name, header, n)
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

// İki yazım aynı programdır: kodlu iskeletleri (kodlar, sayılar, işleçler, yerleştirmeler)
// bire bir aynıdır; yalnızca kullanıcı adları, yorumlar ve metin düz yazıları farklıdır.
func TestExamplesSameStructure(t *testing.T) {
	turkish, english := exampleFiles(t, "tr"), exampleFiles(t, "en")
	for n, name := range turkish {
		enName, ok := english[n]
		if !ok {
			continue
		}
		coded, err := source.Encode(readExample(t, "tr", name))
		if err != nil {
			t.Errorf("examples/tr/%s kodlanamadı: %v", name, err)
			continue
		}
		a, b := structure(coded), structure(readExample(t, "en", enName))
		if i := firstDifference(a, b); i >= 0 {
			t.Errorf("%s ile %s farklı program: %d. sözcük birimi (tr %v / en %v)", name, enName, i, around(a, i), around(b, i))
		}
	}
	trHelpers, _ := reporoot.Glob(reporoot.Path("examples", "tr", "helpers", "*.tr"))
	for _, p := range trHelpers {
		coded, _ := source.Encode(readExample(t, "tr", filepath.Join("helpers", filepath.Base(p))))
		enHelpers, _ := reporoot.Glob(reporoot.Path("examples", "en", "helpers", "*.tr"))
		found := false
		for _, q := range enHelpers {
			if firstDifference(structure(coded), structure(readExample(t, "en", filepath.Join("helpers", filepath.Base(q))))) < 0 {
				found = true
			}
		}
		if !found {
			t.Errorf("%s'in iskeletiyle eşleşen bir examples/en/helpers dosyası yok", filepath.Base(p))
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

func around(tokens []string, i int) []string {
	lo, hi := max(i-3, 0), min(i+4, len(tokens))
	return tokens[lo:hi]
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

// runExample örneği kendi (temiz) kök klasöründe çalıştırır: çıkış kodu 0, uyarı yok.
func runExample(t *testing.T, root, lang, name string) []string {
	t.Helper()
	// Her yazım kendi dilinin etkin olduğu ortamda koşar (VS Code'da tr.yazimDili): yerleşik adı
	// gölgeleyen kullanıcı adı uyarısı etkin dile bağlıdır (input, sample ... İngilizce yerleşiklerdir).
	s := testkit.RunBinary(t, trBinary, root, "", []string{"TR_DIL=" + lang}, filepath.ToSlash(filepath.Join("examples", lang, name)))
	if s.Code != 0 {
		t.Fatalf("examples/%s/%s çıkış kodu %d\n%s%s", lang, name, s.Code, s.Stdout, s.Stderr)
	}
	if strings.Contains(s.Stderr, "Uyar") || strings.Contains(s.Stderr, "Warning") {
		t.Errorf("examples/%s/%s uyarı verdi: %s", lang, name, s.Stderr)
	}
	return numberBag(s.Stdout+s.Stderr, root)
}

// Çiftler aynı programı çalıştırır: ikisi de 0 koduyla biter, uyarı vermez ve çıktılarının
// sayıları aynıdır. Her yazım kendi temiz kopyasında koşar (koşu klasörleri çakışmaz).
func TestExamplesSameOutput(t *testing.T) {
	turkish, english := exampleFiles(t, "tr"), exampleFiles(t, "en")
	trRoot, enRoot := t.TempDir(), t.TempDir()
	for _, k := range []string{"examples", "tests", "docs", "lib"} {
		testkit.CopyDir(t, reporoot.Path(k), filepath.Join(trRoot, k))
		testkit.CopyDir(t, reporoot.Path(k), filepath.Join(enRoot, k))
	}
	for n, name := range turkish {
		enName, ok := english[n]
		if !ok {
			continue
		}
		t.Run(n, func(t *testing.T) {
			// İki yazım ayrı kopyalarda aynı anda koşar.
			a, b := pair(t,
				func(t *testing.T) []string { return runExample(t, trRoot, "tr", name) },
				func(t *testing.T) []string { return runExample(t, enRoot, "en", enName) })
			if i := firstDifference(a, b); i >= 0 {
				t.Errorf("%s ile %s farklı sayılar üretti (%d. sayı)\ntr: %v\nen: %v", name, enName, i, a, b)
			}
		})
	}
}

// Kök README'deki örnek tablosu her örneği (iki yazımıyla) ve her hata örneğini listeler.
func TestExamplesListedInReadme(t *testing.T) {
	data, err := os.ReadFile(reporoot.Path("README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	for _, lang := range []string{"tr", "en"} {
		for _, name := range exampleFiles(t, lang) {
			if !strings.Contains(readme, "(examples/"+lang+"/"+name+")") {
				t.Errorf("README.md örnek tablosunda examples/%s/%s yok", lang, name)
			}
		}
	}
	for lang, dir := range errorsDir {
		for _, p := range errorExamples(t, lang, dir) {
			if !strings.Contains(readme, "(examples/"+lang+"/"+dir+"/"+filepath.Base(p)+")") {
				t.Errorf("README.md hata örnekleri tablosunda examples/%s/%s/%s yok", lang, dir, filepath.Base(p))
			}
		}
	}
}
