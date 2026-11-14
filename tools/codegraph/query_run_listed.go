package main

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Bu dosya tests-for cevabında adı geçen testleri koşu satırına katar: cevap bir testi listeliyor ama hiçbir `run:`
// satırı onu koşmuyorsa ajan onu koşmaz (28. bölümün G4'ü: TestDocTRCodeBlocks listelendi, koşulmadı). Koşu satırları
// cevabın başında, adı geçen testler gövdededir: gövde önce bir tampona yazılır, koşu satırları onun adlarıyla kurulup
// ayrılan yere konur. Yalnız adlandırır: hangi satırın commit'ten önce koşulacağına karar vermez.

// heavyTestPackages bu makinede -run'sız koşulmayan ağır paketlerdir (yan oturumların genel kancası onları durdurur; iki
// listeyi ana oturum eşit tutar). Koşu satırı bunları bütün olarak önermez.
var heavyTestPackages = []string{"cmd/trlang/e2e", "internal/compute/gpu", "internal/detmath", "internal/lang/parser", "tools/codegraph"}

func isHeavyPackage(dir string) bool { return slices.Contains(heavyTestPackages, dir) }

// defaultLimit --limit'in varsayılanıdır (grup başına satır): koşu satırı bu sınırla yazılan cevapta adı geçenleri alır.
const defaultLimit = 12

// runPlanSlot gövdede koşu satırlarının yeridir.
const runPlanSlot = "\x00run-plan\x00\n"

// answerBuffer tests-for cevabının gövdesidir; plan doluysa yer, gövdede adı geçen testlerle kurulan satırlarla dolar.
type answerBuffer struct {
	strings.Builder
	plan func(listedTests) runPlan
}

// listedTests cevabın gövdesinde adı geçen Go testleri ve TR test dosyalarıdır (varsayılan çıktı).
type listedTests struct {
	goTests []goTest
	trFiles []string
}

// writeRunPlan koşu satırlarını yazar; w bir answerBuffer ise yalnız yerini ayırır.
func writeRunPlan(w io.Writer, plan func(listedTests) runPlan) {
	if ab, ok := w.(*answerBuffer); ok && ab.plan == nil {
		ab.plan = plan
		ab.WriteString(runPlanSlot)
		return
	}
	printRunPlan(w, plan(listedTests{}))
}

// headEndSlot gövdede kısa cevabın bittiği yerdir (codegraph dilim 6, D6-1): dosya satırı, koşu satırları ve boyut
// satırı ondan öncedir, gruplar sonra. Dal satırı olan cevapta (`no tests found…`) yazılmaz: o cevap kısa kipte de tamdır.
const headEndSlot = "\x00head-end\x00\n"

// markHeadEnd kısa cevabın sonunu işaretler; w bir answerBuffer değilse bir şey yazmaz.
func markHeadEnd(w io.Writer) {
	if ab, ok := w.(*answerBuffer); ok {
		ab.WriteString(headEndSlot)
	}
}

// flushAnswer gövdeyi w'ye yazar (--list ve --all); listed doluysa adlar ondan (--all'da varsayılan sınırla yazılmış
// gövdeden) okunur.
func flushAnswer(w io.Writer, ab *answerBuffer, listed *answerBuffer) {
	text := strings.Replace(ab.String(), headEndSlot, "", 1)
	if ab.plan != nil {
		var run strings.Builder
		printRunPlan(&run, ab.plan(listedIn(listed.String())))
		text = strings.Replace(text, runPlanSlot, run.String(), 1)
	}
	_, _ = io.WriteString(w, text)
}

// flushShortAnswer tests-for'un bayraksız cevabıdır (D6-1, Karar 1): gövde --list'in gövdesidir, yazılan yalnız kısa
// cevabın sonuna kadarki kısmı ve gövdede adı geçen testlerin sayısıdır (S1). Koşu satırları aynı adlarla kurulur;
// gruplara gönderen satırları --list'e gönderir (S8).
func flushShortAnswer(w io.Writer, ab *answerBuffer) {
	text := ab.String()
	listed := listedIn(text)
	head, _, cut := strings.Cut(text, headEndSlot)
	if !cut {
		head = text
	}
	if ab.plan != nil {
		var run strings.Builder
		printRunPlanText(&run, ab.plan(listed), true)
		head = strings.Replace(head, runPlanSlot, run.String(), 1)
	}
	_, _ = io.WriteString(w, head)
	if n := len(listed.goTests) + len(listed.trFiles); cut && n > 0 {
		_, _ = fmt.Fprintf(w, "  tests: %d listed (--list shows the first page of each group, --all every test)\n", n)
	}
}

// listedGoTest ve listedTRTest cevabın test satırlarıdır: `ad [func] dosya:satır` (describe) ya da `... at dosya:satır`
// (kenar satırı); TR testinin adında boşluk olur, dosyası yeter.
var (
	listedGoTest = regexp.MustCompile(`(?m)^ {4,}(\S+) \[func\] (?:at )?(\S+_test\.go):\d+`)
	listedTRTest = regexp.MustCompile(`\[tr_test\] (?:at )?(\S+\.tr):\d+`)
)

// listedIn metinde adı geçen Go testlerini ve TR test dosyalarını sırasıyla, tekrarsız verir.
func listedIn(text string) listedTests {
	var out listedTests
	for _, m := range listedGoTest.FindAllStringSubmatch(text, -1) {
		name := m[1][strings.LastIndex(m[1], ".")+1:]
		if t := (goTest{path.Dir(m[2]), name}); isTestName(name) && !strings.HasPrefix(name, "Benchmark") && !slices.Contains(out.goTests, t) {
			out.goTests = append(out.goTests, t)
		}
	}
	for _, m := range listedTRTest.FindAllStringSubmatch(text, -1) {
		if !slices.Contains(out.trFiles, m[1]) {
			out.trFiles = append(out.trFiles, m[1])
		}
	}
	return out
}

// fitNames heavy paketlerin adlarından komut satırına sığan ilk adlardır; cut sığmayanların sayısıdır.
func (p runPlan) fitNames(pkgs, names []string) (fit []string, cut int) {
	n := len(p.command(pkgs, nil)) + len(" -run '^()$'") - 1 // ilk adın önünde "|" yok
	for i, name := range names {
		if n += len(name) + 1; n > commandLineLimit {
			return names[:i], len(names) - i
		}
	}
	return names, 0
}
