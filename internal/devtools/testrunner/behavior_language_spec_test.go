package testrunner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/lib"
)

// Faz 3 kapısı: "her kural bir testte". Dil tanımında (docs/spec/NN_*.md) numarası geçen her
// kuralın (**4.2** gibi) tests/ klasöründe adı o numarayla başlayan en az bir testi olmalı;
// tersine, her test numarası tanımda bir kurala karşılık gelmeli. Ardından tüm testler geçmelidir.

// Kural numaraları harfli olabilir (**2.9a**, **6.3a**); testin numarası da harfiyle
// birlikte okunur ve tanımdaki bir kurala tam olarak karşılık gelmelidir (test "6.3a ...",
// kural **6.3a**). Harf yalnızca harfli kurallar içindir; aynı kuralın birden çok testi aynı
// numarayı taşır.
var (
	ruleRe = regexp.MustCompile(`\*\*(\d+\.\d+[a-z]?)\*\*`)
	testRe = regexp.MustCompile(`(?m)^(?:test|TR_ctx_test) "(\d+\.\d+[a-z]?)\s`) // kodlu dosyada TR_ctx_test
	// Kuralın tanımı satır başında yazılır (**4.2** ...); metnin içindeki **13.1** gibi
	// geçişler başka bir bölümdeki kurala başvurudur.
	ruleDefRe     = regexp.MustCompile(`(?m)^\*\*(\d+)\.\d+[a-z]?\*\*`)
	sectionFileRe = regexp.MustCompile(`^(\d\d)_[a-z0-9_]+\.md$`)
)

// languageSpec docs/spec/ altındaki bölüm dosyalarını (NN_ad.md) sırayla okur.
func languageSpec(t *testing.T) map[string]string {
	t.Helper()
	files, err := reporoot.Glob(reporoot.Path("docs/spec/*.md"))
	if err != nil {
		t.Fatal(err)
	}
	sections := map[string]string{}
	for _, d := range files {
		if filepath.Base(d) == "README.md" {
			continue
		}
		if !sectionFileRe.MatchString(filepath.Base(d)) {
			t.Errorf("docs/spec/%s: bölüm dosyalarının adı NN_ad.md biçiminde olmalı", filepath.Base(d))
			continue
		}
		data, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		sections[filepath.Base(d)] = string(data)
	}
	if len(sections) == 0 {
		t.Fatal("docs/spec/ altında bölüm dosyası bulunamadı")
	}
	return sections
}

// Her kural kendi bölümünün dosyasında tanımlanır: **4.2**, docs/spec/04_*.md'dedir. Böylece
// kural numarası dosyayı da söyler.
func TestRuleInItsOwnSection(t *testing.T) {
	for ad, text := range languageSpec(t) {
		no := sectionFileRe.FindStringSubmatch(ad)[1]
		for _, m := range ruleDefRe.FindAllStringSubmatch(text, -1) {
			n, _ := strconv.Atoi(m[1])
			if fmt.Sprintf("%02d", n) != no {
				t.Errorf("docs/spec/%s: §%d kuralı tanımlanıyor; kural docs/spec/%02d_*.md dosyasına taşınmalı", ad, n, n)
			}
		}
	}
}

func TestEveryRuleHasATest(t *testing.T) {
	rules := map[string]bool{}
	for _, text := range languageSpec(t) {
		for _, m := range ruleRe.FindAllStringSubmatch(text, -1) {
			rules[m[1]] = true
		}
	}
	if len(rules) < 50 {
		t.Fatalf("tanımda yalnızca %d kural bulundu; biçim değişmiş olabilir", len(rules))
	}

	files, _ := reporoot.Glob(reporoot.Path("tests/*_test.tr"))
	withTests := map[string]bool{}
	for _, d := range files {
		data, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range testRe.FindAllStringSubmatch(string(data), -1) {
			withTests[m[1]] = true
			if !rules[m[1]] {
				t.Errorf("%s: %s numaralı test tanımda bir kurala karşılık gelmiyor", filepath.Base(d), m[1])
			}
		}
	}

	var missing []string
	for k := range rules {
		if !withTests[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("testi olmayan kurallar: %v", missing)
	}
	// Harfli kurallar kapıdan kaçmamalı (eski düzenli ifade onları hiç saymıyordu).
	for _, k := range []string{"2.9a", "6.3a"} {
		if !rules[k] {
			t.Errorf("harfli kural %s tanımda bulunamadı; kapının düzenli ifadesi harfli kuralları kapsamalı", k)
		}
	}
}

// tests/negative/ altındaki bilerek hatalı modüller (yükleme hatası beklenen olumsuz
// testler) bir teste bağlı olmalı: kullanılmayan bir fikstür hiçbir şeyi sınamaz.
func TestNegativeFixturesUsed(t *testing.T) {
	fixtures, _ := reporoot.Glob(reporoot.Path("tests/negative/*.tr"))
	if len(fixtures) == 0 {
		t.Fatal("tests/negative/ altında fikstür bulunamadı")
	}
	var all []byte
	files, _ := reporoot.Glob(reporoot.Path("tests/*_test.tr"))
	for _, d := range files {
		data, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, data...)
	}
	for _, f := range fixtures {
		ad := strings.TrimSuffix(filepath.Base(f), ".tr")
		// Test dosyaları kodlu saklanır: aktarım `TR_kw_import "negative/..."` diye yazılır.
		if !bytes.Contains(all, []byte(`aktar "negative/`+ad+`"`)) && !bytes.Contains(all, []byte(`TR_kw_import "negative/`+ad+`"`)) {
			t.Errorf("%s hiçbir testte aktarılmıyor", filepath.Base(f))
		}
	}
}

// Dil tanımının bütün testleri (tests/) yorumlayıcının içinden koşar. Tam denetim
// (tools/full_check.sh) Go testlerini -short ile koşar ve aynı takımı bir kez
// `tr test tests` ile koşar; takım iki derleme ayarında üç kez koşmasın diye burada atlanır.
func TestLanguageSpecTestsPass(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: dil tanımı testleri tam denetimde `tr test tests` ile koşar")
	}
	// Komut satırındaki gibi dilin kitaplığı bağlanır (11.6: aktar "tr/...").
	interp.SetLibrary(lib.Files)
	defer interp.SetLibrary(nil)
	var out bytes.Buffer
	summary, err := Run(reporoot.Path("tests"), &out)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.OK() {
		t.Fatalf("dil tanımı testleri kaldı: %s\n%s", summary, out.String())
	}
}
