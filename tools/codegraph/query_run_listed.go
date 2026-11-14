package main

import (
	"io"
	"slices"
	"strings"
)

// Bu dosya tests-for cevabının gövdesini tamponda kurar: koşu satırlarının ve kısa cevabın sonunun yeri ayrılır, koşu
// satırları cevap yazılırken (kısa ya da gruplu) yerine konur. Koşu satırı cevapta adı geçen testleri artık almaz: dar
// satır yakın testlerdir (query_run_line.go, codegraph 7 A1).

// heavyTestPackages bu makinede -run'sız koşulmayan ağır paketlerdir: koşu satırı bunları bütün olarak önermez, adla
// koşturur. Kural (codegraph 7, K2): G4'te 30 sn'yi ya da dizüstünde bütün koşusu 2 dakikayı aşan paket ağırdır
// (`internal/stdlib/httpserver` G4'te 67,8 sn; `internal/stdlib/process` dizüstünde en çok 133,3 sn). Yan oturumların
// genel kancası da bunları durdurur; commit kipinin commit_late_packages'ı aynı listedir (TestCommitHeavyTestsExist).
var heavyTestPackages = []string{"cmd/trlang/e2e", "internal/compute/gpu", "internal/detmath", "internal/lang/parser",
	"internal/stdlib/httpserver", "internal/stdlib/process", "tools/codegraph"}

func isHeavyPackage(dir string) bool { return slices.Contains(heavyTestPackages, dir) }

// defaultLimit --limit'in varsayılanıdır (grup başına satır).
const defaultLimit = 12

// runPlanSlot gövdede koşu satırlarının yeridir.
const runPlanSlot = "\x00run-plan\x00\n"

// answerBuffer tests-for cevabının gövdesidir; plan doluysa yer, cevap yazılırken koşu satırlarıyla dolar.
type answerBuffer struct {
	strings.Builder
	plan func() runPlan
}

// writeRunPlan koşu satırlarını yazar; w bir answerBuffer ise yalnız yerini ayırır.
func writeRunPlan(w io.Writer, plan func() runPlan) {
	if ab, ok := w.(*answerBuffer); ok && ab.plan == nil {
		ab.plan = plan
		ab.WriteString(runPlanSlot)
		return
	}
	printRunPlan(w, plan())
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

// flushAnswer gövdeyi w'ye yazar (--list ve --all).
func flushAnswer(w io.Writer, ab *answerBuffer) {
	text := strings.Replace(ab.String(), headEndSlot, "", 1)
	if ab.plan != nil {
		var run strings.Builder
		printRunPlan(&run, ab.plan())
		text = strings.Replace(text, runPlanSlot, run.String(), 1)
	}
	_, _ = io.WriteString(w, text)
}

// flushShortAnswer tests-for'un bayraksız cevabıdır (D6-1, Karar 1): gövde --list'in gövdesidir, yazılan yalnız kısa
// cevabın sonuna kadarki kısmıdır; gruplara gönderen satırlar --list'e gönderir (S8). Sayı satırı koşu satırlarının
// altındadır (codegraph 7 A3).
func flushShortAnswer(w io.Writer, ab *answerBuffer) {
	head, _, cut := strings.Cut(ab.String(), headEndSlot)
	if !cut {
		head = ab.String()
	}
	if ab.plan != nil {
		var run strings.Builder
		printRunPlanText(&run, ab.plan(), true)
		head = strings.Replace(head, runPlanSlot, run.String(), 1)
	}
	_, _ = io.WriteString(w, head)
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
