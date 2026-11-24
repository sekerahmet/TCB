package testrunner

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// Kural (Ahmet 2026-11-24: "Evet, başla"; DIL UZMANI'nın status düzeni önerisi): docs/status.md
// bir günlük değil, bugünkü durumdur. Bölümlerdeki her madde ("- **" ile başlayan satır ve onun
// girintili alt satırları):
//   - kalıcı bir kimlikle başlar (`- **S-412 Yapılacaklar (D, Ö3; sahip T1):**`); kimlik tektir,
//     başlıktaki "Sıradaki kimlik"ten küçüktür ve silinen maddenin kimliği yeniden kullanılmaz.
//     Mesaj, devir ve denetim maddeyi satır numarasıyla değil kimlikle anar;
//   - en çok statusItemMaxRunes harftir;
//   - ölçü sayısı kopyalamaz (ölçünün kimliği yazılır), tazelik notu ("eski: N commit") taşımaz;
//   - süreç izi taşımaz: kim hangi gün hangi saatte karar verdi, commit mesajında durur.
//
// Henüz yeni biçime çevrilmemiş bölümler statusUnconvertedSections'tadır; orada yalnız kimlik
// kuralı denetlenir. Çevrilen bölüm listeden çıkar; sorunu kalmamış bölüm listede kalırsa test
// düşer, böylece liste yalnız küçülür.
func TestStatusItemsFollowTheFormat(t *testing.T) {
	for _, p := range statusFormatProblems(statusDoc(t), statusUnconvertedSections) {
		t.Error(p)
	}
}

// statusItemMaxRunes bir maddenin (alt satırlarıyla) en çok harf sayısı.
const statusItemMaxRunes = 500

// statusUnconvertedSections yeni biçime henüz çevrilmemiş bölümler: "15" (## 15. ...) ya da
// sayısız bölümün adı ("Bölümü olmayan işler").
var statusUnconvertedSections = map[string]bool{
	"1": true, "3": true, "4": true, "5": true, "6": true, "7": true, "8": true, "10": true,
	"11": true, "12": true, "13": true, "15": true, "16": true, "17": true, "18": true, "19": true,
	"20": true, "21": true, "24": true, "25": true, "28": true, "Bölümü olmayan işler": true,
}

var (
	statusNextIDRe   = regexp.MustCompile(`\*\*Sıradaki kimlik:\*\* S-(\d+)`)
	statusItemIDRe   = regexp.MustCompile(`^- \*\*S-(\d+) `)
	statusSectionKey = regexp.MustCompile(`^## (?:(\d+)\. .*|(Bölümü olmayan işler))$`)
	backtickSpanRe   = regexp.MustCompile("`[^`]*`")
	// Kopyalanmış ölçü: birimli sayı, yüzde ya da tazelik notu.
	copiedMeasureRe = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_-])(\d+(?:,\d+)? ?(?:sn|ms|µs|ns|kat|katı|GB|MB|KB|GiB|MiB|KiB|token/sn)|%\d+(?:,\d+)?)(?:[^\p{L}\p{N}]|$)|eski: \d+ commit`)
	// Süreç izi: saat ya da kimliğin parçası olmayan tarih.
	processTraceRe = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_:-])(\d{1,2}:\d{2}(?::\d{2})?|20\d\d-\d\d-\d\d)(?:[^\p{N}-]|$)`)
)

// statusItem bölümdeki bir madde: ilk satırı ve girintili alt satırlarıyla.
type statusItem struct {
	section string
	line    int
	text    string
}

// statusItems başlığın ilk sayılı ya da adlı bölümünden sonraki maddeleri verir.
func statusItems(doc string) []statusItem {
	var items []statusItem
	section := ""
	for i, l := range strings.Split(doc, "\n") {
		if m := statusSectionKey.FindStringSubmatch(l); m != nil {
			section = m[1] + m[2]
			continue
		}
		if strings.HasPrefix(l, "## ") {
			section = ""
			continue
		}
		if section == "" {
			continue
		}
		switch {
		case strings.HasPrefix(l, "- **"):
			items = append(items, statusItem{section, i + 1, l})
		case strings.HasPrefix(l, "  ") && len(items) > 0 && items[len(items)-1].line+strings.Count(items[len(items)-1].text, "\n") == i:
			items[len(items)-1].text += "\n" + l
		}
	}
	return items
}

// statusFormatProblems biçim sorunlarını satır numarasıyla verir.
func statusFormatProblems(doc string, unconverted map[string]bool) []string {
	var problems []string
	next := 0
	if m := statusNextIDRe.FindStringSubmatch(doc); m != nil {
		next, _ = strconv.Atoi(m[1])
	} else {
		problems = append(problems, "docs/status.md: başlıkta \"**Sıradaki kimlik:** S-<n>\" yok")
	}
	seen := map[int]int{}
	dirty := map[string]bool{}
	for _, it := range statusItems(doc) {
		where := fmt.Sprintf("docs/status.md:%d", it.line)
		if m := statusItemIDRe.FindStringSubmatch(it.text); m == nil {
			problems = append(problems, where+": madde kimlikle başlamıyor (`- **S-<n> ...`)")
		} else {
			id, _ := strconv.Atoi(m[1])
			if prev, ok := seen[id]; ok {
				problems = append(problems, fmt.Sprintf("%s: S-%d ikinci kez (ilki satır %d)", where, id, prev))
			}
			seen[id] = it.line
			if next > 0 && id >= next {
				problems = append(problems, fmt.Sprintf("%s: S-%d sıradaki kimlikten (S-%d) küçük değil", where, id, next))
			}
		}
		var own []string
		if n := utf8.RuneCountInString(it.text); n > statusItemMaxRunes {
			own = append(own, fmt.Sprintf("%s: madde %d harf, sınır %d", where, n, statusItemMaxRunes))
		}
		plain := backtickSpanRe.ReplaceAllString(it.text, "``")
		if m := copiedMeasureRe.FindStringSubmatch(plain); m != nil {
			hit := m[1]
			if hit == "" {
				hit = m[0]
			}
			own = append(own, fmt.Sprintf("%s: kopyalanmış ölçü ya da tazelik notu %q; ölçünün kimliğini yaz", where, hit))
		}
		if m := processTraceRe.FindStringSubmatch(plain); m != nil {
			own = append(own, fmt.Sprintf("%s: süreç izi %q; karar ve tarih commit mesajında durur", where, m[1]))
		}
		if len(own) > 0 {
			dirty[it.section] = true
			if !unconverted[it.section] {
				problems = append(problems, own...)
			}
		}
	}
	var keys []string
	for k := range unconverted {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !dirty[k] {
			problems = append(problems, fmt.Sprintf("statusUnconvertedSections'ta bayat bölüm: %s (biçim sorunu kalmadı, listeden çıkar)", k))
		}
	}
	return problems
}

// Bilinen olumlu örnek: her kural kendi bozulmasını yakalar, temiz madde yakalanmaz.
func TestStatusFormatCatchesBadItems(t *testing.T) {
	long := strings.Repeat("a", statusItemMaxRunes)
	doc := "# Durum\n\n**Sıradaki kimlik:** S-10.\n\n## Önem ve öncelik\n\n- **A, sessiz yanlış:** başlık maddesi, kimliksiz kalır (2026-11-03).\n\n" +
		"## 1. Program\n\n### 1.1\n\n" +
		"- **S-1 Yapılacaklar (D, Ö3; sahip T1):** temiz madde [ölçüldü: TG-2026-11-23-HIGHRAM-T1-4; `t[1:10]`, `x.go:29-34`].\n" +
		"- **Mevcut durum:** kimliksiz.\n" +
		"- **S-1 Mevcut durum:** ikinci kez.\n" +
		"- **S-12 Mevcut durum:** sıradakinden büyük.\n" +
		"- **S-2 Mevcut durum:** " + long + "\n" +
		"- **S-3 Mevcut durum:** okuma 1,07 katı.\n" +
		"- **S-4 Mevcut durum:** tazelik [eski: 52 commit].\n" +
		"- **S-5 Mevcut durum:** karar TCB 2026-11-24 14:11.\n" +
		"- **S-6 Mevcut durum:** alt satırlı madde\n  - " + long + "\n\n" +
		"## 2. Sözcük\n\n- **S-7 Mevcut durum:** çevrilmemiş bölümde 48 ms.\n\n" +
		"## 3. Tipler\n\n- **S-8 Mevcut durum:** temiz.\n"
	got := strings.Join(statusFormatProblems(doc, map[string]bool{"2": true, "3": true}), "\n")
	for _, want := range []string{
		":14: madde kimlikle başlamıyor", ":15: S-1 ikinci kez (ilki satır 13)", ":16: S-12 sıradaki kimlikten (S-10) küçük değil",
		":17: madde", `:18: kopyalanmış ölçü ya da tazelik notu "1,07 katı"`, `:19: kopyalanmış ölçü ya da tazelik notu "eski: 52 commit"`,
		`:20: süreç izi "2026-11-24"`, ":21: madde", "bayat bölüm: 3",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q yakalanmadı:\n%s", want, got)
		}
	}
	for _, clean := range []string{":13:", ":26:", ":7:", "bayat bölüm: 2"} {
		if strings.Contains(got, clean) {
			t.Errorf("%s temiz, yakalanmamalıydı:\n%s", clean, got)
		}
	}
	if n := strings.Count(got, "\n") + 1; n != 9 {
		t.Errorf("dokuz sorun beklendi, %d geldi:\n%s", n, got)
	}
}
