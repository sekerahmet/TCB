package kaynak

import (
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/denetci"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/sozcuk"
)

// dilOgesi bir ad sözcüğünün (kodla yazılmamış, kaçışsız TANIMLAYICI) bulunduğu yerde
// c.dil'in hangi öğesi olduğudur; kullanıcının adıysa kod.Yok (17.3):
//
//   - tip açıklaması (değişken x: Tam): tip adı;
//   - üye adı (x.ad) ile yapı alanı ve metodu tanımı: üye kodu (Y5, tipten bağımsız; 'kurucu'
//     bağlamsal A102);
//   - test deyiminin ve aktar ... olarak'ın sözcükleri (A101, A100);
//   - değer olarak okunan ad: kullanıcının görünen bir tanımı yoksa yerleşik ya da tip adı,
//     kendi bloğunda örtük ad (o, birikim, hata);
//   - adlı argüman: çağrılan dilin öğesiyse parametre kodu (denetci.AdBaglari).
//
// Tip adının tek yazımı vardır (3.14): kabul edilmeyen bir yazım (tam) tip adı değildir,
// yazıldığı gibi kalır; denetleyici onu hata olarak bildirir.
func (c *cozumleme) dilOgesi(r sozcuk.Kayit) kod.Kod {
	konum := [2]int{r.Line, r.Column}
	ad := r.Literal
	switch c.roller[konum] {
	case ayristir.RolTip:
		return c.bul(kod.AlanTip, ad)
	case ayristir.RolUye, ayristir.RolYapiAlani, ayristir.RolYapiMetodu:
		if ad == c.dil.Ad(kod.A102) {
			return kod.A102
		}
		return c.bul(kod.AlanUye, ad)
	case ayristir.RolTest:
		return c.baglamsal(kod.A101, ad)
	case ayristir.RolOlarak:
		return c.baglamsal(kod.A100, ad)
	}
	if bg, ok := c.baglar[konum]; ok && bg.Sozcuk.Literal == ad {
		switch bg.Tur {
		case denetci.BagYerlesik, denetci.BagOrtuk, denetci.BagAdli:
			return bg.Kod
		}
	}
	return kod.Yok
}

// bul adın c.dil'de verilen ad alanındaki canlı kodudur.
func (c *cozumleme) bul(al kod.Alan, ad string) kod.Kod {
	if k, ok := c.dil.Bul(al, ad); ok {
		return k
	}
	return kod.Yok
}

// baglamsal ad c.dil'de k bağlamsal sözcüğünün adıysa k'dir.
func (c *cozumleme) baglamsal(k kod.Kod, ad string) kod.Kod {
	if c.dil.Ad(k) == ad {
		return k
	}
	return kod.Yok
}
