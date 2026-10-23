package codeindex

import (
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Bir Go test dizesinde ya da başka bir metinde bulunan TR kodunun kullandığı yerleşik adlar: dizin dosyadan değil
// metinden kurulur ve dosyalarla aynı çözümleyici (resolve) çalışır, bu yüzden yerel adlar, kapsam ve üye/adlı argüman
// kuralları dosyadakiyle aynıdır. Kullanıcı tanımlı işlevler tek dosyalı metinde yoktur; yalnızca ad kodları döner.

// SnippetUse bir metinde kullanılan ad kodudur ("T720", "Y010", "T984") ve ilk kullanıldığı satır (1'den).
type SnippetUse struct {
	Code string
	Line int
}

// Snippet src'yi TR programı olarak ayrıştırıp kullandığı yerleşik, üye ve adlı argüman kodlarını verir (koda göre
// sıralı, her kod bir kez). Metin ayrıştırılamıyorsa nil: TR olmayan bir metin ya da eksik parça.
func Snippet(src string) []SnippetUse {
	x := &indexer{root: "", lang: source.Turkish(), files: map[string]*file{}, nodes: map[string]*Node{}, edges: map[edgeKey]*Edge{}}
	x.addFile("snippet.tr", []byte(src), nil)
	if len(x.order) == 0 {
		return nil
	}
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
		if n := x.nodes[o.Node]; n != nil && n.Kind == KindBuiltin && strings.HasPrefix(o.Pos.File, "snippet.tr") {
			if l, seen := first[n.Name]; !seen || o.Pos.Line < l {
				first[n.Name] = o.Pos.Line
			}
		}
	}
	out := make([]SnippetUse, 0, len(first))
	for code, line := range first {
		out = append(out, SnippetUse{code, line})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}
