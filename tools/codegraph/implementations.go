package main

import (
	"go/types"
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// "Hangi somut tip hangi arayüzü sağlar" sorusunun TEK cevabı: grafik çıkarıcısı (`implements` ve `dispatches_to`
// kenarları) ve `dead-methods` denetimi aynı fonksiyonu kullanır. Yalnızca testsiz paketlerin (goload'un Base
// evreni) bildirilmiş adlı tipleri alınır; tip parametreli tipler ve takma adlar dışarıdadır.

// implPair bir somut tipin sağladığı bir arayüzdür.
type implPair struct {
	concrete, iface *types.Named
}

// moduleImplementations modülün adlı arayüzlerini (en az bir yöntemli), somut tiplerini ve her somut tipin sağladığı
// arayüzleri (değer ya da gösterici alıcısıyla) paket yoluna ve ada göre sıralı verir.
func moduleImplementations(m *goload.Module) (concrete, ifaces []*types.Named, pairs []implPair) {
	paths := make([]string, 0, len(m.Base))
	for p := range m.Base {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		scope := m.Base[p].Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok || named.TypeParams().Len() > 0 {
				continue
			}
			if it, isIface := named.Underlying().(*types.Interface); isIface {
				if it.NumMethods() > 0 {
					ifaces = append(ifaces, named)
				}
			} else {
				concrete = append(concrete, named)
			}
		}
	}
	for _, t := range concrete {
		ptr := types.NewPointer(t)
		for _, i := range ifaces {
			it := i.Underlying().(*types.Interface)
			if types.Implements(t, it) || types.Implements(ptr, it) {
				pairs = append(pairs, implPair{t, i})
			}
		}
	}
	return concrete, ifaces, pairs
}

// implementingMethods arayüzün her yönteminin (gömülü arayüzlerinki dahil) somut tipteki karşılığını verir:
// (arayüz yöntemi, uygulayan yöntem) çiftleri. Uygulama gömülü bir alandan geliyorsa o alanın yöntemidir.
func implementingMethods(p implPair) [][2]*types.Func {
	it := p.iface.Underlying().(*types.Interface)
	set := types.NewMethodSet(types.NewPointer(p.concrete))
	var out [][2]*types.Func
	for k := 0; k < it.NumMethods(); k++ {
		im := it.Method(k)
		if sel := set.Lookup(im.Pkg(), im.Name()); sel != nil {
			if fn, ok := sel.Obj().(*types.Func); ok {
				out = append(out, [2]*types.Func{im, fn})
			}
		}
	}
	return out
}

// dispatchNote bir arayüz yönteminin uygulamalarını "dispatches to: a, b  [INFERRED]" olarak (alt satır) verir; arayüz
// yöntemi değilse boş. `callees` çağrının arayüzde kesilip uygulamaya geçişini gösterir.
func (ix *index) dispatchNote(id string) string {
	var names []string
	for _, e := range ix.out[id] {
		if e.Kind == edgeDispatches {
			names = append(names, ix.byID[e.To].Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return "\n        dispatches to: " + strings.Join(names, ", ") + "  [INFERRED]"
}
