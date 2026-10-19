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
func (c *analysis) langElement(r lexer.Record) codes.Code {
	pos := [2]int{r.Line, r.Column}
	name := r.Literal
	switch c.roles[pos] {
	case parser.RoleType:
		return c.find(codes.NamespaceType, name)
	case parser.RoleMember, parser.RoleStructField, parser.RoleStructMethod:
		if name == c.lang.Name(codes.A102) {
			return codes.A102
		}
		return c.find(codes.NamespaceMember, name)
	case parser.RoleDictKey:
		return c.find(codes.NamespaceMember, name)
	case parser.RoleTest:
		return c.contextual(codes.A101, name)
	case parser.RoleAs:
		return c.contextual(codes.A100, name)
	}
	if bg, ok := c.bindings[pos]; ok && bg.Token.Literal == name {
		switch bg.Kind {
		case checker.BindBuiltin, checker.BindImplicit, checker.BindNamed:
			return bg.Code
		}
	}
	return codes.None
}

// bul adın c.dil'de verilen ad alanındaki canlı kodudur.
func (c *analysis) find(ns codes.Namespace, name string) codes.Code {
	if k, ok := c.lang.Find(ns, name); ok {
		return k
	}
	return codes.None
}

// baglamsal ad c.dil'de k bağlamsal sözcüğünün adıysa k'dir.
func (c *analysis) contextual(k codes.Code, name string) codes.Code {
	if c.lang.Name(k) == name {
		return k
	}
	return codes.None
}
