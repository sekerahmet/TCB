package source

import (
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// dilOgesi bir ad sözcüğünün (kodla yazılmamış, kaçışsız TANIMLAYICI) bulunduğu yerde
// c.dil'in hangi öğesi olduğudur; kullanıcının adıysa kod.Yok (17.3):
//
//   - tip açıklaması (değişken x: Tam): tip adı;
//   - üye adı (x.ad) ile yapı alanı ve metodu tanımı: üye kodu (Y5, tipten bağımsız; 'kurucu'
//     bağlamsal A102);
//   - sözlük değişmezinin çıplak ad anahtarı ({ad: değer}): adı bir üye koduysa o kod (anahtar
//     x.ad ile okunur; iki yer birlikte çevrilsin);
//   - test deyiminin ve aktar ... olarak'ın sözcükleri (A101, A100);
//   - değer olarak okunan ad: kullanıcının görünen bir tanımı yoksa yerleşik ya da tip adı,
//     kendi bloğunda örtük ad (o, birikim, hata);
//   - adlı argüman: çağrılan dilin öğesiyse parametre kodu (denetci.AdBaglari).
//
// Tip adının tek yazımı vardır (3.14): kabul edilmeyen bir yazım (tam) tip adı değildir,
// yazıldığı gibi kalır; denetleyici onu hata olarak bildirir.
func (c *cozumleme) dilOgesi(r lexer.Record) codes.Code {
	konum := [2]int{r.Line, r.Column}
	ad := r.Literal
	switch c.roller[konum] {
	case parser.RoleType:
		return c.bul(codes.NamespaceType, ad)
	case parser.RoleMember, parser.RoleStructField, parser.RoleStructMethod:
		if ad == c.dil.Name(codes.A102) {
			return codes.A102
		}
		return c.bul(codes.NamespaceMember, ad)
	case parser.RoleDictKey:
		return c.bul(codes.NamespaceMember, ad)
	case parser.RoleTest:
		return c.baglamsal(codes.A101, ad)
	case parser.RoleAs:
		return c.baglamsal(codes.A100, ad)
	}
	if bg, ok := c.baglar[konum]; ok && bg.Token.Literal == ad {
		switch bg.Kind {
		case checker.BindBuiltin, checker.BindImplicit, checker.BindNamed:
			return bg.Code
		}
	}
	return codes.None
}

// bul adın c.dil'de verilen ad alanındaki canlı kodudur.
func (c *cozumleme) bul(al codes.Namespace, ad string) codes.Code {
	if k, ok := c.dil.Find(al, ad); ok {
		return k
	}
	return codes.None
}

// baglamsal ad c.dil'de k bağlamsal sözcüğünün adıysa k'dir.
func (c *cozumleme) baglamsal(k codes.Code, ad string) codes.Code {
	if c.dil.Name(k) == ad {
		return k
	}
	return codes.None
}
