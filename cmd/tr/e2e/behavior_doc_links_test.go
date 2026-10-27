package e2e

import (
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Belgelerdeki göreli bağlantılar (`[yazı](yol)`, `[yazı](yol#başlık)`, `[yazı](#başlık)`) var
// olan bir dosyaya ya da klasöre, başlık verilmişse o dosyadaki bir başlığa gitmelidir. Bir
// belge taşındığında ya da bölündüğünde kırılan bağlantıları bu test yakalar.

var (
	linkRe    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	headingRe = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)
)

// uncodedLines belgenin satırlarıdır; ``` ile çevrili kod bloklarının satırları boş gelir
// (satır numaraları korunur).
func uncodedLines(text string) []string {
	var lines []string
	code := false
	for _, s := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(s), "```") {
			code = !code
			s = ""
		}
		if code {
			s = ""
		}
		lines = append(lines, strings.TrimRight(s, "\r"))
	}
	return lines
}

// headingAnchor bir başlıktan barındırma sitesinin ürettiği çapayı üretir: küçük harf, harf,
// rakam, boşluk, '-' ve '_' dışındaki karakterler atılır, boşluklar '-' olur.
func headingAnchor(heading string) string {
	var b strings.Builder
	for _, r := range heading {
		switch {
		case r == 'İ':
			b.WriteString("i̇") // büyük İ küçültülünce noktalı i + birleşen nokta olur
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || unicode.Is(unicode.Mn, r):
			b.WriteRune(unicode.ToLower(r))
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

// anchors belgedeki başlıkların çapalarıdır (aynı başlık tekrar ederse -1, -2 ... eklenir).
func anchors(text string) map[string]bool {
	result := map[string]bool{}
	counter := map[string]int{}
	for _, s := range uncodedLines(text) {
		m := headingRe.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		c := headingAnchor(m[1])
		if n := counter[c]; n > 0 {
			result[c+"-"+strconv.Itoa(n)] = true
		} else {
			result[c] = true
		}
		counter[c]++
	}
	return result
}

func TestDocLinks(t *testing.T) {
	readValue := map[string]string{}
	read := func(filePath string) (string, bool) {
		if m, ok := readValue[filePath]; ok {
			return m, true
		}
		data, err := os.ReadFile(reporoot.Path(filePath))
		if err != nil {
			return "", false
		}
		readValue[filePath] = string(data)
		return readValue[filePath], true
	}
	total := 0
	for _, belge := range belgeListesi(t) {
		text, ok := read(belge)
		if !ok {
			t.Fatalf("%s okunamadı", belge)
		}
		for i, line := range uncodedLines(text) {
			for _, m := range linkRe.FindAllStringSubmatch(line, -1) {
				target := m[1]
				if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				total++
				filePath, anchor, _ := strings.Cut(target, "#")
				targetFile := belge
				if filePath != "" {
					targetFile = path.Clean(path.Join(path.Dir(belge), filePath))
				}
				if strings.HasPrefix(targetFile, "../") {
					t.Errorf("%s:%d: %q depo dışına gidiyor", belge, i+1, target)
					continue
				}
				info, err := os.Stat(reporoot.Path(targetFile))
				if err != nil {
					t.Errorf("%s:%d: %q bağlantısının hedefi yok: %s", belge, i+1, target, targetFile)
					continue
				}
				if anchor == "" {
					continue
				}
				if info.IsDir() || !strings.HasSuffix(targetFile, ".md") {
					t.Errorf("%s:%d: %q yalnızca bir .md dosyasının başlığına çapa verebilir", belge, i+1, target)
					continue
				}
				targetText, _ := read(targetFile)
				if !anchors(targetText)[anchor] {
					t.Errorf("%s:%d: %q bağlantısındaki #%s başlığı %s içinde yok", belge, i+1, target, anchor, targetFile)
				}
			}
		}
	}
	if total < 30 {
		t.Fatalf("yalnızca %d göreli bağlantı bulundu; düzenli ifade bozulmuş olabilir", total)
	}
}

func TestHeadingAnchors(t *testing.T) {
	for heading, expected := range map[string]string{
		"C. Tensörler TR_and Otomatik Türev":          "c-tensörler-ve-otomatik-türev",
		"7. Test Yazma TR_and Çalıştırma (`tr test`)": "7-test-yazma-ve-çalıştırma-tr-test",
		"Örnekler":      "örnekler",
		"B. İstatistik": "b-i̇statistik",
	} {
		if c := headingAnchor(heading); c != expected {
			t.Errorf("baslikCapasi(%q) = %q, beklenen %q", heading, c, expected)
		}
	}
}
