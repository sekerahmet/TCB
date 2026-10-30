package process_test

import (
	"io"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
)

const options = `{"öğrenme_oranı": 0.001, "adım": 5000, "veri": "veri/train", "katmanlar": [64, 64], "hızlı": TR_false, "etiket": TR_none, "hf_token": ""}`

// commandLine komut_satırı'nı verilen argümanlarla çalıştırır; sonucu (sözlüğün görünümü),
// yorumlayıcıyı ve hatayı döndürür.
func commandLine(t *testing.T, code string, arguments ...string) (string, *interp.VM, error) {
	t.Helper()
	m := interp.New()
	m.Out, m.Err = io.Discard, io.Discard
	m.SourceFile = "egit.tr"
	m.SetArgs(arguments)
	p := parser.New(lexer.NewBody(code, true))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi: %v", p.Errors())
	}
	v, err := m.Run(prog)
	if err != nil {
		return "", m, err
	}
	return v.Inspect(), m, nil
}

// Seçeneğin adı metin anahtarının kendisidir ve çevrilmez (3.7, 3.14): {"step": 1} yalnız --step
// ile verilir, sonuç sözlüğünde anahtar "step"tir ve köşeli parantezle okunur. Bir dildeki ad
// (--adım) ayrı bir seçenektir; sözlüğün üye adı da (length) seçenek olabilir.
func TestOptionNameIsTheTextKey(t *testing.T) {
	got, _, err := commandLine(t, `TR_var a = TR_command_line({"step": 1, "renk": 2, "length": 3})
[a["step"], a["length"], a.TR_keys()]`, "--step", "5", "--length", "4")
	if want := `[5, 4, ["step", "renk", "length"]]`; err != nil || got != want {
		t.Errorf("%s (%v), beklenen %s", got, err, want)
	}
	if _, _, err := commandLine(t, `TR_command_line({"step": 1})`, "--adım", "3"); err == nil {
		t.Error("seçenek başka bir dildeki adıyla kabul edilmemeli")
	}
}

func TestCommandLineValues(t *testing.T) {
	code := "TR_command_line(" + options + ")"
	for _, d := range []struct {
		arg  []string
		want string
	}{
		{nil, `{"öğrenme_oranı": 0.001, "adım": 5000, "veri": "veri/train", "katmanlar": [64, 64], "hızlı": false, "etiket": none, "hf_token": ""}`},
		{[]string{"--adım", "5_000", "--öğrenme_oranı=1e-3", "--hızlı", "--katmanlar", "128, 32,8", "--etiket", "deneme"},
			`{"öğrenme_oranı": 0.001, "adım": 5000, "veri": "veri/train", "katmanlar": [128, 32, 8], "hızlı": true, "etiket": "deneme", "hf_token": ""}`},
		{[]string{"--öğrenme_oranı", "3", "--hızlı=yanlış", "--katmanlar=", "--veri", "-x"},
			`{"öğrenme_oranı": 3.0, "adım": 5000, "veri": "-x", "katmanlar": [], "hızlı": false, "etiket": none, "hf_token": ""}`},
		{[]string{"--adım", "-5"}, `{"öğrenme_oranı": 0.001, "adım": -5, "veri": "veri/train", "katmanlar": [64, 64], "hızlı": false, "etiket": none, "hf_token": ""}`},
	} {
		got, _, err := commandLine(t, code, d.arg...)
		if err != nil || got != d.want {
			t.Errorf("%v:\n  %s (%v)\nbeklenen\n  %s", d.arg, got, err, d.want)
		}
	}
}

// Kullanıcının argüman hataları programı 2 koduyla bitirme isteğidir; mesaj yol gösterir.
// Çıkış isteği yalnızca metni taşır: burada iletinin son biçimi (Hata: öneki, sondaki nokta,
// öneri) sınanır; hangi iletinin verildiği paketin içinden kodla sınanır (TestParseErrorCodes).
func TestCommandLineUsageErrors(t *testing.T) {
	code := "TR_command_line(" + options + ")"
	for _, d := range []struct {
		arg  []string
		want string
	}{
		{[]string{"--ogrenme_orani", "0.01"}, "Hata: bilinmeyen seçenek '--ogrenme_orani'. Bunu mu demek istediniz: '--öğrenme_oranı'? Seçenekler için: tr egit.tr --help"},
		{[]string{"--öğrenme-oranı", "0.01"}, "Bunu mu demek istediniz: '--öğrenme_oranı'?"},
		{[]string{"--yardim"}, "Bunu mu demek istediniz: '--help'?"},
		{[]string{"-adım", "5"}, "bilinmeyen seçenek '-adım'. Bunu mu demek istediniz: '--adım'?; seçenekler iki tireyle yazılır"},
		{[]string{"--adım", "beş"}, `'--adım' Tam bekler, "beş" verildi. Seçenekler için`},
		{[]string{"--adım", "3.0"}, `'--adım' Tam bekler, "3.0" verildi; tam sayı nokta ya da üs içermez.`},
		{[]string{"--katmanlar", "1,x"}, `'--katmanlar' virgülle ayrılmış Tam listesi bekler; 2. öğe "x".`},
		{[]string{"--hızlı=evet"}, `'--hızlı' Mantık bir seçenek; doğru ya da yanlış verin (yalnız --hızlı yazmak doğru demektir), "evet" verildi.`},
		{[]string{"--adım"}, "'--adım' bir değer bekler (ör. --adım 5000 ya da --adım=5000)"},
		// Mantık seçeneğinin değeri ayrı argüman değildir (Go flag: -flag=false); öneri doğru biçimi verir (B17).
		{[]string{"--hızlı", "yanlış"}, "--hızlı=yanlış"},
		{[]string{"--hızlı", "true"}, "--hızlı=true"},
		// Yardımın her kabul edilen yazımı öneri adayıdır (K1): --hel, --help'e yakındır.
		{[]string{"--hel"}, "Bunu mu demek istediniz: '--help'?"},
	} {
		_, _, err := commandLine(t, code, d.arg...)
		c, ok := interp.IsExitRequest(err)
		if !ok || c.Code != 2 || !strings.Contains(c.Message, d.want) || !strings.HasPrefix(c.Message, "Hata: ") {
			t.Errorf("%.60v: 2 kodlu, %q içeren çıkış isteği bekleniyordu: %v", d.arg, d.want, err)
		}
	}
}

// --help ve -h seçenekleri listeleyip programı 0 koduyla bitirir; hatalı argümanlar olsa da.
func TestCommandLineHelp(t *testing.T) {
	code := `TR_command_line(` + options + `, TR_description: "Küçük dil modelini eğitir", TR_descriptions: {"öğrenme_oranı": "öğrenme oranı", "adım": "eğitim adımı sayısı"})`
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
  --help, -h                  bu yardımı gösterir`
	// --help bir ad kodudur (TR_opt_help): her yazım dilinin adı (--help) da yardımı verir.
	for _, arg := range [][]string{{"--help"}, {"-h"}, {"--adım", "x", "-h"}, {"--help"}} {
		_, _, err := commandLine(t, code, arg...)
		c, ok := interp.IsExitRequest(err)
		if !ok || c.Code != 0 || c.Message != want {
			t.Errorf("%v: yardım bekleniyordu:\n%v", arg, err)
		}
	}
}

// Kural (B17): varsayılanı doğru olan Mantık seçeneğinin yardımı onu kapatma biçimini de gösterir.
func TestCommandLineHelpShowsHowToTurnOff(t *testing.T) {
	_, _, err := commandLine(t, `TR_command_line({"hızlı": TR_true})`, "--help")
	c, ok := interp.IsExitRequest(err)
	if !ok || !strings.Contains(c.Message, "--hızlı=yanlış") {
		t.Errorf("yardım kapatma biçimini göstermeli: %v", err)
	}
}

// Kurulum hataları sıradan, yakalanabilir hatalardır.
func TestCommandLineSetupErrors(t *testing.T) {
	for _, d := range []struct{ code, want string }{
		{`TR_command_line({"a": {"b": 1}})`, "'a' seçeneğinin varsayılanı Tam, Ondalık, Metin ya da Mantık, Liste ya da yok olmalı"},
		{`TR_command_line({"adım": 1}, TR_descriptions: {"adim": "x"})`, "açıklamalar içindeki 'adim' bir seçenek değil. Bunu mu demek istediniz: 'adım'?"},
		{`TR_command_line([1])`, "1. argüman olarak Sözlük bekler"},
	} {
		_, _, err := commandLine(t, d.code)
		if _, exit := interp.IsExitRequest(err); err == nil || exit || !strings.Contains(err.Error(), d.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", d.code, d.want, err)
		}
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`TR_command_line({"help": TR_true})`, messages.H7221},
		{`TR_command_line({"h": TR_true})`, messages.H7221},
		{`TR_command_line({"a b": 1})`, messages.H7220},
		{`TR_command_line({"-a": 1})`, messages.H7220},
		{`TR_command_line({1: 1})`, messages.H7219},
		{`TR_command_line({"a": {"b": 1}})`, messages.H7225},
		{`TR_command_line({"a": [1, "x"]})`, messages.H7224},
		{`TR_command_line({"a": [[1]]})`, messages.H7223},
		{`TR_command_line({"lr": 1}, TR_descriptions: {"b": "x"})`, messages.H7228},
		{`TR_command_line({"adım": 1}, TR_descriptions: {"adim": "x"})`, messages.H7229},
		{`TR_command_line({"a": 1}, TR_descriptions: {"a": 5})`, messages.H7230},
		{`TR_command_line({"a": 1}, TR_descriptions: 5)`, messages.H7227},
		{`TR_command_line({"a": 1}, TR_description: 5)`, messages.H7226},
		{`TR_command_line({"a": 1}, açklama: "x")`, messages.H7900},
		{`TR_command_line({"a": 1}); TR_command_line({"a": 1})`, messages.H7218},
	} {
		_, _, err := commandLine(t, d.code)
		if _, exit := interp.IsExitRequest(err); exit || !messages.Has(err, d.want) {
			t.Errorf("%s: %s bekleniyordu: %v", d.code, d.want, err)
		}
	}
	// Kurulum hatası çağrıyı tüketmez; dene/yakala yakalar.
	if got, _, err := commandLine(t, `TR_try { TR_command_line({"a b": 1}) } TR_catch { }
TR_command_line({"a": 1})`); err != nil || got != `{"a": 1}` {
		t.Errorf("hatalı kurulumdan sonra çağrı geçerli olmalı: %s %v", got, err)
	}
}

// Çıkış isteği dene/yakala ile yakalanmaz; ertele blokları çalışır.
func TestExitRequestNotCaught(t *testing.T) {
	_, m, err := commandLine(t, `TR_var iz = []
TR_func kur() -> TR_Dict[TR_String, TR_Int] {
    TR_defer iz.TR_append("ertele")
    TR_try { TR_return TR_command_line({"a": 1}) } TR_catch h { iz.TR_append("yakalandı") }
}
TR_defer TR_print(iz)
kur()`, "--b")
	if c, ok := interp.IsExitRequest(err); !ok || c.Code != 2 {
		t.Fatalf("çıkış isteği bekleniyordu: %v", err)
	}
	if v, _ := m.GlobalEnv.Get("iz"); v == nil || v.Inspect() != `["ertele"]` {
		t.Errorf("ertele çalışmalı, dene yakalamamalı: %v", v)
	}
}

// Gizli adlı seçeneğin (sayı olmayan) değeri süzgece girer; sayı değerli olan girmez.
func TestCommandLineSecretValue(t *testing.T) {
	_, m, err := commandLine(t, `TR_command_line({"hf_token": "", "max_tokens": 0, "api_key": ""})`, "--hf_token", "hf_abcdefgh123", "--max_tokens", "131072", "--api_key=123456789")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Mask("a hf_abcdefgh123 131072 123456789"); got != "a *** 131072 123456789" {
		t.Errorf("maskeleme: %q", got)
	}
	// Bilinmeyen gizli adlı seçeneğin değeri de hata mesajında görünmez.
	_, m, err = commandLine(t, `TR_command_line({"a": 1})`, "--hf_token_x=hf_yazimhatasi")
	if c, ok := interp.IsExitRequest(err); !ok || strings.Contains(c.Message, "hf_yazimhatasi") || m.Mask("hf_yazimhatasi") != interp.SecretMask {
		t.Errorf("bilinmeyen gizli seçeneğin değeri mesajda olmamalı ve süzgece girmeli: %v", err)
	}
}

// Tek tireyle yanlış yazılmış bir seçeneğin değeri (-api_key=...) hata mesajında görünmez;
// liste türündeki gizli seçeneğin her öğesi de (varsayılanda ve komut satırında) gizlenir.
func TestCommandLineSecretLeaks(t *testing.T) {
	_, _, err := commandLine(t, `TR_command_line({"api_key": ""})`, "-api_key=gizlideger99")
	if c, ok := interp.IsExitRequest(err); !ok || strings.Contains(c.Message, "gizlideger99") {
		t.Errorf("tek tireli seçeneğin değeri mesajda olmamalı: %v", err)
	}
	_, m, err := commandLine(t, `TR_command_line({"api_key": ["varsayilangizli1"]})`, "--api_key=anahtarbir1,anahtariki2")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Mask("anahtarbir1 anahtariki2"); got != interp.SecretMask+" "+interp.SecretMask {
		t.Errorf("liste öğeleri gizlenmeli: %q", got)
	}
	_, m, err = commandLine(t, `TR_command_line({"api_key": ["varsayilangizli1", "varsayilangizli2"]})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Mask("varsayilangizli1 varsayilangizli2"); got != interp.SecretMask+" "+interp.SecretMask {
		t.Errorf("varsayılan liste öğeleri gizlenmeli: %q", got)
	}
}
