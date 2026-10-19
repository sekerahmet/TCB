// Package process, programın içinde çalıştığı süreçle ilgili TR yerleşiklerini içerir:
// ortam değişkenini okuyan ortam_değişkeni (G800) ve betiğin komut satırı seçeneklerini
// tanımlayıp dolduran komut_satırı (G801). Adı (Türkçe kuralla) gizli bir sözcük içeren
// ortam değişkenlerinin ve seçeneklerin değerleri yorumlayıcının gizli değer süzgecine
// verilir (yurut.VM.GizliDegerEkle): insanın göreceği çıktılarda *** olarak görünürler.
//
// Komut satırının sonucu (Durum.Ayarlar) başka motorlar için de saklanır: eğitim günlüğü
// (internal/stdlib/logging) onu başlangıç kaydına yazar, kontrol noktası da kaydına koyabilir.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package process

import (
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Durum bir programın (VM'nin) süreç yerleşiklerinin durumudur: komut_satırı çağrıldı mı
// ve ne döndürdü. Kaydet her VM için bir tane kurar.
type Durum struct {
	mu       sync.Mutex
	cagrildi bool
	ayarlar  *interp.DictVal
}

// Ayarlar komut_satırı'nın döndürdüğü sözlüktür; komut_satırı çağrılmadıysa nil. Dönen
// sözlük programın değiştirebileceği canlı değerdir; yazmadan önce okunmalıdır (yorumlayıcı
// kilidi altında ya da program bitince).
func (d *Durum) Ayarlar() *interp.DictVal {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ayarlar
}

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	adAciklama    = bind.KodAd(codes.G910)
	adAciklamalar = bind.KodAd(codes.G911)
)

// Kaydet süreç yerleşiklerini VM'ye kaydeder ve programın süreç durumunu döndürür.
func Kaydet(v *interp.VM) *Durum {
	d := &Durum{}
	v.RegisterNoNamed(codes.G800, ortamDegiskeni)
	v.RegisterBuiltinCode(codes.G801, func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(codes.Name(codes.G801), func() (interp.Value, error) { return d.komutSatiri(m, args, named) })
	})
	return d
}

// gizliSozcukler adın sözcüklerinden biri bunlardan biriyse değer gizli sayılır (ASCII,
// büyük harf; sözcükler karşılaştırmadan önce ASCII'ye çevrilip büyütülür: şifresi →
// SIFRESI). Eşleşme TAM SÖZCÜKLE yapılır: TOKENIZER, MAX_TOKENS, KEYBOARD, PASSAGE,
// BYPASS gizli değildir. Bitişik yazılan yaygın bileşikler (APIKEY, AUTHTOKEN) ayrıca
// listededir.
var gizliSozcukler = map[string]bool{
	"TOKEN": true, "KEY": true, "SECRET": true, "PASSWORD": true, "PASSWD": true, "PASS": true,
	"PASSPHRASE": true, "AUTH": true, "CREDENTIAL": true, "CREDENTIALS": true, "PRIVATE": true,
	"APIKEY": true, "ACCESSKEY": true, "SECRETKEY": true, "PRIVATEKEY": true, "AUTHTOKEN": true,
	"ACCESSTOKEN": true, "APITOKEN": true,
	"GIZLI": true, "SIFRE": true, "SIFRESI": true, "PAROLA": true, "PAROLASI": true,
	"ANAHTAR": true, "ANAHTARI": true,
}

// GizliAdMi bir ortam değişkeninin ya da komut satırı seçeneğinin adı değerinin gizli
// olduğunu söylüyor mu (HF_TOKEN, OPENAI_API_KEY, apiKey, db_şifresi ...)? Ad sözcüklere
// bölünür (_, -, ., boşluk gibi ayırıcılar ve küçükten büyüğe harf geçişleri: apiKey → api
// Key) ve her sözcük, Türkçe harfler ASCII karşılıklarıyla ve büyük/küçük harf ayrımı
// olmadan, gizliSozcukler ile tam olarak karşılaştırılır.
func GizliAdMi(ad string) bool {
	for _, s := range sozcuklereBol(ad) {
		if gizliSozcukler[strings.ToUpper(turkish.ASCII(s))] {
			return true
		}
	}
	return false
}

// sozcuklereBol adı sözcüklerine ayırır: harf ve rakam olmayan karakterler ayırıcıdır;
// küçük harften (ya da rakamdan) büyük harfe geçiş ve büyük harf dizisinin sonundaki
// büyük+küçük harf (APIKey → API Key) yeni bir sözcük başlatır.
func sozcuklereBol(ad string) []string {
	var out []string
	r := []rune(ad)
	bas := -1
	for i, c := range r {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			if bas >= 0 {
				out = append(out, string(r[bas:i]))
				bas = -1
			}
			continue
		}
		if bas >= 0 && unicode.IsUpper(c) {
			onceki := r[i-1]
			sonrakiKucuk := i+1 < len(r) && unicode.IsLower(r[i+1])
			if unicode.IsLower(onceki) || unicode.IsDigit(onceki) || (unicode.IsUpper(onceki) && sonrakiKucuk) {
				out = append(out, string(r[bas:i]))
				bas = i
			}
		}
		if bas < 0 {
			bas = i
		}
	}
	if bas >= 0 {
		out = append(out, string(r[bas:]))
	}
	return out
}

// gizliEkle gizli adlı bir değişkenin ya da seçeneğin değerini gizli değer süzgecine verir.
// Yalnızca sayı ya da mantık değeri olan değerler kaydedilmez: gizli bir değer neredeyse hiç
// saf sayı değildir ve onu maskelemek programın bütün sayı çıktılarını bozardı.
func gizliEkle(m *interp.VM, deger string) {
	if m == nil || sayiYaDaMantikMi(deger) {
		return
	}
	m.AddSecret(deger)
}

// sayiYaDaMantikMi değer (boşluklar atılınca) bir sayı ya da mantık değeri mi?
func sayiYaDaMantikMi(s string) bool {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "true", "false", codes.Name(codes.A040), codes.Name(codes.A041):
		return true
	}
	if _, _, err := sayiBicimi(s); err == nil {
		return true
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
