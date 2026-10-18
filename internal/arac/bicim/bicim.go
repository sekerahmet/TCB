// Package bicim TR kaynak kodunu standart biçime getirir ('tr biçimle').
//
// Biçimlendirici sözcük birimi (token) tabanlıdır: kaynak, yorumlar da dahil olmak üzere
// sözcük birimlerine ayrılır; yalnızca aralarındaki boşluklar ve girintiler yeniden
// yazılır. Sözcük birimlerinin kendisi (metin sabitleri bayt bayt) ve yorumların içeriği
// korunur. Satır yapısı da korunur; tek istisna, bir gövdenin '{' işaretinin ve
// 'değilse'/'yakala' sözcüklerinin önceki satıra alınmasıdır.
//
// Standart biçim: 4 boşluk girinti, ikili operatörlerin iki yanında ve virgülden sonra
// birer boşluk, '{' aynı satırda, satır sonunda boşluk yok, art arda en fazla bir boş
// satır ve dosya sonunda tek bir satır sonu.
//
// Güvenlik: Biçimlenmiş kod yeniden ayrıştırılır; sözcük birimi dizisi ya da AST dökümü
// (ast.Dokum) değişmişse sonuç reddedilir. Böylece biçimlendirici programın anlamını asla
// değiştirmez.
package bicim

import (
	"fmt"
	"reflect"
	"strings"
	"tr_dili/internal/dil/ast"
	"tr_dili/internal/dil/lexer"
	"tr_dili/internal/dil/parser"
)

const girintiBirimi = "    "

// tokYorum bir yorum öğesinin türüdür (sözcük birimi türleriyle karışmaz).
const tokYorum lexer.TokenType = "YORUM"

// oge biçimlendirilecek akıştaki bir öğedir: sözcük birimi ya da yorum.
type oge struct {
	tur     lexer.TokenType
	metin   string // kaynaktaki ham metin
	satirA  int    // öncesindeki boşlukta geçen satır sonu sayısı
	satir   int    // sözcük biriminin kaynak satırı (yorumlarda 0)
	sutun   int
	bitisik bool // kaynakta bir önceki öğeye boşluksuz bitişik mi ('->' tanımak için)
	tekli   bool // tekli (önek) operatör mü: -x, !x
	sozluk  bool // sözlük sabitinin '{' ya da '}' işareti mi (içine boşluk konmaz: {"a": 1})
}

// AnlamDegistiHatasi biçimlenmiş kodun anlamı özgün koddan farklı çıktığında döner.
// Bu bir biçimlendirici hatasıdır; dosya yazılmamalıdır.
type AnlamDegistiHatasi struct {
	Ayrinti string
}

func (e *AnlamDegistiHatasi) Error() string {
	return "biçimlendirme programın anlamını değiştirecekti, dosyaya dokunulmadı (" + e.Ayrinti + ")"
}

// SozdizimiHatasi kaynak ayrıştırılamadığında döner; hatalı kod biçimlenmez.
type SozdizimiHatasi struct {
	Hatalar []string
}

func (e *SozdizimiHatasi) Error() string {
	return "sözdizimi hatası olan kod biçimlenmez: " + e.Hatalar[0]
}

// Bicimle kaynağı standart biçime getirir. Kaynak ayrıştırılamıyorsa *SozdizimiHatasi,
// sonuç özgün kodla aynı anlama gelmiyorsa *AnlamDegistiHatasi döner.
func Bicimle(kaynak string) (string, error) {
	// Dosya başındaki UTF-8 BOM korunur (kodlama biçimlendiricinin işi değildir)
	bom := ""
	if strings.HasPrefix(kaynak, "\xef\xbb\xbf") {
		bom = "\xef\xbb\xbf"
		kaynak = kaynak[len(bom):]
	}
	// Satır sonları LF yazılır; metinlerin içindekiler de (sözcük çözümleyici çok satırlı
	// metindeki CRLF'yi zaten '\n' okur, anlam değişmez): karışık satır sonu kalmaz.
	kaynak = strings.ReplaceAll(kaynak, "\r\n", "\n")
	p := parser.New(lexer.New(kaynak))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return "", &SozdizimiHatasi{Hatalar: p.Errors()}
	}
	cikti := donustur(kaynak, suslular(prog))
	if err := AnlamAyniMi(kaynak, cikti); err != nil {
		return "", err
	}
	return bom + cikti, nil
}

// AnlamAyniMi iki kaynağın aynı sözcük birimi dizisine ve aynı AST'ye sahip olduğunu
// denetler; farklıysa *AnlamDegistiHatasi döner.
func AnlamAyniMi(once, sonra string) error {
	p1 := parser.New(lexer.New(once))
	prog1 := p1.ParseProgram()
	p2 := parser.New(lexer.New(sonra))
	prog2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 {
		return &AnlamDegistiHatasi{Ayrinti: "biçimlenmiş kod ayrıştırılamadı: " + p2.Errors()[0]}
	}
	t1, t2 := sozcukler(once), sozcukler(sonra)
	if len(t1) != len(t2) {
		return &AnlamDegistiHatasi{Ayrinti: "sözcük birimi sayısı değişti"}
	}
	for i := range t1 {
		if t1[i].Type != t2[i].Type || t1[i].Literal != t2[i].Literal {
			return &AnlamDegistiHatasi{Ayrinti: fmt.Sprintf("satır %d: %q sözcüğü değişti", t2[i].Line, t1[i].Literal)}
		}
	}
	if prog1.String() != prog2.String() {
		return &AnlamDegistiHatasi{Ayrinti: "AST farklı"}
	}
	return nil
}

func sozcukler(kaynak string) []lexer.Token {
	l := lexer.New(kaynak)
	var t []lexer.Token
	for {
		tok := l.NextToken()
		t = append(t, tok)
		if tok.Type == lexer.TOKEN_EOF {
			return t
		}
	}
}

type konum struct{ satir, sutun int }

// Süslü parantez türleri (bkz. suslular)
const (
	susluSozluk = 1 // sözlük sabiti: {"a": 1}
	susluBlok   = 2 // sondaki blok (o-lambda): liste.filtrele { o > 2 }
)

// suslular ifade içindeki '{' işaretlerinin konumlarını türleriyle toplar: sözlük sabitleri
// ve sondaki bloklar. Bunlar ifadenin parçasıdır; yeni satırdaki böyle bir '{' önceki satıra
// alınırsa anlam değişebilir. Listede olmayan her '{' bir gövde başlangıcıdır (eğer, döngü,
// fonksiyon, yapı, dene...) ve ayrıştırıcı onu satırdan bağımsız okur.
func suslular(prog *ast.Program) map[konum]int {
	sonuc := map[konum]int{}
	var gez func(v reflect.Value)
	gez = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Interface, reflect.Ptr:
			if !v.IsNil() {
				gez(v.Elem())
			}
		case reflect.Struct:
			if _, ok := v.Interface().(lexer.Token); ok {
				return
			}
			switch n := v.Addr().Interface().(type) {
			case *ast.DictLiteral:
				sonuc[konum{n.Token.Line, n.Token.Column}] = susluSozluk
			case *ast.FunctionLiteral:
				if n.Token.Type == lexer.TOKEN_LBRACE {
					sonuc[konum{n.Token.Line, n.Token.Column}] = susluBlok
				}
			}
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					gez(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				gez(v.Index(i))
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				gez(v.MapIndex(k))
			}
		}
	}
	gez(reflect.ValueOf(prog))
	return sonuc
}

// ogeler kaynağı sözcük birimleri ve yorumlardan oluşan bir akışa çevirir.
func ogeler(kaynak string) []oge {
	l := lexer.New(kaynak)
	girdi := l.Girdi()
	var sonuc []oge
	onceki := 0 // önceki öğenin bittiği indeks
	for {
		tok := l.NextToken()
		bas, son := l.SonAralik()
		if bas > len(girdi) {
			bas = len(girdi)
		}
		sonuc = append(sonuc, araliktakiYorumlar(girdi[onceki:bas])...)
		if tok.Type == lexer.TOKEN_EOF {
			// Dosya sonundaki boşluk satır sayısı için işaretçi öğe
			sonuc = append(sonuc, oge{tur: lexer.TOKEN_EOF, satirA: satirSayisi(girdi[onceki:bas])})
			return sonuc
		}
		o := oge{
			tur:     tok.Type,
			metin:   string(girdi[bas:son]),
			satirA:  satirSayisi(girdi[onceki:bas]),
			satir:   tok.Line,
			sutun:   tok.Column,
			bitisik: bas == onceki,
		}
		sonuc = append(sonuc, o)
		onceki = son
	}
}

// araliktakiYorumlar iki sözcük birimi arasındaki boşluk + yorum bölgesini ayrıştırır.
// Her yorum için, öncesindeki satır sonu sayısıyla birlikte bir öğe üretir. Bölgenin
// yorumlardan sonra kalan kısmının satır sayısını çağıran (satirSayisi ile) hesaplar.
func araliktakiYorumlar(r []rune) []oge {
	var sonuc []oge
	satirA := 0
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case c == '\n':
			satirA++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < len(r) && r[i+1] == '/':
			j := i
			for j < len(r) && r[j] != '\n' {
				j++
			}
			sonuc = append(sonuc, oge{tur: tokYorum, metin: strings.TrimRight(string(r[i:j]), " \t\r"), satirA: satirA})
			satirA = 0
			i = j
		case c == '/' && i+1 < len(r) && r[i+1] == '*':
			j := i + 2
			for j+1 < len(r) && !(r[j] == '*' && r[j+1] == '/') {
				j++
			}
			j += 2
			if j > len(r) {
				j = len(r)
			}
			satirlar := strings.Split(string(r[i:j]), "\n")
			for k := range satirlar {
				satirlar[k] = strings.TrimRight(satirlar[k], " \t\r")
			}
			sonuc = append(sonuc, oge{tur: tokYorum, metin: strings.Join(satirlar, "\n"), satirA: satirA})
			satirA = 0
			i = j
		default:
			i++
		}
	}
	return sonuc
}

// satirSayisi boşluk + yorum bölgesinde, son yorumdan sonra gelen satır sonlarını sayar.
func satirSayisi(r []rune) int {
	n := 0
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case c == '\n':
			n++
			i++
		case c == '/' && i+1 < len(r) && r[i+1] == '/':
			for i < len(r) && r[i] != '\n' {
				i++
			}
			n = 0
		case c == '/' && i+1 < len(r) && r[i+1] == '*':
			i += 2
			for i+1 < len(r) && !(r[i] == '*' && r[i+1] == '/') {
				i++
			}
			i += 2
			n = 0
		default:
			i++
		}
	}
	return n
}

// Operatör sınıfları
var ikiliOperatorler = map[lexer.TokenType]bool{
	lexer.TOKEN_PLUS: true, lexer.TOKEN_MINUS: true, lexer.TOKEN_ASTERISK: true, lexer.TOKEN_SLASH: true,
	lexer.TOKEN_PERCENT: true, lexer.TOKEN_POWER: true, lexer.TOKEN_MATMUL: true,
	lexer.TOKEN_EQ: true, lexer.TOKEN_NOT_EQ: true, lexer.TOKEN_LT: true, lexer.TOKEN_GT: true,
	lexer.TOKEN_LTE: true, lexer.TOKEN_GTE: true, lexer.TOKEN_AND: true, lexer.TOKEN_OR: true,
	lexer.TOKEN_PIPE: true, lexer.TOKEN_NULL_COALESC: true, lexer.TOKEN_ARROW: true,
	lexer.TOKEN_ASSIGN: true, lexer.TOKEN_WALRUS: true, lexer.TOKEN_PLUS_ASSIGN: true,
	lexer.TOKEN_MINUS_ASSIGN: true, lexer.TOKEN_MUL_ASSIGN: true, lexer.TOKEN_DIV_ASSIGN: true,
}

// Satır sonunda kalırsa sonraki satırı devam satırı yapan sözcükler
var devamEttirenSon = map[lexer.TokenType]bool{
	lexer.TOKEN_VE: true, lexer.TOKEN_VEYA: true, lexer.TOKEN_DOT: true, lexer.TOKEN_SAFE_DOT: true,
	lexer.TOKEN_COLON: true, lexer.TOKEN_RANGE: true, lexer.TOKEN_RANGE_INCL: true,
}

// Satır başında gelirse o satırı devam satırı yapan sözcükler ('-' bilerek yok: satır başında
// tekli eksi gibi görünür)
var devamEdenBas = map[lexer.TokenType]bool{
	lexer.TOKEN_PIPE: true, lexer.TOKEN_DOT: true, lexer.TOKEN_SAFE_DOT: true, lexer.TOKEN_NULL_COALESC: true,
	lexer.TOKEN_AND: true, lexer.TOKEN_OR: true, lexer.TOKEN_VE: true, lexer.TOKEN_VEYA: true,
	lexer.TOKEN_PLUS: true, lexer.TOKEN_ASTERISK: true, lexer.TOKEN_SLASH: true, lexer.TOKEN_PERCENT: true,
	lexer.TOKEN_POWER: true, lexer.TOKEN_MATMUL: true, lexer.TOKEN_EQ: true, lexer.TOKEN_NOT_EQ: true,
	lexer.TOKEN_LT: true, lexer.TOKEN_GT: true, lexer.TOKEN_LTE: true, lexer.TOKEN_GTE: true,
	lexer.TOKEN_RANGE: true, lexer.TOKEN_RANGE_INCL: true,
}

func acilis(t lexer.TokenType) bool {
	return t == lexer.TOKEN_LBRACE || t == lexer.TOKEN_LPAREN || t == lexer.TOKEN_LBRACKET
}

func kapanis(t lexer.TokenType) bool {
	return t == lexer.TOKEN_RBRACE || t == lexer.TOKEN_RPAREN || t == lexer.TOKEN_RBRACKET
}

// tekliMi '-' işaretinin önceki sözcüğe bakarak önek (tekli) olup olmadığını belirler.
func tekliMi(onceki *oge) bool {
	if onceki == nil {
		return true
	}
	t := onceki.tur
	switch {
	case t == lexer.TOKEN_RPAREN || t == lexer.TOKEN_RBRACKET || t == lexer.TOKEN_RBRACE:
		return false
	case t == lexer.TOKEN_IDENT || t == lexer.TOKEN_INT || t == lexer.TOKEN_FLOAT || t == lexer.TOKEN_STRING:
		return false
	case t == lexer.TOKEN_DOGRU || t == lexer.TOKEN_YANLIS || t == lexer.TOKEN_YOK || t == lexer.TOKEN_KENDI:
		return false
	}
	return true
}

// aradaBosluk aynı satırdaki iki komşu öğe arasında bir boşluk olup olmayacağını söyler.
func aradaBosluk(p, c *oge) bool {
	pt, ct := p.tur, c.tur
	switch {
	case pt == tokYorum || ct == tokYorum:
		return true
	case ct == lexer.TOKEN_COMMA || ct == lexer.TOKEN_SEMICOLON || ct == lexer.TOKEN_RPAREN || ct == lexer.TOKEN_RBRACKET:
		return false
	case pt == lexer.TOKEN_LPAREN || pt == lexer.TOKEN_LBRACKET:
		return false
	case pt == lexer.TOKEN_DOT || pt == lexer.TOKEN_SAFE_DOT || ct == lexer.TOKEN_DOT || ct == lexer.TOKEN_SAFE_DOT:
		return false
	case pt == lexer.TOKEN_LBRACE:
		return ct != lexer.TOKEN_RBRACE && !p.sozluk
	case ct == lexer.TOKEN_RBRACE:
		return !c.sozluk
	case p.tekli:
		return false
	// Dönüş tipi oku: fonksiyon f() -> Tam
	case pt == lexer.TOKEN_MINUS && ct == lexer.TOKEN_GT && c.bitisik:
		return false
	case ct == lexer.TOKEN_LPAREN || ct == lexer.TOKEN_LBRACKET:
		switch pt {
		case lexer.TOKEN_IDENT, lexer.TOKEN_RPAREN, lexer.TOKEN_RBRACKET, lexer.TOKEN_RBRACE,
			lexer.TOKEN_STRING, lexer.TOKEN_KENDI, lexer.TOKEN_FONKSIYON, lexer.TOKEN_KESME_NOKTASI:
			return false
		}
		return true
	case pt == lexer.TOKEN_RANGE || pt == lexer.TOKEN_RANGE_INCL || ct == lexer.TOKEN_RANGE || ct == lexer.TOKEN_RANGE_INCL:
		return false
	case ct == lexer.TOKEN_COLON:
		return false
	}
	return true
}

// donustur Bicimle'nin kullandığı dönüşümdür; testler anlam denetimini sınamak için
// bilerek bozuk bir dönüşümle değiştirir.
var donustur = bicimle

// bicimle asıl biçimlendirme işini yapar (anlam denetimi Bicimle'dedir).
func bicimle(kaynak string, sus map[konum]int) string {
	og := ogeler(kaynak)

	// 1. Tekli operatörleri işaretle; gövde '{' ve 'değilse'/'yakala' sözcüklerini önceki satıra al.
	var oncekiTok *oge
	var susYigin []*oge // açık '{' öğeleri (sözlük kapanışını eşlemek için)
	for i := range og {
		o := &og[i]
		if o.tur == tokYorum || o.tur == lexer.TOKEN_EOF {
			continue
		}
		if o.tur == lexer.TOKEN_BANG {
			o.tekli = true
		} else if o.tur == lexer.TOKEN_MINUS {
			oku := i+1 < len(og) && og[i+1].tur == lexer.TOKEN_GT && og[i+1].bitisik &&
				oncekiTok != nil && (oncekiTok.tur == lexer.TOKEN_RPAREN)
			o.tekli = !oku && tekliMi(oncekiTok)
		}
		switch o.tur {
		case lexer.TOKEN_LBRACE:
			o.sozluk = sus[konum{o.satir, o.sutun}] == susluSozluk
			susYigin = append(susYigin, o)
		case lexer.TOKEN_RBRACE:
			if len(susYigin) > 0 {
				o.sozluk = susYigin[len(susYigin)-1].sozluk
				susYigin = susYigin[:len(susYigin)-1]
			}
		}
		if o.satirA > 0 && i > 0 && og[i-1].tur != tokYorum {
			onceki := og[i-1].tur
			switch {
			case o.tur == lexer.TOKEN_LBRACE && sus[konum{o.satir, o.sutun}] == 0:
				o.satirA = 0
			case (o.tur == lexer.TOKEN_DEGILSE || o.tur == lexer.TOKEN_YAKALA) && onceki == lexer.TOKEN_RBRACE:
				o.satirA = 0
			}
		}
		oncekiTok = o
	}

	// 2. Satırlara böl.
	type satir struct {
		ogeler  []*oge
		bosOnce bool // öncesinde boş satır var mı
	}
	var satirlar []satir
	for i := range og {
		o := &og[i]
		if o.tur == lexer.TOKEN_EOF {
			break
		}
		if len(satirlar) == 0 || o.satirA > 0 {
			satirlar = append(satirlar, satir{bosOnce: o.satirA >= 2 && len(satirlar) > 0})
		}
		s := &satirlar[len(satirlar)-1]
		s.ogeler = append(s.ogeler, o)
	}

	// 3. Girintileri hesapla ve yaz.
	type acik struct{ girinti int }
	var yigin []acik
	var sb strings.Builder
	var oncekiSatirSonTok *oge // önceki satırın son sözcük birimi (yorumlar hariç)
	for si, s := range satirlar {
		ilk := s.ogeler[0]
		girinti := 0
		// Satır başındaki kapanışlar
		basKapanis := 0
		for _, o := range s.ogeler {
			if !kapanis(o.tur) {
				break
			}
			basKapanis++
		}
		if basKapanis > 0 {
			k := len(yigin) - basKapanis
			if k < 0 {
				k = 0
			}
			if k < len(yigin) {
				girinti = yigin[k].girinti
			}
		} else {
			if len(yigin) > 0 {
				girinti = yigin[len(yigin)-1].girinti + 1
			}
			if oncekiSatirSonTok != nil && ilk.tur != tokYorum &&
				(ikiliOperatorler[oncekiSatirSonTok.tur] && !oncekiSatirSonTok.tekli || devamEttirenSon[oncekiSatirSonTok.tur] || devamEdenBas[ilk.tur]) {
				girinti++
			}
		}

		// Boş satır kuralları: dosya başında, '{' / '(' / '[' ile biten satırdan sonra ve
		// kapanışla başlayan satırdan önce boş satır olmaz.
		if s.bosOnce && si > 0 && basKapanis == 0 && !(oncekiSatirSonTok != nil && acilis(oncekiSatirSonTok.tur) && sonOge(satirlar[si-1].ogeler) == oncekiSatirSonTok) {
			sb.WriteString("\n")
		}

		var satirSb strings.Builder
		satirSb.WriteString(strings.Repeat(girintiBirimi, girinti))
		for j, o := range s.ogeler {
			if j > 0 && aradaBosluk(s.ogeler[j-1], o) {
				satirSb.WriteString(" ")
			}
			satirSb.WriteString(o.metin)
			if acilis(o.tur) {
				yigin = append(yigin, acik{girinti: girinti})
			} else if kapanis(o.tur) && len(yigin) > 0 {
				yigin = yigin[:len(yigin)-1]
			}
			if o.tur != tokYorum {
				oncekiSatirSonTok = o
			}
		}
		sb.WriteString(strings.TrimRight(satirSb.String(), " \t"))
		sb.WriteString("\n")
	}
	return sb.String()
}

func sonOge(o []*oge) *oge {
	return o[len(o)-1]
}
