// Package testrunner 'tr test' komutunun çekirdeğidir: .tr dosyalarındaki
// test "ad" { ... } bloklarını bulur, her birini yalıtılmış bir kapsamda çalıştırır
// ve sonuçları Türkçe olarak raporlar.
//
// Layer: developer tools (internal/devtools); dil, motor ve hesap katmanlarını içe aktarabilir.
package testrunner

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	s := fmt.Sprintf("%d test, %d geçti, %d kaldı", o.Total, o.Passed, o.Failed)
	if o.Unloadable > 0 {
		s += fmt.Sprintf(", %d dosya yüklenemedi", o.Unloadable)
	}
	return s
}

// TestFiles hedefe göre çalıştırılacak dosyaları bulur. Hedef bir dosyaysa yalnızca
// odur; klasörse altındaki (alt klasörler dahil) tüm *_test.tr dosyaları sıralı döner.
func TestFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("%s bulunamadı", target)
	}
	if !info.IsDir() {
		return []string{target}, nil
	}
	var files []string
	err = filepath.WalkDir(target, func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Gizli klasörler (.git, .claude) ve bağımlılık klasörleri taranmaz
			ad := d.Name()
			if fpath != target && (strings.HasPrefix(ad, ".") || ad == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.tr") {
			files = append(files, fpath)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// Run hedefteki tüm test dosyalarını çalıştırır, raporu cikti'ya yazar ve özeti döndürür.
func Run(target string, out io.Writer) (Summary, error) {
	files, err := TestFiles(target)
	if err != nil {
		return Summary{}, err
	}
	if len(files) == 0 {
		return Summary{}, fmt.Errorf("%s içinde *_test.tr dosyası bulunamadı", target)
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
	ready := source.Prepare(string(data), source.Options{TypeCheck: true})
	text, prog := ready.Text, ready.Program
	if len(ready.Warnings) > 0 {
		// Uyarılar testleri durdurmaz ve sonucu etkilemez.
		fmt.Fprintln(out, "  ⚠️ uyarılar:")
		for _, u := range ready.Warnings {
			writeError(out, u, text)
		}
	}
	if ready.Stage != "" {
		fmt.Fprintf(out, "  ✗ %s hataları nedeniyle testler çalıştırılamadı:\n", ready.Stage)
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
		fmt.Fprintln(out, "  (bu dosyada test bloğu yok)")
		return summary
	}

	machine := interp.New()
	machine.Out = out
	machine.Err = out // eşzamanlı görev hataları da raporda görünür
	machine.SourceFile = fpath
	machine.SourceCode = text
	if _, err := machine.Run(prog); err != nil {
		fmt.Fprintln(out, "  ✗ dosyanın üst düzey kodu hata verdi, testler çalıştırılamadı:")
		for _, m := range interp.ErrorItems(err, text) {
			fmt.Fprintf(out, "    • %s\n", m.Message)
			fmt.Fprint(out, indent(m.Frame, "      "))
		}
		summary.Unloadable++
		return summary
	}

	for _, t := range tests {
		summary.Total++
		if err := machine.RunTest(t); err != nil {
			summary.Failed++
			if !located(err) {
				// Konumsuz hatalarda (ör. fırlat) testin satırını göster
				err = &interp.LineError{Line: t.Line(), Err: err}
			}
			// Modüle ait hatanın çerçevesi modülün kaynağından çizilir.
			for i, m := range interp.ErrorItems(err, text) {
				if i == 0 {
					fmt.Fprintf(out, "  ✗ %s — %s\n", t.Name, m.Message)
				} else {
					fmt.Fprintf(out, "      • %s\n", m.Message)
				}
				fmt.Fprint(out, indent(m.Frame, "      "))
			}
			continue
		}
		summary.Passed++
		fmt.Fprintf(out, "  ✓ %s\n", t.Name)
	}
	return summary
}

// lineStart konumlu bir hata iletisinin başıdır ("Satır 12: ..."). İletinin içinde geçen
// "Satır " sözcüğü (ör. fırlat "Satır sayısı yanlış") konum sayılmaz.
var lineStart = regexp.MustCompile(`^Satır [0-9]+:`)

// located hatanın konumu var mı: satırlı ya da bir dosyaya (modül) bağlı hata.
func located(err error) bool {
	var le *interp.LineError
	var me *interp.ModuleError
	return errors.As(err, &le) || errors.As(err, &me) || lineStart.MatchString(err.Error())
}

func writeError(out io.Writer, message, source string) {
	fmt.Fprintf(out, "    • %s\n", message)
	fmt.Fprint(out, indent(turkish.CodeFrame(source, message), "      "))
}

func indent(text, prefix string) string {
	if text == "" {
		return ""
	}
	var sb strings.Builder
	for _, s := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		sb.WriteString(prefix + s + "\n")
	}
	return sb.String()
}
