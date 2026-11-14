package process

import (
	"math"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yardımın gösterdiği her varsayılan, okuyucuya geri verilince aynı değeri verir (DENETİM 3. adım,
// bulgu 3): yardım kişinin dilinde yazar, okuyucu her gömülü yazım dilinin değer adını kabul eder
// (17.8). Önceden sonsuz varsayılan hiçbir dilde, TR_LANG=en'in "true" yazımı da okunmuyordu.
func TestHelpDefaultsReadBack(t *testing.T) {
	f := func(x float64) interp.Value { return &interp.FloatVal{Val: x} }
	list := func(v ...interp.Value) interp.Value { return &interp.ListVal{Elements: v} }
	for _, c := range []struct {
		kind, item optKind
		value      interp.Value
	}{
		{kindInt, 0, interp.NewInt(3)},
		{kindFloat, 0, f(0.5)}, {kindFloat, 0, f(math.Inf(1))}, {kindFloat, 0, f(math.Inf(-1))}, {kindFloat, 0, f(math.NaN())},
		{kindText, 0, &interp.StringVal{Val: "x"}},
		{kindList, kindBool, list(interp.True, interp.False)},
		{kindList, kindFloat, list(f(math.Inf(1)), f(1.5))},
		{kindList, kindInt, list(interp.NewInt(1), interp.NewInt(2))},
	} {
		s := &option{name: "x", optKind: c.kind, itemKind: c.item, defaultVal: c.value}
		text := defaultText(c.value)
		got, err := parseValue(nil, s, text, true)
		if err != nil || got.Inspect() != c.value.Inspect() {
			t.Errorf("%s varsayılanı %q geri okunamadı: %v, %v", c.value.Type(), text, got, err)
		}
	}
	// Mantık değeri her dilin adıyla (kişinin dili: yardım) okunur; sonsuz ve tanımsızın adı her dilde
	// "inf" ve "nan"dır (3.2). Okunan değerin metni dilden bağımsızdır (3.14).
	for text, want := range map[string]string{"true": "true", "doğru": "true", "false": "false", "inf": "inf", "-inf": "-inf", "nan": "nan"} {
		kind := kindFloat
		if want == "true" || want == "false" {
			kind = kindBool
		}
		if got, err := parsePlainValue(kind, text); err != nil || got.Inspect() != want {
			t.Errorf("%q: %v, %v; beklenen %s", text, got, err, want)
		}
	}
}

// TestUnknownOptionIsCutAfterMask: H7209'un argümanı önce maskelenir, sonra 60 karakterde kesilir
// (19.4, DENETİM 641 N1). Yakın ad önerisi uzun bir adreste tam yoldan çıkmaz; seçenek listesi
// burada adresin bir harf farklı eşini taşır (önerilen ad programın metnidir, iletinin kendi maskesi
// onu yazarken örter). Maskesiz kesim verilen argümanı 60 karakterde ":Veki…" diye keserdi.
func TestUnknownOptionIsCutAfterMask(t *testing.T) {
	prefix := "http://" + strings.Repeat("a", 45)
	near := &option{name: prefix + ":Vekil2026@y", optKind: kindInt}
	err := unknownOption(interp.New(), "--"+prefix+":Vekil2026@x", []*option{near})
	if !messages.Has(err, messages.H7209) {
		t.Fatalf("H7209 bekleniyordu: %v", err)
	}
	got := err.Error()
	if !strings.Contains(got, "--"+prefix+":***@x") || strings.Count(got, "Vek") != 1 {
		t.Errorf("verilen argüman kesimden önce maskelenmeli: %q", got)
	}
}

// NUL ile başlayan metin anahtarı seçeneğin adı olarak bir kez kodlanır: sonuç sözlüğünde anahtar
// yazıldığı metindir (iki karakter). Önceden seçenek sözlüğün iç kodlu anahtarını ad alıyordu,
// sonuç sözlüğü onu yeniden kodluyordu: anahtar 18 karakterdi (D1'in ölçümü).
func TestOptionNameStartingWithNULIsEncodedOnce(t *testing.T) {
	dict := interp.NewDictVal()
	dict.Set("\x00a", &interp.IntVal{Val: 1})
	options, err := buildOptions(nil, "TR_command_line", dict)
	if err != nil || len(options) != 1 || options[0].name != "\x00a" {
		t.Fatalf("seçeneğin adı yazılan metin olmalı: %v %v", options, err)
	}
	result, err := applyArguments(nil, options, "")
	if err != nil || len(result.Order) != 1 {
		t.Fatalf("tek anahtar bekleniyordu: %v", err)
	}
	key, ok := result.KeyValue(result.Order[0]).(*interp.StringVal)
	if !ok || key.Val != "\x00a" {
		t.Errorf("sonuç sözlüğünün anahtarı %q olmalı, gelen %v", "\x00a", result.KeyValue(result.Order[0]))
	}
}

// Parolasında `=` olan adres bilinmeyen seçenek olarak yazılınca ileti parolanın başını göstermez
// (19.4, DENETİM 660 N1). Önceden `=`'deki kesim maskeden önceydi: kalan `postgres://u:abcdef`'te `@`
// olmadığından maske adresi tanımıyor, parolanın başı iletide açık kalıyordu. Sayı ayrılan yolun
// ikisini sınar: `--` ile (seçenek adı `=`'de bölünür) ve tek tire ile.
func TestUnknownOptionWithEqualsInPasswordIsMasked(t *testing.T) {
	options := []*option{{name: "adım", optKind: kindInt}}
	for _, arg := range []string{"--postgres://u:abcdef=gh@h/x", "-postgres://u:abcdef=gh@h/x"} {
		_, err := parse(interp.New(), []string{arg}, options)
		if err == nil {
			t.Fatalf("%s: hata bekleniyordu", arg)
		}
		got := err.Error()
		if strings.Contains(got, "abcdef") || strings.Contains(got, "gh@") || !strings.Contains(got, "***") {
			t.Errorf("%s: parola iletide görünmemeli, maske görünmeli: %q", arg, got)
		}
	}
}
