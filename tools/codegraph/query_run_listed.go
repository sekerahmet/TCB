package main

import (
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

// flushAnswer gövdeyi w'ye yazar; listed doluysa adlar ondan (--all'da varsayılan sınırla yazılmış gövdeden) okunur.
func flushAnswer(w io.Writer, ab *answerBuffer, listed *answerBuffer) {
	text := ab.String()
	if ab.plan != nil {
		var run strings.Builder
		printRunPlan(&run, ab.plan(listedIn(listed.String())))
		text = strings.Replace(text, runPlanSlot, run.String(), 1)
	}
	_, _ = io.WriteString(w, text)
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
