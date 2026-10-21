package internal_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

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
