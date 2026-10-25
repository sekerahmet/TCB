package main

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Bu dosya JavaScript dosyalarının yüklediği depo dosyalarını bağlar (eklentinin kodu ve testleri, editors/vscode).
// Eklentinin testleri Go'dan koşar: e2e.TestExtensionJSTests `editors/vscode/test/*_test.js`'yi node ile çalıştırır
// (yol kalıbı test_paths.go'da `tests` kenarı olur); test dosyası sınadığı modülü `require('../coded_view')` ile yükler.
// Yükleme kenarı olmadan `tests-for editors/vscode/coded_view.js` modülü sınayan test dosyasını ve onu koşan Go testini
// göremiyordu (TCB 2026-10-25, 4. madde). Okunan biçimler: `require('./x')`, `require.resolve('../x')`, `import ... from
// './x'`, `import('./x')` ve bütün parçaları metin olan `path.join(__dirname, '..', 'media', 'x.js')`
// (`path.resolve` da). Değişkenle kurulan yol görülmez. Yalnızca göreli yollar sayılır; paket adları (`vscode`, `fs`)
// depo dosyası değildir.

var (
	jsSpecifier = regexp.MustCompile(`(?:require(?:\.resolve)?\(\s*|\bfrom\s*|\bimport\s*\(?\s*)['"](\.\.?/[^'"\s]+)['"]`)
	jsDirJoin   = regexp.MustCompile(`path\.(?:join|resolve)\(\s*__dirname((?:\s*,\s*'[^']*'|\s*,\s*"[^"]*")+)\s*\)`)
	jsSegment   = regexp.MustCompile(`['"]([^'"]*)['"]`)
)

// jsExts JavaScript kaynak uzantılarıdır.
var jsExts = map[string]bool{".js": true, ".mjs": true, ".cjs": true}

// extractJSLoads her izlenen JavaScript dosyasından yüklediği depo dosyasına `loads` kenarı çeker. Dosya düğümleri
// kurulduktan sonra çalışır (extractDataFiles).
func extractJSLoads(root string, b *builder) {
	files := map[string]bool{}
	var js []string
	for _, n := range b.nodes {
		if n.Kind == kindFile && n.File != "" {
			files[n.File] = true
			if jsExts[path.Ext(n.File)] {
				js = append(js, n.File)
			}
		}
	}
	sort.Strings(js)
	for _, rel := range js {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, spec := range jsLoadSpecs(line) {
				if target := resolveJSLoad(path.Dir(rel), spec, files); target != "" && target != rel {
					b.addEdge("file:"+rel, "file:"+target, edgeLoads, extracted, rel, i+1)
				}
			}
		}
	}
}

// jsLoadSpecs bir satırdaki göreli yükleme yollarıdır.
func jsLoadSpecs(line string) []string {
	var out []string
	for _, m := range jsSpecifier.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}
	for _, m := range jsDirJoin.FindAllStringSubmatch(line, -1) {
		parts := []string{"."}
		for _, s := range jsSegment.FindAllStringSubmatch(m[1], -1) {
			parts = append(parts, s[1])
		}
		out = append(out, path.Join(parts...))
	}
	return out
}

// resolveJSLoad Node'un göreli çözüm sırasıdır: yolun kendisi, `.js` eki, klasörün `index.js`'i.
func resolveJSLoad(dir, spec string, files map[string]bool) string {
	p := path.Join(dir, spec)
	for _, c := range []string{p, p + ".js", p + "/index.js"} {
		if files[c] {
			return c
		}
	}
	return ""
}
