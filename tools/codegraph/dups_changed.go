package main

import (
	"fmt"
	"strings"
)

// Bu dosya `dups --changed`: yalnızca bir dalın eklediği ya da değiştirdiği satırlara dokunan tekrar
// adaylarını bırakır. Asıl soru "bu dalda yazılan kod, depoda zaten var olan koda benziyor mu"dur:
// kümenin bir konumu değişen satırlarda, öbürleri nerede olursa olsun. `changed` komutunun hunk
// eşlemesi kullanılır (git diff -U0); dal çıkış yapılmış (HEAD) olmalıdır, çünkü satır numaraları
// çalışma ağacındaki dosyalarındır.

// changedLines dosya başına değişen (eklenen ya da değişen) satır aralıklarıdır.
type changedLines map[string][][2]int

// changedGoLines ref'in (dal ya da a..b / a...b aralığı) Go dosyalarında yeni taraftaki değişen satırlarını verir.
func changedGoLines(root, base, ref string) (changedLines, error) {
	rng, right := base+"..."+ref, ref
	if strings.Contains(ref, "..") {
		rng, right = ref, ref[strings.LastIndex(ref, ".")+1:]
	}
	if !sameCommit(root, right, "HEAD") {
		return nil, fmt.Errorf("--changed %s: %s is not the checked-out commit; check the branch out first (the line numbers are those of the working tree)", ref, right)
	}
	hs, err := diffHunks(root, rng)
	if err != nil {
		return nil, err
	}
	out := changedLines{}
	for _, h := range hs {
		if strings.HasSuffix(h.file, ".go") && h.newCount > 0 {
			out[h.file] = append(out[h.file], [2]int{h.newStart, h.newStart + h.newCount - 1})
		}
	}
	return out, nil
}

// touches [from, to] aralığı değişen satırlardan birine dokunuyor mu.
func (cl changedLines) touches(file string, from, to int) bool {
	for _, s := range cl[file] {
		if from <= s[1] && s[0] <= to {
			return true
		}
	}
	return false
}

// filterChanged değişen satırlara dokunan kümeleri bırakır ve o konumları işaretler.
func filterChanged(clusters []*dupCluster, root string, o *dupOpts) ([]*dupCluster, error) {
	cl, err := changedGoLines(root, o.base, o.changed)
	if err != nil {
		return nil, err
	}
	var out []*dupCluster
	for _, c := range clusters {
		hit := false
		for i := range c.Locs {
			l := &c.Locs[i]
			if cl.touches(l.File, l.From, l.To) {
				l.Changed, hit = true, true
			}
		}
		if hit {
			out = append(out, c)
		}
	}
	return out, nil
}
