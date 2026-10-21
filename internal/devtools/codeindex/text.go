package codeindex

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Bu dosya yeniden adlandırma etkisinin "olası" tarafıdır: dizinin bağlayamadığı yerleri (belge,
// yorum, metin, ileti tablosu, bağlanamayan koddaki adlar) sözcük olarak tarar. Hiçbir eşleşme
// kesin sayılmaz.

// Metnin bulunduğu yer (TextHit.Where).
const (
	WhereCode    = "code"    // .tr ya da .go kodunda dizinin bağlamadığı ad (yerel değişken, sözlük anahtarı, ...)
	WhereComment = "comment" // .tr yorumu
	WhereString  = "string"  // .tr metin sabiti (iç ifadeler dahil)
	WhereDoc     = "doc"     // .md belge
	WhereData    = "data"    // .json (ileti ve ad tabloları)
)

// Eşleşme biçimi (TextHit.Match).
const (
	MatchExact    = "exact"    // sözcüğün kendisi
	MatchSuffixed = "suffixed" // Türkçe ekli biçim (başı, başları, başında)
	MatchCompound = "compound" // alt çizgiyle ayrılmış ad parçası (kv_baş, baş_boyutu)
	MatchDerived  = "derived"  // türemiş sıfat (başlı, başsız)
)

// TextHit bir metin eşleşmesidir.
type TextHit struct {
	Pos        Pos    `json:"pos"`
	Where      string `json:"where"`
	Match      string `json:"match"`
	Word       string `json:"word"`    // yazıldığı gibi
	Context    string `json:"context"` // satırın metni (kırpılmış)
	Suggestion string `json:"suggestion,omitempty"`
	Check      string `json:"check,omitempty"` // "vowel_harmony": yeni sözcüğün ünlü uyumu eskisinden farklı, ek elle denetlenmeli
}

const (
	maxTextFileSize = 4 << 20
	maxContext      = 160
)

// textFiles yeniden adlandırma taramasının dosyalarıdır: `.git` varsa yalnızca izlenenler (git ls-files;
// izlenmeyen dosyalar kullanıcınındır), yoksa DefaultSkipDirs dışındaki klasörler dolaşılır.
func textFiles(root string) []string {
	var rels []string
	tracked, ok := gitTracked(root)
	if ok {
		rels = tracked
	} else {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if p != root && (DefaultSkipDirs[d.Name()] || d.Name() == CacheDir) {
					return filepath.SkipDir
				}
				return nil
			}
			r, _ := filepath.Rel(root, p)
			rels = append(rels, filepath.ToSlash(r))
			return nil
		})
	}
	var out []string
	for _, r := range rels {
		switch strings.ToLower(filepath.Ext(r)) {
		case ".md", ".json", ".tr", ".go":
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

// displayText bir .tr dosyasının Türkçe gösterim metnidir (kodlu dosya önce çözülür; çözülemezse
// ham metin).
func displayText(text string) string {
	parts, err := source.Split(text)
	if err != nil {
		return text
	}
	if parts.Coded {
		if turkish, err := source.Decode(text, source.Turkish()); err == nil {
			if p, err := source.Split(turkish); err == nil {
				return p.Body
			}
		}
	}
	return parts.Body
}

// wordMatcher aranan sözcüğü ve ekli biçimlerini tanır.
type wordMatcher struct {
	word    string // Türkçe katlanmış
	suffix  map[string]bool
	derived map[string]bool
	repl    string // yeni sözcük
	harmOK  bool   // yeni sözcüğün ünlü uyumu eskisiyle aynı
}

func newWordMatcher(word, repl string) *wordMatcher {
	m := &wordMatcher{word: fold(word), repl: repl, suffix: suffixTails(fold(word)), derived: derivedTails(fold(word))}
	oldH, _ := lastHarmony(fold(word))
	newH, ok := lastHarmony(fold(repl))
	m.harmOK = ok && oldH == newH
	return m
}

func lastHarmony(s string) (harmony, bool) {
	runes := []rune(s)
	for i := len(runes) - 1; i >= 0; i-- {
		if h, ok := harmonyOf(runes[i]); ok {
			return h, true
		}
	}
	return harmony{}, false
}

// classify bir sözcüğün (alt çizgisiyle) aranan sözcükle ilişkisini söyler; ilişkisi yoksa "" ve
// başında aranan sözcük varsa other=true (başka, başla: sayılır ama listelenmez).
func (m *wordMatcher) classify(token string) (match, suggestion, check string, other bool) {
	t := fold(token)
	if t == m.word {
		return MatchExact, m.repl, "", false
	}
	if strings.Contains(t, "_") {
		parts := strings.Split(token, "_")
		for i, p := range parts {
			if fold(p) == m.word {
				parts[i] = m.repl
				return MatchCompound, strings.Join(parts, "_"), "", false
			}
		}
	}
	if tail, ok := strings.CutPrefix(t, m.word); ok && tail != "" {
		if m.suffix[tail] {
			chk := ""
			if !m.harmOK {
				chk = "vowel_harmony"
			}
			return MatchSuffixed, m.repl + tail, chk, false
		}
		if m.derived[tail] {
			return MatchDerived, m.repl + tail, "", false
		}
		return "", "", "", !strings.Contains(tail, "_")
	}
	return "", "", "", false
}

// ScanText kökteki metin dosyalarında word'ü arar; exclude eşleşmelerin atlanacağı konumlardır
// ("dosya:satır:sütun": dizinin zaten bağladığı yerler). Eşleşmeler dosya ve satır sırasıyla,
// other başka sözcük sayısıdır (listelenmez).
func ScanText(root, word, repl string, exclude map[string]bool) (hits []TextHit, other int) {
	m := newWordMatcher(word, repl)
	for _, rel := range textFiles(root) {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || info.Size() > maxTextFileSize {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		text := string(data)
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".tr" {
			text = displayText(text)
		}
		h, o := scanFile(rel, ext, text, m, exclude)
		hits = append(hits, h...)
		other += o
	}
	return hits, other
}

// lexState bir kaynak dosyasının satırlar boyunca sözcük durumudur: .tr ve .go dosyalarında yorum ve
// metin sabitlerinin içinde olup olmadığı. Belge (.md) ve veri (.json) dosyalarında durum yoktur.
type lexState struct {
	ext     string
	inBlock bool // /* ... */ (yalnızca .go)
	inRaw   bool // `...` (yalnızca .go)
}

// line bir satırın her kod noktasının bulunduğu yeri (kod, yorum, metin) verir.
func (s *lexState) line(runes []rune) []string {
	where := make([]string, len(runes))
	base := WhereDoc
	switch s.ext {
	case ".json":
		base = WhereData
	case ".tr", ".go":
		base = WhereCode
	}
	if s.ext != ".tr" && s.ext != ".go" {
		for i := range where {
			where[i] = base
		}
		return where
	}
	inStr, comment := false, false
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		cur := base
		switch {
		case comment || s.inBlock:
			cur = WhereComment
			if s.inBlock && c == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				where[i], where[i+1] = cur, cur
				i++
				s.inBlock = false
				continue
			}
		case s.inRaw:
			cur = WhereString
			if c == '`' {
				s.inRaw = false
			}
		case inStr:
			cur = WhereString
			if c == '\\' && i+1 < len(runes) {
				where[i] = cur
				i++
				cur = WhereString
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr, cur = true, WhereString
		case s.ext == ".go" && c == '`':
			s.inRaw, cur = true, WhereString
		case c == '/' && i+1 < len(runes) && runes[i+1] == '/':
			comment, cur = true, WhereComment
		case s.ext == ".go" && c == '/' && i+1 < len(runes) && runes[i+1] == '*':
			s.inBlock, cur = true, WhereComment
		}
		where[i] = cur
	}
	return where
}

// scanFile tek dosyanın eşleşmeleridir.
func scanFile(rel, ext, text string, m *wordMatcher, exclude map[string]bool) ([]TextHit, int) {
	var hits []TextHit
	other := 0
	state := &lexState{ext: ext}
	for li, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		runes := []rune(line)
		where := state.line(runes) // durum her satırda ilerler (blok yorum ve ham metin)
		if !strings.Contains(fold(line), m.word) {
			continue
		}
		for i := 0; i < len(runes); i++ {
			if !wordRune(runes[i]) {
				continue
			}
			j := i
			for j < len(runes) && wordRune(runes[j]) {
				j++
			}
			token := string(runes[i:j])
			match, suggestion, check, isOther := m.classify(token)
			switch {
			case match != "" && !exclude[rel+":"+strconv.Itoa(li+1)+":"+strconv.Itoa(i+1)]:
				hits = append(hits, TextHit{Pos: Pos{File: rel, Line: li + 1, Col: i + 1, EndLine: li + 1, EndCol: j + 1},
					Where: where[i], Match: match, Word: token, Context: clip(line), Suggestion: suggestion, Check: check})
			case isOther:
				other++
			}
			i = j - 1
		}
	}
	return hits, other
}

func clip(line string) string {
	line = strings.TrimSpace(line)
	if r := []rune(line); len(r) > maxContext {
		return string(r[:maxContext]) + "…"
	}
	return line
}

// gitTracked `.git` varsa ve git çalışıyorsa izlenen dosyalardır (git ls-files); aksi halde ok=false:
// çağıran klasörü dolaşır.
func gitTracked(root string) ([]string, bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil, false
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return nil, false
	}
	var rels []string
	for _, name := range bytes.Split(out, []byte{0}) {
		if len(name) > 0 {
			rels = append(rels, string(name))
		}
	}
	return rels, true
}
