package internal_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural (docs/terimler.md): terim sözlüğünün "Kullanılmaz (eski)" sütunundaki çok kelimeli kalıplar
// kullanıcıya görünen metinlerde geçmez: belgeler (docs/, README'ler), ileti ve ad metinleri
// (tr.json), kitaplık ve örnekler (.tr). Tek kelimelik eski adlar mekanik olarak denetlenmez
// (ör. "baş" başlangıç anlamında meşrudur); parantezli açıklamalar ("çekirdek (bu anlamda)") da.
// Neden: terimler değişince (baş → bakış, çekirdek → kernel) eski kalıp sessizce geri gelmesin.
func TestRetiredTermsStayRetired(t *testing.T) {
	root := reporoot.Root()
	table, err := os.ReadFile(filepath.Join(root, "docs", "terimler.md"))
	if err != nil {
		t.Fatal(err)
	}
	phrases := retiredPhrases(string(table))
	if len(phrases) == 0 {
		t.Fatal("docs/terimler.md'de denetlenecek kalıp bulunamadı (tablo biçimi değişti mi?)")
	}
	// Yalnızca depoda izlenen dosyalar: kökteki izlenmeyen dosyalar kullanıcınındır.
	listed, err := exec.Command("git", "-C", root, "ls-files").Output()
	if err != nil {
		t.Skipf("git ls-files: %v", err)
	}
	var files []string
	for _, rel := range strings.Split(strings.TrimSpace(string(listed)), "\n") {
		switch {
		case rel == "docs/terimler.md", rel == "docs/roadmap.md": // tarihçe eski adı anar
		case strings.HasSuffix(rel, ".md"), strings.HasSuffix(rel, ".tr") && !strings.HasSuffix(rel, "_test.tr"),
			rel == "internal/lang/messages/tr.json", rel == "internal/lang/codes/tr.json":
			files = append(files, rel)
		}
	}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if os.IsNotExist(err) {
			continue // izlenen ama çalışma ağacında silinmiş (henüz commit'lenmemiş)
		}
		if err != nil {
			t.Fatal(err)
		}
		for n, line := range strings.Split(string(data), "\n") {
			low := strings.ToLowerSpecial(unicode.TurkishCase, line)
			for _, p := range phrases {
				if containsPhrase(low, p) {
					t.Errorf("%s:%d: eski terim %q (docs/terimler.md): %s", rel, n+1, p, strings.TrimSpace(line))
				}
			}
		}
	}
}

// retiredPhrases tablodaki "Kullanılmaz (eski)" sütununun çok kelimeli ve parantezsiz kalıplarını
// küçük harfle verir.
func retiredPhrases(md string) []string {
	var out []string
	col := -1
	for _, line := range strings.Split(md, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), "|")
		if col < 0 {
			for i, c := range cells {
				if strings.HasPrefix(strings.TrimSpace(c), "Kullanılmaz") {
					col = i
				}
			}
			continue
		}
		if col >= len(cells) {
			continue
		}
		for _, p := range strings.Split(cells[col], ",") {
			p = strings.TrimSpace(p)
			if strings.Contains(p, " ") && !strings.ContainsAny(p, "()—:") {
				out = append(out, strings.ToLowerSpecial(unicode.TurkishCase, p))
			}
		}
	}
	return out
}

// containsPhrase kalıp satırda sözcük başında geçiyor mu (önündeki karakter harf ya da rakam
// değil)? Sonu açıktır: "çok başlı" "çok başlıdır"ı da yakalar.
func containsPhrase(line, p string) bool {
	for i := 0; ; {
		j := strings.Index(line[i:], p)
		if j < 0 {
			return false
		}
		at := i + j
		if at == 0 || !isWordRune(lastRune(line[:at])) {
			return true
		}
		i = at + len(p)
	}
}

func lastRune(s string) rune {
	r := []rune(s)
	return r[len(r)-1]
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// Kural (docs/terimler.md "Eş adlar", dil tanımı 17): aynı adı taşıyan her canlı kod grubu eş
// adlar tablosunda gerekçesiyle yazılıdır; tablo güncel kalır (eksik ya da fazla kod yok) ve
// "ayrı alan" gerekçesi gerçekten bir komut satırı koduyla (komut, seçenek, kabuk, hata ayıklayıcı) bir dil kodunu ayırır.
// Neden: dilin içinde farklı işler aynı adı sessizce taşımasın (eski tensör birleştir'i ile
// liste .birleştir'i gibi).
func TestSharedNamesListed(t *testing.T) {
	table, err := os.ReadFile(filepath.Join(reporoot.Root(), "docs", "terimler.md"))
	if err != nil {
		t.Fatal(err)
	}
	listed := sharedNameRows(t, string(table))
	groups := map[string][]codes.Code{}
	for _, k := range codes.Codes() {
		groups[codes.Source().Name(k)] = append(groups[codes.Source().Name(k)], k)
	}
	for name, ks := range groups {
		if len(ks) < 2 {
			continue
		}
		row, ok := listed[name]
		if !ok {
			t.Errorf("%q birden çok koda verilmiş (%v) ama docs/terimler.md eş adlar tablosunda yok", name, ks)
			continue
		}
		if got, want := row.codes, codeList(ks); got != want {
			t.Errorf("%q: tablodaki kodlar %s, kod listesindekiler %s", name, got, want)
		}
		if strings.HasPrefix(row.reason, "ayrı alan") && !mixesCommandLine(ks) {
			t.Errorf("%q: \"ayrı alan\" gerekçesi komut satırı koduyla dil kodunu ayırmalı: %v", name, ks)
		}
	}
	for name := range listed {
		if len(groups[name]) < 2 {
			t.Errorf("docs/terimler.md eş adlar tablosunda %q var ama ad artık tek koda ait", name)
		}
	}
}

type sharedNameRow struct{ codes, reason string }

// sharedNameRows "## Eş adlar" başlığının altındaki tabloyu okur: ad → kodlar ve gerekçe.
// Gerekçe "aynı anlam" ya da "ayrı alan" ile başlamalıdır.
func sharedNameRows(t *testing.T, md string) map[string]sharedNameRow {
	_, after, ok := strings.Cut(md, "\n## Eş adlar\n")
	if !ok {
		t.Fatal(`docs/terimler.md'de "## Eş adlar" bölümü yok`)
	}
	if i := strings.Index(after, "\n## "); i >= 0 {
		after = after[:i]
	}
	rows := map[string]sharedNameRow{}
	for _, line := range strings.Split(after, "\n") {
		cells := strings.Split(strings.Trim(line, "| "), "|")
		if !strings.HasPrefix(line, "|") || len(cells) != 3 || strings.TrimSpace(cells[0]) == "Ad" ||
			strings.HasPrefix(strings.TrimSpace(cells[0]), ":--") {
			continue
		}
		name, reason := strings.TrimSpace(cells[0]), strings.TrimSpace(cells[2])
		if !strings.HasPrefix(reason, "aynı anlam") && !strings.HasPrefix(reason, "ayrı alan") {
			t.Errorf("eş ad %q: gerekçe \"aynı anlam\" ya da \"ayrı alan\" ile başlamalı: %s", name, reason)
		}
		if _, dup := rows[name]; dup {
			t.Errorf("eş ad %q tabloda iki kez var", name)
		}
		rows[name] = sharedNameRow{codes: strings.TrimSpace(cells[1]), reason: reason}
	}
	if len(rows) == 0 {
		t.Fatal("docs/terimler.md eş adlar tablosu boş (biçim değişti mi?)")
	}
	return rows
}

func codeList(ks []codes.Code) string {
	s := make([]string, len(ks))
	for i, k := range ks {
		s[i] = k.String()
	}
	sort.Strings(s)
	return strings.Join(s, ", ")
}

func mixesCommandLine(ks []codes.Code) bool {
	var cmd, lang bool
	for _, k := range ks {
		if k.ExpectedKind() == codes.KindCommand {
			cmd = true
		} else {
			lang = true
		}
	}
	return cmd && lang
}
