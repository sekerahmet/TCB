package parcalayici

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// ekliParcalayici testdata'daki bir parçalayıcıya verilen içeriklerle
// normalized=true ek tokenlar ekler (kimlikler sözlüğün sonundan).
func ekliParcalayici(t *testing.T, dosya string, icerikler ...string) *Parcalayici {
	t.Helper()
	veri, err := os.ReadFile("testdata/" + dosya)
	if err != nil {
		t.Fatal(err)
	}
	var ham map[string]any
	if err := json.Unmarshal(veri, &ham); err != nil {
		t.Fatal(err)
	}
	p, err := Ayristir(veri)
	if err != nil {
		t.Fatal(err)
	}
	id := len(p.idVar)
	ekler, _ := ham["added_tokens"].([]any)
	for i, c := range icerikler {
		ekler = append(ekler, map[string]any{"id": id + i, "content": c, "single_word": false, "lstrip": false,
			"rstrip": false, "normalized": true, "special": false})
	}
	ham["added_tokens"] = ekler
	yeni, _ := json.Marshal(ham)
	if p, err = Ayristir(yeni); err != nil {
		t.Fatal(err)
	}
	return p
}

// normalized=true ek tokenlar normalleştirilmiş metinde, içerikleri de
// normalleştirilerek aranır; token metinleri normalleştirilmiş içeriktir.
// Beklenen değerler başvuru uygulamasının çıktısıdır.
func TestNormalizeEkTokenlar(t *testing.T) {
	durumlar := []struct {
		dosya string
		metin string
		idler []int
		cozum string
	}{
		// Küçük harfe çeviren normalleştirici: "ÇAY" içeriği "çay" olarak aranır.
		{"p_kelime_bpe.json", "çay ÇAY", []int{1, 802, 802, 2}, "çayçay"},
		{"p_kelime_bpe.json", " <|x|> yeni ÇAY ", []int{1, 800, 801, 802, 2}, "<|x|>yeniçay"},
		{"p_kelime_bpe.json", "Yeni\tYeni</s>x<|x|>ÇAY", []int{1, 801, 801, 2, 145, 800, 802, 2}, "yeniyenix <|x|>çay"},
		// Başa "▁" ekleyen normalleştirici: "yeni" içeriği "▁yeni" olarak aranır.
		{"p_llama_tarzi.json", "ayeni", []int{1, 214, 77, 42}, "ayeni"},
		{"p_llama_tarzi.json", "a yeni ÇAY", []int{1, 301, 648, 649}, "a yeni ÇAY"},
		{"p_llama_tarzi.json", "yeni", []int{1, 648}, "yeni"},
	}
	for _, d := range durumlar {
		p := ekliParcalayici(t, d.dosya, "<|x|>", "yeni", "ÇAY")
		ids, err := p.GuvenliKodla(d.metin, Secenekler{})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids, d.idler) {
			t.Errorf("%s %q: kimlikler %v, beklenen %v", d.dosya, d.metin, ids, d.idler)
		}
		if got := p.Coz(ids, true); got != d.cozum {
			t.Errorf("%s %q: çözüm %q, beklenen %q", d.dosya, d.metin, got, d.cozum)
		}
	}
	// Kimlik -> token normalleştirilmiş, token -> kimlik özgün içerikle.
	p := ekliParcalayici(t, "p_kelime_bpe.json", "<|x|>", "yeni", "ÇAY")
	if s, _ := p.KimlikToken(802); s != "çay" {
		t.Errorf("KimlikToken(802) = %q, beklenen \"çay\"", s)
	}
	if id, ok := p.TokenKimligi("ÇAY"); !ok || id != 802 {
		t.Errorf("TokenKimligi(ÇAY) = %d %v", id, ok)
	}
	if _, ok := p.TokenKimligi("çay"); ok {
		t.Error("TokenKimligi(çay) ek token olarak bulunmamalı")
	}
}
