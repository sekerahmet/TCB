// Package testrunner 'trlang test' komutunun çekirdeğidir: .tr dosyalarındaki
// TR_test "ad" { ... } bloklarını bulur, her birini yalıtılmış bir kapsamda çalıştırır
// ve sonuçları Türkçe olarak raporlar.
//
// Layer: developer tools (internal/devtools); dil, motor ve hesap katmanlarını içe aktarabilir.
package testrunner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	_ "github.com/sekerahmet/TCB/internal/devtools/debugger" // koddaki kesme_noktası hata ayıklayıcıyı açar
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	_ "github.com/sekerahmet/TCB/internal/stdlib" // testler dilin tamamıyla (motor yerleşikleri dahil) çalışır
)

// Summary bir test çalıştırmasının sayısal sonucudur.
type Summary struct {
	Total      int // çalıştırılan test sayısı
	Passed     int
	Failed     int
	Unloadable int // sözdizimi/tip/üst düzey hata nedeniyle testleri çalıştırılamayan ya da en dışında testi olmayan dosya sayısı
	File       int // incelenen dosya sayısı
}

// OK hiç test kalmadıysa ve tüm dosyalar yüklenebildiyse doğrudur.
func (o Summary) OK() bool { return o.Failed == 0 && o.Unloadable == 0 }

func (o *Summary) add(b Summary) {
	o.Total += b.Total
	o.Passed += b.Passed
	o.Failed += b.Failed
	o.Unloadable += b.Unloadable
	o.File += b.File
}

// String özet satırını üretir: "12 test, 11 geçti, 1 kaldı".
func (o Summary) String() string {
	return messages.Text(messages.H8500, o.Total, o.Passed, o.Failed, o.Unloadable)
}

// fixtureDir taramanın inmediği klasörün adıdır: bilerek hatalı dosyalar orada durur (Go'nun kuralı).
// Dosya ya da klasör olarak açıkça verilirse koşar.
const fixtureDir = "testdata"

// testSuffix test dosyası olduğunu adıyla söyleyen dosyanın sonudur: böyle dosya TR_test yazmasa da
// (kimliksiz test "ad", testi silinmiş dosya) elenmez, sonucunu runFile verir.
const testSuffix = "_test.tr"

// TestFiles hedeflere göre çalıştırılacak dosyaları, hedeflerin verildiği sırayla bulur. Test dosyası
// TR_test içeren dosyadır, adı ne olursa olsun (12.1). Klasörde testsiz dosya olağandır, atlanır;
// verilen dosyada test yoksa ya da klasörün toplamında hiç test yoksa H8503, olmayan hedef H8502:
// hedeflerden biri koşmadan sonuç "geçti" görünmesin. Adı testSuffix ile biten dosya hiç elenmez.
func TestFiles(targets []string) ([]string, error) {
	var files []string
	for _, target := range targets {
		found, err := source.WalkFiles(target, ".tr", func(target string, _ error) error {
			return messages.New(messages.H8502, target)
		}, nil)
		if err != nil {
			return nil, err
		}
		found = slices.DeleteFunc(found, func(f string) bool { return inFixtureDir(target, f) || !holdsTests(f) })
		if len(found) == 0 {
			return nil, messages.New(messages.H8503, target)
		}
		files = append(files, found...)
	}
	return files, nil
}

// inFixtureDir f, hedefin altında fixtureDir adlı bir klasörde mi.
func inFixtureDir(target, f string) bool {
	rel, err := filepath.Rel(target, f)
	return err == nil && slices.Contains(strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/"), fixtureDir)
}

// holdsTests dosya koşulacak mı. Okunamayan dosya ve adı test dosyası diyen dosya elenmez: bozuksa
// "yüklenemedi", testi yoksa H8503 olur.
func holdsTests(f string) bool {
	if strings.HasSuffix(filepath.Base(f), testSuffix) {
		return true
	}
	data, err := os.ReadFile(f)
	return err != nil || source.HasTestBlock(string(data))
}

// Run hedeflerdeki tüm test dosyalarını ve onların aktardığı modüllerin testlerini (12.1) çalıştırır,
// raporu out'a yazar ve özeti döndürür. Her dosya bir kez, kendi makinesinde koşar. TestFiles'ın
// reddettiği hedef varsa hiçbir test çalışmaz.
func Run(targets []string, out io.Writer) (Summary, error) {
	files, err := TestFiles(targets)
	if err != nil {
		return Summary{}, err
	}
	var total Summary
	done := map[string]bool{}
	for len(files) > 0 {
		f := files[0]
		files = files[1:]
		if key := fileKey(f); !done[key] {
			done[key] = true
			summary, modules := runFile(f, out)
			total.add(summary)
			files = append(slices.DeleteFunc(modules, func(m string) bool { return !holdsTests(m) }), files...)
		}
	}
	fmt.Fprintln(out)
	if total.OK() {
		fmt.Fprintf(out, "✅ %s\n", total)
	} else {
		fmt.Fprintf(out, "❌ %s\n", total)
	}
	return total, nil
}

// fileKey aynı dosyanın iki yazımını birleştirir (alt\\a.tr, alt/a.tr, ./alt/a.tr).
func fileKey(f string) string {
	if abs, err := filepath.Abs(f); err == nil {
		return abs
	}
	return filepath.Clean(f)
}

// RunFile tek bir dosyanın test bloklarını çalıştırır.
func RunFile(fpath string, out io.Writer) Summary {
	summary, _ := runFile(fpath, out)
	return summary
}

// runFile tek bir dosyanın test bloklarını çalıştırır, aktardığı modül dosyalarını da verir. Önce dosya normal biçimde
// yüklenir (test blokları atlanarak üst düzey tanımlar oluşur); ardından her test, bu
// tanımların üzerinde açılan yeni bir kapsamda tek tek çalışır. Bir testin kalması ya da
// hata vermesi diğerlerini durdurmaz. En dışında testi olmayan dosya koşuyu düşürür (H8503).
func runFile(fpath string, out io.Writer) (Summary, []string) {
	summary := Summary{File: 1}
	fmt.Fprintf(out, "📄 %s\n", fpath)

	data, err := os.ReadFile(fpath)
	if err != nil {
		fmt.Fprintf(out, "  ✗ %v\n", turkish.FileError("", fpath, messages.H2100, err))
		summary.Unloadable++
		return summary, nil
	}
	ready := source.Prepare(string(data), source.Options{TypeCheck: true, Library: interp.LibraryFileName(fpath, string(data))})
	text, prog := ready.Text, ready.Program
	if len(ready.Warnings) > 0 {
		// Uyarılar testleri durdurmaz ve sonucu etkilemez.
		fmt.Fprintln(out, "  ⚠️ "+messages.Text(messages.H8504))
		for _, u := range ready.Warnings {
			writeError(out, u, text)
		}
	}
	if ready.Stage != "" {
		fmt.Fprintf(out, "  ✗ %s\n", messages.Text(messages.H8505, ready.Stage))
		for _, e := range ready.Errors {
			writeError(out, e, text)
		}
		summary.Unloadable++
		return summary, nil
	}

	var tests []*ast.TestStatement
	for _, st := range prog.Statements {
		if t, ok := st.(*ast.TestStatement); ok {
			tests = append(tests, t)
		}
	}
	if len(tests) == 0 {
		fmt.Fprintf(out, "  ✗ %s\n", messages.Text(messages.H8503, fpath))
		summary.Unloadable++
		return summary, nil
	}

	// Aktarılan modüller de testler yüklenmeden önce denetlenir (1.3).
	modules, err := interp.ImportedModuleFiles(fpath, prog)
	if err != nil {
		writeLoadFailure(out, err, text)
		summary.Unloadable++
		return summary, nil
	}

	machine := interp.New()
	machine.Out = out
	machine.Err = out // eşzamanlı görev hataları da raporda görünür
	machine.SourceFile = fpath
	machine.SourceCode = text
	if _, err := machine.Run(prog); err != nil {
		writeLoadFailure(out, err, text)
		summary.Unloadable++
		return summary, modules
	}

	for _, t := range tests {
		summary.Total++
		if err := machine.RunTest(t); err != nil {
			summary.Failed++
			writeTestFailure(out, t, err, text)
			continue
		}
		summary.Passed++
		fmt.Fprintf(out, "  ✓ %s\n", t.Name)
	}
	return summary, modules
}

// writeTestFailure başarısız testin satırını raporlar: konumsuz hataya (ör. TR_throw) testin satırı
// eklenir; modüle ait hatanın çerçevesi modülün kaynağından çizilir.
func writeTestFailure(out io.Writer, t *ast.TestStatement, err error, text string) {
	if !interp.HasLine(err) {
		err = &interp.LineError{Line: t.Line(), Err: err}
	}
	for i, m := range interp.ErrorItems(err, text) {
		if i == 0 {
			fmt.Fprintf(out, "  ✗ %s — %s\n", turkish.Printable(t.Name), turkish.Printable(m.Message))
		} else {
			fmt.Fprintf(out, "      • %s\n", m.Message)
		}
		fmt.Fprint(out, turkish.IndentLines(m.Frame, "      "))
	}
}

// writeLoadFailure dosyanın yüklenemediğini (modül denetimi ya da üst düzey hata) ve hatayı yazar.
func writeLoadFailure(out io.Writer, err error, text string) {
	fmt.Fprintln(out, "  ✗ "+messages.Text(messages.H8507))
	for _, m := range interp.ErrorItems(err, text) {
		fmt.Fprintf(out, "    • %s\n", m.Message)
		fmt.Fprint(out, turkish.IndentLines(m.Frame, "      "))
	}
}

func writeError(out io.Writer, err error, source string) {
	fmt.Fprintf(out, "    • %s\n", turkish.Printable(err.Error()))
	fmt.Fprint(out, turkish.Printable(turkish.IndentLines(turkish.ErrorFrame(source, err), "      ")))
}
