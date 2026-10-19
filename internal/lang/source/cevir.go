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

// SozdizimiHatasi çevrilecek kaynak ayrıştırılamadığında döner: dilin sözcükleri kapsam
// bilgisi olmadan ayırt edilemez, kaynağa dokunulmaz.
type SozdizimiHatasi struct {
	Hatalar []string
}

func (e *SozdizimiHatasi) Error() string {
	return "sözdizimi hatası olan kaynak çevrilmez: " + e.Hatalar[0]
}

// ErrUTF8 çevrilecek gövde geçerli UTF-8 değilse döner.
var ErrUTF8 = errors.New("dosya geçerli bir UTF-8 metni değil (2.1); bozuk baytlar çeviride korunamaz, dosyaya dokunulmadı: dosyayı UTF-8 olarak kaydedin")

// ErrKayip çevirinin sonucu özgün programla aynı anlama gelmediğinde döner. Bu bir çevirici
// hatasıdır; dosya yazılmamalıdır.
var ErrKayip = errors.New("çeviri kayıpsız olmadı (programın anlamı ya da metni değişecekti), dosyaya dokunulmadı; lütfen bu dosyayı bir hata kaydıyla bildirin")

// Kodla Türkçe yazılmış bir kaynağı (dosyanın tamamı) kodlu biçime çevirir. Kaynak zaten
// kodluysa olduğu gibi döner. '$ad' kaçışlarının '$'ı atılır (2.11). Sonuç denetlenir:
// kodlu programın ağacı özgününkiyle aynı olmalı ve kodlu metin çözülüp yeniden kodlanınca
// değişmemelidir; değilse ErrKayip döner.
func Kodla(metin string) (string, error) {
	b, err := Ayir(metin)
	if err != nil || b.Kodlu {
		return metin, err
	}
	c, err := cozumle(b, codes.Etkin())
	if err != nil {
		return "", err
	}
	kodlu := Bolumler{BOM: b.BOM, Kodlu: true, Govde: c.kodluGovde()}
	kodlu.SatirSonu = satirSonuBul(kodlu.Govde)
	sonuc := kodlu.Birlestir()
	if err := c.ayniAnlam(kodlu); err != nil {
		return "", err
	}
	if geri, err := Coz(sonuc, codes.Etkin()); err != nil || !yenidenKodlanir(geri, sonuc) {
		return "", ErrKayip
	}
	return sonuc, nil
}

// yenidenKodlanir kodlu metnin çözülmüş hâli kodlanınca aynı kodlu metni veriyor mu?
func yenidenKodlanir(cozulmus, kodlu string) bool {
	b, err := Ayir(cozulmus)
	if err != nil || b.Kodlu {
		return false
	}
	c, err := cozumle(b, codes.Etkin())
	if err != nil {
		return false
	}
	yeni := Bolumler{BOM: b.BOM, Kodlu: true, Govde: c.kodluGovde()}
	yeni.SatirSonu = satirSonuBul(yeni.Govde)
	return yeni.Birlestir() == kodlu
}

// kodluGovde Türkçe gövdenin kodlu biçimidir: anahtar kelimeler ve dilin öğesi olan adlar
// kodlarıyla yazılır, '$ad' kaçışlarının '$'ı atılır; başka hiçbir şey değişmez.
func (c *cozumleme) kodluGovde() string {
	var d []degisim
	for _, r := range c.kayitlar {
		if r.Type == lexer.TOKEN_IDENT && c.kacisli(r) {
			d = append(d, degisim{r.Bas, r.Son, r.Literal})
		} else if k := c.sozcukKodu(r); k != codes.Yok {
			d = append(d, degisim{r.Bas, r.Son, k.String()})
		}
	}
	return uygula(c.girdi, d)
}

// Coz kodlu bir kaynağı (dosyanın tamamı) dil'in yazımına çevirir; kaynak kodlu değilse
// olduğu gibi döner. Kodlar dil'in adlarına çevrilir; dil'de dilin bir sözcüğü olarak
// okunacak kullanıcı adları '$ad' biçiminde yazılır (2.11). Başlık atılır, BOM korunur.
func Coz(metin string, dil *codes.Dil) (string, error) {
	b, err := Ayir(metin)
	if err != nil || !b.Kodlu {
		return metin, err
	}
	c, err := cozumle(b, dil)
	if err != nil {
		return "", err
	}
	return Bolumler{BOM: b.BOM, Govde: c.gosterimGovdesi()}.Birlestir(), nil
}

// gosterimGovdesi kodlu gövdenin c.dil'deki yazımıdır.
func (c *cozumleme) gosterimGovdesi() string {
	var d []degisim
	for _, r := range c.kayitlar {
		switch {
		case r.Kod != codes.Yok:
			d = append(d, degisim{r.Bas, r.Son, dilAdi(c.dil, r.Kod)})
		case r.Type == lexer.TOKEN_IDENT && !c.kacisli(r) && c.kacisGerekir(r):
			d = append(d, degisim{r.Bas, r.Son, "$" + r.Literal})
		}
	}
	return uygula(c.girdi, d)
}

// gosterim kodlu bir gövdenin etkin dildeki yazımıdır: hata çerçeveleri ve hata ayıklayıcı
// bu metni gösterir (konumlar ona göredir, 17.5). Gövde ayrıştırılamıyorsa yalnızca dosya
// düzeyindeki kodlar adlarına çevrilir (yerleştirmelerdekiler kod kalır).
func gosterim(b Bolumler) string {
	if c, err := cozumle(b, codes.Etkin()); err == nil {
		return c.gosterimGovdesi()
	}
	l := b.Sozcukler()
	l.Kaydet()
	for l.NextToken().Type != lexer.TOKEN_EOF {
	}
	var d []degisim
	for _, r := range l.Kayitlar() {
		if r.Kod != codes.Yok {
			d = append(d, degisim{r.Bas, r.Son, dilAdi(codes.Etkin(), r.Kod)})
		}
	}
	return uygula(l.Girdi(), d)
}

// dilAdi kodun dil'deki adıdır; dil kodu tanımıyorsa (başka dillerde ölü kodlar isteğe
// bağlıdır, 17.6) kod olduğu gibi kalır.
func dilAdi(dil *codes.Dil, k codes.Kod) string {
	if g, ok := dil.Girdisi(k); ok {
		return g.Ad
	}
	return k.String()
}

// cozumleme bir gövdenin çeviri için çıkarılmış bilgisidir: bütün sözcükler (yerleştirmelerdekiler
// dahil) kaynak aralıklarıyla, ad sözcüklerinin görevleri ve adların bağları (dil'in adlarıyla).
type cozumleme struct {
	dil      *codes.Dil
	girdi    []rune
	kayitlar []lexer.Kayit
	roller   map[[2]int]parser.AdRolu
	baglar   map[[2]int]checker.AdBagi
	// cakisanlar gösterimde kodlu bir sözcüğü yakalayacak kullanıcı adlarıdır; her yerde
	// kaçışla yazılırlar (denetci.AdBaglari).
	cakisanlar map[string]bool
	prog       *ast.Program
}

func cozumle(b Bolumler, dil *codes.Dil) (*cozumleme, error) {
	if !utf8.ValidString(b.Govde) {
		return nil, ErrUTF8
	}
	l := b.Sozcukler()
	l.Kaydet()
	p := parser.New(l)
	p.RolleriKaydet()
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, &SozdizimiHatasi{Hatalar: errs}
	}
	c := &cozumleme{dil: dil, girdi: l.Girdi(), kayitlar: l.Kayitlar(), roller: p.Roller(),
		baglar: map[[2]int]checker.AdBagi{}, prog: prog}
	baglar, cakisanlar := checker.AdBaglari(prog, dil, nil, false)
	for _, bg := range baglar {
		c.baglar[[2]int{bg.Sozcuk.Line, bg.Sozcuk.Column}] = bg
	}
	c.cakisanlar = cakisanlar
	return c, nil
}

// ayniAnlam kodlu gövdenin ağacının özgün ağaçla aynı olduğunu denetler (sözcük birimleri
// konumlarıyla atlanır; kodlu sözcükler etkin dilin adlarıyla okunduğu için ağaçlar birebir
// aynı olmalıdır).
func (c *cozumleme) ayniAnlam(kodlu Bolumler) error {
	p := parser.New(kodlu.Sozcukler())
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 || ast.Dokum(prog) != ast.Dokum(c.prog) {
		return ErrKayip
	}
	return nil
}

// kacisli sözcük kaynakta '$ad' kaçışıyla mı yazılmış?
func (c *cozumleme) kacisli(r lexer.Kayit) bool {
	return r.Bas < len(c.girdi) && c.girdi[r.Bas] == '$'
}

// sozcukKodu bir sözcüğün kodlu biçimdeki kodudur: anahtar kelime, kodla yazılmış sözcük ya da
// dilin öğesi olan ad; kullanıcının adında kod.Yok.
func (c *cozumleme) sozcukKodu(r lexer.Kayit) codes.Kod {
	if r.Kod != codes.Yok {
		return r.Kod
	}
	if r.Type == lexer.TOKEN_IDENT {
		return c.dilOgesi(r)
	}
	return lexer.AnahtarKelimeKodu(r.Type)
}

// kacisGerekir kodlu kaynaktaki bir kullanıcı adı dil'de yazılınca dilin bir sözcüğü olarak
// okunur mu? Öyleyse gösterimde '$ad' yazılır.
func (c *cozumleme) kacisGerekir(r lexer.Kayit) bool {
	if k, ok := c.dil.Bul(codes.AlanGenel, r.Literal); ok && k.AnahtarKelime() {
		return true
	}
	return c.cakisanlar[r.Literal] || c.dilOgesi(r) != codes.Yok
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
