package internal_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Yol haritası da status gibi ölçülerek güncel tutulur (docs/roadmap.md, "Yöntem"; Ahmet,
// 2026-11-01). Neden: 2026-11-01'deki ölçümde eski yol haritasından 102 iş sessizce düşmüştü,
// status'un bölüm başlıklarından 12'si eskimişti; ikisi de yanlış yöne götürüyordu. Aşağıdaki
// üç test DENETİM'in önerisidir (kol-denetim-yol-haritasi-1101): birer bekçi, ölçümün yerine
// geçmez.

// oldRoadmapRef eski yol haritasının evre ve madde numaralarına gönderir: "yol haritası Evre 4,
// madde 8", "roadmap Evre 4 item 7", "yol haritası 10". Bu numaralar bugünkü yol haritasında yok.
// Sıra sayısı (noktalı: "yol haritasının 2. kilometre taşı") gönderme değildir.
var oldRoadmapRef = regexp.MustCompile(`(?i)(yol haritas\p{L}*|roadmap)\W{0,4}((evre|madde|item)\s*\d|\d+([^.\d]|$))`)

// Kural: arşiv dışındaki hiçbir dosya eski yol haritasının numaralarına "yol haritası" diye
// göndermez. Tarihçe gönderisi arşivin dosyasını adıyla anar (docs/archive/roadmap-...) ya da
// "eski yol haritası" der; o satırlar geçerlidir.
func TestNoReferenceToTheOldRoadmapNumbers(t *testing.T) {
	root := reporoot.Root()
	listed, err := exec.Command("git", "-C", root, "ls-files").Output()
	if err != nil {
		t.Skipf("git ls-files: %v", err)
	}
	for _, rel := range strings.Split(strings.TrimSpace(string(listed)), "\n") {
		// Bu dosya kuralın örneklerini anar.
		if strings.HasPrefix(rel, "docs/archive/") || rel == "internal/roadmap_test.go" || !textFile(rel) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue // izlenen ama çalışma ağacında silinmiş
		}
		for n, line := range strings.Split(string(data), "\n") {
			low := strings.ToLower(line)
			if !oldRoadmapRef.MatchString(line) || strings.Contains(low, "archive/roadmap-") ||
				strings.Contains(low, "eski yol haritas") || strings.Contains(low, "old roadmap") {
				continue
			}
			if strings.Contains(line, "20") && regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).MatchString(line) &&
				!regexp.MustCompile(`(?i)(evre|madde|item)\s*\d`).MatchString(line) {
				continue // "yol haritası (2026-11-01)" gibi tarihli anma
			}
			t.Errorf("%s:%d: eski yol haritasının numarasına gönderiyor; bugünkü yerini (status maddesi ya da yol haritasının başlığı) yaz: %s",
				rel, n+1, strings.TrimSpace(line))
		}
	}
}

func textFile(rel string) bool {
	for _, ext := range []string{".go", ".md", ".tr", ".sh", ".py", ".json", ".txt", ".yml", ".yaml"} {
		if strings.HasSuffix(rel, ext) {
			return true
		}
	}
	return false
}

var roadmapMeasured = regexp.MustCompile("Son ölçüm: (\\d{4}-\\d{2}-\\d{2}), main `([0-9a-f]{7,40})`")

// Kural: yol haritasının "Son ölçüm" satırı bir tarih ve depoda var olan, HEAD'in atası olan
// bir commit taşır. Eskime gün sayısıyla sınanmaz (test zamana bağlanmasın); commit'in var
// olması, ölçümün bu geçmişe karşı yapıldığını gösterir.
func TestRoadmapMeasuredAgainstAnAncestor(t *testing.T) {
	root := reporoot.Root()
	data, err := os.ReadFile(filepath.Join(root, "docs", "roadmap.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := roadmapMeasured.FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal("docs/roadmap.md'de \"Son ölçüm: YYYY-AA-GG, main `<commit>`\" satırı yok")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	if err := exec.Command("git", "-C", root, "cat-file", "-e", m[2]+"^{commit}").Run(); err != nil {
		t.Skipf("commit %s bu klonda yok (sığ klon olabilir)", m[2])
	}
	if err := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", m[2], "HEAD").Run(); err != nil {
		t.Errorf("yol haritasının ölçüldüğü commit %s HEAD'in atası değil", m[2])
	}
}

var (
	sectionState = regexp.MustCompile(`(?i)\b(kapandı|bekliyor|bekleniyor)\b`)
	sectionRef   = regexp.MustCompile(`\b\d{1,2}\.\s*bölüm|\bbölüm\w*\s+\d{1,2}\b|\b\d{1,2}\.\d{1,2}[a-z]?\b`)
)

// Kural: bölümlerin denetim durumu (Taslak, Kapandı, "Gemini'yi bekliyor") yalnız status'un
// bölüm başlarında yazılır; yol haritası onu kopyalamaz, status'a gönderir. Neden: aynı liste
// iki yerde yazılınca daha ilk gün ayrıştı (DENETİM, 2026-11-01).
func TestRoadmapHoldsNoSectionState(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(reporoot.Root(), "docs", "roadmap.md"))
	if err != nil {
		t.Fatal(err)
	}
	for n, line := range strings.Split(string(data), "\n") {
		if sectionState.MatchString(line) && sectionRef.MatchString(line) {
			t.Errorf("docs/roadmap.md:%d: bölüm durumu status'un bölüm başındadır, burada kopyalanmaz: %s",
				n+1, strings.TrimSpace(line))
		}
	}
}
