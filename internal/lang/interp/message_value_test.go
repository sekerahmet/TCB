package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ileti(H…, değerler) katalog iletisini bir değer olarak kurar; fırlat ona verilince hata o
// kodla fırlar. Yakalanan hatanın metni iletiyi kuran çağrıdır (3.14, 8.3); kataloğun metni yalnız
// gösterilen hatadadır (eskiden yakalanan metin kataloğun metniydi).
func TestMessageThrown(t *testing.T) {
	out, _, err := runTROut(t, `
		TR_try { TR_throw TR_Message("H6903", -1) } TR_catch h { TR_print(h) }
		TR_print(TR_Type(TR_Message("H6903", 2)))
	`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "Message(\"H6903\", -1)\nMessage\n" {
		t.Errorf("çıktı: %q", out)
	}
	_, _, err = runTROut(t, "TR_throw TR_Message(\"H6903\", -1)")
	if !messages.Has(err, messages.H6903) {
		t.Errorf("yakalanmayan hata H6903 kodunu taşımalı: %v", err)
	}
}

// TR'den atılan iletinin seçimi de değerin kendi anahtarına bakar: sarılı değer hep `other` dalına
// düşüyordu. Kitaplığın 20 dalı ve H7061: TR'den atılanın metni Go'dan kurulanla aynıdır.
func TestMessageSelectFromTR(t *testing.T) {
	cases := [][]string{
		{"H7061", "s.wav", "pcm", "8"}, {"H7061", "s.wav", "float", "8"}, {"H7061", "s.wav", "compressed", "8"},
		{"H7061", "s.wav", "valid", "8"}, {"H7061", "s.wav", "bilinmeyen", "8"},
		{"H7062", "s.wav", "riff"}, {"H7062", "s.wav", "wave"},
		{"H7063", "s.wav", "no_fmt"}, {"H7063", "s.wav", "no_data"}, {"H7063", "s.wav", "data_first"},
		{"H7063", "s.wav", "two_fmt"}, {"H7063", "s.wav", "two_data"},
		{"H7072", "shape", "[4]"}, {"H7072", "channels", "[0, 4]"}, {"H7072", "rate", "0"},
		{"H7079", "f", "rate", "8000", "", ""}, {"H7079", "f", "channels", "[2, 4]", "a", "b"}, {"H7079", "f", "empty", "[1, 0]", "", ""},
		{"H6940", "model", "x", "y"}, {"H6940", "gguf", "x", "y"}, {"H6941", "model"}, {"H6941", "gguf"},
		{"H6942", "model", "x"}, {"H6942", "gguf", "x"}, {"H6943", "model", "x"}, {"H6943", "gguf", "x"},
		{"H6944", "model", "x"}, {"H6944", "gguf", "x"}, {"H6945", "model", "x"}, {"H6945", "gguf", "x"},
		{"H6946", "model", "x"}, {"H6946", "gguf", "x"},
		{"H7096", "n_mels", "0", "positive"}, {"H7096", "d_model", "6", "heads"},
		{"H7096", "activation_function", "relu", "activation"}, {"H7096", "scale_embedding", "true", "scale"},
		{"H7098", "tr", "mono"}, {"H7098", "de", "unknown"},
		{"H7537", "start", "2"}, {"H7537", "end", "2"}, {"H7537", "pre_roll", "0"}, {"H7537", "silence", "0"}, {"H7537", "max_speech", "0"},
	}
	seen := map[string]string{}
	for _, c := range cases {
		code, err := messages.Parse(c[0])
		if err != nil {
			t.Fatal(err)
		}
		call := `"` + strings.Join(c, `", "`) + `"`
		args := make([]any, len(c)-1)
		for i, a := range c[1:] {
			args[i] = a
		}
		_, _, thrown := runTROut(t, "TR_throw TR_Message("+call+")")
		for _, lang := range []string{"tr", "en"} {
			want := messages.TextIn(lang, code, args...)
			if got := messages.ErrorIn(thrown, lang); !strings.Contains(got, want) {
				t.Errorf("%s, %s: %q, %q bekleniyordu", call, lang, got, want)
			}
			if other, ok := seen[lang+want]; ok {
				t.Errorf("%s ile %s aynı metni veriyor: %q", call, other, want)
			}
			seen[lang+want] = call
		}
	}
}

// Metnin içeriği: seçim gerçekten yazılı dala gider (TR'den atılanla Go'dan kurulan birlikte
// bozulsa yukarıdaki test görmezdi).
func TestMessageSelectBranchText(t *testing.T) {
	cases := []struct{ call, want string }{
		{`"H7098", "tr", "mono"`, "tek dillidir"},
		{`"H7062", "s.wav", "riff"`, "`RIFF` yok"},
		{`"H7063", "s.wav", "no_fmt"`, "`fmt ` parçası yok"},
		{`"H7072", "channels", "[0, 4]"`, "en az bir kanalı"},
		{`"H7079", "f", "rate", "8000", "", ""`, "16000 olmalıdır"},
		{`"H7096", "d_model", "6", "heads"`, "`d_model`'i bölmelidir"},
		{`"H7096", "activation_function", "relu", "activation"`, "Whisper GELU kullanır"},
		{`"H7096", "scale_embedding", "true", "scale"`, "gömme ölçeklenmez"},
		{`"H6941", "gguf"`, "hazır_gguf"},
		{`"H7061", "s.wav", "valid", "8"`, "geçerli bit sayısı"},
	}
	for _, c := range cases {
		_, _, err := runTROut(t, "TR_throw TR_Message("+c.call+")")
		if got := messages.ErrorIn(err, "tr"); !strings.Contains(got, c.want) {
			t.Errorf("%s: %q, içinde %q bekleniyordu", c.call, got, c.want)
		}
	}
	if got := Shown(&IntVal{Val: 5}).(messages.Selector).SelectKey(); got != "5" {
		t.Errorf("sayının seçim anahtarı %q", got)
	}
}

// Çoğul dalı TR değerinin sayısına göre seçilir, # değerin kendi metnini yazar: sayı olmayan
// değer `other` dalındadır, bellek adresi yazılmaz. Çoğul dalı yalnız İngilizce metinlerde var.
func TestMessagePluralFromTR(t *testing.T) {
	cases := []struct{ call, want string }{
		{`"H6911", 1, 2`, "(1 element)"},
		{`"H6911", 3, 2`, "(3 elements)"},
		{`"H6911", 0, 2`, "(0 elements)"},
		{`"H6911", -1, 2`, "(-1 element)"},
		{`"H6911", 4611686018427387904, 2`, "(4611686018427387904 elements)"},
		{`"H6911", 2.5, 2`, "(2.5 elements)"},
		{`"H6911", 1.0, 2`, "(1.0 elements)"},
		{`"H6911", "abc", 2`, "(abc elements)"},
		{`"H6911", "1", 2`, "(1 elements)"},
		{`"H6911", TR_true, 2`, "(true elements)"},
		{`"H6911", [1, 2], 2`, "([1, 2] elements)"},
		{`"H6909", 1, 8`, ": 1 position needed"},
		{`"H6909", 5, 8`, ": 5 positions needed"},
		{`"H6912", 1, 8`, ": 1 position exceed"},
		{`"H6912", 5, 8`, ": 5 positions exceed"},
	}
	for _, c := range cases {
		_, _, err := runTROut(t, "TR_throw TR_Message("+c.call+")")
		if got := messages.ErrorIn(err, "en"); !strings.Contains(got, c.want) {
			t.Errorf("%s: %q, içinde %q bekleniyordu", c.call, got, c.want)
		}
	}
}

// doğrula(koşul, ileti(…)) koşul yanlışken iletiyi kendi koduyla fırlatır (H2456'ya sarmaz).
func TestMessageAssert(t *testing.T) {
	_, _, err := runTROut(t, "TR_assert(1 > 2, TR_Message(\"H6903\", 5))")
	if !messages.Has(err, messages.H6903) || messages.Has(err, messages.H2456) {
		t.Errorf("doğrula iletiyi olduğu gibi fırlatmalı: %v", err)
	}
	if _, _, err := runTROut(t, "TR_assert(2 > 1, TR_Message(\"H6903\", 5))"); err != nil {
		t.Errorf("doğru koşul hata vermemeli: %v", err)
	}
}

// hata_kodu(f) f'yi çalıştırır; fırlayan iletinin kodunu verir, hata yoksa yok.
func TestErrorCode(t *testing.T) {
	out, _, err := runTROut(t, `
		TR_print(TR_error_code { TR_throw TR_Message("H6903", -1) })
		TR_print(TR_error_code { TR_assert(TR_false, TR_Message("H6900", 10, 4)) })
		TR_print(TR_error_code { 1 + 1 })
		TR_print(TR_error_code { TR_throw TR_Message("H6903", -1) } == "H6903")
	`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "H6903\nH6900\nnone\ntrue\n"; out != want {
		t.Errorf("çıktı %q, %q bekleniyordu", out, want)
	}
}

// Kural (8.5): düz metin hatanın kodu yoktur; kodsuz hatada TR_error_code koda dönmez, hatayı olduğu
// gibi yeniden fırlatır (kodsuz hatanın gösteriminde de kod yoktur, 8.6): metin, durumlu tip değeri. Fırlatılamayan
// değer (sayı, sözlük) kendi çalışma hatasıdır (H2911, 8.2).
func TestErrorCodeRethrowsACodelessError(t *testing.T) {
	for thrown, want := range map[string]string{`"düz metin"`: "\ndüz metin\n", "Renk.Kırmızı": "\nRenk.Kırmızı\n", "5": "H2911\n\n", `{"kod": 5}`: "H2911\n\n"} {
		source := "TR_enum Renk { Kırmızı }\nTR_var m = \"\"\nTR_try { TR_print(TR_error_code { TR_throw " + thrown + " }) } TR_catch h { m = TR_String(h) }\nTR_print(m)\n"
		out, _, err := runTROut(t, source)
		if err != nil || out != strings.TrimPrefix(want, "\n") {
			t.Errorf("TR_throw %s: çıktı %q, beklenen %q, hata %v", thrown, out, strings.TrimPrefix(want, "\n"), err)
		}
	}
}

// Kural (8.5): TR_error_code hatanın kodunu, o yoksa nedenler zincirinde (TR_cause) dıştan içe ilk
// kodu verir; zincirde kod yoksa hata, nedeniyle birlikte yeniden fırlar. Eskiden kodsuz bir hatanın
// nedenine bakılmıyordu.
func TestErrorCodeFollowsTheCauseChain(t *testing.T) {
	out, _, err := runTROut(t, `
		TR_func zincir(iç: TR_Bool) {
			TR_try {
				TR_try { TR_if iç { TR_throw TR_Message("H6903", -1) } TR_else { TR_throw "iç" } } TR_catch e { TR_throw TR_Error("orta", TR_cause: e) }
			} TR_catch f {
				TR_throw TR_Error("dış", TR_cause: f)
			}
		}
		TR_print(TR_error_code { zincir(TR_true) })
		TR_try { TR_print(TR_error_code { zincir(TR_false) }) } TR_catch h { TR_print(h, h.TR_cause, h.TR_cause.TR_cause) }
	`)
	if want := "H6903\ndış orta iç\n"; err != nil || out != want {
		t.Errorf("çıktı %q, beklenen %q, hata %v", out, want, err)
	}
}

// ileti tanımsız kodu ve yanlış değer sayısını kurulurken bildirir.
func TestMessageErrors(t *testing.T) {
	_, _, err := runTROut(t, `TR_Message("H9998", 1)`)
	if !messages.Has(err, messages.H2528) {
		t.Errorf("tanımsız kod H2528 vermeli: %v", err)
	}
	_, _, err = runTROut(t, "TR_Message(\"H6903\")")
	if !messages.Has(err, messages.H2529) {
		t.Errorf("yanlış değer sayısı H2529 vermeli: %v", err)
	}
	_, _, err = runTROut(t, "TR_Message()")
	if err == nil {
		t.Error("kodsuz TR_Message() hata vermeli")
	}
}

// Kural (3.14, 8.5): iletinin değer olarak metni onu kuran çağrıdır, tip adıyla, her dilde aynı
// (eskiden metnin istendiği dilin katalog metniydi); okuyanın dilindeki metin yalnız gösterilen
// hatada kurulur.
func TestMessageValueTextIsItsCall(t *testing.T) {
	m := &MessageVal{Code: messages.H6903, Args: []Value{&IntVal{Val: -1}}}
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range []string{m.Inspect(), valueText(m, en), valueText(m, codes.Source())} {
		if got != `Message("H6903", -1)` {
			t.Errorf("değerin metni %q", got)
		}
	}
	if got := messages.ErrorIn(m.Error(), "en"); !strings.Contains(got, "cannot be negative, got -1") {
		t.Errorf("gösterilen hata, en: %q", got)
	}
	if got := messages.ErrorIn(m.Error(), "tr"); !strings.Contains(got, "negatif olamaz, -1 verildi") {
		t.Errorf("gösterilen hata, tr: %q", got)
	}
}
