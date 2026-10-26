package codeindex

import (
	"bytes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

// textFiles yeniden adlandırma taramasının dosyalarıdır: yalnızca izlenenler (izlenmeyen dosyalar kullanıcınındır; git
// yoksa klasör gezintisi), ListFiles'ın kuralıyla.
func textFiles(root string) []string {
	return ListFiles(root, nil, DefaultSkipDirs, TrackedFiles, func(rel string) bool {
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".md", ".json", ".tr", ".go":
			return true
		}
		return false
	})
}

// diskBody bir .tr dosyasının gövdesinin diskteki metnidir: kodlu dosyada başlık satırının yerine boş satır, böylece
// satır numaraları dosyadaki gerçek satırdır (read.go ile aynı); bölünemeyen dosya olduğu gibi.
func diskBody(text string) string {
	b, err := source.Split(text)
	switch {
	case err != nil:
		return text
	case b.Coded:
		return "\n" + b.Body
	}
	return b.Body
}

// wordMatcher aranan sözcüğü ve ekli biçimlerini tanır.
type wordMatcher struct {
	word     string // Türkçe katlanmış
	suffix   map[string]bool
	derived  map[string]bool
	repl     string // yeni sözcük
	soft     string // aranan sözcüğün ünsüzü yumuşamış kökü (çekirdek -> çekirdeğ); yoksa ""
	replSoft string // yeni sözcüğün yumuşamış kökü (yeni sözcük de yumuşuyorsa)
	harmOK   bool   // yeni sözcüğün ünlü uyumu eskisiyle aynı
}

func newWordMatcher(word, repl string) *wordMatcher {
	m := &wordMatcher{word: Fold(word), repl: repl, suffix: suffixTails(Fold(word)), derived: derivedTails(Fold(word))}
	oldH, _ := lastHarmony(Fold(word))
	newH, ok := lastHarmony(Fold(repl))
	m.harmOK = ok && oldH == newH
	m.soft, _ = soften(m.word)
	m.replSoft, _ = soften(Fold(repl))
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
	t := Fold(token)
	if t == m.word {
		return MatchExact, m.repl, "", false
	}
	if strings.Contains(t, "_") {
		parts := strings.Split(token, "_")
		for i, p := range parts {
			if Fold(p) == m.word {
				parts[i] = m.repl
				return MatchCompound, strings.Join(parts, "_"), "", false
			}
		}
	}
	if m.soft != "" {
		if tail, ok := strings.CutPrefix(t, m.soft); ok && vowelStart(tail) && m.suffix[tail] {
			repl := m.repl
			if m.replSoft != "" {
				repl = m.replSoft // çekirdeği -> kernel'e uymaz: yeni sözcük de yumuşuyorsa onun yumuşak kökü
			}
			chk := ""
			if !m.harmOK {
				chk = "vowel_harmony"
			}
			return MatchSuffixed, repl + tail, chk, false
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
			text = diskBody(text)
		}
		h, o := scanFile(rel, ext, text, m, exclude)
		for _, hit := range h {
			// Go kodundaki adlar İngilizcedir; Go'da yalnızca yorum ve metin sabiti aranır.
			if ext != ".go" || hit.Where != WhereCode {
				hits = append(hits, hit)
			}
		}
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
		if folded := Fold(line); !strings.Contains(folded, m.word) && (m.soft == "" || !strings.Contains(folded, m.soft)) {
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

// FileSet ListFiles'ın iki kipidir: hangi dosyaların sayıldığı aracın kime hizmet ettiğine bağlıdır.
type FileSet int

const (
	// TrackedFiles depo araçlarınındır (codegraph, renumber, yeniden adlandırmanın metin taraması): git'in izlediği
	// dosyalar; git yoksa klasör gezintisi. Depo kökündeki izlenmeyen dosyalar kullanıcınındır, grafiğe ve yazılan
	// plana girmez.
	TrackedFiles FileSet = iota
	// AllFiles kullanıcı aracınındır (tr dizin; ileride dil sunucusu): kökün altındaki her dosya, .gitignore'a
	// bakılmadan; scratch/ gibi yok sayılan bir klasörde yazılan .tr da dizine girer (örnekler denemeleri orada
	// yapmayı öğütler).
	AllFiles
)

// ListFiles araçların tek dosya listesidir (dizin, kod grafiği, madde taraması; bağımsız denetim T1, Y11): köke göre
// yollar, "/" ile, sıralı; set kipi (TrackedFiles, AllFiles) hangi dosyaların sayıldığını seçer. İki kipte de skip'teki
// adlı bir klasörün (testdata, .claude ...) altı atlanır; dirs boş değilse yalnızca onların altı; keep nil değilse
// süzgeçtir.
func ListFiles(root string, dirs []string, skip map[string]bool, set FileSet, keep func(rel string) bool) []string {
	rels, ok := []string(nil), false
	if set == TrackedFiles {
		rels, ok = GitTracked(root)
	}
	if !ok {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if p != root && skip[d.Name()] {
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
		if underSkipped(r, skip) || !underAnyDir(r, dirs) || keep != nil && !keep(r) {
			continue
		}
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// underSkipped yolun klasörlerinden biri skip'te mi.
func underSkipped(rel string, skip map[string]bool) bool {
	parts := strings.Split(rel, "/")
	for _, part := range parts[:len(parts)-1] {
		if skip[part] {
			return true
		}
	}
	return false
}

// underAnyDir yol dirs'ten birinin altında mı (dirs boşsa ya da "." içeriyorsa hep).
func underAnyDir(rel string, dirs []string) bool {
	if len(dirs) == 0 {
		return true
	}
	for _, d := range dirs {
		if d == "." || rel == d || strings.HasPrefix(rel, strings.TrimSuffix(d, "/")+"/") {
			return true
		}
	}
	return false
}

// GitTracked `.git` varsa ve git çalışıyorsa izlenen dosyalardır (git ls-files); aksi halde ok=false.
func GitTracked(root string) ([]string, bool) { return gitFiles(root) }

// gitFiles `.git` varsa ve git çalışıyorsa `git ls-files args...`'ın listelediği dosyalardır; aksi halde ok=false.
func gitFiles(root string, args ...string) ([]string, bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil, false
	}
	out, err := exec.Command("git", append([]string{"-C", root, "ls-files", "-z"}, args...)...).Output()
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

// markIgnored dizindeki dosyalardan git'in yok saydıklarını Index.Ignored'a yazar (kullanıcı aracında: AllFiles; depo
// aracı yalnız izlenenleri okur, yok sayılanı yoktur). Kurulumdan ve önbellekten okumadan sonra sorulur: .gitignore
// değişince dizin kendiliğinden doğru kalır.
func markIgnored(root string, idx *Index, opts Options) {
	idx.Ignored = nil
	if opts.Tracked {
		return
	}
	ignored, ok := gitFiles(root, "--others", "--ignored", "--exclude-standard")
	if !ok {
		return
	}
	set := map[string]bool{}
	for _, rel := range ignored {
		set[rel] = true
	}
	for _, rel := range idx.Files {
		if set[rel] {
			idx.Ignored = append(idx.Ignored, rel)
		}
	}
}
