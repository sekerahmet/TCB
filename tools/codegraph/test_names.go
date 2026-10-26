package main

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Davranış testleri bir Go işlevini adıyla değil, uyguladığı dil adıyla sınar: TR kodundaki `kn.güncelle(1)`
// TR_mem_update'i kullanır, Go'da `update` TR_mem_update'i uygular. Testin Go çağrı grafiğinde `update` görünmez; bağ ad kodundan geçer. Bu dosya
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
// (kimliğin sınıfı fn, const, mem ya da arg); tip adları, anahtar sözcükler ve komutlar her yerde geçtiği için sayılmaz.
func implementable(n *Node) bool {
	if n.Kind != kindName {
		return false
	}
	switch codes.IdentityClass(n.Name) {
	case codes.ClassFunction, codes.ClassConstant, codes.ClassMember, codes.ClassNamedArg:
		return true
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
				if t.Kind != edgeTests || seenTest[t.From] || !ix.inTestBinary(ix.byID[t.From], n) {
					continue
				}
				seenTest[t.From] = true
				tn := ix.byID[t.From]
				local := path.Dir(tn.File) == path.Dir(n.File)
				s := steps + 1
				if steps == 0 && local {
					s = 0 // bildirimin kendisi uyguluyor, test aynı pakette: doğrudan (parti K2, A: stats.go'nun testi)
				}
				out = append(out, nameTest{test: tn, name: name, steps: s, local: local})
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

// inTestBinary bir Go testi fn'yi çalıştırabilir mi: fn'nin paketi testin ikilisine girer, yani testin paketi odur ya
// da (test dosyalarının içe aktarmaları dahil) onu dolaylı içe aktarır. Ad kodu bağı yalnızca bu koşulla geçer:
// checker'ın `yazdır` geçen testleri `package main`'deki tr.startsWithPrint'i hiç çalıştıramaz, ama ad kodu üzerinden
// onun testi sayılıyordu (TCB 2026-10-25, 3. madde: tests-for cmd/tr/main.go 175 test, başta checker testleri). TR testi
// (tr: düğümü) tr ikilisinde koşar, her paket ona girer.
func (ix *index) inTestBinary(t, fn *Node) bool {
	if t == nil || fn == nil || !strings.HasPrefix(t.ID, "go:") || !strings.HasPrefix(fn.ID, "go:") {
		return true
	}
	from, to := ix.dirPackage(t.File), ix.dirPackage(fn.File)
	if from == to {
		return true
	}
	if ix.linked == nil {
		ix.linked = map[string]map[string]bool{}
	}
	deps, ok := ix.linked[from]
	if !ok {
		deps = map[string]bool{}
		queue := []string{from}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, e := range ix.out[cur] {
				if e.Kind == edgeImports && !deps[e.To] {
					deps[e.To] = true
					queue = append(queue, e.To)
				}
			}
		}
		ix.linked[from] = deps
	}
	return deps[to]
}

// dirPackage bir Go dosyasının paketinin düğüm kimliğidir (dış test paketi `x_test` de klasörün paketidir).
func (ix *index) dirPackage(file string) string {
	if dir := path.Dir(file); dir != "." {
		return pkgID(ix.g.Module + "/" + dir)
	}
	return pkgID(ix.g.Module)
}

func (ix *index) printViaNames(w io.Writer, list []nameTest, limit int) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "  <- tests through the name codes it implements (%d; same package first):\n", len(list))
	forEachLimited(w, list, limit, "    ", moreHint, func(t nameTest) string {
		name, how := t.name.Detail, fmt.Sprintf("%d steps", t.steps)
		if name == "" {
			name = t.name.Name
		}
		if t.steps == 0 {
			how = "direct"
		}
		s := fmt.Sprintf("%s  via %s, %s  [INFERRED]", describe(t.test), name, how)
		if ix.note != nil {
			if note := ix.note(t.test); note != "" {
				s += "\n      " + note
			}
		}
		return s
	})
}
