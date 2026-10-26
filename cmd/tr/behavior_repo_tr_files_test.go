package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// repoWarningAllowed bilerek uyarı veren yerlerdir ("dosya:satır" → gerekçe). Liste küçük tutulur; artık
// uyarmayan kayıt testi düşürür.
var repoWarningAllowed = map[string]string{
	"tests/coded/ornek_kodlu.tr:17": "gölgelemenin kendisini sınar: kodlu dosyada yerleşiğin adıyla aynı yerel ad kod değil, kullanıcının adıdır",
	"lib/katman.tr:81": "döner_konum_tablosu'nun taban parametresi RoPE'un yerleşik terimidir (rope_theta; onaylı kitaplık tablosu " +
		"TR_arg_base taban); gövde yerleşik taban'ı kullanmaz",
}

// Tanımsız ad denetimi (docs/spec/05_deyimler.md §5.14) dinamik dilde yanlış alarm vermemeli:
// depodaki bütün .tr dosyaları (testler, örnekler, kıyas programları, kitaplık) hatasız ve uyarısız
// denetlenir (gölgeleme 2.9a, etkisiz ifade 2.8 ...; bilerek uyaranlar repoWarningAllowed'da). Bilerek
// hatalı fikstürler (tests/negative/) hariçtir. Başka dosyaların aktardığı modüller, yorumlayıcıdaki
// gibi aktaranın genel adlarıyla denetlenir (11.1).
func TestRepoTRFilesPassCheck(t *testing.T) {
	root := reporoot.Root()
	sources := map[string]string{}
	err := reporoot.WalkDir(root, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "negative", "node_modules", ".claude", "testdata", "hatalar", "errors", "scratch": // testdata: araçların kasıtlı bozuk fikstürleri; hatalar, errors: hata örnekleri (examples/); scratch: .gitignore'daki yerel çalışma klasörü, depoda değil
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(filePath, ".tr") {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			sources[filePath] = string(content)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) < 20 {
		t.Fatalf("yalnızca %d .tr dosyası bulundu", len(sources))
	}

	// Modül → aktaranların genel adları (birleşimi); aktaranlardan biri dinamikse dinamik.
	type global struct {
		names   map[string]bool
		dynamic bool
	}
	modules := map[string]*global{}
	for filePath, code := range sources {
		names, dynamic := checker.TopLevelNames(source.Prepare(code, source.Options{}).Program)
		for _, y := range importSites(code) {
			resolved, _, ok := resolveModulePath(y.filePath, filePath, filePath)
			if !ok {
				continue
			}
			absolute, _ := filepath.Abs(resolved)
			g := modules[absolute]
			if g == nil {
				g = &global{names: map[string]bool{}}
				modules[absolute] = g
			}
			for _, a := range names {
				g.names[a] = true
			}
			g.dynamic = g.dynamic || dynamic
		}
	}

	used := map[string]bool{}
	for filePath, code := range sources {
		relative, _ := filepath.Rel(root, filePath)
		relative = filepath.ToSlash(relative)
		absolute, _ := filepath.Abs(filePath)
		var r *source.Result
		if g := modules[absolute]; g != nil {
			var names []string
			for a := range g.names {
				names = append(names, a)
			}
			r = interp.CheckModule(code, names, g.dynamic)
		} else {
			r = source.Prepare(code, source.Options{TypeCheck: true})
		}
		for _, h := range r.Errors {
			t.Errorf("%s: %s", relative, h)
		}
		for _, w := range r.Warnings {
			line, _, _ := messages.Pos(w)
			if at := relative + ":" + strconv.Itoa(line); repoWarningAllowed[at] != "" {
				used[at] = true
				continue
			}
			t.Errorf("%s: uyarı: %s", relative, w)
		}
	}
	for at := range repoWarningAllowed {
		if !used[at] {
			t.Errorf("repoWarningAllowed: %s artık uyarmıyor; kaydı silin", at)
		}
	}
}
