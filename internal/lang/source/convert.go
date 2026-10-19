package source

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Çevirici (17.3, 17.4): Türkçe yazılmış kaynağı kodlu biçime (Kodla) ve kodlu kaynağı bir
// yazım diline (Coz) çevirir. Çeviri yeniden yazdırma değildir: kaynakta yalnızca dilin
// sözcüklerinin aralıkları değişir; boşluklar, satır düzeni, yorumlar, kullanıcının adları,
// metinler ve sayılar bayt bayt korunur. Metin içi yerleştirmelerdeki sözcükler çevrilir; ham
// metnin ve yorumların içi hiçbir zaman çevrilmez. Neyin dilin sözcüğü olduğu sözcüğe bakarak
// tahmin edilmez: ayrıştırıcının ad görevlerinden (ayristir.AdRolu) ve denetleyicinin kapsam
// bilgisinden (denetci.AdBaglari) çıkarılır.

// SyntaxError is returned when the source to convert cannot be parsed: the language's words
// cannot be told apart without scope information, so the source is left untouched.
type SyntaxError struct {
	Errors []string
}

func (e *SyntaxError) Error() string {
	return "sözdizimi hatası olan kaynak çevrilmez: " + e.Errors[0]
}

// ErrUTF8 is returned when the body to convert is not valid UTF-8.
var ErrUTF8 = errors.New("dosya geçerli bir UTF-8 metni değil (2.1); bozuk baytlar çeviride korunamaz, dosyaya dokunulmadı: dosyayı UTF-8 olarak kaydedin")

// ErrLossy is returned when the converted result would not mean the same as the original
// program. It is a converter bug; the file must not be written.
var ErrLossy = errors.New("çeviri kayıpsız olmadı (programın anlamı ya da metni değişecekti), dosyaya dokunulmadı; lütfen bu dosyayı bir hata kaydıyla bildirin")

// Encode converts source written in Turkish (the whole file) to coded form. Source that is
// already coded is returned as is. The '$' of '$name' escapes is dropped (2.11). The result is
// verified: the coded program's tree must equal the original's, and decoding and re-encoding
// the coded text must not change it; otherwise it returns ErrLossy.
func Encode(metin string) (string, error) {
	b, err := Split(metin)
	if err != nil || b.Coded {
		return metin, err
	}
	c, err := cozumle(b, codes.Active())
	if err != nil {
		return "", err
	}
	kodlu := Parts{BOM: b.BOM, Coded: true, Body: c.kodluGovde()}
	kodlu.HeaderEOL = satirSonuBul(kodlu.Body)
	sonuc := kodlu.Join()
	if err := c.ayniAnlam(kodlu); err != nil {
		return "", err
	}
	if geri, err := Decode(sonuc, codes.Active()); err != nil || !yenidenKodlanir(geri, sonuc) {
		return "", ErrLossy
	}
	return sonuc, nil
}

// yenidenKodlanir kodlu metnin çözülmüş hâli kodlanınca aynı kodlu metni veriyor mu?
func yenidenKodlanir(cozulmus, kodlu string) bool {
	b, err := Split(cozulmus)
	if err != nil || b.Coded {
		return false
	}
	c, err := cozumle(b, codes.Active())
	if err != nil {
		return false
	}
	yeni := Parts{BOM: b.BOM, Coded: true, Body: c.kodluGovde()}
	yeni.HeaderEOL = satirSonuBul(yeni.Body)
	return yeni.Join() == kodlu
}

// kodluGovde Türkçe gövdenin kodlu biçimidir: anahtar kelimeler ve dilin öğesi olan adlar
// kodlarıyla yazılır, '$ad' kaçışlarının '$'ı atılır; başka hiçbir şey değişmez.
func (c *cozumleme) kodluGovde() string {
	var d []degisim
	for _, r := range c.kayitlar {
		if r.Type == lexer.TOKEN_IDENT && c.kacisli(r) {
			d = append(d, degisim{r.Start, r.End, r.Literal})
		} else if k := c.sozcukKodu(r); k != codes.None {
			d = append(d, degisim{r.Start, r.End, k.String()})
		}
	}
	return uygula(c.girdi, d)
}

// Decode converts coded source (the whole file) to dil's spelling; source that is not coded
// is returned as is. Codes become dil's names; user names that would read as a word of the
// language in dil are written as '$name' (2.11). The header is dropped and the BOM kept.
func Decode(metin string, dil *codes.Language) (string, error) {
	b, err := Split(metin)
	if err != nil || !b.Coded {
		return metin, err
	}
	c, err := cozumle(b, dil)
	if err != nil {
		return "", err
	}
	return Parts{BOM: b.BOM, Body: c.gosterimGovdesi()}.Join(), nil
}

// gosterimGovdesi kodlu gövdenin c.dil'deki yazımıdır.
func (c *cozumleme) gosterimGovdesi() string {
	var d []degisim
	for _, r := range c.kayitlar {
		switch {
		case r.Code != codes.None:
			d = append(d, degisim{r.Start, r.End, dilAdi(c.dil, r.Code)})
		case r.Type == lexer.TOKEN_IDENT && !c.kacisli(r) && c.kacisGerekir(r):
			d = append(d, degisim{r.Start, r.End, "$" + r.Literal})
		}
	}
	return uygula(c.girdi, d)
}

// gosterim kodlu bir gövdenin etkin dildeki yazımıdır: hata çerçeveleri ve hata ayıklayıcı
// bu metni gösterir (konumlar ona göredir, 17.5). Gövde ayrıştırılamıyorsa yalnızca dosya
// düzeyindeki kodlar adlarına çevrilir (yerleştirmelerdekiler kod kalır).
func gosterim(b Parts) string {
	if c, err := cozumle(b, codes.Active()); err == nil {
		return c.gosterimGovdesi()
	}
	l := b.Lexer()
	l.StartRecording()
	for l.NextToken().Type != lexer.TOKEN_EOF {
	}
	var d []degisim
	for _, r := range l.Records() {
		if r.Code != codes.None {
			d = append(d, degisim{r.Start, r.End, dilAdi(codes.Active(), r.Code)})
		}
	}
	return uygula(l.Input(), d)
}

// dilAdi kodun dil'deki adıdır; dil kodu tanımıyorsa (başka dillerde ölü kodlar isteğe
// bağlıdır, 17.6) kod olduğu gibi kalır.
func dilAdi(dil *codes.Language, k codes.Code) string {
	if g, ok := dil.Entry(k); ok {
		return g.Name
	}
	return k.String()
}

// cozumleme bir gövdenin çeviri için çıkarılmış bilgisidir: bütün sözcükler (yerleştirmelerdekiler
// dahil) kaynak aralıklarıyla, ad sözcüklerinin görevleri ve adların bağları (dil'in adlarıyla).
type cozumleme struct {
	dil      *codes.Language
	girdi    []rune
	kayitlar []lexer.Record
	roller   map[[2]int]parser.NameRole
	baglar   map[[2]int]checker.NameBinding
	// cakisanlar gösterimde kodlu bir sözcüğü yakalayacak kullanıcı adlarıdır; her yerde
	// kaçışla yazılırlar (denetci.AdBaglari).
	cakisanlar map[string]bool
	prog       *ast.Program
}

func cozumle(b Parts, dil *codes.Language) (*cozumleme, error) {
	if !utf8.ValidString(b.Body) {
		return nil, ErrUTF8
	}
	l := b.Lexer()
	l.StartRecording()
	p := parser.New(l)
	p.RecordRoles()
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, &SyntaxError{Errors: errs}
	}
	c := &cozumleme{dil: dil, girdi: l.Input(), kayitlar: l.Records(), roller: p.Roles(),
		baglar: map[[2]int]checker.NameBinding{}, prog: prog}
	baglar, cakisanlar := checker.NameBindings(prog, dil, nil, false)
	for _, bg := range baglar {
		c.baglar[[2]int{bg.Token.Line, bg.Token.Column}] = bg
	}
	c.cakisanlar = cakisanlar
	return c, nil
}

// ayniAnlam kodlu gövdenin ağacının özgün ağaçla aynı olduğunu denetler (sözcük birimleri
// konumlarıyla atlanır; kodlu sözcükler etkin dilin adlarıyla okunduğu için ağaçlar birebir
// aynı olmalıdır).
func (c *cozumleme) ayniAnlam(kodlu Parts) error {
	p := parser.New(kodlu.Lexer())
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 || ast.Dump(prog) != ast.Dump(c.prog) {
		return ErrLossy
	}
	return nil
}

// kacisli sözcük kaynakta '$ad' kaçışıyla mı yazılmış?
func (c *cozumleme) kacisli(r lexer.Record) bool {
	return r.Start < len(c.girdi) && c.girdi[r.Start] == '$'
}

// sozcukKodu bir sözcüğün kodlu biçimdeki kodudur: anahtar kelime, kodla yazılmış sözcük ya da
// dilin öğesi olan ad; kullanıcının adında kod.Yok.
func (c *cozumleme) sozcukKodu(r lexer.Record) codes.Code {
	if r.Code != codes.None {
		return r.Code
	}
	if r.Type == lexer.TOKEN_IDENT {
		return c.dilOgesi(r)
	}
	return lexer.KeywordCode(r.Type)
}

// kacisGerekir kodlu kaynaktaki bir kullanıcı adı dil'de yazılınca dilin bir sözcüğü olarak
// okunur mu? Öyleyse gösterimde '$ad' yazılır.
func (c *cozumleme) kacisGerekir(r lexer.Record) bool {
	if k, ok := c.dil.Find(codes.NamespaceGlobal, r.Literal); ok && k.IsKeyword() {
		return true
	}
	return c.cakisanlar[r.Literal] || c.dilOgesi(r) != codes.None
}

// degisim kaynakta [bas, son) karakter aralığının yerine yazılacak metindir.
type degisim struct {
	bas, son int
	metin    string
}

// uygula değişimleri girdiye uygular; aynı aralık iki kez kaydedilmişse (bir yerleştirme iki
// kez ayrıştırıldıysa) bir kez uygulanır.
func uygula(girdi []rune, d []degisim) string {
	sort.SliceStable(d, func(i, j int) bool { return d[i].bas < d[j].bas })
	var sb strings.Builder
	onceki := 0
	for _, x := range d {
		if x.bas < onceki {
			continue
		}
		sb.WriteString(string(girdi[onceki:x.bas]))
		sb.WriteString(x.metin)
		onceki = x.son
	}
	sb.WriteString(string(girdi[onceki:]))
	return sb.String()
}
