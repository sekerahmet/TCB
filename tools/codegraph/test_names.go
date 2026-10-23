package main

import (
	"fmt"
	"io"
	"path"
	"sort"
)

// Davranış testleri bir Go işlevini adıyla değil, uyguladığı dil adıyla sınar: TR kodundaki `kn.güncelle(1)` Y010'u
// kullanır, Go'da `update` Y010'u uygular. Testin Go çağrı grafiğinde `update` görünmez; bağ ad kodundan geçer. Bu dosya
// bir işlevin (ve onu çağıran işlevlerin) uyguladığı yerleşik, üye ve adlı argüman kodlarının testlerini işlevin
// testleri olarak gösterir: `tests-for checkpoint.Checkpoint.saveOnInterval`, `update` üzerinden `güncelle` testlerini bulur.

// nameTest bir işlevin uyguladığı ad koduyla bulunan test.
type nameTest struct {
	test  *Node
	name  *Node
	steps int  // işlevden testin adına kadar adım (çağıran adımları + ad adımı)
	local bool // test işlevle aynı paket klasöründe
}

// implementable bir ad kodunun test edilebilir bir dil öğesi olup olmadığı: global yerleşik, üye ya da adlı argüman
// (G/T/V/W/Y harfli kodlar); tip adları, anahtar sözcükler ve komutlar her yerde geçtiği için sayılmaz.
func implementable(n *Node) bool {
	return n.Kind == kindName && len(n.Name) > 0 && containsByte("GTVWY", n.Name[0])
}

func containsByte(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
}

// testsThroughNames n'den çağıranlara doğru (walkCallers: tek zincir kuralı) yürür; n'nin ve çağıranlarının uyguladığı
// (uses_name) ad kodlarının testlerini verir. Aynı test bir kez, en yakın haliyle gelir; n ile aynı paketteki testler önce.
func (ix *index) testsThroughNames(n *Node) []nameTest {
	if n.Kind != kindFunc && n.Kind != kindMethod {
		return nil
	}
	seenTest := map[string]bool{}
	var out []nameTest
	collect := func(id string, steps int) {
		for _, e := range ix.out[id] {
			name := ix.byID[e.To]
			if e.Kind != edgeUsesName || name == nil || !implementable(name) {
				continue
			}
			for _, t := range ix.in[name.ID] {
				if t.Kind != edgeTests || seenTest[t.From] {
					continue
				}
				seenTest[t.From] = true
				tn := ix.byID[t.From]
				out = append(out, nameTest{test: tn, name: name, steps: steps + 1, local: path.Dir(tn.File) == path.Dir(n.File)})
			}
		}
	}
	collect(n.ID, 0)
	walkCallers(n.ID, ix.callersOf, ix.pkgOf, func(r reached) bool {
		if isTestNode(ix.byID[r.ID]) {
			return false
		}
		collect(r.ID, r.Steps)
		return true
	})
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.local != b.local {
			return a.local
		}
		if a.steps != b.steps {
			return a.steps < b.steps
		}
		return a.test.Name < b.test.Name
	})
	return out
}

func (ix *index) printViaNames(w io.Writer, list []nameTest, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests through the name codes it implements (%d; same package first):\n", len(list))
	forEachLimited(w, list, limit, "    ", moreHint, func(t nameTest) string {
		s := fmt.Sprintf("%s  via %s, %d steps  [INFERRED]", describe(t.test), t.name.Detail, t.steps)
		if t.name.Detail == "" {
			s = fmt.Sprintf("%s  via %s, %d steps  [INFERRED]", describe(t.test), t.name.Name, t.steps)
		}
		if ix.note != nil {
			if note := ix.note(t.test); note != "" {
				s += "\n      " + note
			}
		}
		return s
	})
}
