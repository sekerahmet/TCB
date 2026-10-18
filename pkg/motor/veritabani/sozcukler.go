package veritabani

import (
	"fmt"
	"strings"
	"unicode"
)

// Veritabanı sorgu dilinin (vt.sorgula("SEÇ * TABLO personel NEREDE maas > 5")) ve satır
// koşullarının sözcükleri yalnızca Türkçedir: her işin tek yazımı vardır. Büyük/küçük harf
// fark etmez; harfler Türkçe kurala göre büyütülür (seç → SEÇ, limit → LİMİT, değil →
// DEĞİL). İngilizce sözcükler (SELECT, FROM, WHERE, AND, TRUE ...) ve Türkçe karakterleri
// ASCII'ye çevrilmiş yazımlar (SEC, SIL, DOGRU, LIMIT ...) kabul edilmez; yazıldıklarında
// hata mesajı Türkçe karşılığı önerir.
const (
	sqlSec    = "SEÇ"
	sqlSil    = "SİL"
	sqlTablo  = "TABLO"
	sqlNerede = "NEREDE"
	sqlSirala = "SIRALA"
	sqlAzalan = "AZALAN"
	sqlLimit  = "LİMİT"

	kosulVe     = "VE"
	kosulVeya   = "VEYA"
	kosulDegil  = "DEĞİL"
	kosulDogru  = "DOĞRU"
	kosulYanlis = "YANLIŞ"
	kosulYok    = "YOK"
)

// bellekYolu bellekte (diske yazılmadan) açılan veritabanının yoludur: veritabanı_aç(":bellek:").
const bellekYolu = ":bellek:"

var sqlSozcukleri = []string{sqlSec, sqlSil, sqlTablo, sqlNerede, sqlSirala, sqlAzalan, sqlLimit,
	kosulVe, kosulVeya, kosulDegil, kosulDogru, kosulYanlis, kosulYok}

// ingilizceSQL kaldırılan İngilizce (ve eş anlamlı) sözcüklerin Türkçe karşılıklarıdır. Bu
// sözcükler KABUL EDİLMEZ; yalnızca hata mesajında karşılık önerilir.
var ingilizceSQL = map[string]string{
	"SELECT": sqlSec, "DELETE": sqlSil, "FROM": sqlTablo, "DAN": sqlTablo, "WHERE": sqlNerede,
	"ORDER": sqlSirala, "DESC": sqlAzalan, "LIMIT": sqlLimit,
	"AND": kosulVe, "OR": kosulVeya, "NOT": kosulDegil, "TRUE": kosulDogru, "FALSE": kosulYanlis, "NULL": kosulYok,
}

// sqlBuyuk sözcüğü Türkçe kurala göre büyük harfe çevirir (karşılaştırma biçimi).
func sqlBuyuk(s string) string { return strings.ToUpperSpecial(unicode.TurkishCase, s) }

// sqlSozcuguMu w verilen Türkçe sözcük mü (büyük/küçük harf fark etmez)?
func sqlSozcuguMu(w, sozcuk string) bool { return sqlBuyuk(w) == sozcuk }

// sqlOneri w kabul edilmeyen bir yazımsa (İngilizce sözcük ya da Türkçe bir sözcüğün ASCII
// yazımı) Türkçe karşılığını verir.
func sqlOneri(w string) (string, bool) {
	b := sqlBuyuk(w)
	for _, s := range sqlSozcukleri {
		if b == s {
			return "", false
		}
	}
	if t, ok := ingilizceSQL[strings.ToUpper(w)]; ok {
		return t, true
	}
	a := foldUpperASCII(w)
	for _, s := range sqlSozcukleri {
		if foldUpperASCII(s) == a {
			return s, true
		}
	}
	return "", false
}

// sqlIpucu kabul edilmeyen sözcük için Türkçe karşılığı öneren metindir.
func sqlIpucu(w, karsilik string) string {
	return fmt.Sprintf("'%s' TR'de yok; '%s' yazmak mı istediniz?", w, karsilik)
}

// sqlOneriHatasi metindeki (tırnak dışındaki) ilk kabul edilmeyen sözcük için Türkçe
// karşılığı öneren bir hata döndürür; yoksa nil.
func sqlOneriHatasi(on string, s string) error {
	for _, sp := range scanSQLWordsOutsideQuotes(s) {
		if t, ok := sqlOneri(sp.word); ok {
			return fmt.Errorf("%s: %s", on, sqlIpucu(sp.word, t))
		}
	}
	return nil
}
