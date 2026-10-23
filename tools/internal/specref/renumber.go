package specref

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Bu dosya madde yeniden numaralamanın planını ve uygulamasını içerir. Plan önce çıkar (değişecek her
// anma, dosya:satır); uygulama aynı planı dosyaya yazar. Her düzenleme özgün metindeki konumdan
// hesaplanır, bu yüzden zincirleme olmaz (15.3→15.4 ve 15.4→15.5 birlikte verilebilir). Yalnızca
// tanınan atıf biçimleri değişir; sürüm ya da ondalık sayı olan "15.0" gibi yazılara dokunulmaz.

// ParseMapping "eski yeni" satırlarını okur (`#` açıklama; ayırıcı boşluk, `->` ya da `→`).
func ParseMapping(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.NewReplacer("->", " ", "→", " ").Replace(line)
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 || !reToken.MatchString(fields[0]) || !reToken.MatchString(fields[1]) {
			return nil, fmt.Errorf("%s:%d: want \"old new\" (item numbers like 15.31 15.11)", path, n)
		}
		if _, dup := out[fields[0]]; dup {
			return nil, fmt.Errorf("%s:%d: %s is mapped twice", path, n, fields[0])
		}
		out[fields[0]] = fields[1]
	}
	return out, sc.Err()
}

// Validate eşlemeyi indekse karşı denetler: eski numara bir madde olmalı, yeni numaralar çakışmamalı
// (eşlenmeyen maddeler yerinde kalır).
func Validate(m map[string]string, ix *Index) error {
	taken := map[string]string{}
	for _, id := range ix.Order {
		target := id
		if n, ok := m[id]; ok {
			target = n
		}
		if prev, dup := taken[target]; dup {
			return fmt.Errorf("items %s and %s would both become %s", prev, id, target)
		}
		taken[target] = id
	}
	for old := range m {
		if _, ok := ix.Items[old]; !ok {
			return fmt.Errorf("%s is not an item of the language definition", old)
		}
	}
	return nil
}

// Edit dosyada değişecek bir bayt aralığıdır.
type Edit struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Start, End int    `json:"-"`
	Old, New   string
	Why        string // "renumber", "suffix"
}

// Plan eşlemeyi bütün anmalara uygular. Uygulanamayan anmalar (karmaşık aralıklar) Manual'da döner.
// fixAllSuffixes doğruysa eşlemeyle ilgisi olmayan hatalı ekler de düzeltilir.
func Plan(ms []Mention, files []File, ix *Index, m map[string]string, fixAllSuffixes bool) (edits []Edit, manual []Mention) {
	text := map[string]string{}
	for _, f := range files {
		text[f.Path] = f.Text
	}
	for _, mn := range ms {
		if mn.Kind == "weak" && len(m) == 0 {
			continue
		}
		es, ok := planMention(mn, text[mn.File], ix, m, fixAllSuffixes)
		if !ok {
			manual = append(manual, mn)
			continue
		}
		edits = append(edits, es...)
	}
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].File != edits[j].File {
			return edits[i].File < edits[j].File
		}
		return edits[i].Start < edits[j].Start
	})
	return edits, manual
}

// planMention tek bir anmanın düzenlemelerini hesaplar; ok=false anmanın elle ele alınması gerektiğini söyler.
func planMention(mn Mention, text string, ix *Index, m map[string]string, fixSuffix bool) (edits []Edit, ok bool) {
	toks := mn.Tokens
	if isPlainRange(mn) {
		if es, handled := planRange(mn, text, ix, m); handled {
			return withSuffix(es, mn, text, lastNew(es, mn, m), m, fixSuffix), true
		}
		if rangeTouched(mn, m) {
			return nil, false
		}
	}
	for _, t := range toks {
		if n, hit := m[t.ID]; hit && n != t.ID {
			edits = append(edits, Edit{File: mn.File, Line: mn.Line, Start: t.Start, End: t.End, Old: t.ID, New: n, Why: "renumber"})
		}
	}
	last := toks[len(toks)-1].ID
	if n, hit := m[last]; hit {
		last = n
	}
	return withSuffix(edits, mn, text, last, m, fixSuffix), true
}

// withSuffix numara değiştiyse (ya da fixSuffix ise) anmanın ekini yeni numaraya göre düzeltir.
func withSuffix(es []Edit, mn Mention, text, last string, m map[string]string, fixSuffix bool) []Edit {
	if mn.Suffix == "" || len(mn.Tokens) == 0 {
		return es
	}
	lastOld := mn.Tokens[len(mn.Tokens)-1].ID
	if !fixSuffix && last == lastOld {
		return es
	}
	want, ok := CheckSuffix(last, mn.Suffix)
	if ok || want == "" {
		return es
	}
	return append(es, Edit{File: mn.File, Line: mn.Line, Start: mn.SufPos, End: mn.SufPos + len(mn.Suffix),
		Old: mn.Suffix, New: want, Why: "suffix"})
}

// isPlainRange anma iki uçlu bir aralık mı (A–B).
func isPlainRange(mn Mention) bool { return mn.Range && len(mn.Tokens) == 2 }

// rangeTouched aralığın uçlarından biri eşlemede mi.
func rangeTouched(mn Mention, m map[string]string) bool {
	for _, t := range mn.Tokens {
		if _, hit := m[t.ID]; hit {
			return true
		}
	}
	return false
}

// planRange A–B aralığındaki bütün maddeleri yeni numaralarına çevirir, ardışık olanları aralık, olmayanları
// virgülle yazar ("15.31–15.38" → "15.11–15.15, 15.20–15.22"). Uç maddeler indekste yoksa ya da aralık iki
// bölüme yayılıyorsa handled=false döner.
func planRange(mn Mention, text string, ix *Index, m map[string]string) (edits []Edit, handled bool) {
	a, b := mn.Tokens[0].ID, mn.Tokens[1].ID
	ids := between(ix, a, b)
	if len(ids) == 0 {
		return nil, false
	}
	moved := false
	var news []string
	for _, id := range ids {
		n := id
		if x, hit := m[id]; hit {
			n = x
			moved = true
		}
		news = append(news, n)
	}
	if !moved {
		return nil, true
	}
	runs := compress(news)
	var rendered string
	var start, end int
	switch mn.Style {
	case "bold-inner", "bold-split":
		rendered = renderBold(runs, mn.Style == "bold-split")
		start, end = mn.Start, mn.End
	default:
		rendered = renderPlain(runs)
		start, end = mn.Tokens[0].Start, mn.Tokens[1].End
	}
	if end > len(text) || start > end {
		return nil, false
	}
	return []Edit{{File: mn.File, Line: mn.Line, Start: start, End: end, Old: text[start:end], New: rendered, Why: "renumber"}}, true
}

// lastNew aralık düzenlemesinden sonra son numarayı verir.
func lastNew(es []Edit, mn Mention, m map[string]string) string {
	last := mn.Tokens[len(mn.Tokens)-1].ID
	if len(es) == 1 {
		fields := reToken.FindAllString(es[0].New, -1)
		if len(fields) > 0 {
			return fields[len(fields)-1]
		}
	}
	return last
}

// between a ile b arasındaki (uçlar dahil) maddeleri dosya sırasıyla verir; ikisi de indekste olmalı.
func between(ix *Index, a, b string) []string {
	i, j := -1, -1
	for k, id := range ix.Order {
		if id == a {
			i = k
		}
		if id == b {
			j = k
		}
	}
	if i < 0 || j < i || chapterOf(a) != chapterOf(b) {
		return nil
	}
	return ix.Order[i : j+1]
}

// run ardışık numaralı bir madde dizisidir (tek madde ise Lo == Hi).
type run struct{ Lo, Hi string }

// compress numaraları sıralar, yinelenenleri atar ve ardışık olanları aralığa toplar.
func compress(ids []string) []run {
	sorted := append([]string(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return less(sorted[i], sorted[j]) })
	var runs []run
	for _, id := range sorted {
		if n := len(runs); n > 0 && (runs[n-1].Hi == id || follows(runs[n-1].Hi, id)) {
			runs[n-1].Hi = id
			continue
		}
		runs = append(runs, run{id, id})
	}
	return runs
}

// less numaraları bölüm ve madde sırasıyla karşılaştırır (harfli sonekler düz metin sırasıyla).
func less(a, b string) bool {
	ca, cb := chapterOf(a), chapterOf(b)
	if ca != cb {
		return ca < cb
	}
	ma, mb := itemNumber(a), itemNumber(b)
	if ma != mb {
		return ma < mb
	}
	return a < b
}

// itemNumber "15.27b" için 27.
func itemNumber(id string) int {
	n, _ := strconv.Atoi(strings.TrimRight(id[strings.IndexByte(id, '.')+1:], "abcdefghijklmnopqrstuvwxyz"))
	return n
}

// follows b, a'nın hemen ardından gelen madde mi (aynı bölüm, harfsiz, numara bir fazla).
func follows(a, b string) bool {
	if chapterOf(a) != chapterOf(b) || strings.TrimRight(a, "abcdefghijklmnopqrstuvwxyz") != a ||
		strings.TrimRight(b, "abcdefghijklmnopqrstuvwxyz") != b {
		return false
	}
	return itemNumber(b) == itemNumber(a)+1
}

func renderPlain(runs []run) string {
	parts := make([]string, len(runs))
	for i, r := range runs {
		parts[i] = r.Lo
		if r.Lo != r.Hi {
			parts[i] = r.Lo + "–" + r.Hi
		}
	}
	return strings.Join(parts, ", ")
}

func renderBold(runs []run, split bool) string {
	parts := make([]string, len(runs))
	for i, r := range runs {
		switch {
		case r.Lo == r.Hi:
			parts[i] = "**" + r.Lo + "**"
		case split:
			parts[i] = "**" + r.Lo + "**–**" + r.Hi + "**"
		default:
			parts[i] = "**" + r.Lo + "–" + r.Hi + "**"
		}
	}
	return strings.Join(parts, ", ")
}

// Apply düzenlemeleri kökteki dosyalara yazar (özgün baytlar korunur; CRLF değişmez). Her düzenlemenin
// eski metni dosyada o konumda değilse (dosya planından sonra değişmiş) hata verir ve hiçbir şey yazmaz.
func Apply(root string, edits []Edit) error {
	byFile := map[string][]Edit{}
	var names []string
	for _, e := range edits {
		if _, seen := byFile[e.File]; !seen {
			names = append(names, e.File)
		}
		byFile[e.File] = append(byFile[e.File], e)
	}
	updated := map[string][]byte{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		es := byFile[name]
		sort.Slice(es, func(i, j int) bool { return es[i].Start > es[j].Start })
		for k, e := range es {
			if e.End > len(data) || string(data[e.Start:e.End]) != e.Old {
				return fmt.Errorf("%s:%d: file changed since the plan (expected %q)", name, e.Line, e.Old)
			}
			if k > 0 && es[k-1].Start < e.End {
				return fmt.Errorf("%s:%d: overlapping edits", name, e.Line)
			}
			data = append(data[:e.Start:e.Start], append([]byte(e.New), data[e.End:]...)...)
		}
		updated[name] = data
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), updated[name], 0o644); err != nil {
			return err
		}
	}
	return nil
}
