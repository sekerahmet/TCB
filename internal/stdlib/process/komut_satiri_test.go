package process_test

import (
	"io"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
)

const secenekler = `{öğrenme_oranı: 0.001, adım: 5000, veri: "veri/train", katmanlar: [64, 64], hızlı: yanlış, etiket: yok, hf_token: ""}`

// komutSatiri komut_satırı'nı verilen argümanlarla çalıştırır; sonucu (sözlüğün görünümü)
// ya da hatayı döndürür.
func komutSatiri(t *testing.T, kod string, argumanlar ...string) (string, error, *interp.VM) {
	t.Helper()
	m := interp.New()
	m.Out, m.Err = io.Discard, io.Discard
	m.SourceFile = "egit.tr"
	m.ArgumanlariAyarla(argumanlar)
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi: %v", p.Errors())
	}
	v, err := m.Run(prog)
	if err != nil {
		return "", err, m
	}
	return v.Inspect(), nil, m
}

func TestKomutSatiriDegerleri(t *testing.T) {
	kod := "komut_satırı(" + secenekler + ")"
	for _, d := range []struct {
		arg  []string
		want string
	}{
		{nil, `{"öğrenme_oranı": 0.001, "adım": 5000, "veri": "veri/train", "katmanlar": [64, 64], "hızlı": yanlış, "etiket": yok, "hf_token": ""}`},
		{[]string{"--adım", "5_000", "--öğrenme_oranı=1e-3", "--hızlı", "--katmanlar", "128, 32,8", "--etiket", "deneme"},
			`{"öğrenme_oranı": 0.001, "adım": 5000, "veri": "veri/train", "katmanlar": [128, 32, 8], "hızlı": doğru, "etiket": "deneme", "hf_token": ""}`},
		{[]string{"--öğrenme_oranı", "3", "--hızlı=yanlış", "--katmanlar=", "--veri", "-x"},
			`{"öğrenme_oranı": 3.0, "adım": 5000, "veri": "-x", "katmanlar": [], "hızlı": yanlış, "etiket": yok, "hf_token": ""}`},
		{[]string{"--adım", "-5"}, `{"öğrenme_oranı": 0.001, "adım": -5, "veri": "veri/train", "katmanlar": [64, 64], "hızlı": yanlış, "etiket": yok, "hf_token": ""}`},
	} {
		got, err, _ := komutSatiri(t, kod, d.arg...)
		if err != nil || got != d.want {
			t.Errorf("%v:\n  %s (%v)\nbeklenen\n  %s", d.arg, got, err, d.want)
		}
	}
}

// Kullanıcının argüman hataları programı 2 koduyla bitirme isteğidir; mesaj yol gösterir.
func TestKomutSatiriKullanimHatalari(t *testing.T) {
	kod := "komut_satırı(" + secenekler + ")"
	for _, d := range []struct {
		arg  []string
		want string
	}{
		{[]string{"--ogrenme_orani", "0.01"}, "Hata: bilinmeyen seçenek '--ogrenme_orani'. Bunu mu demek istediniz: '--öğrenme_oranı'? Seçenekler için: tr egit.tr --yardım"},
		{[]string{"--öğrenme-oranı", "0.01"}, "Bunu mu demek istediniz: '--öğrenme_oranı'?"},
		{[]string{"--help"}, "Bunu mu demek istediniz: '--yardım'?"},
		{[]string{"-adım", "5"}, "bilinmeyen seçenek '-adım'. Bunu mu demek istediniz: '--adım'?; seçenekler iki tireyle yazılır"},
		{[]string{"--adım", "5", "--adım", "6"}, "'--adım' iki kez verildi"},
		{[]string{"--adım", "beş"}, `'--adım' Tam bekler, "beş" verildi.`},
		{[]string{"--adım", "3.0"}, "tam sayı nokta ya da üs içermez"},
		{[]string{"--öğrenme_oranı", "0,001"}, "ondalık ayırıcı nokta olmalı (0.001)"},
		{[]string{"--katmanlar", "1,x"}, `virgülle ayrılmış Tam listesi bekler; 2. öğe "x"`},
		{[]string{"--hızlı=evet"}, "doğru ya da yanlış verin"},
		{[]string{"--adım"}, "'--adım' bir değer bekler (ör. --adım 5000 ya da --adım=5000)"},
		{[]string{"--veri", "--adım", "5"}, "'--veri' bir değer bekler"},
		{[]string{"veri/"}, "'veri/' bir seçenek değil: argümanlar --ad değer biçiminde verilir (konumlu argüman yoktur)"},
		{[]string{"--"}, "'--' tek başına kullanılmaz"},
		{[]string{"--veri", strings.Repeat("a", 1<<20+1)}, "değeri çok uzun"},
	} {
		_, err, _ := komutSatiri(t, kod, d.arg...)
		c, ok := interp.CikisIstegiMi(err)
		if !ok || c.Kod != 2 || !strings.Contains(c.Mesaj, d.want) || !strings.HasPrefix(c.Mesaj, "Hata: ") {
			t.Errorf("%.60v: 2 kodlu, %q içeren çıkış isteği bekleniyordu: %v", d.arg, d.want, err)
		}
	}
}

// --yardım ve -h seçenekleri listeleyip programı 0 koduyla bitirir; hatalı argümanlar olsa da.
func TestKomutSatiriYardim(t *testing.T) {
	kod := `komut_satırı(` + secenekler + `, açıklama: "Küçük dil modelini eğitir", açıklamalar: {öğrenme_oranı: "öğrenme oranı", adım: "eğitim adımı sayısı"})`
	want := `Kullanım: tr egit.tr [seçenekler]

Küçük dil modelini eğitir

Seçenekler:
  --öğrenme_oranı <Ondalık>   öğrenme oranı (varsayılan: 0.001)
  --adım <Tam>                eğitim adımı sayısı (varsayılan: 5000)
  --veri <Metin>              (varsayılan: veri/train)
  --katmanlar <Tam,Tam,...>   (varsayılan: 64,64)
  --hızlı                     (varsayılan: yanlış)
  --etiket <Metin>            (varsayılan: yok)
  --hf_token <Metin>          (varsayılan: "")
  --yardım, -h                bu yardımı gösterir`
	for _, arg := range [][]string{{"--yardım"}, {"-h"}, {"--adım", "x", "-h"}} {
		_, err, _ := komutSatiri(t, kod, arg...)
		c, ok := interp.CikisIstegiMi(err)
		if !ok || c.Kod != 0 || c.Mesaj != want {
			t.Errorf("%v: yardım bekleniyordu:\n%v", arg, err)
		}
	}
}

// Kurulum hataları sıradan, yakalanabilir hatalardır.
func TestKomutSatiriKurulumHatalari(t *testing.T) {
	for _, d := range []struct{ kod, want string }{
		{`komut_satırı({uzunluk: 3})`, `"uzunluk" seçenek adı olamaz: Sözlük tipinin bir üyesidir`},
		{`komut_satırı({anahtarlar: 3})`, `"anahtarlar" seçenek adı olamaz`},
		{`komut_satırı({yardım: doğru})`, `"yardım" seçenek adı olamaz`},
		{`komut_satırı({"a b": 1})`, `"a b" seçenek adı olamaz`},
		{`komut_satırı({1: 1})`, "seçenek adları Metin olmalıdır"},
		{`komut_satırı({a: {b: 1}})`, "'a' seçeneğinin varsayılanı Tam, Ondalık, Metin ya da Mantık, Liste ya da yok olmalı"},
		{`komut_satırı({a: [1, "x"]})`, "öğeler aynı tipte olmalı"},
		{`komut_satırı({a: [[1]]})`, "yalnızca aynı tipte"},
		{`komut_satırı({a: 1}, açıklamalar: {b: "x"})`, "açıklamalar içindeki 'b' bir seçenek değil"},
		{`komut_satırı({adım: 1}, açıklamalar: {adim: "x"})`, "Bunu mu demek istediniz: 'adım'?"},
		{`komut_satırı({a: 1}, açıklama: 5)`, "açıklama: Metin olmalıdır"},
		{`komut_satırı({a: 1}, açklama: "x")`, "Bunu mu demek istediniz: 'açıklama'?"},
		{`komut_satırı([1])`, "1. argüman olarak Sözlük bekler"},
		{`komut_satırı({a: 1}); komut_satırı({a: 1})`, "programda bir kez çağrılır"},
	} {
		_, err, _ := komutSatiri(t, d.kod)
		if _, cikis := interp.CikisIstegiMi(err); err == nil || cikis || !strings.Contains(err.Error(), d.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", d.kod, d.want, err)
		}
	}
	// Kurulum hatası çağrıyı tüketmez; dene/yakala yakalar.
	if got, err, _ := komutSatiri(t, `dene { komut_satırı({uzunluk: 1}) } yakala { }
komut_satırı({a: 1})`); err != nil || got != `{"a": 1}` {
		t.Errorf("hatalı kurulumdan sonra çağrı geçerli olmalı: %s %v", got, err)
	}
}

// Çıkış isteği dene/yakala ile yakalanmaz; ertele blokları çalışır.
func TestCikisIstegiYakalanmaz(t *testing.T) {
	_, err, m := komutSatiri(t, `değişken iz = []
fonksiyon kur() {
    ertele iz.ekle("ertele")
    dene { sonuç komut_satırı({a: 1}) } yakala h { iz.ekle("yakalandı") }
}
ertele yazdır(iz)
kur()`, "--b")
	if c, ok := interp.CikisIstegiMi(err); !ok || c.Kod != 2 {
		t.Fatalf("çıkış isteği bekleniyordu: %v", err)
	}
	if v, _ := m.GlobalEnv.Get("iz"); v == nil || v.Inspect() != `["ertele"]` {
		t.Errorf("ertele çalışmalı, dene yakalamamalı: %v", v)
	}
}

// Gizli adlı seçeneğin (sayı olmayan) değeri süzgece girer; sayı değerli olan girmez.
func TestKomutSatiriGizliDeger(t *testing.T) {
	_, err, m := komutSatiri(t, `komut_satırı({hf_token: "", max_tokens: 0, api_key: ""})`, "--hf_token", "hf_abcdefgh123", "--max_tokens", "131072", "--api_key=123456789")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Maskele("a hf_abcdefgh123 131072 123456789"); got != "a *** 131072 123456789" {
		t.Errorf("maskeleme: %q", got)
	}
	// Bilinmeyen gizli adlı seçeneğin değeri de hata mesajında görünmez.
	_, err, m = komutSatiri(t, `komut_satırı({a: 1})`, "--hf_token_x=hf_yazimhatasi")
	if c, ok := interp.CikisIstegiMi(err); !ok || strings.Contains(c.Mesaj, "hf_yazimhatasi") || m.Maskele("hf_yazimhatasi") != interp.GizliMaske {
		t.Errorf("bilinmeyen gizli seçeneğin değeri mesajda olmamalı ve süzgece girmeli: %v", err)
	}
}
