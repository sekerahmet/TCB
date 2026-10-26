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
// (`path.resolve` da). Test bir veri dosyasını çoğu zaman bir dizin sabitiyle ya da bir yardımcıyla okur: `const ROOT =
// path.join(__dirname, '..')` sonra `path.join(ROOT, 'l10n', 'x.json')`, zincirli sabit (`FILE = path.join(FOLDER,
// 'deneme.tr')`) ve `readJSON = (...parts) => ... path.join(ROOT, ...parts)` ile `readJSON('l10n', 'x.json')`; bunlar da
// okunur (eksikler listesi, ana oturum 2026-10-26: eklentinin Türkçe paketi). Bilinmeyen değişkenle kurulan yol görülmez.
// Yalnızca göreli yollar sayılır; paket adları (`vscode`, `fs`) depo dosyası değildir.

var (
	jsSpecifier = regexp.MustCompile(`(?:require(?:\.resolve)?\(\s*|\bfrom\s*|\bimport\s*\(?\s*)['"](\.\.?/[^'"\s]+)['"]`)
	jsSegment   = regexp.MustCompile(`['"]([^'"]*)['"]`)
	// jsJoin bir dizinden (`__dirname` ya da bir dizin sabiti) bütün parçaları metin olan path.join/resolve'dur.
	jsJoin = regexp.MustCompile(`path\.(?:join|resolve)\(\s*([A-Za-z_$][\w$]*)((?:\s*,\s*'[^']*'|\s*,\s*"[^"]*")*)\s*\)`)
	// jsPathConst bir sabite atanan path.join/resolve'dur (`const ROOT = path.join(__dirname, '..')`).
	jsPathConst = regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*path\.(?:join|resolve)\(`)
	// jsRestHelper kalan argümanlarını bir dizine ekleyen ok işlevidir (`readJSON = (...parts) => ... path.join(ROOT, ...parts)`).
	jsRestHelper = regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*\(\s*\.\.\.([A-Za-z_$][\w$]*)\s*\)\s*=>.*path\.(?:join|resolve)\(\s*([A-Za-z_$][\w$]*)\s*,\s*\.\.\.([A-Za-z_$][\w$]*)\s*\)`)
	// jsStringCall bütün argümanları metin olan bir çağrıdır (`readJSON('l10n', 'x.json')`).
	jsStringCall = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)\(((?:\s*'[^']*'|\s*"[^"]*")(?:\s*,\s*'[^']*'|\s*,\s*"[^"]*")*)\s*\)`)
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
		paths := jsPaths(string(data))
		for i, line := range strings.Split(string(data), "\n") {
			for _, spec := range paths.specs(line) {
				if target := resolveJSLoad(path.Dir(rel), spec, files); target != "" && target != rel {
					b.addEdge("file:"+rel, "file:"+target, edgeLoads, extracted, rel, i+1)
				}
			}
		}
	}
}

// jsPathNames bir JavaScript dosyasının yol adlarıdır: dizin sabitleri ve kalan argümanlarını bir dizine ekleyen
// yardımcılar; değerleri dosyanın klasörüne göre yollardır (`__dirname` ".").
type jsPathNames struct {
	dirs, helpers map[string]string
}

// jsPaths dosyanın dizin sabitlerini ve yol yardımcılarını sırayla okur: bir sabit önceki sabitlerle kurulabilir.
func jsPaths(text string) jsPathNames {
	p := jsPathNames{dirs: map[string]string{"__dirname": "."}, helpers: map[string]string{}}
	for _, line := range strings.Split(text, "\n") {
		if m := jsRestHelper.FindStringSubmatch(line); m != nil && m[2] == m[4] {
			if base, ok := p.dirs[m[3]]; ok {
				p.helpers[m[1]] = base
			}
			continue
		}
		if c := jsPathConst.FindStringSubmatchIndex(line); c != nil {
			if j := jsJoin.FindStringSubmatch(line[c[1]-len("path.join("):]); j != nil {
				if v, ok := p.join(j[1], j[2]); ok {
					p.dirs[line[c[2]:c[3]]] = v
				}
			}
		}
	}
	return p
}

// join bir dizin adına metin parçalarını ekler.
func (p jsPathNames) join(base, segments string) (string, bool) {
	dir, ok := p.dirs[base]
	if !ok {
		return "", false
	}
	parts := []string{dir}
	for _, s := range jsSegment.FindAllStringSubmatch(segments, -1) {
		parts = append(parts, s[1])
	}
	return path.Join(parts...), true
}

// specs bir satırdaki göreli yükleme ve okuma yollarıdır.
func (p jsPathNames) specs(line string) []string {
	var out []string
	for _, m := range jsSpecifier.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}
	for _, m := range jsJoin.FindAllStringSubmatch(line, -1) {
		if v, ok := p.join(m[1], m[2]); ok {
			out = append(out, v)
		}
	}
	for _, m := range jsStringCall.FindAllStringSubmatch(line, -1) {
		if base, ok := p.helpers[m[1]]; ok {
			parts := []string{base}
			for _, s := range jsSegment.FindAllStringSubmatch(m[2], -1) {
				parts = append(parts, s[1])
			}
			out = append(out, path.Join(parts...))
		}
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
