package main

import (
	"fmt"
	"maps"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Planlama sınavının ad denetimi (TestPlanningQuestionsAreAnswered'ın başı): iki aracın çıplak TR adına aynı adayları
// verdiği. planning_exam_test.go'dan ayrıldı (dosya 600 satır sınırındaydı).

// trNameSetMismatches her TR adını iki araca sorar: `trlang index` (codeindex.Resolve) ile codegraph (trByName, kaynakta
// yazıldığı gibi sorulunca lookup'ın kendisi) çıplak ad için aynı TR aday kümesini vermeli (deneme 7 R1: tanım 3,
// yeniden_adlandır 7, query 4 aday veriyordu; bağımsız denetim grup 6 R1: büyük harfli sorgu, modül + yerleşik ve
// yerleşik + test çakışması farklıydı). Adlar: her bildirimin kısa adı, modülün adı ve takma adları, testin açıklaması,
// dizindeki her yerleşiğin bütün adları; her ad yazıldığı gibi, küçük harfle, Türkçe ve ASCII büyük harfle (`GİZLİ_DURUM`, `GIZLI_DURUM`) sorulur.
// Karşılaştırma trlang index'in bildiği düğümlerdedir: grafiğin ad kodlarından yalnızca dizinde yerleşik düğümü olanlar
// sayılır (anahtar sözcükler ve kullanılmayan yerleşikler yalnızca grafiktedir).
func trNameSetMismatches(g *Graph, idx *codeindex.Index, reader *codeindex.Reader) (bad []string, checked int) {
	ix := newIndex(g)
	known := map[string]bool{} // trlang index'in düğümlerinin grafik kimlikleri
	names := map[string]bool{}
	for _, n := range idx.Nodes {
		id := graphID(n)
		known[id] = true
		switch n.Kind {
		case codeindex.KindBuiltin:
			if gn := ix.byID[id]; gn != nil {
				for _, a := range nameNodeNames(gn) {
					names[a] = true
				}
			}
		case codeindex.KindModule:
			names[n.Name] = true
			for _, a := range n.Aliases {
				names[a] = true
			}
		case codeindex.KindTest:
			names[n.Name] = true // açıklamada '.' olabilir: kısaltılmaz
		default:
			for _, a := range codeindex.DeclNames(&n) {
				names[a] = true
			}
			if codeindex.IsLibraryDecl(&n) {
				names[n.Code] = true
			}
		}
	}
	var asks []nameAsk // trlang index'in cevabı sırayla toplanır (başka paketin okuyucusu)
	for _, name := range slices.Sorted(maps.Keys(names)) {
		for vi, q := range []string{name, codeindex.Fold(name), strings.ToUpperSpecial(unicode.TurkishCase, name), strings.ToUpper(name)} {
			if vi > 0 && q == name {
				continue
			}
			want := map[string]bool{}
			for _, n := range reader.Resolve(q, "") {
				want[graphID(*n)] = true
			}
			asks = append(asks, nameAsk{q, vi == 0, want})
		}
	}
	// codegraph'ın her araması bütün düğümleri tarar; sınavın en büyük kalemiydi (G4 profili 2026-11-14: 80 sn'nin
	// 41'i). Aramalar paralel koşar; arama yolunun iki tembel önbelleği önce bir kez dolar.
	ix.nodeFolds()
	ix.trNameTable()
	out := make([]string, len(asks))
	var wg sync.WaitGroup
	var next atomic.Int64
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for i := int(next.Add(1)) - 1; i < len(asks); i = int(next.Add(1)) - 1 {
				out[i] = ix.trNameMismatch(asks[i], known)
			}
		})
	}
	wg.Wait()
	for _, b := range out {
		if b != "" {
			bad = append(bad, b)
		}
	}
	sort.Strings(bad)
	return bad, len(asks)
}

// nameAsk trNameSetMismatches'in bir sorusudur: sorulan yazım, yazıldığı gibi mi sorulduğu ve trlang index'in cevabı.
type nameAsk struct {
	q     string
	first bool
	want  map[string]bool
}

// trNameMismatch bir sorunun codegraph cevabını trlang index'inkiyle karşılaştırır; aynıysa boş döner.
func (ix *index) trNameMismatch(a nameAsk, known map[string]bool) string {
	got := map[string]bool{}
	found, _ := ix.trByName(a.q, nil)
	if a.first && (!strings.ContainsAny(a.q, "./:") || codeindex.IdentityQuery(a.q)) {
		found = ix.lookup(a.q) // yazıldığı gibi sorulan adda sorgunun kendisi
	}
	for _, n := range found {
		if known[n.ID] {
			got[n.ID] = true
		}
		// Kitaplık kodunun kimliği grafikte ad düğümünü, dizinde bildirimi gösterir: ikisi aynı koddur.
		if k, err := codes.Parse(strings.TrimPrefix(n.ID, "name:")); err == nil && k.IsLibrary() {
			for _, e := range ix.in[n.ID] {
				if e.Kind == edgeUsesName && known[e.From] && strings.HasPrefix(e.From, "tr:") {
					got[e.From] = true
				}
			}
		}
	}
	if maps.Equal(a.want, got) {
		return ""
	}
	return fmt.Sprintf("%q: trlang index %d %v, codegraph %d %v", a.q, len(a.want), slices.Sorted(maps.Keys(a.want)), len(got), slices.Sorted(maps.Keys(got)))
}
