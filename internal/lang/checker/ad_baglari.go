package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Ad bağları: kodlu saklama (docs/spec/17_kodlar.md, 17.3) bir adın dilin öğesi mi
// kullanıcının adı mı olduğunu sözcüğe bakarak değil, tanımsız ad denetiminin kapsam
// kuralıyla (5.12, 5.14) bulur. Aynı gezinti burada hata aramadan her adın neye bağlandığını
// verir: kullanıcı bir yerleşiği gölgelediyse (değişken yol = ...) o kapsamdaki 'yol'
// kullanıcının adıdır; sondaki bloğun 'o'su ve adı yazılmamış yakala'nın 'hata'sı yalnızca
// kendi bloklarında bağlamsal sözcüktür.

// BagTuru bir adın bağlandığı şeyin türüdür.
type BagTuru uint8

const (
	// BagKullanici: ad görünen bir kapsamda kullanıcının tanımıdır.
	BagKullanici BagTuru = iota
	// BagYerlesik: ad hiçbir kullanıcı kapsamında yok; dilin bir yerleşiği ya da tip adıdır
	// (Kod). Takma adsız 'aktar'ın altında da yerleşik sayılır (modülün adları çalışma
	// anında gelir; bir modülün yerleşiği gölgelemesi ancak çalışma anında bilinir).
	BagYerlesik
	// BagOrtuk: ad bağlamsal bir örtük addır (Kod: A103 hata, A104 o, A105 birikim).
	BagOrtuk
	// BagAdli: adlı argümanın adı dilin bir adıdır (Kod: x9xx parametre kodu ya da yapı
	// kurucusunda alan adının üye kodu).
	BagAdli
	// BagTanimsiz: ad ne kullanıcının ne dilin; tanımsız ad.
	BagTanimsiz
)

// AdBagi bir ad sözcüğünün (değer olarak okunan ad ya da adlı argüman adı) bağıdır.
type AdBagi struct {
	Sozcuk lexer.Token
	Tur    BagTuru
	Kod    codes.Kod
}

// AdBaglari programdaki her adın bağını dil'in adlarıyla verir. Örtük adlar ve yerleşikler
// dil'in adlarıyla aranır: dil etkin dil değilse sonuç, sözcüğün o dilde yazılsaydı neye
// bağlanacağıdır (kodlu dosya başka bir yazım dilinde gösterilirken '$ad' kaçışının
// gerekip gerekmediği böyle bulunur). cakisanlar, kodlu kaynakta kodla yazılmış bir değer
// adının (G100) dil'deki adını, bulunduğu yerde kendi tanımıyla yakalayacak kullanıcı adlarıdır
// (dil'de 'yazdır' adlı bir kullanıcı değişkeni yerleşik 'yazdır'ı gölgelerdi); bu adlar
// gösterimde her yerde kaçışla yazılır. genelAdlar ve dinamik GenelKapsam'daki gibidir (modül).
func AdBaglari(prog *ast.Program, dil *codes.Dil, genelAdlar []string, dinamik bool) (baglar []AdBagi, cakisanlar map[string]bool) {
	d := &adDenetleyici{baglar: &baglar, dil: dil, cakisanlar: map[string]bool{}}
	ust := yeniAdKapsami(nil)
	for _, a := range genelAdlar {
		ust.tanimla(a)
	}
	ust.dinamik = dinamik
	k := yeniAdKapsami(ust)
	d.tanimlar(prog.Statements, k)
	for _, st := range prog.Statements {
		d.deyim(st, k)
	}
	return baglar, d.cakisanlar
}

// cozum adın görünen kapsamlardaki bağıdır; yapı, ad bir kullanıcı yapısıysa onun tanımıdır.
func (d *adDenetleyici) cozum(ad string, k *adKapsami) (BagTuru, codes.Kod, *ast.StructStatement) {
	belirsiz := false
	for c := k; c != nil; c = c.ust {
		for _, o := range c.ortuk {
			if o != codes.Yok && d.dil.Ad(o) == ad {
				return BagOrtuk, o, nil
			}
		}
		if c.adlar[ad] {
			return BagKullanici, codes.Yok, c.yapilar[ad]
		}
		belirsiz = belirsiz || c.dinamik
	}
	if k, ok := d.dil.Bul(codes.AlanGenel, ad); ok && k.Yerlesik() {
		return BagYerlesik, k, nil
	}
	if k, ok := d.dil.Bul(codes.AlanTip, ad); ok {
		return BagYerlesik, k, nil
	}
	if belirsiz {
		return BagKullanici, codes.Yok, nil
	}
	return BagTanimsiz, codes.Yok, nil
}

// sozcukCozumu bir ad sözcüğünün bağıdır: kodlu kaynakta kodla yazılmış sözcük (Token.Kod)
// kodun kendisine bağlıdır, başka her ad cozum ile kapsamlarda aranır.
func (d *adDenetleyici) sozcukCozumu(tok lexer.Token, ad string, k *adKapsami) (BagTuru, codes.Kod, *ast.StructStatement) {
	switch kd := tok.Kod; {
	case kd == codes.Yok:
		return d.cozum(ad, k)
	case kd.Alan() == codes.AlanGenel && !kd.Yerlesik():
		return BagOrtuk, kd, nil
	default:
		return BagYerlesik, kd, nil
	}
}

// bagla değer olarak okunan (ya da atanan) bir adın bağını kaydeder.
func (d *adDenetleyici) bagla(id *ast.Identifier, k *adKapsami) {
	tur, kd, _ := d.sozcukCozumu(id.Token, id.Value, k)
	*d.baglar = append(*d.baglar, AdBagi{Sozcuk: id.Token, Tur: tur, Kod: kd})
	if kd := id.Token.Kod; kd.Yerlesik() || kd.Alan() == codes.AlanTip {
		if g, ok := d.dil.Girdisi(kd); ok {
			if t, _, _ := d.cozum(g.Ad, k); t == BagKullanici {
				d.cakisanlar[g.Ad] = true
			}
		}
	}
}

// adliBaglar bir çağrının adlı argümanlarının bağlarını kaydeder. Adlı argüman adı yalnızca
// çağrılan dilin bir öğesiyse dilin adıdır: yerleşik bir fonksiyon (gölgelenmemiş) ya da
// dilin bir üyesi (x.ad, 17.3); kullanıcının fonksiyonlarına verilen adlı argümanlar
// parametre adlarıdır, kullanıcının adıdır. Kurucusu olmayan bir yapının oluşturulmasında
// adlı argümanlar alan adlarıdır; alan adı bir üye koduyla saklanıyorsa (Y5) argüman da öyle.
func (d *adDenetleyici) adliBaglar(c *ast.CallExpression, k *adKapsami) {
	if len(c.NamedArgs) == 0 {
		return
	}
	alan := codes.AlanParametre
	switch f := c.Function.(type) {
	case *ast.Identifier:
		tur, _, yapi := d.sozcukCozumu(f.Token, f.Value, k)
		switch {
		case tur == BagYerlesik:
		case yapi != nil && yapi.Methods[codes.Ad(codes.A102)] == nil:
			alan = codes.AlanUye
		default:
			return
		}
	case *ast.MemberExpression:
		if _, ok := d.dil.Bul(codes.AlanUye, f.Property); !ok && f.AdSozcugu.Kod == codes.Yok {
			return
		}
	default:
		return
	}
	for _, na := range c.NamedArgs {
		if kd, ok := d.dil.Bul(alan, na.Name); ok {
			*d.baglar = append(*d.baglar, AdBagi{Sozcuk: na.Token, Tur: BagAdli, Kod: kd})
		}
	}
}

// sondakiBlokOrtukleri sondaki bloğun (liste.filtrele { o > 2 }) örtük parametrelerinin
// bağlamsal kodlarıdır: 'o' (A104) ve indirge bloğunda 'birikim' (A105).
func sondakiBlokOrtukleri(f *ast.FunctionLiteral) [2]codes.Kod {
	var o [2]codes.Kod
	for i, p := range f.Parameters {
		if i >= len(o) {
			break
		}
		switch p.Name {
		case codes.Ad(codes.A104):
			o[i] = codes.A104
		case codes.Ad(codes.A105):
			o[i] = codes.A105
		}
	}
	return o
}
