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
	if got := ArgTypeError(codes.FnInt, 1, Null, codes.TypeInt, codes.TypeFloat, codes.TypeString).Error(); got != `tam() 1. argüman olarak Tam, Ondalık ya da Metin bekler, Yok (yok) verildi` {
		t.Errorf("çok tipli tip hatası: %s", got)
	}
	if got := ArgCountError(codes.FnRange, Counts(1, 2), 3, "bitiş", "başlangıç, bitiş").Error(); got != "aralık() 1 ya da 2 argüman bekler, 3 verildi (kullanım: aralık(bitiş) ya da aralık(başlangıç, bitiş))" {
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
		{ArgCountError(codes.FnRange, Counts(1, 2), 3), "range() takes 1 or 2 arguments, 3 given"},
		{ArgCountError(codes.FnPath, AtLeast(1), 0), "takes 1 or more arguments, 0 given"},
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
