package main

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// scriptExts ortam değişkeni kullanımları okunan betik türleridir: kabuk ve Python. Betik değişkeni koyar ya da okur
// (`TR_GPU_ESZAMANLI=1 tr ...`, `export TR_DIL=en`, `${TR_DIL:-tr}`, `os.environ["TR_DIL"]`); hepsinde değişkenin adı
// düz metin olarak geçer.
var scriptExts = map[string]bool{".sh": true, ".py": true}

var envWord = regexp.MustCompile(`\bTR_[A-Za-z0-9_]+\b`)

// extractScriptEnv izlenen her betiğin (scriptExts) yorum satırı dışında andığı kodlu ortam değişkenine (her dildeki adı,
// ortak adı ya da kimliği) dosya düğümünden `uses_env` kenarı çeker. Kodsuz değişkenin (TR_RECORD_DIR) düğümü yoktur.
func extractScriptEnv(root string, b *builder) {
	env := envCodes(root)
	for _, n := range b.nodes {
		if n.Kind != kindFile || !scriptExts[path.Ext(n.File)] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.File)))
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			for _, w := range envWord.FindAllString(line, -1) {
				if code := env[w]; code != "" {
					b.addEdge(n.ID, "name:"+code, edgeUsesEnv, extracted, n.File, i+1)
				}
			}
		}
	}
}
