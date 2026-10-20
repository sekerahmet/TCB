package process

import (
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Gizli ad kuralı: ad sözcüklere bölünür ve sözcükler tam eşlenir; TOKENIZER, MAX_TOKENS
// gibi yapay zekâ işinde sık geçen adlar gizli sayılmaz.
func TestIsSecretName(t *testing.T) {
	secret := []string{
		"HF_TOKEN", "OPENAI_API_KEY", "apiKey", "APIKey", "api-key", "db_şifresi", "GIZLI_ANAHTAR",
		"GİZLİ_ANAHTAR", "veritabanı.parola", "PASSWORD", "db_pass", "AWS_SECRET_ACCESS_KEY",
		"githubAuthToken", "AUTHTOKEN", "api_anahtarı", "private_key", "token", "Şifre",
		"GOOGLE_APPLICATION_CREDENTIALS",
	}
	negated := []string{
		"TOKENIZER_PATH", "tokenizer", "max_tokens", "KEYBOARD_LAYOUT",
		"PASSAGE_DIR", "BYPASS", "XAUTHORITY", "MONKEY", "öğrenme_oranı", "adım", "TR_VERİ_KLASÖRÜ",
		"tokens", "keys", "", "_",
	}
	for _, name := range secret {
		if !IsSecretName(name) {
			t.Errorf("%q gizli sayılmalı (sözcükler: %q)", name, splitWords(name))
		}
	}
	for _, name := range negated {
		if IsSecretName(name) {
			t.Errorf("%q gizli sayılmamalı (sözcükler: %q)", name, splitWords(name))
		}
	}
}

func TestSplitWords(t *testing.T) {
	for _, d := range []struct {
		name string
		want []string
	}{
		{"apiKey", []string{"api", "Key"}},
		{"APIKey", []string{"API", "Key"}},
		{"HF_TOKEN", []string{"HF", "TOKEN"}},
		{"db-şifresi.x y", []string{"db", "şifresi", "x", "y"}},
		{"model2Path", []string{"model2", "Path"}},
		{"ÖğrenmeOranı", []string{"Öğrenme", "Oranı"}},
		{"__", nil},
	} {
		if got := splitWords(d.name); !slices.Equal(got, d.want) {
			t.Errorf("splitWords(%q) = %q, beklenen %q", d.name, got, d.want)
		}
	}
}

// Değeri yalnızca sayı ya da mantık olan gizli adlı değerler süzgece alınmaz.
func TestIsNumberOrBool(t *testing.T) {
	for _, s := range []string{"100000", " 131072 ", "1e-3", "5_000", "-2.5", "true", "False", "doğru", "yanlış"} {
		if !isNumberOrBool(s) {
			t.Errorf("%q sayı ya da mantık sayılmalı", s)
		}
	}
	for _, s := range []string{"hf_abcdefgh", "D:/veri/tokenizer.json", "12ab34", "sk-123456", ""} {
		if isNumberOrBool(s) {
			t.Errorf("%q sayı ya da mantık sayılmamalı", s)
		}
	}
}

func TestNumberParsing(t *testing.T) {
	ints := map[string]int64{"5000": 5000, "5_000": 5000, "-7": -7, "+3": 3, "0": 0}
	for s, want := range ints {
		if n, err := parseInt(s); err != nil || n != want {
			t.Errorf("parseInt(%q) = %d, %v; beklenen %d", s, n, err, want)
		}
	}
	for _, s := range []string{"3.0", "1e3", "5,000", "_5", "5_", "5__0", "abc", "", "0x10", "99999999999999999999"} {
		if _, err := parseInt(s); err == nil {
			t.Errorf("parseInt(%q) hata vermeliydi", s)
		}
	}
	floats := map[string]float64{"0.001": 0.001, "1e-3": 0.001, "3": 3, "2.5E4": 25000, "1_000.5": 1000.5, "-0.5": -0.5}
	for s, want := range floats {
		if f, err := parseFloat(s); err != nil || f != want {
			t.Errorf("parseFloat(%q) = %v, %v; beklenen %v", s, f, err, want)
		}
	}
	for _, s := range []string{"0,001", "NaN", "inf", ".5", "1.", "1e", "1e400"} {
		if _, err := parseFloat(s); err == nil {
			t.Errorf("parseFloat(%q) hata vermeliydi", s)
		}
	}
	if _, err := parseFloat("0,001"); !messages.Has(err, messages.H7215) {
		t.Errorf("virgüllü ondalık yol göstermeli: %v", err)
	}
	if _, err := parseInt("3.0"); !messages.Has(err, messages.H7216) {
		t.Errorf("noktalı tam sayı yol göstermeli: %v", err)
	}
	if _, err := parseInt("99999999999999999999"); !messages.Has(err, messages.H7217) {
		t.Errorf("aralık dışı tam sayı: %v", err)
	}
	if _, err := parseFloat("1e400"); !messages.Has(err, messages.H7217) {
		t.Errorf("aralık dışı ondalık: %v", err)
	}
}

// Argüman hatalarının kodları (çıkış isteği yalnızca metni taşıdığından parse doğrudan sınanır;
// son biçim TestCommandLineUsageErrors'tadır).
func TestParseErrorCodes(t *testing.T) {
	opts := []*option{
		{name: "adım", optKind: kindInt},
		{name: "öğrenme_oranı", optKind: kindFloat},
		{name: "veri", optKind: kindText},
		{name: "katmanlar", optKind: kindList, itemKind: kindInt},
		{name: "hızlı", optKind: kindBool},
	}
	for _, d := range []struct {
		arg  []string
		want []messages.Code
	}{
		{[]string{"--ogrenme_orani", "0.01"}, []messages.Code{messages.H7209}},
		{[]string{"--bambaşka"}, []messages.Code{messages.H7208}},
		{[]string{"-adım", "5"}, []messages.Code{messages.H7206, messages.H7209}},
		{[]string{"--adım", "5", "--adım", "6"}, []messages.Code{messages.H7202}},
		{[]string{"--adım", "beş"}, []messages.Code{messages.H7211}},
		{[]string{"--adım", "3.0"}, []messages.Code{messages.H7211, messages.H7216}},
		{[]string{"--öğrenme_oranı", "0,001"}, []messages.Code{messages.H7211, messages.H7215}},
		{[]string{"--katmanlar", "1,x"}, []messages.Code{messages.H7212}},
		{[]string{"--hızlı=evet"}, []messages.Code{messages.H7210}},
		{[]string{"--adım"}, []messages.Code{messages.H7203}},
		{[]string{"--veri", "--adım", "5"}, []messages.Code{messages.H7203}},
		{[]string{"veri/"}, []messages.Code{messages.H7207}},
		{[]string{"--"}, []messages.Code{messages.H7205}},
		{[]string{strings.Repeat("a", 1<<20+1)}, []messages.Code{messages.H7201}},
		{[]string{"--veri", strings.Repeat("a", 1<<20+1)}, []messages.Code{messages.H7204}},
	} {
		_, err := parse(nil, d.arg, opts)
		for _, c := range d.want {
			if !messages.Has(err, c) {
				t.Errorf("%.40v: %s bekleniyordu: %v", d.arg, c, err)
			}
		}
	}
}
