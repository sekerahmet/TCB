package testrunner

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Kural (TCB 2026-11-23 11:07 ve 11:57; Ahmet 10:53: "13.1a zaten tamamlanmışsa neden
// düzeltilmemiş zamanında ?"): docs/status.md'nin başındaki Ö0 ve Ö1 özet listeleri ile
// bölümlerin açık işleri iki yönde uyuşur.
//   - Listeden bölüme: listedeki her madde kendi bölümünde aynı öncelikle açık bir iş taşır
//     (başlığı "Yapılacaklar" ya da "uygulanacak" olan, etiketindeki ilk öncelik Ö0 ya da Ö1
//     olan satır); Ö0'da etiket "N. sıra" diyorsa N listedeki yerdir. Kapanan iş bölümünden
//     silinince listede kalan maddesi testi düşürür.
//   - Bölümden listeye: açık her Ö0 ve Ö1 satırı listede durur; Ö0 ve Ö1 yalnız A ya da B
//     sınıfınındır (status'un öncelik tablosu), öbür sınıf ancak priorityExceptions'ta adıyla.
//
// Listede madde numarası (15.23a), bölüm numarası ("17'nin …"; satır ### olmadan bölümün
// başında) ya da tırnak içinde ### kısa adı ("Yazıyı konuşmaya çevirme") yazılır.
func TestPriorityListsNameOpenItems(t *testing.T) {
	for _, p := range priorityListProblems(statusDoc(t), priorityExceptions) {
		t.Error(p)
	}
}

// priorityExceptions A ya da B olmadığı hâlde Ö1 taşıyan açık satırlar: bölüm başlığı → neden.
// Listeye girmezler. Satır Ö1'ini yitirince girdi bayatlar ve test düşer.
var priorityExceptions = map[string]string{
	"15.22a": "bf16 ve fp16 matris çarpımının tensör çekirdekli yolu: sınıfsız (hız), Ö1'i TCB'nin 2026-11-14 15:09 kararı (DENETİM 719, satırda yazılı); listeye girmez (TCB 2026-11-23 11:57)",
}

// Bilinen olumlu örnek: iki yönün her bozulma türü yakalanır, temiz maddeler yakalanmaz.
func TestPriorityListsCatchAClosedItem(t *testing.T) {
	doc := "## Önem ve öncelik\n\n**Ö0, sırayla:**\n1. 2.1, birinci (A).\n2. 2.2, ikinci (A).\n3. 2'nin sessiz yanlışları: yazma yolu (2.5a, veri eziyor).\n\n" +
		"**Ö1, dilimlerin kesin listeleri:** (2.9 burada değildi; kapandı.) 4a: 2.3, 2.4 (ya da 4c). Kol B: 21.2'nin sınırı, 9.9. " +
		"Sahipli kollar: X: 3'ün kod numarası, \"Kısa ad\", \"v0.2 ikilisi\".\n\n" +
		"## 2. Sözcük\n\n### 2.1\n\n- **Yapılacaklar (A, Ö0, 2. sıra; sahip X):** iş.\n\n" +
		"### 2.2\n\n- **Mevcut durum:** kapandı [test: `TestX`].\n\n" +
		"### 2.3\n\n- **Gemini Koşu 6, uygulanacak (yol ve `TR_f[T]` A, Ö1, 4a):** iş.\n\n" +
		"### 2.4\n\n- **Yapılacaklar (D, Ö3, 4a):** sessizliği kapandı.\n\n" +
		"### 2.5\n\n- **Mevcut durum:** yazma yolu kapandı [test: `TestY`].\n\n" +
		"### 2.6\n\n- **Yapılacaklar (A, Ö1; sahip Y):** listeye yazılmamış iş.\n\n" +
		"### 2.7\n\n- **Yapılacaklar (D, Ö1, 4a):** sınıfı Ö1'e uymuyor.\n\n" +
		"### 2.8\n\n- **Yapılacaklar (sınıfsız: hız; Ö1; sahip Z):** adlı istisna.\n\n" +
		"## 3. Kodlar\n\n- **Yapılacaklar (A, Ö1; sahip X):** numarası yeniden kullanılan kod.\n\n### 3.1\n\n- **Mevcut durum:** iş yok.\n\n" +
		"## Bölümü olmayan işler\n\n- **Yapılacaklar (B, Ö1; sahip X):** başlığı olmayan iş.\n\n" +
		"### Kısa ad\n\n- **Yapılacaklar (A, Ö1; sahip X):** başlıklı iş.\n\n" +
		"### v0.2 ikilisi (sürüm)\n\n- **Yapılacaklar (B, Ö1; sahip X):** ikinci başlıklı iş.\n\n" +
		"## 21. Dış\n\n### 21.1–21.8\n\n- **Yapılacaklar (A, Ö1, Kol B):** iş.\n"
	got := strings.Join(priorityListProblems(doc, map[string]string{"2.8": "neden", "2.10": "bayat"}), "\n")
	for _, want := range []string{
		"Ö0 listesinin 1. maddesi 2.1:", "Ö0 listesinin 2. maddesi 2.2:", "Ö0 listesinin 3. maddesi 2.5a:",
		"Ö1 listesindeki 2.4:", "Ö1 listesindeki 9.9:",
		"2.6 bölümündeki açık Ö1 satırı Ö1 listesinde yok", "2.7 bölümündeki açık Ö1 satırının sınıfı A ya da B değil",
		"Bölümü olmayan işler bölümündeki açık Ö1 satırının ### kısa ad başlığı yok", "istisnası bayat: 2.10",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q yakalanmadı:\n%s", want, got)
		}
	}
	for _, clean := range []string{"2.3", "2.8", "21.1–21.8", "Kısa ad", "v0.2 ikilisi", "3. Kodlar"} {
		if strings.Contains(got, " "+clean+" bölümündeki") || strings.Contains(got, "listesindeki "+clean+":") {
			t.Errorf("%s temiz, yakalanmamalıydı:\n%s", clean, got)
		}
	}
	if n := strings.Count(got, "\n") + 1; n != 9 {
		t.Errorf("dokuz sorun beklendi, %d geldi:\n%s", n, got)
	}
}

var (
	priorityItemRe    = regexp.MustCompile(`(?:^|[^\d.])(\d{1,2}\.\d{1,2}[a-z]?)(?:[^\d]|$)`)
	priorityChapterRe = regexp.MustCompile(`(?:^|[\s(])(\d{1,2})'(?:n?[iıuü]n)`)
	priorityNameRe    = regexp.MustCompile(`"([^"]+)"`)
	priorityO0LineRe  = regexp.MustCompile(`^(\d+)\. `)
	statusSubRe       = regexp.MustCompile(`^### (\d{1,2})\.(\d{1,2})([a-z]?)(?:–(\d{1,2})\.(\d{1,2}))?\s*$`)
	statusChapterRe   = regexp.MustCompile(`^## (\d{1,2})\. `)
	openItemHeadRe    = regexp.MustCompile(`^- \*\*([^*]*(?:Yapılacaklar|uygulanacak)[^*]*)\*\*`)
	firstPriorityRe   = regexp.MustCompile(`Ö\d`)
	classABRe         = regexp.MustCompile(`(?:^|[^\p{L}])[AB](?:[^\p{L}]|$)`)
	bracketRe         = regexp.MustCompile(`\[[^\]]*\]`)
	orderInLabelRe    = regexp.MustCompile(`(\d+)\. sıra`)
)

// priorityEntry listenin bir maddesi: kind "item" (madde numarası), "chapter" (bölüm numarası)
// ya da "name" (### kısa adı).
type priorityEntry struct {
	prio, kind, key, order string
	line                   int
}

// priorityListProblems iki yönün sorunlarını satır numarasıyla verir.
func priorityListProblems(doc string, exceptions map[string]string) []string {
	lines := strings.Split(doc, "\n")
	entries := parsePriorityLists(lines)
	var problems []string
	for _, e := range entries {
		problems = append(problems, checkPriorityEntry(lines, e)...)
	}
	return append(problems, unlistedPriorityItems(lines, entries, exceptions)...)
}

// parsePriorityLists Ö0 listesinin satırlarını ve Ö1 listesinin maddelerini okur.
func parsePriorityLists(lines []string) []priorityEntry {
	var entries []priorityEntry
	inO0 := false
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "**Ö0, sırayla:**"):
			inO0 = true
		case inO0 && strings.TrimSpace(l) != "":
			m := priorityO0LineRe.FindStringSubmatch(l)
			if m == nil {
				inO0 = false
				break
			}
			item := priorityItemRe.FindStringSubmatch(l[len(m[0]):])
			key := ""
			if item != nil {
				key = item[1]
			}
			entries = append(entries, priorityEntry{"Ö0", "item", key, m[1], i})
		}
		rest, ok := strings.CutPrefix(l, "**Ö1, dilimlerin kesin listeleri:**")
		if !ok {
			continue
		}
		rest = dropLeadingParen(rest)
		for _, m := range priorityNameRe.FindAllStringSubmatch(rest, -1) {
			entries = append(entries, priorityEntry{"Ö1", "name", m[1], "", i})
		}
		rest = priorityNameRe.ReplaceAllString(rest, "")
		seen := map[string]bool{}
		for _, m := range priorityItemRe.FindAllStringSubmatch(rest, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				entries = append(entries, priorityEntry{"Ö1", "item", m[1], "", i})
			}
		}
		for _, m := range priorityChapterRe.FindAllStringSubmatch(rest, -1) {
			entries = append(entries, priorityEntry{"Ö1", "chapter", m[1], "", i})
		}
	}
	return entries
}

// dropLeadingParen listenin başındaki açıklama ayracını (kapanmış bir maddenin notu) atar.
func dropLeadingParen(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "(") {
		return s
	}
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[i+1:]
			}
		}
	}
	return s
}

// checkPriorityEntry listenin maddesinin bölümünde aynı öncelikle açık bir iş arar.
func checkPriorityEntry(lines []string, e priorityEntry) []string {
	where := fmt.Sprintf("docs/status.md:%d: %s listesindeki %s", e.line+1, e.prio, e.key)
	if e.order != "" {
		where = fmt.Sprintf("docs/status.md:%d: Ö0 listesinin %s. maddesi %s", e.line+1, e.order, e.key)
		if e.key == "" {
			return []string{where + ": madde numarası (15.23a gibi) taşımıyor"}
		}
	}
	body, heading, ok := priorityEntrySection(lines, e)
	if !ok {
		return []string{where + ": status'ta bu maddenin bölümü yok"}
	}
	var labels []string
	for _, l := range body {
		if m := openItemHeadRe.FindStringSubmatch(l); m != nil && firstPriority(m[1]) == e.prio {
			labels = append(labels, m[1])
		}
	}
	if len(labels) == 0 {
		return []string{fmt.Sprintf("%s: bölümünde (%s) etiketinde %s geçen açık bir Yapılacaklar yok; madde kapandıysa ya da önceliği değiştiyse listeden çıkar", where, heading, e.prio)}
	}
	for _, lab := range labels {
		if m := orderInLabelRe.FindStringSubmatch(lab); e.order != "" && m != nil && m[1] != e.order {
			return []string{fmt.Sprintf("%s: bölümündeki etiket \"%s. sıra\" diyor, listedeki yeri %s", where, m[1], e.order)}
		}
	}
	return nil
}

// priorityEntrySection maddenin bölümünü bulur: madde numarası için statusSubsection, bölüm
// numarası için ## N.'nin ilk ### başlığına kadarki kısmı, kısa ad için ### ad (ya da "ad (…)").
func priorityEntrySection(lines []string, e priorityEntry) ([]string, string, bool) {
	switch e.kind {
	case "item":
		return statusSubsection(lines, e.key)
	case "chapter":
		for i, l := range lines {
			if m := statusChapterRe.FindStringSubmatch(l); m != nil && m[1] == e.key {
				return subsectionBody(lines, i), l, true
			}
		}
	case "name":
		for i, l := range lines {
			if h, ok := strings.CutPrefix(l, "### "); ok && namesHeading(h, e.key) {
				return subsectionBody(lines, i), l, true
			}
		}
	}
	return nil, "", false
}

func namesHeading(heading, name string) bool {
	return heading == name || strings.HasPrefix(heading, name+" (")
}

// firstPriority etiketteki ilk önceliği verir ("HTTP Ö0 tasarımı" gibi sonraki sözler sayılmaz).
func firstPriority(label string) string {
	return firstPriorityRe.FindString(label)
}

// unlistedPriorityItems bölümlerdeki açık Ö0 ve Ö1 satırlarından listede olmayanları, sınıfı A
// ya da B olmayanları ve bayat istisnaları verir.
func unlistedPriorityItems(lines []string, entries []priorityEntry, exceptions map[string]string) []string {
	var problems []string
	used := map[string]bool{}
	chapNum, chapTitle, sub := "", "", ""
	for i, l := range lines {
		if h, ok := strings.CutPrefix(l, "## "); ok {
			chapTitle, sub, chapNum = h, "", ""
			if m := statusChapterRe.FindStringSubmatch(l); m != nil {
				chapNum = m[1]
			}
			continue
		}
		if h, ok := strings.CutPrefix(l, "### "); ok {
			sub = strings.TrimSpace(h)
			continue
		}
		m := openItemHeadRe.FindStringSubmatch(l)
		if m == nil || chapTitle == "" {
			continue
		}
		prio := firstPriority(m[1])
		if prio != "Ö0" && prio != "Ö1" {
			continue
		}
		place := sub
		if place == "" {
			place = chapTitle
		}
		where := fmt.Sprintf("docs/status.md:%d: %s bölümündeki açık %s satırı", i+1, place, prio)
		if _, ok := exceptions[sub]; ok && sub != "" && prio == "Ö1" {
			used[sub] = true
			continue
		}
		if before := bracketRe.ReplaceAllString(m[1][:strings.Index(m[1], prio)], ""); !classABRe.MatchString(before) {
			problems = append(problems, where+"nın sınıfı A ya da B değil; Ö0 ve Ö1 yalnız A ya da B'nindir (status'un öncelik tablosu), istisnaysa priorityExceptions'a nedeniyle yazılır")
			continue
		}
		if sub == "" && chapNum == "" {
			problems = append(problems, where+"nın ### kısa ad başlığı yok; listede o adla anılır")
			continue
		}
		if !priorityCovered(entries, prio, chapNum, sub) {
			problems = append(problems, where+" "+prio+" listesinde yok; dilimiyle listeye yaz")
		}
	}
	for k := range exceptions {
		if !used[k] {
			problems = append(problems, "priorityExceptions'ın istisnası bayat: "+k+" bölümünde sınıfı A ya da B olmayan açık Ö1 satırı yok")
		}
	}
	return problems
}

// priorityCovered satırın bölümü listede mi: numaralı başlıkta madde (harfli ya da aralıktaki
// madde dahil), adlı başlıkta tırnaklı ad, başlıksız bölüm satırında bölüm numarası.
func priorityCovered(entries []priorityEntry, prio, chapNum, sub string) bool {
	for _, e := range entries {
		if e.prio != prio {
			continue
		}
		switch {
		case sub == "" && e.kind == "chapter":
			if e.key == chapNum {
				return true
			}
		case sub != "" && e.kind == "name":
			if namesHeading(sub, e.key) {
				return true
			}
		case sub != "" && e.kind == "item":
			if e.key == sub || strings.TrimRight(e.key, "abcdefghijklmnopqrstuvwxyz") == sub || inRangeHeading(sub, e.key) {
				return true
			}
		}
	}
	return false
}

// inRangeHeading "21.1–21.8" gibi aralıklı başlık maddeyi (21.2) içeriyor mu.
func inRangeHeading(sub, item string) bool {
	m := statusSubRe.FindStringSubmatch("### " + sub)
	if m == nil || m[4] == "" || m[1] != m[4] {
		return false
	}
	ch, num, ok := splitItem(item)
	lo, _ := strconv.Atoi(m[2])
	hi, _ := strconv.Atoi(m[5])
	return ok && strconv.Itoa(ch) == m[1] && lo <= num && num <= hi
}

// statusSubsection maddenin ### bölümünün satırlarını verir: önce tam başlık (### 15.23a),
// yoksa harfsiz başlık (13.1a için ### 13.1), yoksa maddeyi içeren aralık (### 21.1–21.8).
func statusSubsection(lines []string, item string) ([]string, string, bool) {
	if _, _, ok := splitItem(item); !ok {
		return nil, "", false
	}
	base := strings.TrimRight(item, "abcdefghijklmnopqrstuvwxyz")
	var baseStart, rangeStart = -1, -1
	for i, l := range lines {
		if statusSubRe.FindStringSubmatch(l) == nil {
			continue
		}
		h := strings.TrimSpace(strings.TrimPrefix(l, "### "))
		switch {
		case h == item:
			return subsectionBody(lines, i), l, true
		case h == base && baseStart < 0:
			baseStart = i
		case inRangeHeading(h, item) && rangeStart < 0:
			rangeStart = i
		}
	}
	for _, s := range []int{baseStart, rangeStart} {
		if s >= 0 {
			return subsectionBody(lines, s), lines[s], true
		}
	}
	return nil, "", false
}

func subsectionBody(lines []string, start int) []string {
	end := start + 1
	for end < len(lines) && !strings.HasPrefix(lines[end], "### ") && !strings.HasPrefix(lines[end], "## ") {
		end++
	}
	return lines[start+1 : end]
}

// splitItem "15.23a"yı bölüm (15) ve madde (23) sayısına ayırır; harf eki aralığı etkilemez.
func splitItem(item string) (int, int, bool) {
	a, b, ok := strings.Cut(strings.TrimRight(item, "abcdefghijklmnopqrstuvwxyz"), ".")
	if !ok {
		return 0, 0, false
	}
	ch, err1 := strconv.Atoi(a)
	num, err2 := strconv.Atoi(b)
	return ch, num, err1 == nil && err2 == nil
}
