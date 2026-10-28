package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kapsam: dilin her adı examples/tr altındaki bir örnekte
// geçer. Ad kontrolü metin aramasıyla değil kodlarladır: örnek `source.Encode` ile kodlanır ve
// iskeletindeki kodlar (TR_kw_if, TR_fn_print, .TR_mem_append, TR_arg_lr: ...) sayılır. Komut satırı adları (cmd, opt, repl, dbg, env:
// `tr test`, `tr biçimle`, --dil ...) .tr dosyasında geçmez; örneklerin yorum satırlarında ve
// examples/README.md'de aranır.

// uncoveredNames bilerek ya da henüz örneği yazılmamış adlardır ve gerekçesiyle durur. Liste
// yalnızca küçülür: örneği olan bir ad listede kalırsa test düşer.
var uncoveredNames = map[string]string{
	// Değer adları kaynakta sözcük olarak yazılmaz, yalnızca metinde görünür (3.2:
	// TR_print(TR_Float("inf"))); kodlu metinde geçemezler. Kodlar iç (sınıflı) yazımladır.
	"TR_val_inf": "değer adı (inf), kaynakta sözcük değil",
	"TR_val_nan": "değer adı (nan), kaynakta sözcük değil",
}

// exampleTexts bütün örnek .tr dosyalarını (examples/tr, alt klasörleriyle) verir.
func exampleTexts(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	root := reporoot.Path("examples", "tr")
	err := reporoot.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".tr") {
			return err
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, p)
		text := string(data)
		if view, verr := source.Show(text, source.Turkish()); verr == nil {
			text = view // kodlu örnek Türkçe görünümüyle okunur
		}
		out[filepath.ToSlash(rel)] = text
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// usedCodes örneklerin kodlu metninde geçen kodlardır (yerleştirmelerdekiler dahil).
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
		b, err := source.Split(coded)
		if err != nil || !b.Coded {
			t.Fatalf("examples/tr/%s: kodlu metin başlıksız: %v", name, err)
		}
		l := b.IdentityLexer()
		l.StartRecording()
		parser.New(l).ParseProgram()
		for _, r := range l.Records() {
			if r.Code != codes.None {
				used[r.Code.String()] = true
			}
			if k := lexer.KeywordCode(r.Type); k != codes.None {
				used[k.String()] = true
			}
		}
	}
	return used
}

// commandLineSection komut satırı adlarının aranacağı metindir: examples/README.md'nin "Komut satırı" bölümü
// (bir sonraki "## " başlığına kadar).
func commandLineSection(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path("examples", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "\n## Komut satırı")
	if start < 0 {
		t.Fatal("examples/README.md'de '## Komut satırı' bölümü yok")
	}
	rest := text[start+1:]
	if end := strings.Index(rest[3:], "\n## "); end >= 0 {
		rest = rest[:end+3]
	}
	return rest
}

func TestExamplesCoverNames(t *testing.T) {
	used := usedCodes(t)
	live := map[string]bool{}
	var missing, staleExceptions []string
	for _, k := range codes.Codes() {
		id := k.String()
		// Kitaplık kodlarını TestExamplesCoverLibraryNames sınar.
		if k.ExpectedKind() == codes.KindCommand || k.IsLibrary() {
			continue
		}
		live[id] = true
		_, excepted := uncoveredNames[id]
		switch {
		case !used[id] && !excepted:
			missing = append(missing, id+" "+codes.Source().Name(k))
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

// Komut satırı adları (komut, seçenek, kabuk, hata ayıklayıcı, ortam değişkeni) examples/README.md'nin "Komut satırı"
// bölümünde tam sözcük olarak geçer (seçenek adlarının sözcükleri tireyle de yazılabilir).
func TestExamplesCoverCommandLine(t *testing.T) {
	text := commandLineSection(t)
	var missing []string
	for _, k := range codes.Codes() {
		if k.ExpectedKind() != codes.KindCommand {
			continue
		}
		found := false
		for _, name := range codes.Source().Spellings(k) {
			for _, form := range []string{name, strings.ReplaceAll(name, "_", "-")} {
				word := regexp.MustCompile(`(^|[^\p{L}\p{N}_])` + regexp.QuoteMeta(form) + `($|[^\p{L}\p{N}_])`)
				if word.MatchString(text) {
					found = true
				}
			}
		}
		if !found {
			missing = append(missing, k.String()+" "+codes.Source().Name(k))
		}
	}
	if len(missing) > 0 {
		t.Errorf("examples/README.md'nin Komut satırı bölümünde geçmeyen adlar: %s", strings.Join(missing, ", "))
	}
}
