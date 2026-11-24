package testrunner

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (Ahmet 2026-11-24: "Evet, başla"; DIL UZMANI'nın status düzeni önerisi): docs/status.md
// bir günlük değil, bugünkü durumdur. Bölümlerdeki her madde ("- **" ile başlayan satır ve onun
// girintili alt satırları):
//   - kalıcı bir kimlik ve türle başlar: `- **S-412 Yapılacaklar (D, Ö3; sahip T1):**` ya da
//     `- **S-413 Mevcut durum:**`; tür yalnız bu ikisidir. Kimlik tektir ve başlıktaki "Sıradaki
//     kimlik"ten küçüktür. Silinen maddenin kimliği statusDeletedIDsFile'da durur ve yeniden
//     kullanılmaz: sayacın altındaki her kimlik ya status'ta ya listededir, ikisinde birden değil.
//     Mesaj, devir ve denetim maddeyi satır numarasıyla değil kimlikle anar;
//   - en çok statusItemMaxRunes harftir;
//   - kimlikli ölçünün sayısını kopyalamaz (ölçünün kimliği yazılır), tazelik notu ("eski: N
//     commit") taşımaz; ters tırnak içi de sayılır;
//   - süreç izi taşımaz: saat, kimliğin parçası olmayan tarih ve kanıt etiketinin dışında DENETİM
//     numarası. Kim hangi gün hangi kararı verdi, commit mesajında durur; DENETİM numarası yalnız
//     kanıt etiketinde kaynak olarak kalır ([ölçüldü: DENETİM 295]).
//
// Ayracın sahibi bu testin dışındadır (DENETİM 980 Ş1, TCB: sahip bu geçişte doldurulmaz).
func TestStatusItemsFollowTheFormat(t *testing.T) {
	for _, p := range statusFormatProblems(statusDoc(t), statusFormatExceptions, statusDeletedIDs(t)) {
		t.Error(p)
	}
}

// statusItemMaxRunes bir maddenin (alt satırlarıyla) en çok harf sayısı.
const statusItemMaxRunes = 500

// statusDeletedIDsFile silinen kimliklerin listesi: satır başına bir "S-<n>"; "#" ile başlayan
// satır açıklamadır.
const statusDeletedIDsFile = "internal/devtools/testrunner/testdata/status_deleted_ids.txt"

// statusFormatExceptions biçimi henüz denetlenmeyen tek tek maddeler: kimlik → neden. Başka
// oturumun açık işindeki madde o iş bitince yeni biçime girer; sorunu kalmayan girdi bayattır.
// Kimlik ve tür kuralı istisnasızdır.
var statusFormatExceptions = map[int]string{
	562: "T1'in açık GPU hızı işi; iş bitince yeni biçime girer",
	563: "T1'in açık GPU hızı işi; iş bitince yeni biçime girer",
}

var (
	statusNextIDRe   = regexp.MustCompile(`\*\*Sıradaki kimlik:\*\* S-(\d+)`)
	statusItemIDRe   = regexp.MustCompile(`^- \*\*S-(\d+) `)
	statusItemKindRe = regexp.MustCompile(`^- \*\*S-\d+ (?:Yapılacaklar|Mevcut durum)(?: \(|:\*\*)`)
	statusSectionKey = regexp.MustCompile(`^## (?:(\d+)\. .*|(Bölümü olmayan işler))$`)
	statusDeletedRe  = regexp.MustCompile(`^S-(\d+)$`)
	backtickSpanRe   = regexp.MustCompile("`[^`]*`")
	evidenceSpanRe   = regexp.MustCompile(`\[[^\]]*\]`)
	// Kopyalanmış ölçü: birimli sayı (ekli birim dahil: "katına", "dakikada"), "2,9x", yüzde ya da
	// tazelik notu.
	copiedMeasureRe = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_-])(\d+(?:,\d+)?(?: ?(?:sn|ms|µs|ns|saniye\p{L}*|dakika\p{L}*|kat\p{L}*|GB|MB|KB|GiB|MiB|KiB|token/sn)|x)|%\d+(?:,\d+)?)(?:[^\p{L}\p{N}]|$)|eski: \d+ commit`)
	// Süreç izi: saat ya da kimliğin parçası olmayan tarih.
	processTraceRe = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_:-])(\d{1,2}:\d{2}(?::\d{2})?|20\d\d-\d\d-\d\d)(?:[^\p{N}-]|$)`)
	// Süreç izi: kanıt etiketinin dışındaki DENETİM numarası.
	reviewTraceRe = regexp.MustCompile(`(?:^|[^\p{L}])(DENETİM(?: istek)? \d+)`)
)

// statusDeletedIDs silinen kimliklerin listesini okur.
func statusDeletedIDs(t *testing.T) map[int]bool {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path(statusDeletedIDsFile))
	if err != nil {
		t.Fatal(err)
	}
	ids, problems := parseStatusDeletedIDs(string(data))
	for _, p := range problems {
		t.Error(p)
	}
	return ids
}

// parseStatusDeletedIDs listeyi kimlik kümesine çevirir; bozuk ve yinelenen satırları sorun verir.
func parseStatusDeletedIDs(text string) (map[int]bool, []string) {
	ids := map[int]bool{}
	var problems []string
	for i, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		m := statusDeletedRe.FindStringSubmatch(l)
		if m == nil {
			problems = append(problems, fmt.Sprintf("%s:%d: %q, \"S-<n>\" bekleniyordu", statusDeletedIDsFile, i+1, l))
			continue
		}
		id, _ := strconv.Atoi(m[1])
		if ids[id] {
			problems = append(problems, fmt.Sprintf("%s:%d: S-%d ikinci kez", statusDeletedIDsFile, i+1, id))
		}
		ids[id] = true
	}
	return ids, problems
}

// statusItem bölümdeki bir madde: ilk satırı ve girintili alt satırlarıyla.
type statusItem struct {
	line int
	text string
}

// statusItems başlığın ilk sayılı ya da adlı bölümünden sonraki maddeleri verir.
func statusItems(doc string) []statusItem {
	var items []statusItem
	inSection := false
	for i, l := range strings.Split(doc, "\n") {
		if strings.HasPrefix(l, "## ") {
			inSection = statusSectionKey.MatchString(l)
			continue
		}
		if !inSection {
			continue
		}
		switch {
		case strings.HasPrefix(l, "- **"):
			items = append(items, statusItem{i + 1, l})
		case strings.HasPrefix(l, "  ") && len(items) > 0 && items[len(items)-1].line+strings.Count(items[len(items)-1].text, "\n") == i:
			items[len(items)-1].text += "\n" + l
		}
	}
	return items
}

// statusFormatProblems biçim sorunlarını satır numarası ve kimlikle verir.
func statusFormatProblems(doc string, exceptions map[int]string, deleted map[int]bool) []string {
	var problems []string
	next := 0
	if m := statusNextIDRe.FindStringSubmatch(doc); m != nil {
		next, _ = strconv.Atoi(m[1])
	} else {
		problems = append(problems, "docs/status.md: başlıkta \"**Sıradaki kimlik:** S-<n>\" yok")
	}
	seen := map[int]int{}
	dirtyID := map[int]bool{}
	for _, it := range statusItems(doc) {
		where := fmt.Sprintf("docs/status.md:%d", it.line)
		id := -1
		if m := statusItemIDRe.FindStringSubmatch(it.text); m == nil {
			problems = append(problems, where+": madde kimlikle başlamıyor (`- **S-<n> ...`)")
		} else {
			id, _ = strconv.Atoi(m[1])
			where += fmt.Sprintf(" (S-%d)", id)
			if prev, ok := seen[id]; ok {
				problems = append(problems, fmt.Sprintf("%s: kimlik ikinci kez (ilki satır %d)", where, prev))
			}
			seen[id] = it.line
			if next > 0 && id >= next {
				problems = append(problems, fmt.Sprintf("%s: sıradaki kimlikten (S-%d) küçük değil", where, next))
			}
			if deleted[id] {
				problems = append(problems, fmt.Sprintf("%s: silinen kimlik yeniden yazılmış (%s); yeni madde sıradaki kimliği alır", where, statusDeletedIDsFile))
			}
			if !statusItemKindRe.MatchString(it.text) {
				problems = append(problems, where+": tür yalnız \"Yapılacaklar\" ya da \"Mevcut durum\" olur")
			}
		}
		var own []string
		if n := utf8.RuneCountInString(it.text); n > statusItemMaxRunes {
			own = append(own, fmt.Sprintf("%s: madde %d harf, sınır %d", where, n, statusItemMaxRunes))
		}
		if m := copiedMeasureRe.FindStringSubmatch(it.text); m != nil {
			hit := m[1]
			if hit == "" {
				hit = m[0]
			}
			own = append(own, fmt.Sprintf("%s: kopyalanmış ölçü ya da tazelik notu %q; ölçünün kimliğini yaz", where, hit))
		}
		plain := backtickSpanRe.ReplaceAllString(it.text, "``")
		if m := processTraceRe.FindStringSubmatch(plain); m != nil {
			own = append(own, fmt.Sprintf("%s: süreç izi %q; karar ve tarih commit mesajında durur", where, m[1]))
		}
		if m := reviewTraceRe.FindStringSubmatch(evidenceSpanRe.ReplaceAllString(plain, "[]")); m != nil {
			own = append(own, fmt.Sprintf("%s: süreç izi %q; DENETİM numarası yalnız kanıt etiketinde kaynak olur", where, m[1]))
		}
		if len(own) > 0 {
			if _, ok := exceptions[id]; ok {
				dirtyID[id] = true
				continue
			}
			problems = append(problems, own...)
		}
	}
	for id := 1; id < next; id++ {
		if _, ok := seen[id]; !ok && !deleted[id] {
			problems = append(problems, fmt.Sprintf("S-%d ne status'ta ne %s'de: silinen kimlik listeye yazılır", id, statusDeletedIDsFile))
		}
	}
	var ids []int
	for id := range deleted {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if next > 0 && id >= next {
			problems = append(problems, fmt.Sprintf("%s: S-%d sıradaki kimlikten (S-%d) küçük değil", statusDeletedIDsFile, id, next))
		}
	}
	ids = ids[:0]
	for id := range exceptions {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if !dirtyID[id] {
			problems = append(problems, fmt.Sprintf("statusFormatExceptions'ta bayat madde: S-%d (biçim sorunu kalmadı ya da madde yok, listeden çıkar)", id))
		}
	}
	return problems
}

// Bilinen olumlu örnek: her kural kendi bozulmasını yakalar, temiz madde yakalanmaz.
func TestStatusFormatCatchesBadItems(t *testing.T) {
	long := strings.Repeat("a", statusItemMaxRunes)
	items := []string{
		"- **S-1 Yapılacaklar (D, Ö3; sahip T1):** temiz madde [ölçüldü: TG-2026-11-23-HIGHRAM-T1-4; `t[1:10]`, `x.go:29-34`, `0x80`, DENETİM 295].",
		"- **Mevcut durum:** kimliksiz.",
		"- **S-1 Mevcut durum:** ikinci kez.",
		"- **S-14 Mevcut durum:** sıradakinden büyük.",
		"- **S-2 Mevcut durum:** " + long,
		"- **S-3 Mevcut durum:** okuma 1,07 katı.",
		"- **S-4 Mevcut durum:** tazelik [eski: 52 commit].",
		"- **S-5 Mevcut durum:** karar TCB 2026-11-24 14:11.",
		"- **S-6 Mevcut durum:** alt satırlı madde\n  - " + long,
		"- **S-7 Bulgu (D, Ö3):** başka tür.",
		"- **S-8 Mevcut durum:** silinmiş kimlik.",
		"- **S-9 Mevcut durum:** okuma 2,9 katına çıktı.",
		"- **S-10 Mevcut durum:** ters tırnakta `48 ms` ve 3x.",
		"- **S-11 Mevcut durum:** DENETİM 961 buldu.",
		"- **S-12 Mevcut durum:** iki dakikada değil, 5 dakikada.",
	}
	doc := "# Durum\n\n**Sıradaki kimlik:** S-13.\n\n## Önem ve öncelik\n\n- **A, sessiz yanlış:** başlık maddesi, kimliksiz kalır (2026-11-03, DENETİM 76).\n\n" +
		"## 1. Program\n\n" + strings.Join(items, "\n") + "\n\n## Bölümü olmayan işler\n\n- **S-13 Yapılacaklar (C, Ö2):** sayaç.\n"
	got := strings.Join(statusFormatProblems(doc, map[int]string{5: "açık iş", 13: "bayat"}, map[int]bool{8: true, 12: true, 20: true}), "\n")
	for _, want := range []string{
		"madde kimlikle başlamıyor", "(S-1): kimlik ikinci kez (ilki satır 11)", "(S-14): sıradaki kimlikten (S-13) küçük değil",
		"(S-2): madde", `(S-3): kopyalanmış ölçü ya da tazelik notu "1,07 katı"`, `(S-4): kopyalanmış ölçü ya da tazelik notu "eski: 52 commit"`,
		"(S-6): madde", `(S-7): tür yalnız`, "(S-8): silinen kimlik yeniden yazılmış", `(S-9): kopyalanmış ölçü ya da tazelik notu "2,9 katına"`,
		`(S-10): kopyalanmış ölçü ya da tazelik notu "48 ms"`, `(S-11): süreç izi "DENETİM 961"`, `(S-12): kopyalanmış ölçü ya da tazelik notu "5 dakikada"`,
		"(S-12): silinen kimlik yeniden yazılmış", "(S-13): sıradaki kimlikten (S-13) küçük değil",
		"status_deleted_ids.txt: S-20 sıradaki kimlikten", "bayat madde: S-13",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q yakalanmadı:\n%s", want, got)
		}
	}
	// S-1 temiz (kanıt etiketindeki DENETİM numarası, ters tırnaktaki saat ve 0x80); S-5'in süreç izi
	// adlı istisnadır; S-8 ve S-12 silinmiş sayılır, sayaçta eksik kimlik yok.
	for _, clean := range []string{"(S-1): madde", "(S-1): kopyalanmış", "(S-1): süreç", "(S-5)", "bayat madde: S-5", "ne status'ta"} {
		if strings.Contains(got, clean) {
			t.Errorf("%s temiz, yakalanmamalıydı:\n%s", clean, got)
		}
	}
	if n := strings.Count(got, "\n") + 1; n != 17 {
		t.Errorf("on yedi sorun beklendi, %d geldi:\n%s", n, got)
	}
	// Eksik kimlik: sayacın altında olup ne status'ta ne listede olan kimlik.
	if gap := strings.Join(statusFormatProblems("**Sıradaki kimlik:** S-3.\n\n## 1. Program\n\n- **S-1 Mevcut durum:** bir.\n", nil, nil), "\n"); gap != "S-2 ne status'ta ne "+statusDeletedIDsFile+"'de: silinen kimlik listeye yazılır" {
		t.Errorf("eksik S-2 yakalanmadı:\n%s", gap)
	}
	if _, problems := parseStatusDeletedIDs("# açıklama\nS-3\nS-3\n4\n"); len(problems) != 2 {
		t.Errorf("listenin yinelenen ve bozuk satırı yakalanmadı: %q", problems)
	}
}
