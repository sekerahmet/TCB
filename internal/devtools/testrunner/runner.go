// Package testrunner 'trlang test' komutunun çekirdeğidir: .tr dosyalarındaki
// test "ad" { ... } bloklarını bulur, her birini yalıtılmış bir kapsamda çalıştırır
// ve sonuçları Türkçe olarak raporlar.
//
// Layer: developer tools (internal/devtools); dil, motor ve hesap katmanlarını içe aktarabilir.
package testrunner

import (
	"fmt"
	"io"
	"os"

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
	Unloadable int // sözdizimi/tip/üst düzey hata nedeniyle testleri çalıştırılamayan dosya sayısı
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

// TestFiles hedeflere göre çalıştırılacak dosyaları, hedeflerin verildiği sırayla bulur. Hedef bir
// dosyaysa yalnızca odur; klasörse altındaki (alt klasörler dahil) tüm *_test.tr dosyaları sıralı
// döner. Olmayan hedef (H8502) ve test dosyası bulunmayan klasör (H8503) hatadır: hedeflerden
// biri koşmadan sonuç "geçti" görünmesin.
func TestFiles(targets []string) ([]string, error) {
	var files []string
	for _, target := range targets {
		found, err := source.WalkFiles(target, "_test.tr", func(target string, _ error) error {
			return messages.New(messages.H8502, target)
		}, nil)
		if err != nil {
			return nil, err
		}
		if len(found) == 0 {
			return nil, messages.New(messages.H8503, target)
		}
		files = append(files, found...)
	}
	return files, nil
}

// Run hedeflerdeki tüm test dosyalarını çalıştırır, raporu out'a yazar ve özeti döndürür. Hedeflerden
// biri bulunamazsa hiçbir test çalışmaz.
func Run(targets []string, out io.Writer) (Summary, error) {
	files, err := TestFiles(targets)
	if err != nil {
		return Summary{}, err
	}
	var total Summary
	for _, d := range files {
		total.add(RunFile(d, out))
	}
	fmt.Fprintln(out)
	if total.OK() {
		fmt.Fprintf(out, "✅ %s\n", total)
	} else {
		fmt.Fprintf(out, "❌ %s\n", total)
	}
	return total, nil
}

// RunFile tek bir dosyanın test bloklarını çalıştırır. Önce dosya normal biçimde
// yüklenir (test blokları atlanarak üst düzey tanımlar oluşur); ardından her test, bu
// tanımların üzerinde açılan yeni bir kapsamda tek tek çalışır. Bir testin kalması ya da
// hata vermesi diğerlerini durdurmaz.
func RunFile(fpath string, out io.Writer) Summary {
	summary := Summary{File: 1}
	fmt.Fprintf(out, "📄 %s\n", fpath)

	data, err := os.ReadFile(fpath)
	if err != nil {
		fmt.Fprintf(out, "  ✗ %v\n", turkish.FileError("", fpath, messages.H2100, err))
		summary.Unloadable++
		return summary
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
		return summary
	}

	var tests []*ast.TestStatement
	for _, st := range prog.Statements {
		if t, ok := st.(*ast.TestStatement); ok {
			tests = append(tests, t)
		}
	}
	if len(tests) == 0 {
		fmt.Fprintln(out, "  "+messages.Text(messages.H8506))
		return summary
	}

	// Aktarılan modüller de testler yüklenmeden önce denetlenir (1.3).
	if err := interp.CheckImportedModules(fpath, prog); err != nil {
		writeLoadFailure(out, err, text)
		summary.Unloadable++
		return summary
	}

	machine := interp.New()
	machine.Out = out
	machine.Err = out // eşzamanlı görev hataları da raporda görünür
	machine.SourceFile = fpath
	machine.SourceCode = text
	if _, err := machine.Run(prog); err != nil {
		writeLoadFailure(out, err, text)
		summary.Unloadable++
		return summary
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
	return summary
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
