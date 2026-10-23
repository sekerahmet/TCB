package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// Bu dosya denetim komutlarının (dead-methods, package-state, time-ceilings, spec-refs) ortak yanıdır:
// bayraklar, modül yükleme ve konum yardımcıları. Komutlar grafiği okumaz; kaynağı tip bilgisiyle
// tarar ve yalnızca sonuç listeler. Karar vermezler: çıktı gözden geçirilecek adaylardır.

// auditOpts denetim komutlarının ayarlarıdır.
type auditOpts struct {
	root, in string
	asJSON   bool
	limit    int
}

// parseAuditFlags --root, --in, --json ve --limit/--all bayraklarını okur; hata varsa nil.
func parseAuditFlags(cmd string, args []string, stderr io.Writer) (*auditOpts, []string) {
	return parseAuditFlagsWith(cmd, args, stderr, nil)
}

// parseAuditFlagsWith ortak bayrakların yanında komuta özgü bayrakları extra ile tanımlar.
func parseAuditFlagsWith(cmd string, args []string, stderr io.Writer, extra func(*flag.FlagSet)) (*auditOpts, []string) {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	o := &auditOpts{}
	fs.StringVar(&o.root, "root", "", "repository root (default: found from the current directory)")
	fs.StringVar(&o.in, "in", "", "only results under this path (a package directory or a file)")
	fs.BoolVar(&o.asJSON, "json", false, "machine-readable output")
	fs.IntVar(&o.limit, "limit", 0, "lines per list (0: all)")
	all := fs.Bool("all", false, "no limit (the default)")
	if extra != nil {
		extra(fs)
	}
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return nil, nil
	}
	if *all {
		o.limit = 0
	}
	return o, fs.Args()
}

// loadAudit modülü tip bilgisiyle yükler. Hata çıkış koduyla birlikte yazılır.
func loadAudit(o *auditOpts, stderr io.Writer, withTypes bool) (*goload.Module, int) {
	root, err := resolveRoot(o.root)
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return nil, exitFail
	}
	m, err := goload.Load(root, goload.Options{Selections: true, Types: withTypes})
	if err != nil {
		fmt.Fprintln(stderr, "codegraph:", err)
		return nil, exitFail
	}
	return m, exitOK
}

// position bir konumun köke göre dosyasını ve satırını verir.
func position(m *goload.Module, p token.Pos) (file string, line int) {
	pos := m.Fset.Position(p)
	return m.Rel(pos.Filename), pos.Line
}

// isTestFile yolu _test.go ile biten dosya mı.
func isTestFile(file string) bool { return strings.HasSuffix(file, "_test.go") }

// shortPkg bir paket yolunun modül önekini atar.
func shortPkg(m *goload.Module, path string) string {
	if path == m.Path {
		return path[strings.LastIndex(path, "/")+1:] // kök paket: modül yolunun son öğesi
	}
	return strings.TrimPrefix(path, m.Path+"/")
}

// underIn dosya --in değerinin altında mı (boşsa her dosya).
func underIn(file, in string) bool {
	in = strings.TrimSuffix(strings.TrimPrefix(strings.ReplaceAll(in, "\\", "/"), "./"), "/")
	return in == "" || underDir(file, in)
}

// forEachFile birimlerdeki her dosyayı (adıyla) dolaşır.
func forEachFile(m *goload.Module, fn func(u *goload.Unit, f *ast.File, rel string)) {
	for _, u := range m.Units {
		for _, f := range u.Files {
			pos := m.Fset.Position(f.Pos())
			fn(u, f, m.Rel(pos.Filename))
		}
	}
}

// limited ilk n öğeyi verir (n 0 ise hepsini) ve kesilip kesilmediğini söyler.
func limited[T any](list []T, n int) ([]T, bool) {
	if n > 0 && len(list) > n {
		return list[:n], true
	}
	return list, false
}
