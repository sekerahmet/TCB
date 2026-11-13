package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Hata yardımcıları dilin adlarını koddan alır: yerleşik ve tip adları Go metninde yazılmaz.
func TestArgErrorsFormat(t *testing.T) {
	if got := ArgTypeError(codes.FnZeros, 1, &StringVal{Val: "a"}, codes.TypeInt).Error(); got != `sıfırlar() 1. argüman olarak Tam bekler, Metin ("a") verildi` {
		t.Errorf("tip hatası: %s", got)
	}
	if got := ArgTypeError(codes.TypeInt, 1, Null, codes.TypeInt, codes.TypeFloat, codes.TypeString).Error(); got != `Tam() 1. argüman olarak Tam, Ondalık ya da Metin bekler, YokTipi (yok) verildi` {
		t.Errorf("çok tipli tip hatası: %s", got)
	}
	if got := ArgCountError(codes.FnRound, Counts(1, 2), 3, "x", "x, basamak").Error(); got != "yuvarla() 1 ya da 2 argüman bekler, 3 verildi (kullanım: yuvarla(x) ya da yuvarla(x, basamak))" {
		t.Errorf("sayı hatası: %s", got)
	}
	if got := ArgCountError(codes.MemFilter, 1, 0).Error(); got != "liste.filtrele() 1 argüman bekler, 0 verildi" {
		t.Errorf("üye sayı hatası: %s", got)
	}
	// Kural (4D denetimi, ayrı not): beklenen sayı iletiye sayı olarak gider, çoğul ona göre seçilir
	// ("takes 1 argument"); birden çok sayı ve alt sınır iletinin dilinde yazılan parçadır.
	for _, c := range []struct {
		err error
		en  string
	}{
		{ArgCountError(codes.FnAbs, 1, 2), "abs() takes 1 argument, 2 given"},
		{ArgCountError(codes.FnAbs, 2, 1), "abs() takes 2 arguments, 1 given"},
		{ArgCountError(codes.FnRound, Counts(1, 2), 3), "round() takes 1 or 2 arguments, 3 given"},
		{ArgCountError(codes.TypePath, AtLeast(1), 0), "takes 1 or more arguments, 0 given"},
	} {
		if got := messages.ErrorIn(c.err, "en"); !strings.Contains(got, c.en) {
			t.Errorf("İngilizce sayı hatası %q, %q bekleniyordu", got, c.en)
		}
	}
	if got := Usage(codes.FnInput, "", "istem").MessageText(codes.SourceLanguage); got != "girdi() ya da girdi(istem)" {
		t.Errorf("kullanım: %s", got)
	}
	// Kural: kullanım biçimindeki dil adı (alıcı tipi) kod başvurusudur, iletinin dilinde yazılır.
	err := ArgCountError(codes.FnJsonParse, 1, 0, codes.ReceiverRef(codes.TypeString))
	if en, tr := messages.ErrorIn(err, "en"), messages.ErrorIn(err, "tr"); !strings.Contains(en, "json_parse(string)") || !strings.Contains(tr, "(metin)") {
		t.Errorf("kullanımdaki alıcı adı iletinin dilinde olmalı: %q / %q", en, tr)
	}
	// Büyük değerler özetlenir: hata mesajı binlerce elemanı yazdırmaz
	upper := &ListVal{}
	for i := 0; i < 10000; i++ {
		upper.Elements = append(upper.Elements, NewInt(int64(i)))
	}
	if got := summaryIn(upper, codes.Source()); got != "[0, 1, 2, …], 10000 eleman" {
		t.Errorf("liste özeti: %s", got)
	}
	long := summaryIn(&StringVal{Val: strings.Repeat("ğ", 100)}, codes.Source())
	if !strings.HasSuffix(long, `…"`) || len([]rune(long)) != summaryLimit+2 {
		t.Errorf("metin özeti: %s", long)
	}
	if got := TypeAndSummary(nil).Error(); got != "hiçbir değer" {
		t.Errorf("nil: %s", got)
	}
}

// Kural (13.11): çekirdek metodun sayı hatası (H2011) denetleyicininkiyle aynıdır: ad ve kullanım
// programdaki kimlikle yazılır; görünüm adı (list.sort) ve argümansız metodun boş kullanımı yerine
// Go'nun <nil>'i iletiye girmez.
func TestMemberArgCountUsesIdentities(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"[3, 1].TR_reverse(1)", "`.TR_reverse()`"},
		{"[3, 1].TR_sort((x) => x, 1)", "`.TR_sort([function])`"},
		{"[\"a\"].TR_join(\" \", 1)", "`.TR_join([separator])`"},
		{"[1].TR_append()", "`.TR_append(eleman, ...)`"},
	} {
		_, _, err := runTROut(t, c.source+"\n")
		got := messages.ErrorIn(err, "en")
		if !messages.Has(err, messages.H2011) || !strings.Contains(got, c.want) || strings.Contains(got, "<nil>") || strings.Contains(got, "list.") {
			t.Errorf("%s: kimlikli H2011 beklenirdi (%s), gelen %q", c.source, c.want, got)
		}
	}
}

// Kural (13.11, ön şart): çekirdek metotların (Metin, Liste, Sözlük, Aralık) argüman sayısı tanım
// dosyasının imzasıyla aynıdır; denetleyici sayıyı oradan okur (zorunlu, '?' isteğe bağlı, '*' ve
// '+' sınırsız).
func TestMethodCountsMatchDefinitions(t *testing.T) {
	n := 0
	for k, cm := range coreMethods {
		def, _ := codes.DefinitionOf(k)
		for typ, m := range []*builtinMethod{cm.text, cm.listVal, cm.dictVal, cm.rangeVal} {
			if m == nil {
				continue
			}
			n++
			sig, ok := memberSignatureOf(def, typeCodes[typ])
			if !ok {
				t.Errorf("%s: %s tanımda bu tipte yok", k, typeCodes[typ])
				continue
			}
			if minN, maxN := sig.PositionalCounts(); minN != m.sig.minN || maxN != m.sig.maxN {
				t.Errorf("%s (%s): motor %d..%d argüman alıyor, tanım %d..%d", k, typeCodes[typ], m.sig.minN, m.sig.maxN, minN, maxN)
			}
		}
	}
	if n < 20 {
		t.Errorf("yalnız %d metot sınandı", n)
	}
}

// Değer özeti metni kesmeden önce maskeler (19.4): kesim gizli değerin ortasına düşse de başı iletide
// kalmaz. Her satır çıktıya giden metni, VM.Mask'tan geçmiş iletiyi denetler.
func TestSummaryIsCutAfterMask(t *testing.T) {
	t.Cleanup(ClearRegisteredSecrets)
	v := New()
	shown := func(given Value) string {
		return v.Mask(ArgTypeError(codes.FnZeros, 1, given, codes.TypeInt).Error())
	}
	text := func(s string) Value { return &StringVal{Val: s} }
	// Adres: 61 karakter; maskesiz kesimde "Vekil20" kalırdı.
	address := "postgres://uygulama_kullanicisi:Vekil2026@db.sirket.local/ana"
	v.SetArgs([]string{address})
	if got := shown(text(address)); strings.Contains(got, "Vekil20") {
		t.Errorf("adresin parolasının başı kalmamalı: %q", got)
	}
	key := "sk-ozet-anahtari-0123456789abcdefghijklmnopqr"
	if len(key) != 45 {
		t.Fatalf("anahtar %d karakter", len(key))
	}
	v.AddSecret(key)
	if got := shown(text(key)); strings.Contains(got, key[:8]) {
		t.Errorf("adla gizli değerin başı kalmamalı: %q", got)
	}
	// Uzun liste (H2425): öğe önce kendi özetinden (metin değeri) geçer; bu satır öğenin yolunu sınar.
	long := &ListVal{Elements: []Value{text(key), NewInt(1), NewInt(2), NewInt(3), NewInt(4), NewInt(5)}}
	if got := shown(long); strings.Contains(got, key[:8]) {
		t.Errorf("uzun listenin önizlemesinde anahtarın başı kalmamalı: %q", got)
	}
	// Kısa liste: köşeli ayraç ve tırnakla metin 40 karakteri aşar, öbür değerlerin yolundan kesilir.
	if got := shown(&ListVal{Elements: []Value{text(key)}}); strings.Contains(got, key[:8]) {
		t.Errorf("kısa listenin özetinde anahtarın başı kalmamalı: %q", got)
	}
	// Kısa değer kesilmez; çıktıda maskelenir.
	v.AddSecret("kisa-gizli-2026")
	if got := shown(text("kisa-gizli-2026")); !strings.Contains(got, SecretMask) || strings.Contains(got, "kisa-gizli") {
		t.Errorf("kısa gizli değer *** görünmeli: %q", got)
	}
	// Kayıtsız adresin kısa parolası (bağlamlı maske): 42 karakter; maskesiz kesimde '@' düşer, "Kisa"
	// kalırdı. Maskeli metin 40 karakterdir, kesilmez.
	unregistered := "http://" + strings.Repeat("a", 27) + ":Kisa1@h"
	if got := shown(text(unregistered)); strings.Contains(got, "Kisa") || !strings.Contains(got, SecretMask) {
		t.Errorf("kayıtsız adresin parolası kesimden önce örtülmeli: %q", got)
	}
	// Tırnaklı gizli değerin tek öğeli listesi (H2424): liste metni değeri kaçışlı yazar; özette
	// değerin başı kaçışlı biçimiyle de kalmaz.
	quoted := "sk-tirnak\"li-anahtar-0123456789abcdefghijklmn"
	if len(quoted) != 45 {
		t.Fatalf("anahtar %d karakter", len(quoted))
	}
	v.AddSecret(quoted)
	if got := shown(&ListVal{Elements: []Value{text(quoted)}}); strings.Contains(got, "sk-tirn") {
		t.Errorf("listedeki tırnaklı anahtarın başı kalmamalı: %q", got)
	}
}

func memberSignatureOf(def codes.Definition, typ codes.Code) (codes.Signature, bool) {
	for _, u := range def.Members {
		if u.Type == typ {
			return u.Signature, true
		}
	}
	return codes.Signature{}, false
}
