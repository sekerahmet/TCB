package codeindex

import (
	"io/fs"
	"slices"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Bir Go test dizesinde ya da başka bir metinde bulunan TR kodunun kullandığı yerleşik adlar: dizin dosyadan değil
// metinden kurulur ve dosyalarla aynı çözümleyici (resolve) çalışır, bu yüzden yerel adlar, kapsam ve üye/adlı argüman
// kuralları dosyadakiyle aynıdır. Kullanıcı tanımlı işlevler tek dosyalı metinde yoktur; yalnızca ad kodları döner.

// SnippetUse bir metinde kullanılan ad kodudur ("TR_fn_default_device", "TR_mem_update", "TR_arg_every_minutes") ve ilk kullanıldığı satır (1'den).
type SnippetUse struct {
	Code string
	Line int
}

// Snippet src'yi TR programı olarak ayrıştırıp sınadığı yerleşik, üye ve adlı argüman kodlarını verir (koda göre
// sıralı, her kod bir kez; test bloğuyla aynı kural, exercisesBuiltin: işlev yerleşiği çağrılınca sayılır). Metin ayrıştırılamıyorsa nil: TR olmayan bir metin ya da eksik parça.
// library verilirse (dilin gömülü kitaplığı, lib.Files) metnin `aktar "tr/ad"` ile aldığı kitaplık modülleri dosyadaki
// kuralla (importTarget, addLibrary) çözülür; imports onların kitaplıktaki yollarıdır ("lib/hazir.tr"; kaynak sırasıyla,
// iç bloklardakiler dahil, her biri bir kez): metni koşan test aktardığı modülü de koşar.
func Snippet(src string, library fs.FS) (uses []SnippetUse, imports []string) {
	x := &indexer{root: "", lang: source.Turkish(), library: library, files: map[string]*file{}, nodes: map[string]*Node{}, edges: map[edgeKey]*Edge{},
		constructSeen: map[string]bool{}}
	x.addFile("snippet.tr", []byte(src), nil)
	if len(x.order) == 0 {
		return nil, nil
	}
	x.addLibrary()
	for _, rel := range x.order {
		x.declare(x.files[rel])
	}
	for _, rel := range x.order {
		x.resolveImports(x.files[rel])
	}
	for _, rel := range x.order {
		x.resolve(x.files[rel])
	}
	first := map[string]int{}
	for _, o := range x.finish().Occurrences {
		if n := x.nodes[o.Node]; n != nil && x.exercisesBuiltin(n.ID, o.Role, o.Basis) && strings.HasPrefix(o.Pos.File, "snippet.tr") {
			if l, seen := first[n.Name]; !seen || o.Pos.Line < l {
				first[n.Name] = o.Pos.Line
			}
		}
	}
	uses = make([]SnippetUse, 0, len(first))
	for code, line := range first {
		uses = append(uses, SnippetUse{code, line})
	}
	sort.Slice(uses, func(i, j int) bool { return uses[i].Code < uses[j].Code })
	for _, imp := range x.files["snippet.tr"].imports {
		if rel, ok := strings.CutPrefix(imp.file, libraryPrefix); ok && !slices.Contains(imports, rel) {
			imports = append(imports, rel)
		}
	}
	return uses, imports
}
