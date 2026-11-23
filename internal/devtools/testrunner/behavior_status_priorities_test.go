package testrunner

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Kural (TCB 2026-11-23 11:07; Ahmet 10:53: "13.1a zaten tamamlanmışsa neden düzeltilmemiş
// zamanında ?"): docs/status.md'nin başındaki Ö0 ve Ö1 özet listelerinin her maddesi kendi
// bölümünde aynı öncelikle açık bir iş taşır: başlığı "Yapılacaklar" ya da "uygulanacak" olan,
// etiketinde Ö0 (Ö1) geçen bir satır. Ö0'da etiket "N. sıra" diyorsa N listedeki yerdir.
// Kapanan iş bölümünden silinince listede kalan maddesi bu testi düşürür.
func TestPriorityListsNameOpenItems(t *testing.T) {
	for _, p := range priorityListProblems(statusDoc(t)) {
		t.Error(p)
	}
}

// Bilinen olumlu örnek: kapanmış (bölümünde açık işi kalmamış) madde, sırası kaymış madde ve
// bölümü olmayan madde yakalanır; aralıklı başlık (### 21.1–21.8) maddesini bulur.
func TestPriorityListsCatchAClosedItem(t *testing.T) {
	doc := "## Önem ve öncelik\n\n**Ö0, sırayla:**\n1. 2.1, birinci (A).\n2. 2.2, ikinci (A).\n3. 2'nin sessiz yanlışları: yazma yolu (2.5a, veri eziyor).\n\n" +
		"**Ö1, dilimlerin kesin listeleri:** (2.9 burada değildi; kapandı.) 4a: 2.3, 2.4 (ya da 4c). Kol B: 21.2'nin sınırı, 9.9.\n\n" +
		"## 2. Sözcük\n\n### 2.1\n\n- **Yapılacaklar (A, Ö0, 2. sıra; sahip X):** iş.\n\n" +
		"### 2.2\n\n- **Mevcut durum:** kapandı [test: `TestX`].\n\n" +
		"### 2.3\n\n- **Gemini Koşu 6, uygulanacak (A, Ö1, 4a):** iş.\n\n" +
		"### 2.4\n\n- **Yapılacaklar (D, Ö3, 4a):** sessizliği kapandı.\n\n" +
		"### 2.5\n\n- **Mevcut durum:** yazma yolu kapandı [test: `TestY`].\n\n" +
		"## 21. Dış\n\n### 21.1–21.8\n\n- **Yapılacaklar (A, Ö1, Kol B):** iş.\n"
	got := strings.Join(priorityListProblems(doc), "\n")
	for _, want := range []string{"Ö0 listesinin 1. maddesi 2.1:", "Ö0 listesinin 2. maddesi 2.2:", "Ö0 listesinin 3. maddesi 2.5a:", "Ö1 listesindeki 2.4:", "Ö1 listesindeki 9.9:"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q yakalanmadı:\n%s", want, got)
		}
	}
	for _, clean := range []string{"2.3", "21.2", "2.9"} {
		if strings.Contains(got, "listesindeki "+clean+":") {
			t.Errorf("%s açık ya da listede değil, yakalanmamalıydı:\n%s", clean, got)
		}
	}
	if n := strings.Count(got, "\n") + 1; n != 5 {
		t.Errorf("beş sorun beklendi, %d geldi:\n%s", n, got)
	}
}

var (
	priorityItemRe   = regexp.MustCompile(`(?:^|[^\d.])(\d{1,2}\.\d{1,2}[a-z]?)(?:[^\d]|$)`)
	priorityO0LineRe = regexp.MustCompile(`^(\d+)\. `)
	statusSubRe      = regexp.MustCompile(`^### (\d{1,2})\.(\d{1,2})([a-z]?)(?:–(\d{1,2})\.(\d{1,2}))?\s*$`)
	openItemHeadRe   = regexp.MustCompile(`^- \*\*([^*]*(?:Yapılacaklar|uygulanacak)[^*]*)\*\*`)
	orderInLabelRe   = regexp.MustCompile(`(\d+)\. sıra`)
)

// priorityListProblems Ö0 ve Ö1 listelerinin, bölümünde aynı öncelikle açık işi olmayan
// maddelerini satır numarasıyla verir.
func priorityListProblems(doc string) []string {
	lines := strings.Split(doc, "\n")
	var problems []string
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
			if item == nil {
				problems = append(problems, fmt.Sprintf("docs/status.md:%d: Ö0 listesinin %s. maddesi madde numarası (15.23a gibi) taşımıyor", i+1, m[1]))
				break
			}
			problems = append(problems, checkPriorityItem(lines, i, "Ö0", item[1], m[1])...)
		}
		if rest, ok := strings.CutPrefix(l, "**Ö1, dilimlerin kesin listeleri:**"); ok {
			seen := map[string]bool{}
			for _, m := range priorityItemRe.FindAllStringSubmatch(dropLeadingParen(rest), -1) {
				if !seen[m[1]] {
					seen[m[1]] = true
					problems = append(problems, checkPriorityItem(lines, i, "Ö1", m[1], "")...)
				}
			}
		}
	}
	return problems
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

// checkPriorityItem maddenin bölümünde etiketinde prio geçen açık bir iş arar; order boş
// değilse ve etiket "N. sıra" diyorsa N order olmalıdır.
func checkPriorityItem(lines []string, at int, prio, item, order string) []string {
	where := fmt.Sprintf("docs/status.md:%d: %s listesindeki %s", at+1, prio, item)
	if order != "" {
		where = fmt.Sprintf("docs/status.md:%d: Ö0 listesinin %s. maddesi %s", at+1, order, item)
	}
	body, heading, ok := statusSubsection(lines, item)
	if !ok {
		return []string{where + ": status'ta bu maddenin bölümü (### " + item + ") yok"}
	}
	var labels []string
	for _, l := range body {
		m := openItemHeadRe.FindStringSubmatch(l)
		if m == nil || !regexp.MustCompile(`(?:^|[^\p{L}\p{N}])`+prio+`(?:[^\p{N}]|$)`).MatchString(m[1]) {
			continue
		}
		labels = append(labels, m[1])
	}
	if len(labels) == 0 {
		return []string{fmt.Sprintf("%s: bölümünde (%s) etiketinde %s geçen açık bir Yapılacaklar yok; madde kapandıysa ya da önceliği değiştiyse listeden çıkar", where, heading, prio)}
	}
	if order == "" {
		return nil
	}
	for _, lab := range labels {
		if m := orderInLabelRe.FindStringSubmatch(lab); m != nil && m[1] != order {
			return []string{fmt.Sprintf("%s: bölümündeki etiket \"%s. sıra\" diyor, listedeki yeri %s", where, m[1], order)}
		}
	}
	return nil
}

// statusSubsection maddenin ### bölümünün satırlarını verir: önce tam başlık (### 15.23a),
// yoksa harfsiz başlık (13.1a için ### 13.1), yoksa maddeyi içeren aralık (### 21.1–21.8).
func statusSubsection(lines []string, item string) ([]string, string, bool) {
	ch, num, ok := splitItem(item)
	if !ok {
		return nil, "", false
	}
	base := strings.TrimRight(item, "abcdefghijklmnopqrstuvwxyz")
	var baseStart, rangeStart = -1, -1
	for i, l := range lines {
		m := statusSubRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		if strings.TrimPrefix(l, "### ") == item {
			return subsectionBody(lines, i), l, true
		}
		if strings.TrimPrefix(l, "### ") == base && baseStart < 0 {
			baseStart = i
		}
		if m[4] != "" && m[1] == m[4] && m[1] == strconv.Itoa(ch) {
			lo, _ := strconv.Atoi(m[2])
			hi, _ := strconv.Atoi(m[5])
			if lo <= num && num <= hi && rangeStart < 0 {
				rangeStart = i
			}
		}
	}
	if baseStart >= 0 {
		return subsectionBody(lines, baseStart), lines[baseStart], true
	}
	if rangeStart < 0 {
		return nil, "", false
	}
	return subsectionBody(lines, rangeStart), lines[rangeStart], true
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
