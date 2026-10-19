// Package process, programın içinde çalıştığı süreçle ilgili TR yerleşiklerini içerir:
// ortam değişkenini okuyan ortam_değişkeni (G800) ve betiğin komut satırı seçeneklerini
// tanımlayıp dolduran komut_satırı (G801). Adı (Türkçe kuralla) gizli bir sözcük içeren
// ortam değişkenlerinin ve seçeneklerin değerleri yorumlayıcının gizli değer süzgecine
// verilir (interp.VM.AddSecret): insanın göreceği çıktılarda *** olarak görünürler.
//
// Komut satırının sonucu (State.Settings) başka motorlar için de saklanır: eğitim günlüğü
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

// State bir programın (VM'nin) süreç yerleşiklerinin durumudur: komut_satırı çağrıldı mı
// ve ne döndürdü. Register her VM için bir tane kurar.
type State struct {
	mu       sync.Mutex
	called   bool
	settings *interp.DictVal
}

// Settings komut_satırı'nın döndürdüğü sözlüktür; komut_satırı çağrılmadıysa nil. Dönen
// sözlük programın değiştirebileceği canlı değerdir; yazmadan önce okunmalıdır (yorumlayıcı
// kilidi altında ya da program bitince).
func (d *State) Settings() *interp.DictVal {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.settings
}

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	nameDesc  = bind.NamedArg(codes.G910)
	nameDescs = bind.NamedArg(codes.G911)
)

// Register süreç yerleşiklerini VM'ye kaydeder ve programın süreç durumunu döndürür.
func Register(v *interp.VM) *State {
	d := &State{}
	v.RegisterNoNamed(codes.G800, envVar)
	v.RegisterBuiltinCode(codes.G801, func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Safe(codes.Name(codes.G801), func() (interp.Value, error) { return d.commandLine(m, args, named) })
	})
	return d
}

// secretWords adın sözcüklerinden biri bunlardan biriyse değer gizli sayılır (ASCII,
// büyük harf; sözcükler karşılaştırmadan önce ASCII'ye çevrilip büyütülür: şifresi →
// SIFRESI). Eşleşme TAM SÖZCÜKLE yapılır: TOKENIZER, MAX_TOKENS, KEYBOARD, PASSAGE,
// BYPASS gizli değildir. Bitişik yazılan yaygın bileşikler (APIKEY, AUTHTOKEN) ayrıca
// listededir.
var secretWords = map[string]bool{
	"TOKEN": true, "KEY": true, "SECRET": true, "PASSWORD": true, "PASSWD": true, "PASS": true,
	"PASSPHRASE": true, "AUTH": true, "CREDENTIAL": true, "CREDENTIALS": true, "PRIVATE": true,
	"APIKEY": true, "ACCESSKEY": true, "SECRETKEY": true, "PRIVATEKEY": true, "AUTHTOKEN": true,
	"ACCESSTOKEN": true, "APITOKEN": true,
	"GIZLI": true, "SIFRE": true, "SIFRESI": true, "PAROLA": true, "PAROLASI": true,
	"ANAHTAR": true, "ANAHTARI": true,
}

// IsSecretName bir ortam değişkeninin ya da komut satırı seçeneğinin adı değerinin gizli
// olduğunu söylüyor mu (HF_TOKEN, OPENAI_API_KEY, apiKey, db_şifresi ...)? Ad sözcüklere
// bölünür (_, -, ., boşluk gibi ayırıcılar ve küçükten büyüğe harf geçişleri: apiKey → api
// Key) ve her sözcük, Türkçe harfler ASCII karşılıklarıyla ve büyük/küçük harf ayrımı
// olmadan, secretWords ile tam olarak karşılaştırılır.
func IsSecretName(name string) bool {
	for _, s := range splitWords(name) {
		if secretWords[strings.ToUpper(turkish.ASCII(s))] {
			return true
		}
	}
	return false
}

// splitWords adı sözcüklerine ayırır: harf ve rakam olmayan karakterler ayırıcıdır;
// küçük harften (ya da rakamdan) büyük harfe geçiş ve büyük harf dizisinin sonundaki
// büyük+küçük harf (APIKey → API Key) yeni bir sözcük başlatır.
func splitWords(name string) []string {
	var out []string
	r := []rune(name)
	start := -1
	for i, c := range r {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			if start >= 0 {
				out = append(out, string(r[start:i]))
				start = -1
			}
			continue
		}
		if start >= 0 && unicode.IsUpper(c) {
			previous := r[i-1]
			nextLower := i+1 < len(r) && unicode.IsLower(r[i+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextLower) {
				out = append(out, string(r[start:i]))
				start = i
			}
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, string(r[start:]))
	}
	return out
}

// addSecret gizli adlı bir değişkenin ya da seçeneğin değerini gizli değer süzgecine verir.
// Yalnızca sayı ya da mantık değeri olan değerler kaydedilmez: gizli bir değer neredeyse hiç
// saf sayı değildir ve onu maskelemek programın bütün sayı çıktılarını bozardı.
func addSecret(m *interp.VM, value string) {
	if m == nil || isNumberOrBool(value) {
		return
	}
	m.AddSecret(value)
}

// addSecretItems gizli bir seçeneğin komut satırındaki değerini ve (liste seçeneğinde)
// virgülle ayrılmış her öğesini gizli değer süzgecine verir: öğe tek başına yazdırılsa ya
// da günlüğe girse de gizlenir.
func addSecretItems(m *interp.VM, value string) {
	addSecret(m, value)
	if strings.Contains(value, ",") {
		for _, p := range strings.Split(value, ",") {
			addSecret(m, strings.TrimSpace(p))
		}
	}
}

// isNumberOrBool değer (boşluklar atılınca) bir sayı ya da mantık değeri mi?
func isNumberOrBool(s string) bool {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "true", "false", codes.Name(codes.A040), codes.Name(codes.A041):
		return true
	}
	if _, _, err := numberFormat(s); err == nil {
		return true
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
