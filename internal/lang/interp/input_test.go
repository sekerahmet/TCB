package interp

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Kural (13.1): girdi() girdinin sonunu bir kez TR_none ile bildirir; sonraki çağrı H2561'dir
// (Python'un EOFError'ı gibi). Boş satıra kadar okuyan döngü (s == "") girdi bitince sessizce
// dönmez, TR_none'dan sonraki çağrıda H2561 ile durur (DENETİM 400, p3.tr). İletinin önerdiği
// yazım (TR_none'ı sınayıp çıkmak; tr ve en iletisinden kopyalanır) girdi sonunda hatasız biter, boş
// satırda bitmez; sonu satır sonsuz son satır da
// okunur. Hata TR_error_code ile yakalanır.
func TestInputAfterTheEndIsAnError(t *testing.T) {
	const loop = "TR_var n = 0\nTR_for TR_true {\n    TR_var s = TR_input()\n    TR_if s == \"\" { TR_break }\n" +
		"    n += 1\n    TR_if n > 5 { TR_break }\n}\nTR_print(\"okunan: \\(n)\")"
	run := func(src, input string) (string, error) {
		var out bytes.Buffer
		m := New()
		m.Out = &out
		m.Stdin = strings.NewReader(input)
		_, err := m.Run(mustParseProgram(t, src))
		return strings.TrimSpace(out.String()), err
	}
	for _, input := range []string{"bir\niki\n", "bir\niki", ""} {
		out, err := run(loop, input)
		if !messages.Has(err, messages.H2561) || strings.Contains(out, "okunan") {
			t.Errorf("girdi %q: döngü TR_none'dan sonraki çağrıda H2561 ile durmalıydı; çıktı %q, hata %v", input, out, err)
		}
	}
	out, err := run(loop, "bir\n\niki\n") // boş satır döngüyü bitirir, girdi sonuna varılmaz
	if err != nil || out != "okunan: 1" {
		t.Errorf("boş satırda: çıktı %q, hata %v", out, err)
	}
	// İletinin son iki yazımı (satırı değişkene alan atama ve TR_none'ı sınayan eğer) iletiden
	// kopyalanır, iki dilin iletisinden de (DENETİM 400 kapanış K-N2, K-N3).
	_, ended := run("TR_var a = TR_input()\nTR_var b = TR_input()", "")
	if !messages.Has(ended, messages.H2561) {
		t.Fatalf("ikinci çağrı H2561 vermeli: %v", ended)
	}
	for _, lang := range []string{"tr", "en"} {
		spans := regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(messages.ErrorIn(ended, lang), -1)
		if len(spans) < 2 {
			t.Errorf("%s: H2561'de iki yazım yok: %s", lang, messages.ErrorIn(ended, lang))
			continue
		}
		fixed := "TR_var n = 0\nTR_for TR_true {\n    " + spans[len(spans)-2][1] + "\n    " + spans[len(spans)-1][1] + "\n" +
			"    n += 1\n}\nTR_print(\"okunan: \\(n)\")"
		for input, want := range map[string]string{"bir\niki\n": "okunan: 2", "bir\nson": "okunan: 2", "": "okunan: 0", "\n": "okunan: 1", "a\n\nb\n": "okunan: 3"} {
			if out, err := run(fixed, input); err != nil || out != want {
				t.Errorf("%s: önerilen yazım, girdi %q: çıktı %q (beklenen %q), hata %v\n%s", lang, input, out, want, err, fixed)
			}
		}
	}
	out, err = run("TR_var a = TR_input()\nTR_print(a, TR_error_code { TR_input() })", "")
	if err != nil || out != "none H2561" {
		t.Errorf("H2561 yakalanmalı: çıktı %q, hata %v", out, err)
	}
}

// Kural (13.1, H1162): H1162'nin önerdiği koşul uçtan uca çalışır: denetleyicinin iletisinden (tr ve en) kopyalanan
// yazım denetimden geçer ve döngü girdi bitince de, bitiş değeri gelince de durur (DENETİM 400 Ş2;
// eski öneri '(... ?? "") != "son"' girdi sonunda dönmeye sürüyordu). Test burada, çünkü denetleyici
// yorumlayıcıyı içe aktaramaz (internal/layers_test.go, langOrder).
func TestInputLoopSuggestionEnds(t *testing.T) {
	fixPattern := regexp.MustCompile("`(\\([^`]*\\?\\?[^`]*)`")
	check := func(src string) []error {
		prog := mustParseProgram(t, src)
		c := checker.New()
		c.SetFile(checker.ProgramFile, src)
		c.CheckProgram(prog)
		return c.Errs()
	}
	for cond, runs := range map[string]map[string]string{
		`TR_input() != ""`:        {"a\nb\n": "2", "": "0", "a\n\nb\n": "1"},
		`TR_input("> ") != "son"`: {"a\nb\n": "2", "": "0", "a\nson\nb\n": "1"},
		`"son" != TR_input()`:     {"a\nb\n": "2", "": "0", "son\n": "0"},
	} {
		errs := check("TR_for " + cond + " {\n}")
		if len(errs) != 1 || !messages.Has(errs[0], messages.H1162) {
			t.Errorf("%s: tek H1162 bekleniyordu, gelen %v", cond, errs)
			continue
		}
		for _, lang := range []string{"tr", "en"} { // iki dilin iletisi de (DENETİM 400 kapanış K-N3)
			text := messages.ErrorIn(errs[0], lang)
			fix := fixPattern.FindStringSubmatch(text)
			if fix == nil {
				t.Errorf("%s, %s: iletide ?? ile bir yazım yok: %s", cond, lang, text)
				continue
			}
			src := "TR_var n = 0\nTR_for " + fix[1] + " {\n    n += 1\n}\nTR_print(n)"
			if errs := check(src); len(errs) > 0 {
				t.Errorf("%s: önerilen %s denetimden geçmeli: %v", lang, fix[1], errs)
			}
			for input, want := range runs {
				var out bytes.Buffer
				m := New()
				m.Out = &out
				m.Stdin = strings.NewReader(input)
				_, err := m.Run(mustParseProgram(t, src))
				if got := strings.TrimSpace(out.String()); err != nil || !strings.HasSuffix(got, want) {
					t.Errorf("%s: önerilen %s, girdi %q: çıktı %q (sonu %q olmalı), hata %v", lang, fix[1], input, got, want, err)
				}
			}
		}
	}
}

// girdi() her çağrıda yeni bir tamponlu okuyucu açınca ilk okuma kaynağın tamamını
// tampona çekiyor, sonraki satırlar tamponla birlikte kayboluyordu. VM başına tek okuyucu
// kullanılır: art arda okumalar satırları sırayla verir.
func TestInputReadsConsecutiveLines(t *testing.T) {
	prog := mustParseProgram(t, `TR_var a = TR_input()
TR_var b = TR_input()
TR_var c = TR_input()
TR_print("a=[\(a)] b=[\(b)] c=[\(c)]")`)
	var out bytes.Buffer
	m := New()
	m.Out = &out
	m.Stdin = strings.NewReader("bir\r\niki\n")
	if _, err := m.Run(prog); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "a=[bir] b=[iki] c=[none]" { // girdi sonu TR_none (13.1)
		t.Fatalf("girdi satırları: %q", got)
	}
}

// Kabuk ve hata ayıklayıcı da VM'in okuyucusunu kullanır: girdi() ile birlikte okununca
// satır kaybolmaz.
func TestInputReaderShared(t *testing.T) {
	m := New()
	m.Stdin = strings.NewReader("komut\nveri\n")
	if s, _ := m.InputReader().ReadString('\n'); s != "komut\n" {
		t.Fatalf("ilk satır: %q", s)
	}
	_, err := m.Run(parser.New(bodyLexer(`TR_var result = TR_input()`)).ParseProgram())
	val, _ := m.GlobalEnv.Get("result")
	if err != nil || val == nil || val.Inspect() != "veri" {
		t.Fatalf("girdi() ikinci satırı okumalıydı: %v %v", val, err)
	}
	if r1, r2 := StdinReader(), StdinReader(); r1 != r2 {
		t.Fatal("standart giriş okuyucusu süreç başına tek olmalı")
	}
	if New().InputReader() != StdinReader() {
		t.Fatal("Stdin verilmemiş VM standart girişin ortak okuyucusunu kullanmalı")
	}
}

// Bir durdurma girdi() beklemesini bırakır (1.5); bırakılan okuma sürer ve okuduğu satırı sonra
// girdi isteyen ertelenen iş alır, satır yitmez.
func TestAbandonedInputKeepsTheLine(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	m := New()
	m.Stdin = reader
	var out bytes.Buffer
	m.Out = &out
	stopAfter(m, 2)
	prog := mustParseProgram(t, `# TR 4
TR_defer { TR_print("ertele: \(TR_input())") }
hazır()
TR_input()
`)
	done := make(chan error, 1)
	go func() {
		_, err := m.Run(prog)
		done <- err
	}()
	time.Sleep(300 * time.Millisecond) // durdurma ilk girdi() beklerken gelir, ertelenen iş girdi bekler
	if _, err := writer.Write([]byte("bir\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrInterrupted) || !strings.Contains(out.String(), "ertele: bir") {
			t.Fatalf("ertelenen iş bırakılan okumanın satırını almalıydı: %v %q", err, out.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("program 30 sn içinde bitmedi")
	}
}
