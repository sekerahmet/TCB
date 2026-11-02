package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/layoutrules"
)

// Bu dosya tests-for <Go dosyası>'nın boyut satırını kurar: dosyayı tarayan boyut kuralları (TestLayoutFileSize,
// TestLayoutFuncSize) için bugünkü değer ve sınır (DENETİM 2026-10-29: compiler.assignTo 80/80 satırdaydı, iki satır
// eklendi, TestLayoutFuncSize commit'ten sonra düştü; tests-for testin adını gösteriyordu, değeri göstermiyordu).
// Sınırlar internal/layoutrules'tan (düzen testinin okuduğu kaynak), dosya ve fonksiyon boyları sorgu anında dosyadan
// okunur (fonksiyonlar go/parser ve düzenin ölçüsüyle: derleme kırıkken ve eski grafik kullanılırken de günceldir;
// sözdizimi hatası yüzünden ayrıştırılamıyorsa boyut satırı boyun bayat olduğunu açıkça söyler).

// nearPercent bu orandan (yüzde) büyük ya da eşit boyda olan fonksiyon sınıra yakındır ve yazılır.
const nearPercent = 90

// fileSizeTest ve funcSizeTest boyut satırının kurallarıdır: dosyayı tarayan dosya ve fonksiyon sınırı testleri.
const (
	fileSizeTest = "TestLayoutFileSize"
	funcSizeTest = "TestLayoutFuncSize"
)

// sized bir dosya ya da fonksiyonun bugünkü boyu ve aşamayacağı boydur.
type sized struct {
	name      string
	lines     int
	max       int
	exception bool // istisna listesinde: max tavan + düzeltme payı
}

func (s sized) near() bool { return s.lines*100 >= s.max*nearPercent }

func (s sized) String() string {
	var notes []string
	if s.exception {
		notes = append(notes, "an exception: its limit is the ceiling plus the fix margin")
	}
	switch {
	case s.lines > s.max:
		notes = append(notes, "over the limit: the test fails")
	case s.near():
		notes = append(notes, "split before adding")
	}
	out := fmt.Sprintf("%s %d/%d", s.name, s.lines, s.max)
	if len(notes) > 0 {
		out += " (" + strings.Join(notes, "; ") + ")"
	}
	return out
}

// fileSizes dosyanın boy kurallarına göre bugünkü boyudur: dosya (TestLayoutFileSize tarıyorsa) ve fonksiyonları
// (TestLayoutFuncSize tarıyorsa ve dosya test dosyası değilse). Düzenin ölçmediği dosya (üretilmiş, testdata gibi
// atlanan bir klasörde) ve okunamayan dosya sayılmaz. Dosya sözdizimi hatası yüzünden ayrıştırılamıyorsa staleFuncs
// true döner ve boyut satırı fonksiyon boyunun bayat olduğunu açıkça yazar.
func (ix *index) fileSizes(file *Node) (whole *sized, funcs []sized, staleFuncs bool, tests []string) {
	if file.Detail == "generated" || ix.root == "" || slices.ContainsFunc(strings.Split(path.Dir(file.File), "/"), layoutrules.SkippedDir) {
		return nil, nil, false, nil
	}
	scanned := map[string]bool{}
	for _, h := range ix.scansCovering(file.File) {
		for _, t := range h.tests {
			scanned[t.Name[strings.LastIndex(t.Name, ".")+1:]] = true
		}
	}
	data, err := os.ReadFile(filepath.Join(ix.root, filepath.FromSlash(file.File)))
	if err != nil {
		return nil, nil, false, nil
	}
	if scanned[fileSizeTest] {
		max, exc := layoutrules.FileMax(file.File)
		whole = &sized{"file", layoutrules.FileLines(data), max, exc}
		tests = append(tests, fileSizeTest)
	}
	if scanned[funcSizeTest] && !isTestFile(file.File) {
		var ok bool
		funcs, ok = funcSizes(file.File, data)
		staleFuncs = !ok
		tests = append(tests, funcSizeTest)
	}
	return whole, funcs, staleFuncs, tests
}

// funcSizes dosyadaki func bildirimlerinin boyudur; adları düzen testinin istisna anahtarındaki gibi ("Alici.Ad").
// Her Go dosyası sorgu anında go/parser ve düzen testinin ölçüsüyle (layoutrules.FuncName, FuncLines) okunur; böylece
// derleme kırıkken ve eski grafik kullanılırken de fonksiyon boyları günceldir. Sözdizimi hatası varsa false döner.
func funcSizes(file string, data []byte) ([]sized, bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, data, parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	var out []sized
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			name := layoutrules.FuncName(fd)
			max, exc := layoutrules.FuncMax(file + ":" + name)
			out = append(out, sized{name, layoutrules.FuncLines(fset, fd), max, exc})
		}
	}
	return sortSizes(out), true
}

// sortSizes sınırı aşanları önce, sonra sınıra oranca en yakın olanı koyar.
func sortSizes(out []sized) []sized {
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ao, bo := a.lines > a.max, b.lines > b.max; ao != bo {
			return ao
		}
		if r, q := a.lines*b.max, b.lines*a.max; r != q {
			return r > q
		}
		return a.name < b.name
	})
	return out
}

// printSizes boyut satırını yazar (koşu satırlarının altında, kancanın gösterdiği ilk satırlarda): dosyanın boyu ve
// sınıra %nearPercent'ten yakın fonksiyonlar; yakın yoksa sınıra en yakını; sözdizimi hatası varsa bayat uyarısı.
func (ix *index) printSizes(w io.Writer, file *Node) {
	whole, funcs, staleFuncs, tests := ix.fileSizes(file)
	if len(tests) == 0 {
		return
	}
	var parts []string
	if whole != nil {
		parts = append(parts, whole.String())
	}
	switch {
	case staleFuncs:
		parts = append(parts, "functions: stale (syntax error: cannot parse file)")
	case len(funcs) > 0:
		var near []string
		for _, f := range funcs {
			if f.lines > f.max || f.near() {
				near = append(near, f.String())
			}
		}
		if len(near) > 0 {
			parts = append(parts, fmt.Sprintf("functions at >=%d%% of their limit: %s", nearPercent, strings.Join(near, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("functions: none at >=%d%% (closest %s)", nearPercent, funcs[0]))
		}
	}
	fmt.Fprintf(w, "  size (%s): %s\n", strings.Join(tests, ", "), strings.Join(parts, "; "))
}
