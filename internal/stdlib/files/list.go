package files

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// listBuiltin klasörü_listele([klasör = "."], [kalıp = "*"]) yerleşiğidir: kalıba uyan
// dosya ve klasörleri sıralı bir Yol listesi olarak verir.
func listBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G121)
	if len(args) > 2 {
		return nil, interp.ArgCountError(codes.G121, "0, 1 ya da 2", len(args), "", "klasör", "klasör, kalıp")
	}
	base, glob := ".", "*"
	if len(args) > 0 {
		t, err := readPath(codes.G121, 1, args[0])
		if err != nil {
			return nil, err
		}
		base = t
	}
	if len(args) > 1 {
		s, ok := args[1].(*interp.StringVal)
		if !ok {
			return nil, interp.ArgTypeError(codes.G121, 2, args[1], codes.A202)
		}
		glob = s.Val
	}
	k, err := compilePattern(glob)
	if err != nil {
		return nil, fmt.Errorf("%s: kalıp %q geçersiz: %v", fn, glob, err)
	}
	resolved, tried := m.ResolvePath(filepath.FromSlash(base))
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, interp.ReadError(fn, base, messages.H2114, err, tried)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s: %q bir dosya, klasör değil; listelenecek klasörü verin", fn, base)
	}
	g := &walker{k: k, seen: map[string]bool{}}
	m.Blocking(func() { g.walk(resolved, "", 0) })
	if g.unreadableCount > 0 {
		m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %s: %d alt klasör okunamadı ve atlandı (ilki: %q: %s)\n",
			fn, g.unreadableCount, g.firstUnreadable, turkish.FileErrorReason(g.firstUnreadable, g.firstErr)))
	}
	basePath := clean(base)
	paths := make([]string, len(g.found))
	for i, rel := range g.found {
		if basePath == "." {
			paths[i] = rel
		} else {
			paths[i] = clean(filepath.Join(filepath.FromSlash(basePath), filepath.FromSlash(rel)))
		}
	}
	sort.Strings(paths)
	result := make([]interp.Value, len(paths))
	for i, y := range paths {
		result[i] = &Path{fpath: y}
	}
	return &interp.ListVal{Elements: result}, nil
}

// walker kalıbı bir klasör ağacında eşler. Yorumlayıcı kilidi bırakılmışken çalışır; TR
// değerlerine dokunmaz.
type walker struct {
	k               *pattern
	found           []string // tabana göre, / ile
	seen            map[string]bool
	unreadableCount int
	firstUnreadable string
	firstErr        error
}

// walk klasor'ü (tabana göre yolu rel) kalıbın i. bileşeninden başlayarak eşler.
func (g *walker) walk(dir, rel string, i int) {
	if g.k.components[i].double && i == len(g.k.components)-1 {
		g.all(dir, rel) // sondaki **: alttaki her şey
		return
	}
	inputs, err := os.ReadDir(dir)
	if err != nil {
		g.markUnreadable(dir, err)
		return
	}
	g.walkEntries(dir, rel, i, inputs)
}

// walkEntries okunmuş klasör girdilerini kalıbın i. bileşeninden başlayarak eşler. Aradaki
// ** önce sıfır klasör olarak (aynı girdilerle sonraki bileşen), sonra her alt klasöre inerek
// eşlenir; klasör bir kez okunur.
func (g *walker) walkEntries(dir, rel string, i int, inputs []fs.DirEntry) {
	b := g.k.components[i]
	last := i == len(g.k.components)-1
	if b.double {
		if last {
			g.all(dir, rel)
			return
		}
		g.walkEntries(dir, rel, i+1, inputs) // sıfır klasör
	}
	for _, e := range inputs {
		name := e.Name()
		sub, subRel := filepath.Join(dir, name), path.Join(rel, name)
		if b.double {
			// ** gizli klasörlere ve klasör bağlantılarının içine girmez (döngü olmaz).
			if e.IsDir() && name[0] != '.' {
				g.walk(sub, subRel, i)
			}
			continue
		}
		if !b.match(name) {
			continue
		}
		if last {
			g.add(subRel)
		} else if isDir(sub, e) {
			g.walk(sub, subRel, i+1)
		}
	}
}

// all klasörün altındaki gizli olmayan her şeyi ekler (bağlantıların içine girmeden).
func (g *walker) all(dir, rel string) {
	inputs, err := os.ReadDir(dir)
	if err != nil {
		g.markUnreadable(dir, err)
		return
	}
	for _, e := range inputs {
		name := e.Name()
		if name[0] == '.' {
			continue
		}
		subRel := path.Join(rel, name)
		g.add(subRel)
		if e.IsDir() {
			g.all(filepath.Join(dir, name), subRel)
		}
	}
}

// isDir girdi bir klasör mü? Açıkça adı yazılmış (ya da tek bir bileşen kalıbıyla
// eşlenmiş) bir klasör bağlantısının içine girilebilir: orada döngü yoktur.
func isDir(fpath string, e fs.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return false
	}
	b, err := os.Stat(fpath)
	return err == nil && b.IsDir()
}

func (g *walker) add(rel string) {
	if !g.seen[rel] {
		g.seen[rel] = true
		g.found = append(g.found, rel)
	}
}

func (g *walker) markUnreadable(dir string, err error) {
	g.unreadableCount++
	if g.unreadableCount == 1 {
		g.firstUnreadable, g.firstErr = filepath.ToSlash(dir), err
	}
}
