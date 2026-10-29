package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// identityMaps bir kodu yeni anahtarına bağlayan belgelerdir (modül köküne göre). Her tablo
// satırı "| eski | `yeni` | ..." bir yeniden adlandırma olayıdır: geçmiş okunurken eski anahtarın
// silinişi ile yeninin eklenişi tek olaydır (deleted'a girmez, history eski anahtarın geçmişiyle
// sürer), eski anahtar sorgusu yeniye gider (renamedCode). Madde 15 adım 0'ın tablosu ilk satırları
// verdi (eski numaralı kod → TR_kw_if); bir kimlik ileride yeniden adlandırılırsa satırı aynı biçimle bir
// eşleme belgesine yazılır ("| TR_eski | `TR_yeni` |") ve zincir izlenir (eski kod → TR_a → TR_b).
var identityMaps = []string{"docs/code_identities.md"}

// renameRow eşleme tablosunun bir satırıdır: ilk iki hücre iki kod anahtarıdır (ters tırnaklı ya da değil).
var renameRow = regexp.MustCompile("^\\|\\s*`?(" + codeKey + ")`?\\s*\\|\\s*`?(" + codeKey + ")`?\\s*\\|")

// readRenames eşleme belgelerindeki eski → yeni çiftlerini verir; belge yoksa boş.
func readRenames(root string) map[string]string {
	renames := map[string]string{}
	for _, rel := range identityMaps {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if m := renameRow.FindStringSubmatch(strings.TrimSpace(line)); m != nil && m[1] != m[2] {
				renames[m[1]] = m[2]
			}
		}
	}
	return renames
}

// finalKey bir anahtarın yeniden adlandırma zincirinin sonudur (döngüde zincirin başı kalır).
func finalKey(renames map[string]string, k string) string {
	seen := map[string]bool{k: true}
	for next, ok := renames[k]; ok && !seen[next]; next, ok = renames[k] {
		seen[next] = true
		k = next
	}
	return k
}

// formerKeys her bugünkü anahtara zincirle varan eski anahtarlardır (sıralı).
func formerKeys(renames map[string]string) map[string][]string {
	out := map[string][]string{}
	for old := range renames {
		f := finalKey(renames, old)
		out[f] = append(out[f], old)
	}
	for _, list := range out {
		slices.Sort(list)
	}
	return out
}

// followRenames her zincirin olaylarını son anahtarda birleştirir: aynı commit'te aynı dosyada
// eski anahtarın silinişi ve yeninin eklenişi tek "renamed" olayı olur; eski anahtar listeden
// çıkar. Olaylar git log sırasında (yeniden eskiye) kalır.
func followRenames(events map[string][]codeEvent, renames map[string]string) map[string][]codeEvent {
	out := map[string][]codeEvent{}
	for k, list := range events {
		f := finalKey(renames, k)
		out[f] = append(out[f], list...)
	}
	for f, list := range out {
		slices.SortStableFunc(list, func(a, b codeEvent) int { return a.seq - b.seq })
		out[f] = joinRenames(list, renames)
	}
	sortEvents(out)
	return out
}

// joinRenames aynı commit ve dosyadaki "eski silindi" ile "yeni eklendi" çiftini tek olay yapar.
func joinRenames(list []codeEvent, renames map[string]string) []codeEvent {
	type at struct {
		seq       int
		file, key string
	}
	added := map[at]int{} // yeninin eklenişi → listedeki yeri
	for i, e := range list {
		if e.added {
			added[at{e.seq, e.file, e.key}] = i
		}
	}
	drop := map[int]bool{}
	for i, r := range list {
		if j, ok := added[at{r.seq, r.file, renames[r.key]}]; ok && r.removed {
			list[j].added, list[j].from = false, r.key
			drop[i] = true
		}
	}
	out := make([]codeEvent, 0, len(list)-len(drop))
	for i, e := range list {
		if !drop[i] {
			out = append(out, e)
		}
	}
	return out
}

// renamedCode eski bir anahtarın bugünkü ad ya da ileti kodu düğümüdür; q eski anahtar değilse nil. Takma ad değildir:
// eski anahtar yalnız hiçbir düğüme çıkmayınca yönlendirilir, ad çözümü tr index'inkiyle aynı kalır.
func (ix *index) renamedCode(q string) *Node {
	renames := readRenames(ix.root)
	if _, old := renames[q]; !old {
		return nil
	}
	k := finalKey(renames, q)
	if n := ix.byID["name:"+k]; n != nil {
		return n
	}
	return ix.byID["msg:"+k]
}
