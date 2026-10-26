package main

import (
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// libraryImports test dizesindeki TR programının kitaplıktan aktardığı modülleri (`aktar "tr/hazir"`, kodlu yazımda
// `TR_kw_import "tr/hazir"`) kitaplık dosyasına başvuru olarak kaydeder: metni koşan test, metin ister yorumlayıcıya
// ister dosyaya yazılıp `tr`'ye verilsin (TestReadyModelMatchesTransformers), aktardığı modülü koşar. Aktarmalar
// codeindex.Snippet'le dosyadaki kuralla çözülür; kitaplık depodaki lib/ klasörüdür (gömülü lib.Files'ın kaynağı:
// lib.Files'ı içe aktarmak, codegraph'ı dolaylı çağıran her testi gömme yoluyla her kitaplık dosyasına bağlıyordu).
// Geçişli aktarmaları loaderTests izler. Yalnızca "tr/ geçen dize ayrıştırılır.
func (x *goExtractor) libraryImports(fc *fileCtx, owners []string, pos token.Pos, text string) {
	if !strings.Contains(text, `"tr/`) {
		return
	}
	_, imports := codeindex.Snippet(text, os.DirFS(filepath.Join(x.m.Root, "lib")))
	for _, file := range imports {
		for _, owner := range owners {
			x.idx.pathRefs = append(x.idx.pathRefs, pathRef{owner: owner, dir: path.Dir(fc.rel), file: fc.rel, line: x.line(pos), text: file, root: true})
		}
	}
}
