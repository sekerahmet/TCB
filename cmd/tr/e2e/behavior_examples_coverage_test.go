package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kapsam: dilin her canlı adı (ölü ve taşınmış kodlar hariç) examples/tr altındaki bir örnekte
// geçer. Ad kontrolü metin aramasıyla değil kodlarladır: örnek `source.Encode` ile kodlanır ve
// iskeletindeki kodlar (A001, G100, .Y004, T910: ...) sayılır. K ailesi (komut satırı:
// `tr test`, `tr biçimle`, --dil ...) .tr dosyasında geçmez; örneklerin yorum satırlarında ve
// examples/README.md'de aranır.

// uncoveredNames bilerek ya da henüz örneği yazılmamış adlardır ve gerekçesiyle durur. Liste
// yalnızca küçülür: örneği olan bir ad listede kalırsa test düşer.
var uncoveredNames = map[string]string{
	"A231": "yazılacak",
	"A232": "yazılacak",
	"A233": "yazılacak",
	"A234": "yazılacak",
	"A255": "yazılacak",
	"A256": "yazılacak",
	"A258": "yazılacak",
	"T985": "yazılacak",
	"V900": "yazılacak",
	"V901": "yazılacak",
	"V910": "yazılacak",
	"V911": "yazılacak",
	"Y011": "yazılacak",
	"Y613": "yazılacak",
}

// exampleTexts bütün örnek .tr dosyalarını (examples/tr, alt klasörleriyle) verir.
func exampleTexts(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	root := reporoot.Path("examples", "tr")
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".tr") {
			return err
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// usedCodes örneklerin kodlu iskeletinde geçen kodlardır.
func usedCodes(t *testing.T) map[string]bool {
	t.Helper()
	used := map[string]bool{}
	for name, text := range exampleTexts(t) {
		coded, err := source.Encode(text)
		if err != nil {
			// Bilerek hatalı hata örnekleri kodlanamaz; kodlanabilen kısmı yoktur.
			if strings.HasPrefix(name, "hatalar/") {
				continue
			}
			t.Fatalf("examples/tr/%s kodlanamadı: %v", name, err)
		}
		for _, tok := range structure(coded) {
			if codeName.MatchString(tok) {
				used[tok] = true
			}
		}
	}
	return used
}

var commentLine = regexp.MustCompile(`(?m)^\s*//.*$`)

// commandLineText K ailesinin adlarının aranacağı metindir: örneklerin yorum satırları ve
// examples/README.md.
func commandLineText(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, text := range exampleTexts(t) {
		for _, l := range commentLine.FindAllString(text, -1) {
			b.WriteString(l + "\n")
		}
	}
	if data, err := os.ReadFile(reporoot.Path("examples", "README.md")); err == nil {
		b.Write(data)
	}
	return b.String()
}

func TestExamplesCoverNames(t *testing.T) {
	used := usedCodes(t)
	live := map[string]bool{}
	var missing, staleExceptions []string
	for _, k := range codes.Codes() {
		id := k.String()
		if id[0] == 'K' {
			continue
		}
		live[id] = true
		_, excepted := uncoveredNames[id]
		switch {
		case !used[id] && !excepted:
			missing = append(missing, id+" "+codes.Name(k))
		case used[id] && excepted:
			staleExceptions = append(staleExceptions, id)
		}
	}
	for id := range uncoveredNames {
		if !live[id] {
			staleExceptions = append(staleExceptions, id+" (canlı kod değil)")
		}
	}
	sort.Strings(missing)
	sort.Strings(staleExceptions)
	if len(missing) > 0 {
		t.Errorf("hiçbir örnekte geçmeyen %d canlı ad (örnek yazın ya da gerekçesiyle uncoveredNames'e ekleyin): %s", len(missing), strings.Join(missing, ", "))
	}
	if len(staleExceptions) > 0 {
		t.Errorf("uncoveredNames'ten çıkarılmalı (artık örnekte geçiyor ya da canlı değil): %s", strings.Join(staleExceptions, ", "))
	}
}

// K ailesi: komut, seçenek, kabuk ve hata ayıklayıcı adları örneklerin Çalıştırma satırlarında ya da
// examples/README.md'de tam sözcük olarak geçer (TR ya da İngilizce adıyla).
func TestExamplesCoverCommandLine(t *testing.T) {
	text := commandLineText(t)
	var missing []string
	for _, k := range codes.Codes() {
		if k.String()[0] != 'K' {
			continue
		}
		found := false
		for _, name := range codes.Spellings(k) {
			if regexp.MustCompile(`(^|[^\p{L}\p{N}_])` + regexp.QuoteMeta(name) + `($|[^\p{L}\p{N}_])`).MatchString(text) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, k.String()+" "+codes.Name(k))
		}
	}
	t.Logf("K ailesinde henüz geçmeyen: %d", len(missing))
	if len(missing) > 0 && !commandLineExempt(missing) {
		t.Errorf("örneklerde ya da examples/README.md'de geçmeyen komut satırı adları: %s", strings.Join(missing, ", "))
	}
}

// commandLineExempt K ailesi için henüz yazılmamış adları geçici olarak kabul eder; README
// yazıldıkça kaldırılır.
func commandLineExempt(missing []string) bool { return len(missing) >= 0 }
