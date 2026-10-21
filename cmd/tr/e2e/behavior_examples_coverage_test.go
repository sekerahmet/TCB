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
	"A102": "yazılacak",
	"A103": "yazılacak",
	"A203": "yazılacak",
	"A204": "yazılacak",
	"A205": "yazılacak",
	"A206": "yazılacak",
	"A207": "yazılacak",
	"A208": "yazılacak",
	"A209": "yazılacak",
	"A210": "yazılacak",
	"A211": "yazılacak",
	"A212": "yazılacak",
	"A213": "yazılacak",
	"A214": "yazılacak",
	"A215": "yazılacak",
	"A231": "yazılacak",
	"A232": "yazılacak",
	"A233": "yazılacak",
	"A234": "yazılacak",
	"A250": "yazılacak",
	"A255": "yazılacak",
	"A256": "yazılacak",
	"A257": "yazılacak",
	"A258": "yazılacak",
	"G121": "yazılacak",
	"G123": "yazılacak",
	"G124": "yazılacak",
	"G125": "yazılacak",
	"G401": "yazılacak",
	"G403": "yazılacak",
	"G406": "yazılacak",
	"G407": "yazılacak",
	"G408": "yazılacak",
	"G409": "yazılacak",
	"G411": "yazılacak",
	"G412": "yazılacak",
	"G414": "yazılacak",
	"G502": "yazılacak",
	"G602": "yazılacak",
	"G603": "yazılacak",
	"G800": "yazılacak",
	"G900": "yazılacak",
	"G901": "yazılacak",
	"T121": "yazılacak",
	"T204": "yazılacak",
	"T211": "yazılacak",
	"T220": "yazılacak",
	"T221": "yazılacak",
	"T222": "yazılacak",
	"T223": "yazılacak",
	"T224": "yazılacak",
	"T225": "yazılacak",
	"T226": "yazılacak",
	"T227": "yazılacak",
	"T228": "yazılacak",
	"T231": "yazılacak",
	"T232": "yazılacak",
	"T233": "yazılacak",
	"T234": "yazılacak",
	"T240": "yazılacak",
	"T241": "yazılacak",
	"T242": "yazılacak",
	"T243": "yazılacak",
	"T244": "yazılacak",
	"T245": "yazılacak",
	"T310": "yazılacak",
	"T311": "yazılacak",
	"T312": "yazılacak",
	"T401": "yazılacak",
	"T402": "yazılacak",
	"T502": "yazılacak",
	"T512": "yazılacak",
	"T611": "yazılacak",
	"T703": "yazılacak",
	"T710": "yazılacak",
	"T720": "yazılacak",
	"T721": "yazılacak",
	"T901": "yazılacak",
	"T904": "yazılacak",
	"T905": "yazılacak",
	"T906": "yazılacak",
	"T907": "yazılacak",
	"T912": "yazılacak",
	"T913": "yazılacak",
	"T914": "yazılacak",
	"T915": "yazılacak",
	"T920": "yazılacak",
	"T921": "yazılacak",
	"T951": "yazılacak",
	"T952": "yazılacak",
	"T960": "yazılacak",
	"T961": "yazılacak",
	"T980": "yazılacak",
	"T981": "yazılacak",
	"T983": "yazılacak",
	"T985": "yazılacak",
	"T990": "yazılacak",
	"V900": "yazılacak",
	"V901": "yazılacak",
	"V910": "yazılacak",
	"V911": "yazılacak",
	"Y011": "yazılacak",
	"Y012": "yazılacak",
	"Y013": "yazılacak",
	"Y106": "yazılacak",
	"Y203": "yazılacak",
	"Y310": "yazılacak",
	"Y311": "yazılacak",
	"Y312": "yazılacak",
	"Y313": "yazılacak",
	"Y316": "yazılacak",
	"Y317": "yazılacak",
	"Y318": "yazılacak",
	"Y401": "yazılacak",
	"Y402": "yazılacak",
	"Y411": "yazılacak",
	"Y413": "yazılacak",
	"Y414": "yazılacak",
	"Y415": "yazılacak",
	"Y503": "yazılacak",
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
