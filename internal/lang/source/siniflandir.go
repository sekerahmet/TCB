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
func (c *cozumleme) dilOgesi(r lexer.Kayit) codes.Kod {
	konum := [2]int{r.Line, r.Column}
	ad := r.Literal
	switch c.roller[konum] {
	case parser.RolTip:
		return c.bul(codes.AlanTip, ad)
	case parser.RolUye, parser.RolYapiAlani, parser.RolYapiMetodu:
		if ad == c.dil.Ad(codes.A102) {
			return codes.A102
		}
		return c.bul(codes.AlanUye, ad)
	case parser.RolSozlukAnahtari:
		return c.bul(codes.AlanUye, ad)
	case parser.RolTest:
		return c.baglamsal(codes.A101, ad)
	case parser.RolOlarak:
		return c.baglamsal(codes.A100, ad)
	}
	if bg, ok := c.baglar[konum]; ok && bg.Sozcuk.Literal == ad {
		switch bg.Tur {
		case checker.BagYerlesik, checker.BagOrtuk, checker.BagAdli:
			return bg.Kod
		}
	}
	return codes.Yok
}

// bul adın c.dil'de verilen ad alanındaki canlı kodudur.
func (c *cozumleme) bul(al codes.Alan, ad string) codes.Kod {
	if k, ok := c.dil.Bul(al, ad); ok {
		return k
	}
	return codes.Yok
}

// baglamsal ad c.dil'de k bağlamsal sözcüğünün adıysa k'dir.
func (c *cozumleme) baglamsal(k codes.Kod, ad string) codes.Kod {
	if c.dil.Ad(k) == ad {
		return k
	}
	return codes.Yok
}
