package codeindex

import "sort"

// Reader bir dizini sorgulamak için hazırlar: kimlik tablosu ve dosya başına sıralı başvurular.
// Dizin değişmez; Reader onun üzerinde salt okunur bir görünümdür (dil sunucusu her istekte
// aynı Reader'ı kullanır, dosya değişince yeni Index'ten yenisini kurar).
type Reader struct {
	Index  *Index
	byID   map[string]*Node
	byFile map[string][]int // dosya -> Index.Occurrences sıraları (satır, sütun sırasıyla)
	// builtins yerleşik adı (her gömülü dilde) -> düğüm kimlikleri.
	builtins  map[string][]string
	callers   map[string][]string // düğüm -> onu çağıran düğümler (calls kenarları, ters)
	importers map[string][]string // modül -> onu aktaran modüller (imports kenarları, ters)
}

// NewReader dizinden bir Reader kurar.
func NewReader(idx *Index) *Reader {
	r := &Reader{Index: idx, byID: make(map[string]*Node, len(idx.Nodes)), byFile: map[string][]int{}}
	for i := range idx.Nodes {
		r.byID[idx.Nodes[i].ID] = &idx.Nodes[i]
	}
	r.builtins = builtinNames(idx)
	r.callers, r.importers = map[string][]string{}, map[string][]string{}
	for _, e := range idx.Edges {
		switch e.Kind {
		case EdgeCalls, EdgeUses:
			// Zincire giren bağlar codegraph'ınkiyle aynıdır (test_reach.go reachEdge: TR tanımına çağrı ve kullanım;
			// işlev değer olarak geçip başka yerde çağrılabilir; yerleşik ada kullanım grafikte uses_name'dir, zincire
			// girmez, yerleşiği yalnızca çağıran test sınar). İki aracın sınayan kümesi aynı olur (DENETİM doğrulaması T3).
			if e.Kind == EdgeUses && r.byID[e.To] != nil && r.byID[e.To].Kind == KindBuiltin {
				continue
			}
			r.callers[e.To] = append(r.callers[e.To], e.From)
		case EdgeImports:
			r.importers[e.To] = append(r.importers[e.To], e.From)
		}
	}
	for i, o := range idx.Occurrences {
		r.byFile[o.Pos.File] = append(r.byFile[o.Pos.File], i)
	}
	for _, list := range r.byFile {
		sort.SliceStable(list, func(a, b int) bool {
			pa, pb := idx.Occurrences[list[a]].Pos, idx.Occurrences[list[b]].Pos
			if pa.Line != pb.Line {
				return pa.Line < pb.Line
			}
			return pa.Col < pb.Col
		})
	}
	return r
}

// Node kimliği verilen düğümdür (yoksa nil).
func (r *Reader) Node(id string) *Node { return r.byID[id] }

// At dosyada satır ve sütundaki başvuruyu ve işaret ettiği düğümü verir: "imlecin altındaki ad
// neye başvuruyor". Konum adın kapsadığı aralıktaysa (sütun ≥ başlangıç ve < bitiş) eşleşir;
// bulunamazsa nil döner. Dil sunucusunun tanıma git, kullanımları bul ve yeniden adlandırma
// isteklerinin başlangıcıdır.
func (r *Reader) At(file string, line, col int) (*Occurrence, *Node) {
	for _, i := range r.byFile[file] {
		o := &r.Index.Occurrences[i]
		if o.Pos.Line != line || o.Basis == BasisNameOnly {
			continue
		}
		if col >= o.Pos.Col && (o.Pos.EndCol == 0 || col < o.Pos.EndCol) {
			return o, r.byID[o.Node]
		}
	}
	return nil, nil
}

// Occurrences bir düğüme yapılan bütün başvurulardır (tanım dahil), dosya ve satır sırasıyla.
func (r *Reader) Occurrences(id string) []Occurrence {
	var out []Occurrence
	for _, o := range r.Index.Occurrences {
		if o.Node == id {
			out = append(out, o)
		}
	}
	return out
}
