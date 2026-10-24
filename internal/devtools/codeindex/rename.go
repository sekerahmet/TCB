package codeindex

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Bu dosya yeniden adlandırma etkisidir (LSP textDocument/rename ve prepareRename'in karşılığı):
// bir adın değişirse değişecek yerleri listeler, hiçbir dosyaya yazmaz. Sonuç iki bölümdür: kesin
// (tanım, parametre, adlı argüman, kendi.x ve yapıcıdan bilinen yerelin üye erişimleri, komut_satırı
// anahtarları) ve olası (alıcısı bilinmeyen üye erişimleri, gölgelenen parametreler, belge, yorum,
// metin, ileti tabloları). Gerekçe ve kaynaklar README'dedir.

// Reddetme nedenleri (Result.Reason): değişmez tanımlayıcılar.
const (
	ReasonBuiltin      = "builtin"            // yerleşik ad
	ReasonModule       = "module"             // modül adı (ilk sürümde yok)
	ReasonNoSymbol     = "no_symbol"          // konumda ad yok
	ReasonInvalidName  = "invalid_identifier" // yeni ad geçerli bir tanımlayıcı değil
	ReasonLanguageWord = "language_word"      // yeni ad dilin sözcüğü
	ReasonSameName     = "same_name"          // yeni ad eskisiyle aynı
	ReasonAmbiguous    = "ambiguous_name"     // ad birden çok bildirime uyuyor; Candidates adaylardır
)

// Conflict yeni adın aynı kapsamda zaten bulunduğu bir bildirimdir.
type Conflict struct {
	Target string `json:"target"` // adı değişecek düğümün kimliği
	With   Node   `json:"with"`   // aynı kapsamdaki aynı adlı bildirim
}

// Change bir yeniden adlandırma yeridir. Dizinden gelenlerde Node/Role/Confidence/Basis, metin
// taramasından gelenlerde Match/Word dolu olur.
type Change struct {
	Pos         Pos        `json:"pos"`
	Node        string     `json:"node,omitempty"`
	Role        Role       `json:"role,omitempty"`
	Confidence  Confidence `json:"confidence,omitempty"`
	Basis       Basis      `json:"basis,omitempty"`
	Where       string     `json:"where,omitempty"`
	Match       string     `json:"match,omitempty"`
	Word        string     `json:"word,omitempty"`
	Context     string     `json:"context,omitempty"`
	Replacement string     `json:"replacement,omitempty"`
	Check       string     `json:"check,omitempty"`
}

// Dizin kaynaklı olası yerlerin Match değerleri.
const (
	MatchMemberGuess = "member_guess" // alıcı tipi bilinmeyen üye erişimi: aynı adlı üyelerin hepsi aday
	MatchShadowed    = "shadowed"     // gövdede yeniden bağlanan parametre adı
)

// RenameOptions yeniden adlandırma etkisinin seçenekleridir.
type RenameOptions struct {
	Root   string // proje kökü (satır metni ve metin taraması için); boşsa ikisi de yok
	Term   bool   // terim kipi: sembol aranmaz, yalnızca metin eşleşmeleri listelenir
	NoText bool   // metin taramasını (olası belge/yorum/ileti bölümünü) kapat
	Limit  int    // her listenin en çok satırı (0: hepsi); toplam sayı ve Truncated korunur
}

var identifierPattern = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*$`)

// renamable adı değiştirilebilen bir düğüm mü.
func renamable(n *Node) bool {
	return n.Kind != KindBuiltin && n.Kind != KindModule && n.Kind != KindTest
}

// shortName düğümün kendi adıdır ("Yapı.metot.ad" -> "ad").
func shortName(n *Node) string {
	if n.Kind == KindField || n.Kind == KindMethod || n.Kind == KindParameter {
		return n.Name[strings.LastIndex(n.Name, ".")+1:]
	}
	return n.Name
}

// renameTargets sorgunun adını değiştirilecek düğümlere çözer: Resolve'un bulduklarından yeniden adlandırılabilenler.
// Çıplak ad o adı taşıyan bütün bildirimlerdir (alan, parametre, işlev, seçenek ...: terim değişince hepsi değişir);
// nitelenmiş ad, kimlik ya da konum tek bildirimi seçer. Ad çözümü tek yerdedir (Resolve).
func (r *Reader) renameTargets(q string) (targets []*Node, rejected string) {
	for _, n := range r.Resolve(q, "") {
		switch {
		case renamable(n):
			targets = append(targets, n)
		case n.Kind == KindBuiltin:
			rejected = ReasonBuiltin
		case n.Kind == KindModule:
			rejected = ReasonModule
		}
	}
	if len(targets) > 0 {
		return targets, ""
	}
	return nil, rejected
}

// validateNewName yeni adı denetler; reddedilecekse nedenini verir.
func validateNewName(old, name string) string {
	switch {
	case !identifierPattern.MatchString(name):
		return ReasonInvalidName
	case Fold(name) == Fold(old):
		return ReasonSameName
	}
	for _, code := range codes.Languages() {
		l, err := codes.LoadLanguage(code)
		if err != nil {
			continue
		}
		for _, ns := range []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType} {
			if _, ok := l.Find(ns, name); ok {
				return ReasonLanguageWord
			}
		}
	}
	return ""
}

// RenameImpact q adının newName olursa değişecek yerleri listeler (hiçbir şeyi değiştirmez).
func (r *Reader) RenameImpact(q, newName string, o RenameOptions) *Result {
	res := NewResult(CommandRename, Query{Text: q, Language: codes.Display().Code})
	res.NewName = newName
	if o.Term {
		return r.renameTerm(res, q, newName, o)
	}
	targets, rejected := r.renameTargets(q)
	if len(targets) == 0 {
		if rejected != "" {
			res.Status, res.Reason = StatusRejected, rejected
			return res
		}
		if o.Root != "" && !o.NoText && !strings.ContainsAny(q, ".:/#") {
			return r.renameTerm(res, q, newName, o) // sembol yok: terim olarak yalnızca metin
		}
		res.Status = StatusNotFound
		res.DidYouMean = r.suggestNodes(q)
		return res
	}
	if len(targets) > 1 {
		// Çıplak ad birden çok anlamda: hangisinin kastedildiği sessizce seçilmez (çıkış 2).
		return r.ambiguousRename(res, targets)
	}
	words := map[string]string{} // katlanmış sözcük -> yazıldığı gibi
	for _, n := range targets {
		words[Fold(shortName(n))] = shortName(n)
	}
	for _, w := range words {
		if reason := validateNewName(w, newName); reason != "" {
			res.Status, res.Reason = StatusRejected, reason
			return res
		}
	}
	for _, n := range targets {
		res.Targets = append(res.Targets, r.display(*n))
		res.Conflicts = append(res.Conflicts, r.conflictsOf(n, newName)...)
	}
	lines := &lineSource{root: o.Root}
	skip := map[string]bool{}
	r.addLinked(res, targets, skip)
	for _, n := range targets {
		for _, oc := range r.Occurrences(n.ID) {
			ch := Change{Pos: oc.Pos, Node: n.ID, Role: oc.Role, Confidence: oc.Confidence, Basis: oc.Basis,
				Replacement: newName, Context: lines.at(oc.Pos)}
			skip[oc.Pos.File+":"+strconv.Itoa(oc.Pos.Line)+":"+strconv.Itoa(oc.Pos.Col)] = true
			switch {
			case oc.Basis == BasisNameOnly || oc.Basis == BasisNameMatch:
				ch.Where, ch.Match = WhereCode, MatchMemberGuess
				res.Likely = append(res.Likely, ch)
			case oc.Confidence == Inferred && oc.Basis != BasisLocalType:
				ch.Where, ch.Match = WhereCode, MatchShadowed
				res.Likely = append(res.Likely, ch)
			default:
				res.Certain = append(res.Certain, ch)
			}
		}
	}
	if o.Root != "" && !o.NoText {
		for _, w := range sortedValues(words) {
			hits, other := ScanText(o.Root, w, newName, skip)
			res.SkippedWords += other
			for _, h := range hits {
				res.Likely = append(res.Likely, textChange(h, lines))
			}
		}
	}
	sortChanges(res.Certain)
	sortChanges(res.Likely)
	res.Counts = &Counts{Total: len(res.Certain) + len(res.Likely), Certain: len(res.Certain), Likely: len(res.Likely)}
	if o.Limit > 0 {
		var cut1, cut2 bool
		res.Certain, cut1 = limit(res.Certain, o.Limit)
		res.Likely, cut2 = limit(res.Likely, o.Limit)
		res.Truncated = cut1 || cut2
	}
	return res
}

func sortedValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func sortChanges(list []Change) {
	sort.SliceStable(list, func(i, j int) bool { return posLess(&list[i].Pos, &list[j].Pos) })
}

// suggestNodes bulunamayan ad için yakın bildirimler.
func (r *Reader) suggestNodes(q string) []Node {
	var out []Node
	for _, n := range r.Suggest(q, 5) {
		out = append(out, r.display(*n))
	}
	return out
}

// conflictsOf yeni adın n'nin kapsamında zaten bulunduğu bildirimlerdir: yapının alanı ve metodu, işlevin
// parametresi, dosyanın üst düzey adı.
func (r *Reader) conflictsOf(n *Node, newName string) []Conflict {
	if n.Pos == nil || n.Kind == KindOption {
		return nil
	}
	file, prefix := n.Pos.File, ""
	var kinds []Kind
	switch n.Kind {
	case KindField, KindMethod:
		prefix = n.Name[:strings.LastIndex(n.Name, ".")+1]
		kinds = []Kind{KindField, KindMethod}
	case KindParameter:
		prefix = n.Name[:strings.LastIndex(n.Name, ".")+1]
		kinds = []Kind{KindParameter}
	default:
		kinds = []Kind{KindFunction, KindStruct, KindConstant, KindVariable}
	}
	var out []Conflict
	for _, k := range kinds {
		if c := r.byID[NodeID(k, file, prefix+newName)]; c != nil && c.ID != n.ID {
			out = append(out, Conflict{Target: n.ID, With: r.display(*c)})
		}
	}
	return out
}

// PrepareResult prepareRename'in cevabıdır: değişecek adın aralığı ve önerilen yer tutucu.
type PrepareResult struct {
	Range       Pos    `json:"range"`
	Placeholder string `json:"placeholder"`
	Node        Node   `json:"node"`
}

// PrepareRename konumdaki adın yeniden adlandırılıp adlandırılamayacağını söyler: adlandırılabiliyorsa
// aralığı ve yer tutucuyu verir, aksi halde nedeni (Reason* değerlerinden biri).
func (r *Reader) PrepareRename(file string, line, col int) (*PrepareResult, string) {
	oc, n := r.At(file, line, col)
	switch {
	case oc == nil || n == nil:
		return nil, ReasonNoSymbol
	case n.Kind == KindBuiltin:
		return nil, ReasonBuiltin
	case !renamable(n):
		return nil, ReasonModule
	}
	return &PrepareResult{Range: oc.Pos, Placeholder: shortName(n), Node: r.display(*n)}, ""
}

// lineSource değişiklik satırlarının metnini diskten okur (Türkçe gösterim satırları); .tr dosyasını sözdizimi kararları
// (sözlük anahtarı) için bir kez ayrıştırır.
type lineSource struct {
	root   string
	cache  map[string][]string
	parsed map[string]*parsed
}

// read dosyanın ham metnidir (okunamazsa ya da gömülü kitaplıktaysa ok false).
func (s *lineSource) read(file string) (string, bool) {
	if s.root == "" || strings.HasPrefix(file, libraryPrefix) {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(file)))
	return string(data), err == nil
}

func (s *lineSource) at(p Pos) string {
	if s.cache == nil {
		s.cache = map[string][]string{}
	}
	lines, ok := s.cache[p.File]
	if !ok {
		if text, ok := s.read(p.File); ok {
			lines = strings.Split(displayText(text), "\n")
		}
		s.cache[p.File] = lines
	}
	if p.Line < 1 || p.Line > len(lines) {
		return ""
	}
	return clip(strings.TrimRight(lines[p.Line-1], "\r"))
}

// dictKey p'deki ad tırnaksız bir sözlük anahtarı mı (parsed.dictKey).
func (s *lineSource) dictKey(p Pos) bool {
	if s.parsed == nil {
		s.parsed = map[string]*parsed{}
	}
	res, ok := s.parsed[p.File]
	if !ok {
		if text, ok := s.read(p.File); ok {
			res = readFile(text)
		}
		s.parsed[p.File] = res
	}
	return res.dictKey(p.Line, p.Col)
}

// renameTerm terim kipidir: sembol aranmaz, q sözcüğü (ve ekli biçimleri) yalnızca metinde aranır.
// Bir terimin birden çok anlamı olabilir (GPU "çekirdek" yazılım kernel'ı ve donanım çekirdeği); araç
// anlamları ayıramaz, her eşleşme satırın metniyle verilir ve seçim elle yapılır.
func (r *Reader) renameTerm(res *Result, q, newName string, o RenameOptions) *Result {
	res.Mode = ModeTerm
	switch {
	case !identifierPattern.MatchString(newName):
		res.Status, res.Reason = StatusRejected, ReasonInvalidName
		return res
	case Fold(newName) == Fold(q):
		res.Status, res.Reason = StatusRejected, ReasonSameName
		return res
	}
	if o.Root != "" && !o.NoText {
		hits, other := ScanText(o.Root, q, newName, nil)
		res.SkippedWords = other
		for _, h := range hits {
			res.Likely = append(res.Likely, Change{Pos: h.Pos, Where: h.Where, Match: h.Match, Word: h.Word,
				Context: h.Context, Replacement: h.Suggestion, Check: h.Check})
		}
	}
	res.Counts = &Counts{Total: len(res.Likely)}
	if o.Limit > 0 {
		res.Likely, res.Truncated = limit(res.Likely, o.Limit)
	}
	return res
}

// ambiguousRename birden çok bildirime uyan çıplak adı reddeder ve adayları listeler.
func (r *Reader) ambiguousRename(res *Result, targets []*Node) *Result {
	res.Status, res.Reason = StatusRejected, ReasonAmbiguous
	for _, n := range targets {
		res.Candidates = append(res.Candidates, r.display(*n))
	}
	return res
}
