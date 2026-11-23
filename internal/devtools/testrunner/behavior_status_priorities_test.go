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
// başında) ya da tırnak içinde ### kısa adı ("Yazıyı konuşmaya çevirme") yazılır. Ö1 listesi
// "**Ö1, dilimlerin kesin listeleri:**" satırı ile onun altındaki "**Sahipli kollar:**"
// başlığının sahip başına satırlarıdır (TCB 2026-11-23 11:57, 12:30). Harfli madde (13.8a)
// kendi ### başlığı yoksa yalnız kendi numarasını yazan satırı karşılar (DENETİM 955 N1).
func TestPriorityListsNameOpenItems(t *testing.T) {
	for _, p := range priorityListProblems(statusDoc(t), priorityExceptions) {
		t.Error(p)
	}
}

// priorityExceptions A ya da B olmadığı hâlde Ö1 taşıyan açık satırlar: bölüm başlığı → neden.
// Listeye girmezler. Satır Ö1'ini yitirince ya da sınıfı A ya da B olunca girdi bayatlar ve
// test düşer.
var priorityExceptions = map[string]string{
	"15.22a": "bf16 ve fp16 matris çarpımının tensör çekirdekli yolu: sınıfsız (hız), Ö1'i TCB'nin 2026-11-14 15:09 kararı (DENETİM 719, satırda yazılı); listeye girmez (TCB 2026-11-23 11:57)",
}

// Bilinen olumlu örnek: iki yönün her bozulma türü yakalanır, temiz maddeler yakalanmaz.
func TestPriorityListsCatchAClosedItem(t *testing.T) {
	doc := "## Önem ve öncelik\n\n**Ö0, sırayla:**\n1. 2.1, birinci (A).\n2. 2.2, ikinci (A).\n3. 2'nin sessiz yanlışları: yazma yolu (2.5a, veri eziyor).\n\n" +
		"**Ö1, dilimlerin kesin listeleri:** (2.9 burada değildi; kapandı.) 4a: 2.3, 2.4 (ya da 4c), 2.12a, 2.13b. Kol B: 21.2'nin sınırı, 9.9.\n\n" +
		"**Sahipli kollar:**\n- X: 3'ün kod numarası, \"Kısa ad\".\n- Y: \"v0.2 ikilisi\", 2.11.\n\n**Sonraki paragraf:** 2.15.\n\n" +
		"## 2. Sözcük\n\n### 2.1\n\n- **Yapılacaklar (A, Ö0, 2. sıra; sahip X):** iş.\n\n" +
		"### 2.2\n\n- **Mevcut durum:** kapandı [test: `TestX`].\n\n" +
		"### 2.3\n\n- **Gemini Koşu 6, uygulanacak (yol ve `TR_f[T]` A, Ö1, 4a):** iş.\n\n" +
		"### 2.4\n\n- **Yapılacaklar (D, Ö3, 4a):** sessizliği kapandı.\n\n" +
		"### 2.5\n\n- **Mevcut durum:** yazma yolu kapandı [test: `TestY`].\n\n" +
		"### 2.6\n\n- **Yapılacaklar (A, Ö1; sahip Y):** listeye yazılmamış iş.\n\n" +
		"### 2.7\n\n- **Yapılacaklar (D, Ö1, 4a):** sınıfı Ö1'e uymuyor.\n\n" +
		"### 2.8\n\n- **Yapılacaklar (sınıfsız: hız; Ö1; sahip Z):** adlı istisna.\n\n" +
		"### 2.12\n\n- **Yapılacaklar (A, Ö1, 4a):** 2.12a'nın işi.\n- **Yapılacaklar (B, Ö1, 4a):** listeye yazılmamış komşu iş.\n\n" +
		"### 2.13\n\n- **Yapılacaklar (A, Ö1, 4a):** harfli maddeyi anmayan iş.\n\n" +
		"### 2.14\n\n- **Yapılacaklar (A, Ö1; sahip Z):** istisnası bayatlamış iş.\n\n" +
		"### 2.15\n\n- **Mevcut durum:** listeye girmeyen paragrafın maddesi.\n\n" +
		"## 3. Kodlar\n\n- **Yapılacaklar (A, Ö1; sahip X):** numarası yeniden kullanılan kod.\n\n### 3.1\n\n- **Mevcut durum:** iş yok.\n\n" +
		"## Bölümü olmayan işler\n\n- **Yapılacaklar (B, Ö1; sahip X):** başlığı olmayan iş.\n\n" +
		"### Kısa ad\n\n- **Yapılacaklar (A, Ö1; sahip X):** başlıklı iş.\n\n" +
		"### v0.2 ikilisi (sürüm)\n\n- **Yapılacaklar (B, Ö1; sahip X):** ikinci başlıklı iş.\n\n" +
		"## 21. Dış\n\n### 21.1–21.8\n\n- **Yapılacaklar (A, Ö1, Kol B):** iş.\n"
	got := strings.Join(priorityListProblems(doc, map[string]string{"2.8": "neden", "2.10": "bayat", "2.14": "sınıfı artık A"}), "\n")
	for _, want := range []string{
		"Ö0 listesinin 1. maddesi 2.1:", "Ö0 listesinin 2. maddesi 2.2:", "Ö0 listesinin 3. maddesi 2.5a:",
		"Ö1 listesindeki 2.4:", "Ö1 listesindeki 9.9:",
		"2.6 bölümündeki açık Ö1 satırı Ö1 listesinde yok", "2.7 bölümündeki açık Ö1 satırının sınıfı A ya da B değil",
		"Bölümü olmayan işler bölümündeki açık Ö1 satırının ### kısa ad başlığı yok", "istisnası bayat: 2.10",
		// Sahipli kolların alt satırı da listedir: 2.11'in bölümü yok.
		"Ö1 listesindeki 2.11:",
		// Harfli madde komşu satırı karşılamaz, kendisini anmayan satırla karşılanmaz.
		"2.12 bölümündeki açık Ö1 satırı Ö1 listesinde yok", "Ö1 listesindeki 2.13b:", "2.13 bölümündeki açık Ö1 satırı Ö1 listesinde yok",
		// Sınıfı A olan satırın istisnası bayattır; satır da listede aranır.
		"istisnası bayat: 2.14", "2.14 bölümündeki açık Ö1 satırı Ö1 listesinde yok",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q yakalanmadı:\n%s", want, got)
		}
	}
	for _, clean := range []string{"2.3", "2.8", "2.12a", "2.15", "21.1–21.8", "Kısa ad", "v0.2 ikilisi", "3. Kodlar"} {
		if strings.Contains(got, " "+clean+" bölümündeki") || strings.Contains(got, ": "+clean+" (") || strings.Contains(got, "listesindeki "+clean+":") {
			t.Errorf("%s temiz, yakalanmamalıydı:\n%s", clean, got)
		}
	}
	if n := strings.Count(got, "2.12 bölümündeki"); n != 1 {
		t.Errorf("2.12'nin yalnız 2.12a'yı anmayan satırı yakalanmalıydı, %d satır geldi:\n%s", n, got)
	}
	if n := strings.Count(got, "\n") + 1; n != 15 {
		t.Errorf("on beş sorun beklendi, %d geldi:\n%s", n, got)
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
	seen := map[string]bool{}
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
		if rest, ok := strings.CutPrefix(l, "**Ö1, dilimlerin kesin listeleri:**"); ok {
			entries = append(entries, parseO1Line(dropLeadingParen(rest), i, seen)...)
			for _, j := range ownedBranchLines(lines, i) {
				entries = append(entries, parseO1Line(lines[j], j, seen)...)
			}
		}
	}
	return entries
}

// ownedBranchLines Ö1 satırının altındaki "**Sahipli kollar:**" başlığının sahip başına
// satırlarını ("- T1, tensör kolu: …") verir; araya yalnız boş satır girebilir.
func ownedBranchLines(lines []string, o1 int) []int {
	var out []int
	inBranches := false
	for j := o1 + 1; j < len(lines); j++ {
		l := lines[j]
		switch {
		case strings.TrimSpace(l) == "":
		case !inBranches && strings.HasPrefix(l, "**Sahipli kollar:**"):
			inBranches = true
		case inBranches && strings.HasPrefix(l, "- "):
			out = append(out, j)
		default:
			return out
		}
	}
	return out
}

// parseO1Line Ö1 listesinin bir satırındaki tırnaklı adları, madde ve bölüm numaralarını okur;
// seen aynı maddenin listede ikinci kez anılmasını sayar.
func parseO1Line(rest string, i int, seen map[string]bool) []priorityEntry {
	var entries []priorityEntry
	for _, m := range priorityNameRe.FindAllStringSubmatch(rest, -1) {
		entries = append(entries, priorityEntry{"Ö1", "name", m[1], "", i})
	}
	rest = priorityNameRe.ReplaceAllString(rest, "")
	for _, m := range priorityItemRe.FindAllStringSubmatch(rest, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			entries = append(entries, priorityEntry{"Ö1", "item", m[1], "", i})
		}
	}
	for _, m := range priorityChapterRe.FindAllStringSubmatch(rest, -1) {
		entries = append(entries, priorityEntry{"Ö1", "chapter", m[1], "", i})
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
	// Harfli madde kendi başlığında değilse (13.1a, ### 13.1'de) yalnız kendini anan satırdır.
	ownOnly := e.kind == "item" && strings.TrimSpace(strings.TrimPrefix(heading, "### ")) != e.key && hasLetter(e.key)
	var labels []string
	for _, l := range body {
		if m := openItemHeadRe.FindStringSubmatch(l); m != nil && firstPriority(m[1]) == e.prio && (!ownOnly || namesItem(l, e.key)) {
			labels = append(labels, m[1])
		}
	}
	if len(labels) == 0 {
		return []string{fmt.Sprintf("%s: bölümünde (%s) etiketinde %s geçen açık bir Yapılacaklar yok (harfli madde kendi başlığında değilse satırı numarasını anar); madde kapandıysa ya da önceliği değiştiyse listeden çıkar", where, heading, e.prio)}
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

func hasLetter(item string) bool {
	return strings.TrimRight(item, "abcdefghijklmnopqrstuvwxyz") != item
}

// namesItem satır maddenin numarasını (13.8a) yazıyor mu.
func namesItem(line, item string) bool {
	for _, m := range priorityItemRe.FindAllStringSubmatch(line, -1) {
		if m[1] == item {
			return true
		}
	}
	return false
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
		classAB := classABRe.MatchString(bracketRe.ReplaceAllString(m[1][:strings.Index(m[1], prio)], ""))
		// İstisna yalnız sınıfı hâlâ A ya da B olmayan satırı geçirir; A ya da B olan satır
		// listede aranır, istisna bayatlar (DENETİM 955 N1).
		if _, ok := exceptions[sub]; ok && sub != "" && prio == "Ö1" && !classAB {
			used[sub] = true
			continue
		}
		if !classAB {
			problems = append(problems, where+"nın sınıfı A ya da B değil; Ö0 ve Ö1 yalnız A ya da B'nindir (status'un öncelik tablosu), istisnaysa priorityExceptions'a nedeniyle yazılır")
			continue
		}
		if sub == "" && chapNum == "" {
			problems = append(problems, where+"nın ### kısa ad başlığı yok; listede o adla anılır")
			continue
		}
		if !priorityCovered(entries, prio, chapNum, sub, l) {
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

// priorityCovered satır listede mi: numaralı başlıkta başlığın maddesi ya da aralıktaki madde,
// harfli madde (13.8a) ise yalnız kendi başlığında ya da numarasını anan satırda; adlı başlıkta
// tırnaklı ad; başlıksız bölüm satırında bölüm numarası.
func priorityCovered(entries []priorityEntry, prio, chapNum, sub, line string) bool {
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
			if e.key == sub {
				return true
			}
			base := strings.TrimRight(e.key, "abcdefghijklmnopqrstuvwxyz")
			if (base == sub || inRangeHeading(sub, e.key)) && (!hasLetter(e.key) || namesItem(line, e.key)) {
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
