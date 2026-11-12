package main

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Bu dosya JavaScript ve Python dosyalarının yüklediği depo dosyalarını bağlar (eklentinin kodu ve testleri,
// editors/vscode; ölçüm ve veri betikleri, tools/bench/run.py). Eklentinin testleri Go'dan koşar:
// e2e.TestExtensionJSTests `editors/vscode/test/*_test.js`'yi node ile çalıştırır (yol kalıbı test_paths.go'da `tests`
// kenarı olur); test dosyası sınadığı modülü `require('../coded_view')` ile yükler. Yükleme kenarı olmadan
// `tests-for editors/vscode/coded_view.js` modülü sınayan test dosyasını ve onu koşan Go testini göremiyordu (TCB
// 2026-10-25, 4. madde). JavaScript'te okunan biçimler: `require('./x')`, `require.resolve('../x')`, `import ... from
// './x'`, `import('./x')`, bütün parçaları metin olan `path.join(__dirname, '..', 'media', 'x.js')` (`path.resolve` da),
// dizin sabitleri (`const ROOT = path.join(__dirname, '..')`, `FILE = path.join(FOLDER, 'deneme.tr')`) ve `readJSON =
// (...parts) => ... path.join(ROOT, ...parts)` yardımcıları. Python'da okunan biçimler: `__file__` tabanlı
// `os.path.abspath` / `os.path.dirname` / `os.path.join` modül sabitleri (`ROOT`, `BENCH`, `MODEL_TR`) ve bütün
// parçaları sabit olan satır içi `os.path.join(...)` ifadeleri. Bilinmeyen değişkenle kurulan yol ve dış paket adları
// (`vscode`, `fs`, `json`) görülmez.

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

const pyRepoRoot = "/repo"

// extractJSLoads her izlenen JavaScript ve Python dosyasından yüklediği depo dosyasına `loads` kenarı çeker. Dosya
// düğümleri kurulduktan sonra çalışır (extractDataFiles).
func extractJSLoads(root string, b *builder) {
	fileIDs := map[string]string{}
	var js, py []string
	for _, n := range b.nodes {
		if (n.Kind == kindFile || n.Kind == kindDoc) && n.File != "" {
			if _, exists := fileIDs[n.File]; !exists {
				fileIDs[n.File] = n.ID
			}
			if n.Kind == kindFile {
				if ext := path.Ext(n.File); jsExts[ext] {
					js = append(js, n.File)
				} else if ext == ".py" {
					py = append(py, n.File)
				}
			}
		}
	}
	sort.Strings(js)
	sort.Strings(py)
	for _, rel := range js {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		paths := jsPaths(string(data))
		for i, line := range strings.Split(string(data), "\n") {
			for _, spec := range paths.specs(line) {
				if target := resolveJSLoad(path.Dir(rel), spec, fileIDs); target != "" && target != rel {
					b.addEdge(fileIDs[rel], fileIDs[target], edgeLoads, extracted, rel, i+1)
				}
			}
		}
	}
	for _, rel := range py {
		extractPyLoads(root, rel, fileIDs, b)
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
func resolveJSLoad(dir, spec string, fileIDs map[string]string) string {
	p := path.Join(dir, spec)
	for _, c := range []string{p, p + ".js", p + "/index.js"} {
		if fileIDs[c] != "" {
			return c
		}
	}
	return ""
}

type pyPathVal struct {
	val      string
	anchored bool // `__file__` kökünden türemiş depo içi yol
}

func extractPyLoads(root, rel string, fileIDs map[string]string, b *builder) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return
	}
	consts := map[string]pyPathVal{"__file__": {val: pyRepoRoot + "/" + rel, anchored: true}}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i, raw := range lines {
		line := stripPyComment(raw)
		if line == "" {
			continue
		}
		if raw[0] != ' ' && raw[0] != '\t' {
			if name, rhs, ok := splitPyAssign(line); ok {
				if v, ok := evalPyPath(rhs, consts); ok {
					consts[name] = v
				}
			}
		}
		for _, call := range findPyJoinCalls(line) {
			v, ok := evalPyPath(call, consts)
			if !ok || !v.anchored {
				continue
			}
			target, ok := strings.CutPrefix(v.val, pyRepoRoot+"/")
			if ok && target != rel && fileIDs[target] != "" {
				b.addEdge(fileIDs[rel], fileIDs[target], edgeLoads, extracted, rel, i+1)
			}
		}
	}
}

func stripPyComment(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
		} else if c == '#' {
			return strings.TrimSpace(line[:i])
		}
	}
	return strings.TrimSpace(line)
}

func splitPyAssign(line string) (name, rhs string, ok bool) {
	eq := strings.IndexByte(line, '=')
	if eq <= 0 || (eq+1 < len(line) && line[eq+1] == '=') || strings.ContainsRune("!<>=+-*/%&|^:", rune(line[eq-1])) {
		return "", "", false
	}
	name = strings.TrimSpace(line[:eq])
	if !isPyIdent(name) {
		return "", "", false
	}
	return name, strings.TrimSpace(line[eq+1:]), true
}

func isPyIdent(s string) bool {
	if s == "" || s == "None" || s == "True" || s == "False" {
		return false
	}
	for i, r := range s {
		if !(unicode.IsLetter(r) || r == '_' || (i > 0 && unicode.IsDigit(r))) {
			return false
		}
	}
	return true
}

func evalPyPath(expr string, consts map[string]pyPathVal) (pyPathVal, bool) {
	expr = strings.TrimSpace(expr)
	if s, ok := pyStringLit(expr); ok && s != "" {
		return pyPathVal{val: s}, true
	}
	if isPyIdent(expr) {
		v, ok := consts[expr]
		return v, ok
	}
	fn, inner, ok := splitPyCall(expr)
	if !ok {
		return pyPathVal{}, false
	}
	args := splitPyArgs(inner)
	switch {
	case fn == "os.path.abspath" && len(args) == 1:
		v, ok := evalPyPath(args[0], consts)
		if !ok || !v.anchored {
			return pyPathVal{}, false
		}
		return pyPathVal{val: path.Clean(v.val), anchored: true}, true
	case fn == "os.path.dirname" && len(args) == 1:
		v, ok := evalPyPath(args[0], consts)
		if !ok {
			return pyPathVal{}, false
		}
		d := path.Dir(v.val)
		if v.anchored && d != pyRepoRoot && !strings.HasPrefix(d, pyRepoRoot+"/") {
			return pyPathVal{}, false
		}
		return pyPathVal{val: d, anchored: v.anchored}, true
	case fn == "os.path.join" && len(args) >= 1:
		first, ok := evalPyPath(args[0], consts)
		if !ok {
			return pyPathVal{}, false
		}
		parts := []string{first.val}
		for _, a := range args[1:] {
			av, ok := evalPyPath(a, consts)
			if !ok || av.anchored || strings.HasPrefix(av.val, "/") {
				return pyPathVal{}, false
			}
			parts = append(parts, av.val)
		}
		joined := path.Join(parts...)
		if first.anchored && joined != pyRepoRoot && !strings.HasPrefix(joined, pyRepoRoot+"/") {
			return pyPathVal{}, false
		}
		return pyPathVal{val: joined, anchored: first.anchored}, true
	}
	return pyPathVal{}, false
}

func pyStringLit(s string) (string, bool) {
	raw := false
	if len(s) >= 3 && (s[0] == 'r' || s[0] == 'R') && (s[1] == '\'' || s[1] == '"') {
		raw, s = true, s[1:]
	}
	if len(s) < 2 || (s[0] != '\'' && s[0] != '"') || s[len(s)-1] != s[0] {
		return "", false
	}
	q := s[0]
	for i := 1; i < len(s)-1; i++ {
		if s[i] == '\\' && !raw {
			i++
			continue
		}
		if s[i] == q {
			return "", false
		}
	}
	if raw || q == '\'' {
		body := s[1 : len(s)-1]
		if !raw {
			body = strings.ReplaceAll(strings.ReplaceAll(body, `\'`, `'`), `\\`, `\`)
		}
		return body, true
	}
	out, err := strconv.Unquote(s)
	return out, err == nil
}

func splitPyCall(s string) (fn, inner string, ok bool) {
	open := strings.IndexByte(s, '(')
	if open <= 0 || s[len(s)-1] != ')' {
		return "", "", false
	}
	closeIdx := matchPyParen(s, open)
	if closeIdx != len(s)-1 {
		return "", "", false
	}
	return strings.TrimSpace(s[:open]), s[open+1 : closeIdx], true
}

func matchPyParen(s string, open int) int {
	depth := 0
	var quote byte
	for i := open; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitPyArgs(inner string) []string {
	if strings.TrimSpace(inner) == "" {
		return nil
	}
	var args []string
	var quote byte
	depth, start := 0, 0
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				if a := strings.TrimSpace(inner[start:i]); a != "" {
					args = append(args, a)
				}
				start = i + 1
			}
		}
	}
	if last := strings.TrimSpace(inner[start:]); last != "" {
		args = append(args, last)
	}
	return args
}

func findPyJoinCalls(line string) []string {
	const needle = "os.path.join"
	var out []string
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if !strings.HasPrefix(line[i:], needle) || (i > 0 && (unicode.IsLetter(rune(line[i-1])) || unicode.IsDigit(rune(line[i-1])) || line[i-1] == '_' || line[i-1] == '.')) {
			continue
		}
		j := i + len(needle)
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
		if j < len(line) && line[j] == '(' {
			if end := matchPyParen(line, j); end > j {
				out = append(out, line[i:end+1])
			}
		}
	}
	return out
}
