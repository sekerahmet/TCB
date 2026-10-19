package process

import (
	"slices"
	"testing"
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
	if _, err := parseFloat("0,001"); err == nil || err.Error() != "; ondalık ayırıcı nokta olmalı (0.001), binlik ayırıcı için _ yazın (5_000)" {
		t.Errorf("virgüllü ondalık yol göstermeli: %v", err)
	}
}
