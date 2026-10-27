package lexer

import (
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kimliğin yeri (dil tanımı 2.3a). Dilin sözcükleri TR_ ve kimliğin adıyla yazılır (TR_if,
// TR_print(x), x.TR_map(f), TR_lr: 0.003); sınıfı sözcüğün yeri belirler (codes.Place):
//
//   - noktadan ('.', '?.') sonra ve yapı gövdesindeki alan ve metot tanımında üye;
//   - parantez içinde ardından tek ':' gelen ad (adlı argüman, tipli parametre) ve fonksiyonun
//     parametre listesindeki ad argüman; süslü parantez içinde ardından ':' gelen ad (sözlük
//     değişmezinin anahtarı) üye;
//   - öbür yerlerde anahtar kelime, bağlamsal sözcük, değer, tip, fonksiyon ya da sabit. Kitaplık
//     fonksiyonunun gövdesinde parametresi aynı adlı yerleşiği gölgeler (sözcüksel kapsam).
//
// Yer, sözcük çözümleyicinin önceki sözcük birimlerinden ve açık parantezlerden izlediği bağlamdan
// çıkar (track); kod sözcük biriminde gelir, ayrıştırıcı ve çevirici aynı kodu görür.

// bracket açık bir parantezdir.
type bracket struct {
	open       TokenType
	params     bool // fonksiyonun parametre listesi: fonksiyon [ad] (
	structBody bool // yapının gövdesi: yapı Ad {
	// scope fonksiyon gövdesinde (ve parametre listesinde) TR_ ile yazılmış parametrelerdir:
	// yazım → argüman kodu.
	scope map[string]codes.Code
}

// placeState kimliğin yerini bulmak için izlenen bağlamdır.
type placeState struct {
	stack      []bracket
	prev       TokenType // son sözcük biriminin türü
	prev2      TokenType // ondan önceki
	prevLine   int
	paramsLine int // parametre listesinin kapandığı satır (pending ancak o satırda gövdeye bağlanır)
	pending    map[string]codes.Code
	// base yerleştirme alt çözümleyicisinde metnin bulunduğu yerin kapsamıdır.
	base []map[string]codes.Code
	// aliases takma adla aktarılan modüllerin adlarıdır (TR_import "tr/katman" TR_as katman):
	// katman.TR_GPT kitaplığın tipidir, üye değil. importStep aktarma deyiminin okunan sözcüğüdür
	// (1: TR_import, 2: yol, 3: TR_as); prevName son adın metni, afterAlias noktanın bir takma addan
	// sonra geldiğidir.
	aliases    map[string]bool
	importStep int
	prevName   string
	afterAlias bool
}

// track okunan sözcük birimine göre bağlamı günceller.
func (l *Lexer) track(tok Token) {
	s := &l.places
	if s.pending != nil && tok.Line != s.paramsLine {
		s.pending = nil
	}
	switch tok.Type {
	case TokLParen:
		b := bracket{open: TokLParen}
		if s.prev == TokFunction || s.prev == TokIdent && s.prev2 == TokFunction {
			b.params, b.scope = true, map[string]codes.Code{}
		}
		s.stack = append(s.stack, b)
	case TokLBracket:
		s.stack = append(s.stack, bracket{open: TokLBracket})
	case TokLBrace:
		b := bracket{open: TokLBrace, structBody: s.prev == TokIdent && s.prev2 == TokStruct}
		if s.pending != nil {
			b.scope, s.pending = s.pending, nil
		}
		s.stack = append(s.stack, b)
	case TokRParen, TokRBracket, TokRBrace:
		if n := len(s.stack); n > 0 {
			top := s.stack[n-1]
			s.stack = s.stack[:n-1]
			if top.params {
				s.pending, s.paramsLine = top.scope, tok.Line
			}
		}
	}
	l.trackAlias(tok)
	s.prev2, s.prev, s.prevLine = s.prev, tok.Type, tok.Line
}

// trackAlias aktarma deyimlerinin takma adlarını toplar ve noktanın bir takma addan sonra gelip
// gelmediğini tutar.
func (l *Lexer) trackAlias(tok Token) {
	s := &l.places
	isAs := tok.Type == TokIdent && (tok.Code == codes.CtxAs || tok.Literal == l.Language().Name(codes.CtxAs))
	switch {
	case tok.Type == TokImport:
		s.importStep = 1
	case s.importStep == 1 && tok.Type == TokString:
		s.importStep = 2
	case s.importStep == 2 && isAs:
		s.importStep = 3
	case s.importStep == 3 && tok.Type == TokIdent:
		if s.aliases == nil {
			s.aliases = map[string]bool{}
		}
		s.aliases[tok.Literal] = true
		s.importStep = 0
	default:
		s.importStep = 0
	}
	if tok.Type == TokDot {
		s.afterAlias = s.prev == TokIdent && s.aliases[s.prevName]
	}
	s.prevName = ""
	if tok.Type == TokIdent {
		s.prevName = tok.Literal
	}
}

// top en içteki açık parantezdir (yoksa boş).
func (s *placeState) top() bracket {
	if n := len(s.stack); n > 0 {
		return s.stack[n-1]
	}
	return bracket{}
}

// colonAhead adın ardından (aynı satırda, boşluktan sonra) tek ':' geliyor mu (':=' değil)?
func (l *Lexer) colonAhead() bool {
	i := l.position
	for i < len(l.input) && (l.input[i] == ' ' || l.input[i] == '\t') {
		i++
	}
	return i < len(l.input) && l.input[i] == ':' && (i+1 >= len(l.input) || l.input[i+1] != '=')
}

// placeOf okunan adın yeridir; fallback doğruysa ad o yerde bulunmazsa ikinci yere bakılır (yapı
// gövdesinde üye bulunmazsa öbür yer: TR_func, TR_init; takma adlı modülde öbür yer bulunmazsa üye).
func (l *Lexer) placeOf(line int) (place, second codes.Place, fallback bool) {
	s := &l.places
	top := s.top()
	switch {
	case s.prev == TokDot && s.afterAlias:
		// Takma adlı modülün adı: kitaplığın tipi, fonksiyonu ya da sabiti (öbür yer); bulunmazsa üye.
		return codes.PlaceOther, codes.PlaceMember, true
	case s.prev == TokDot || s.prev == TokSafeDot:
		return codes.PlaceMember, 0, false
	case top.params && (s.prev == TokLParen || s.prev == TokComma):
		return codes.PlaceArg, 0, false
	case top.structBody && (s.prev == TokFunction || s.prev == TokLBrace || s.prev == TokComma ||
		s.prev == TokSemicolon || s.prev == TokVar || s.prevLine < line):
		// Yapı gövdesinde alan ve metot adı üyedir; aynı yerde TR_func ve TR_var da yazılır,
		// TR_init bağlamsal sözcüktür: üye bulunmazsa öbür yere bakılır.
		return codes.PlaceMember, codes.PlaceOther, true
	case top.open == TokLParen && l.colonAhead():
		return codes.PlaceArg, 0, false
	case top.open == TokLBrace && l.colonAhead():
		return codes.PlaceMember, 0, false
	}
	return codes.PlaceOther, 0, false
}

// identity TR_ ile başlayan sözcüğü bulunduğu yerin koduna çözer; çözemezse hatayı (önerisiyle)
// kaydeder ve codes.None döner.
func (l *Lexer) identity(lit string, line, col int) codes.Code {
	place, second, fallback := l.placeOf(line)
	if place == codes.PlaceOther {
		if k, ok := l.paramInScope(lit); ok {
			return k
		}
	}
	if k, ok := codes.FromSpelling(place, lit); ok {
		l.declareParam(place, lit, k)
		return k
	}
	if fallback {
		if k, ok := codes.FromSpelling(second, lit); ok {
			return k
		}
	}
	switch l.places.prev {
	case TokVar, TokConst, TokFunction, TokStruct:
		// Tanımlanan ad TR_ ile başlıyor: kullanıcı adı olamaz (2.3a), ileti başka bir ad önerir. Önek
		// ASCII'dir; küçük harfli öneri strings.ToLower ile üretilir (TR_x → tr_x).
		l.addError(line, col, messages.New(messages.H0004, lit, strings.ToLower(lit)))
		return codes.None
	}
	l.addError(line, col, identityError(place, lit))
	return codes.None
}

// declareParam parametre listesindeki kimliği fonksiyonun kapsamına yazar.
func (l *Lexer) declareParam(place codes.Place, lit string, k codes.Code) {
	if top := l.places.top(); place == codes.PlaceArg && top.params {
		top.scope[lit] = k
	}
}

// paramInScope ad çevreleyen bir fonksiyonun TR_ ile yazılmış parametresi mi (en içten başlayarak)?
func (l *Lexer) paramInScope(lit string) (codes.Code, bool) {
	s := &l.places
	for i := len(s.stack) - 1; i >= 0; i-- {
		if k, ok := s.stack[i].scope[lit]; ok {
			return k, true
		}
	}
	for i := len(s.base) - 1; i >= 0; i-- {
		if k, ok := s.base[i][lit]; ok {
			return k, true
		}
	}
	return codes.None, false
}

// scopes yerleştirme içeren metnin bulunduğu yerin kapsamlarıdır (alt çözümleyiciye geçer).
func (l *Lexer) scopes() []map[string]codes.Code {
	out := append([]map[string]codes.Code(nil), l.places.base...)
	for _, b := range l.places.stack {
		if b.scope != nil {
			out = append(out, b.scope)
		}
	}
	return out
}

// removedWords sözlükten kalkan sözcüklerdir: başka dillerden alışkanlıkla yazılırsa ileti TR'deki
// yazımı söyler (2.9).
var removedWords = map[string]messages.Code{
	codes.IdentityPrefix + "while":   messages.H0021,
	codes.IdentityPrefix + "default": messages.H0022,
}

// identityError place'te geçerli olmayan lit için iletidir: kalkmış sözcükse TR'deki yazımı, başka
// bir yerde geçerliyse doğru yeri, değilse bu yerde yazımca en yakın kimliği önerir.
func identityError(place codes.Place, lit string) error {
	switch removedWords[lit] {
	case messages.H0021:
		return messages.New(messages.H0021, lit, codes.Spelling(codes.KwFor))
	case messages.H0022:
		return messages.New(messages.H0022, lit)
	}
	for _, other := range []codes.Place{codes.PlaceOther, codes.PlaceMember, codes.PlaceArg} {
		if other == place {
			continue
		}
		if k, ok := codes.FromSpelling(other, lit); ok {
			return wrongPlace(other, k)
		}
	}
	if s, ok := nearestSpelling(place, lit); ok {
		return messages.New(messages.H0026, lit, s)
	}
	// Başka bir dilin tanıdık adı TR_ ile yazılmış olabilir: TR_def → TR_func, TR_None → TR_none.
	if s, ok := codes.SuggestIdentity(lit[len(codes.IdentityPrefix):]); ok && place == codes.PlaceOther {
		return messages.New(messages.H0026, lit, s)
	}
	return messages.New(messages.H0001, lit)
}

// wrongPlace başka bir yerin (place) kodu olan k'nin doğru yazımını anlatan iletidir.
func wrongPlace(place codes.Place, k codes.Code) error {
	s := codes.Spelling(k)
	switch place {
	case codes.PlaceMember:
		return messages.New(messages.H0027, s, "x."+s)
	case codes.PlaceArg:
		return messages.New(messages.H0028, s, "f("+s+": ...)")
	}
	if k.Class() == codes.ClassFunction {
		return messages.New(messages.H0029, s, s+"(...)")
	}
	return messages.New(messages.H0029, s, s)
}

// nearestSpelling place'teki yazımlardan lit'e en yakın olandır (2.3a): uzaklık TR_'den sonraki
// adda sayılır, iki komşu harfin yer değiştirmesi tek değişikliktir (sınırlı Damerau-Levenshtein,
// Rust'ın derleyicisi gibi); adın uzunluğunun üçte birini (en az bir) aşan değişiklikte öneri
// yoktur. Eşit uzaklıkta alfabe sırasıyla ilki seçilir (ileti kararlı olsun).
func nearestSpelling(place codes.Place, lit string) (string, bool) {
	name := []rune(lit[len(codes.IdentityPrefix):])
	limit := max(1, len(name)/3)
	best, bestDist := "", limit+1
	for _, s := range codes.SpellingsAt(place) {
		cand := []rune(s[len(codes.IdentityPrefix):])
		if abs(len(cand)-len(name)) > limit {
			continue
		}
		d := editDistance(name, cand)
		if d < bestDist || d == bestDist && s < best {
			best, bestDist = s, d
		}
	}
	return best, best != ""
}

// editDistance a ile b arasındaki sınırlı Damerau-Levenshtein uzaklığıdır (ekleme, silme,
// değiştirme ve komşu iki harfin yer değiştirmesi birer değişiklik).
func editDistance(a, b []rune) int {
	prev2 := make([]int, len(b)+1)
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(b)]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// isIdentitySpelling sözcüğün kimlik yazımında olduğudur: TR_ ve en az bir karakter.
func isIdentitySpelling(lit string) bool {
	return codes.IsReservedName(lit) && utf8.RuneCountInString(lit) > len(codes.IdentityPrefix)
}

// codeToken k kodunun sözcük birimidir: metni kodun çözümleyicinin dilindeki adı (programda
// kimlik listesi; dil o kodu adlandırmıyorsa kimlik), anahtar kelimeyse türü.
func (l *Lexer) codeToken(k codes.Code, line, col int) Token {
	name := k.String()
	if g, ok := l.Language().Entry(k); ok {
		name = g.Name
	}
	if t, ok := codeTypes[k]; ok {
		return Token{Type: t, Literal: name, Line: line, Column: col, Code: k}
	}
	return Token{Type: TokIdent, Literal: name, Line: line, Column: col, Code: k}
}
