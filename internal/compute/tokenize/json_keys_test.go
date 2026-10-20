package tokenize

import (
	"os"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestExactKeys yanlış harfli ve yinelenen anahtarların, başvuru uygulamasından farklı bir
// parçalayıcı kurmasın diye reddedildiğini denetler; bilinmeyen anahtarlara ve sözlükteki
// harfçe benzer tokenlara izin verilir.
func TestExactKeys(t *testing.T) {
	data, err := os.ReadFile(smallPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.HasPrefix(strings.TrimSpace(s), "{") || !strings.Contains(s, `"model"`) {
		t.Fatal("beklenmeyen test dosyası")
	}
	// pp dosyadaki son işleyiciyi bilinmeyen bir anahtara taşıyıp yerine yenisini koyar.
	body := strings.Replace(strings.TrimSpace(s)[1:], `"post_processor":`, `"eski_son_isleyici":`, 1)
	pp := `{"post_processor":`
	for ad, d := range map[string]struct {
		json string
		code messages.Code
	}{
		"yanlış harf":     {`{"post_proCessor":null,` + body, messages.H5214},
		"yinelenen":       {`{"normalizer":null,"normalizer":null,` + body, messages.H5215},
		"iç içe":          {strings.Replace(s, `"model": {`, `"model": {"Dropout":0.5,`, 1), messages.H5214},
		"kaçışlı anahtar": {`{"post_processor":null,"post_processor":null,` + body, messages.H5215},
		"şablon öğesi":    {pp + `{"type":"TemplateProcessing","single":[{"sequence":{"id":"A","type_id":0}}],"pair":[],"special_tokens":{}},` + body, messages.H5214},
		"eşlemdeki yapı":  {pp + `{"type":"TemplateProcessing","single":[],"pair":[],"special_tokens":{"x":{"id":"x","ids":[0],"IDS":[1]}}},` + body, messages.H5214},
	} {
		_, err := Parse([]byte(d.json))
		if !messages.Has(err, d.code) {
			t.Errorf("%s: %s kodlu hata bekleniyordu: %v", ad, d.code, err)
		}
	}
	if _, err := Parse([]byte(`{"bilinmeyen_alan":{"Model":1},` + body)); err != nil {
		t.Fatalf("bilinmeyen alan reddedildi: %v", err)
	}
}

// TestSkipValue değer sınırlarının metin içindeki ayraç ve kaçışlardan etkilenmediğini denetler.
func TestSkipValue(t *testing.T) {
	for in, want := range map[string]int{
		`"a\"}"x`:             6,
		`{"a":"}]","b":[1]}x`: 18,
		`[[],{}]`:             7,
		`-1.5e3,`:             6,
		`true}`:               4,
		`"kapanmayan`:         -1,
		`{"a":[}`:             -1,
	} {
		if got := skipValue([]byte(in), 0); got != want {
			t.Errorf("skipValue(%s) = %d, %d bekleniyordu", in, got, want)
		}
	}
}
