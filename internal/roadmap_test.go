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
	checkRoadmapAncestor(t, reporoot.Root())
}

func checkRoadmapAncestor(t *testing.T, root string) {
	t.Helper()
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
	out, err := exec.Command("git", "-C", root, "rev-parse", "--is-shallow-repository").Output()
	if err != nil {
		t.Fatalf("git rev-parse --is-shallow-repository: %v", err)
	}
	if strings.TrimSpace(string(out)) != "false" {
		t.Skip("sığ klon: geçmiş eksik olduğundan ata denetimi atlanıyor")
	}
	if err := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", m[2], "HEAD").Run(); err != nil {
		t.Errorf("yol haritasının ölçüldüğü commit %s HEAD'in atası değil", m[2])
	}
}

// Sığ klona ölçüm commit nesnesi getirilmiş olsa bile (cat-file -e geçer) geçmiş kesik olduğundan
// ata denetimi atlanır; tam klonda aynı ata commit'iyle geçer.
func TestRoadmapShallowAndFullClones(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	tmp := t.TempDir()
	full := filepath.Join(tmp, "full")
	if err := os.MkdirAll(filepath.Join(full, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn := func(dir string, args ...string) string {
		t.Helper()
		cmdArgs := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@e.st", "-c", "commit.gpgsign=false"}, args...)
		out, err := exec.Command("git", cmdArgs...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	writeRoadmap := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(full, "docs", "roadmap.md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(full, "add", "docs/roadmap.md")
	}

	gitIn(full, "init", "-q")
	writeRoadmap("ilk\n")
	gitIn(full, "commit", "-q", "-m", "c0")
	c0 := gitIn(full, "rev-parse", "HEAD")
	gitIn(full, "branch", "ilk")

	writeRoadmap("**Son ölçüm: 2026-11-08, main `" + c0[:8] + "`.**\n")
	gitIn(full, "commit", "-q", "-m", "c1")
	checkRoadmapAncestor(t, full)

	shallow := filepath.Join(tmp, "shallow")
	if out, err := exec.Command("git", "clone", "-q", "--no-local", "--depth", "1", full, shallow).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	gitIn(shallow, "fetch", "-q", "--depth", "1", "origin", "ilk")
	if err := exec.Command("git", "-C", shallow, "cat-file", "-e", c0[:8]+"^{commit}").Run(); err != nil {
		t.Fatalf("c0 nesnesi sığ klonda bulunmalıydı: %v", err)
	}
	t.Run("shallow_skips", func(t *testing.T) {
		checkRoadmapAncestor(t, shallow)
		t.Fatal("sığ klonda atlanmalıydı")
	})
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

var evidenceTag = regexp.MustCompile(`(?i)\[(ölçüldü|hesap|kanıtlı|hipotez|tahmin|görüş|test)(:|\])`)

// Kural: yol haritası yönü ve sırayı tutar; ölçü ve bir işin bugünkü durumu status'ta ve
// docs/measurements/ klasöründe yazılır, burada yalnız bağlantısı (Ahmet, 2026-11-08: "roadmap genel olur
// yön gösterir. status ise detay olur."). Neden: aynı ölçü iki yerde yazılınca yol haritasındaki
// kopya eskidi (2026-11-08: sızıntılı relu satırı kaynaştırmadan sonra da eski profili
// gösteriyordu). Ölçü etiketi ölçünün işaretidir: etiket yoksa ölçü de buraya yazılmamıştır.
func TestRoadmapCarriesNoMeasurements(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(reporoot.Root(), "docs", "roadmap.md"))
	if err != nil {
		t.Fatal(err)
	}
	for n, line := range strings.Split(string(data), "\n") {
		if evidenceTag.MatchString(line) {
			t.Errorf("docs/roadmap.md:%d: ölçü status'ta ya da docs/measurements/'ta yazılır, yol haritası yalnız bağlantı verir: %s",
				n+1, strings.TrimSpace(line))
		}
	}
}
