package codes

import (
	"regexp"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// İki dil vardır (dil tanımı 17.7):
//   - active, programın okunduğu ve çalıştığı kimlik listesidir (IdentityLanguage), hiçbir
//     yazım dili değildir: sözcük çözümleyicinin anahtar kelimeleri ve yerleşiklerin, üyelerin
//     kayıt adları kimliktir (TR_kw_if, TR_fn_print). Böylece bir programın anlamı onu yazanın
//     ya da çalıştıranın yazım diline bağlı değildir (aynı program her makinede aynı adlara,
//     sözlük anahtarlarına ve dosyalara ulaşır).
//   - display, kişinin yazım dilidir (LanguageEnvVar; varsayılan Türkçe): tanı iletileri,
//     yardım ve komut satırı çıktısı ile kodlu dosyanın görünümü bu dildedir (DisplayName).
//     Türkçe, İngilizce ve sonra eklenecek diller yalnızca gösterimdir; hiçbirinin ayrıcalığı yoktur.
// Karşılaştırma: Python ve Swift çalışma anı değerlerini (True, tür adları) hiç yerelleştirmez;
// Excel formülü kanonik saklar, yalnızca ekranda yerelleştirir. TR de anlamı kodda tutar,
// yalnızca gösterimi yerelleştirir. Bir yazım dilinde yazılmış metin (tr kodla - --dil en,
// kabuk satırı) dili açıkça verilerek okunur (lexer.NewBodyIn) ve kodlu biçime çevrilir.

// display LanguageEnvVar'ın seçtiği gösterim dilidir, active programın okunduğu ve çalıştığı
// kimlik listesidir. Kurulamayan bir dil paketi başlatırken süreci kapatmaz
// (paketi kullanan dil sunucusu, codegraph, testler ya da TR'yi gömen bir program da kapanırdı);
// hata LanguageError ile okunur, tr komutu başlarken onu bildirip çıkar:
//   - Geçersiz bir TR_DIL seçimi Türkçeye düşer.
//   - Gömülü Türkçe listenin kendisi kurulamazsa (bozuk ya da yarım değişmiş tr.json) adlar
//     kimliklere düşer (identityLanguage): Go tarafı çalışır, TR metni okunmaz; Türkçeyi
//     isteyen LoadLanguage hatayı verir.
//
// Karşılaştırma (pkg.go.dev, Go 1.25): regexp.MustCompile başlatmadaki hatada panikler (eski
// davranış buydu); sync.OnceValues hatayı ilk kullanana verir. Sözcük çözümleyicinin ve
// denetleyicinin tabloları paket başlatılırken bu listeden kurulduğu için tembel kurulum ilk
// kullanımı ertelemez, yalnız her Name çağrısına bir eşzamanlama okuması ekler; bu yüzden liste
// yine başlatmada kurulur, hata OnceValues'taki gibi ilk kullanana (LoadLanguage,
// LanguageError) saklanır.
var display, active, displayErr, activeErr = startLanguages(messages.ActiveLanguage(), languages)

// startLanguages gösterim dilini ve kimlik listesini langs'tan (dil kodu → dil dosyası; testler bozuk bir
// dosya verir) kurar.
func startLanguages(langCode string, langs map[string][]byte) (display, active *Language, displayErr, activeErr error) {
	// Program içeride kimliklerle çalışır; Türkçe yazım dillerinden biridir.
	source, sourceErr := buildLanguage(langs, SourceLanguage)
	if sourceErr == nil {
		sourceErr = complete(source)
	}
	activeErr = sourceErr
	active = identityLanguage(IdentityLanguage)
	display = source
	if sourceErr != nil {
		display = active
	} else {
		loaded.Store(SourceLanguage, source)
	}
	if langCode != SourceLanguage {
		if d, err := buildLanguage(langs, langCode); err != nil {
			displayErr = messages.New(messages.H2509, LanguageEnvVar, langCode, err)
		} else {
			display = d
		}
	}
	return display, active, displayErr, activeErr
}

// complete Türkçe listede her kodun bir satırı olduğunu denetler: Name eksik kodda paniklerdi,
// ve sözcük çözümleyici ile denetleyici adları paket başlatılırken ister.
func complete(d *Language) error {
	var missing []string
	for k, id := range identities {
		if id != "" && d.nameArr[k] == "" {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return messages.Internalf("code list %s: %d codes are not in the language file: %s", d.Code, len(missing), strings.Join(missing, ", "))
	}
	return nil
}

// identityPattern iletinin parametresinin tamamı olan bir kimliktir (TR_mem_keys, TR_fn_print(),
// liste.TR_mem_append); başka metnin içindeki kimlik (tr dizin'in builtin:TR_fn_sum'ı) ve öneki de
// bir kimlik olan yol (ağırlık anahtarı TR_mem_embedding.TR_mem_table, bir değer) çevrilmez.
var identityPattern = regexp.MustCompile(`^([\p{L}_][\p{L}\p{N}_]*\.)?(` + IdentityPrefix + `[a-z]+_[A-Za-z0-9_]+)(\(\))?$`)

func init() {
	// İletiye giren kimlik iletinin dilindeki adıyla yazılır.
	turkish.CandidateSpelling = func(c string) string {
		if k, err := Parse(c); err == nil && IsReservedName(c) {
			return Source().Name(k)
		}
		return c
	}
	messages.IdentityNames = func(text, lang string) string {
		l, err := LoadLanguage(lang)
		if err != nil {
			l = Source()
		}
		m := identityPattern.FindStringSubmatch(text)
		if m == nil || strings.HasPrefix(m[1], IdentityPrefix) {
			return text // kimliklerden kurulmuş yol (TR_mem_embedding.TR_mem_table) bir değerdir
		}
		if k, err := Parse(m[2]); err == nil {
			if e, ok := l.Entry(k); ok {
				return m[1] + e.Name + m[3]
			}
		}
		return text
	}
}

// IdentityLanguage kimlik listesinin dil kodudur: hiçbir yazım dili değildir.
const IdentityLanguage = "kimlik"

// identityLanguage dil listesi kurulamayınca yerine geçen listedir: her kodun adı kimliğidir
// (TR_kw_if). Kimlikler dilden bağımsızdır ve tekildir, bu yüzden tabloları çakışmasız kurar.
func identityLanguage(langCode string) *Language {
	d := &Language{Code: langCode, entries: make(map[Code]*Entry, len(identities)),
		spellings: make(map[Code][]string, len(identities)), nameArr: make([]string, len(identities))}
	for i := range d.names {
		d.names[i], d.foreignNames[i], d.canonical[i] = map[string]Code{}, map[string][]foreignMatch{}, map[string]string{}
	}
	for i, id := range identities {
		k := Code(i)
		if id == "" || k.Namespace() == NamespaceNone {
			continue
		}
		ns := k.Namespace()
		d.entries[k] = &Entry{Code: k, Name: id}
		d.codeList = append(d.codeList, k)
		d.nameArr[k], d.spellings[k] = id, []string{id}
		d.names[ns][id], d.canonical[ns][id] = k, id
	}
	return d
}

// LanguageError reports why a language the process needs could not be built: the embedded
// Turkish list (names then fell back to identities and no TR text can be read) or the one
// LanguageEnvVar selects (the display language then fell back to Turkish); nil when both are
// valid.
func LanguageError() error {
	if activeErr != nil {
		return activeErr
	}
	return displayErr
}
