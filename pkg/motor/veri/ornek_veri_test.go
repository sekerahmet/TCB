package veri_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pc "tr_dili/pkg/parcalayici"
	dv "tr_dili/pkg/veri"
)

// testler/veri altındaki sentetik test verisi (TR testleri ve ornekler/veri_ile_egitim.tr
// kullanır) bu dosyadaki üreteçle oluşturulur. Varsayılan olarak test, depodaki dosyaların
// üreteçle birebir aynı olduğunu denetler; yeniden üretmek için:
//
//	go test ./pkg/motor/veri -run TestOrnekVeri -guncelle
var guncelle = flag.Bool("guncelle", false, "testler/veri altındaki sentetik veriyi yeniden üret")

const ornekKok = "../../../testler/veri"

// ornekDosyalar depodaki sentetik verinin beklenen içeriğidir (göreli yol → içerik).
func ornekDosyalar(t *testing.T) map[string][]byte {
	t.Helper()
	oku := func(yol string) []byte {
		b, err := os.ReadFile(yol)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	npyU16 := func(v []uint16) []byte {
		var b bytes.Buffer
		if err := dv.NpyYazU16(&b, v); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	npyI32 := func(v []int32) []byte {
		var b bytes.Buffer
		if err := dv.NpyYazI32(&b, v); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	d := map[string][]byte{}
	// Parçalayıcılar: bayt düzeyi BPE (özel tokenlı, <|endoftext|> içerir) ve BERT
	// tarzı WordPiece (son işleyicisi [CLS]/[SEP] ekler; çift kodlama için).
	bpe := oku("../../parcalayici/testdata/p_bayt_bpe.json")
	d["parcalayici.json"] = bpe
	d["parcalayici_bert.json"] = oku("../../parcalayici/testdata/p_wordpiece.json")

	// sayilar/: değerleri konumunu söyleyen iki parça (uint16 ve int32).
	a := make([]uint16, 100)
	for i := range a {
		a[i] = uint16(i)
	}
	b := make([]int32, 150)
	for i := range b {
		b[i] = int32(1000 + i)
	}
	d["sayilar/a.npy"] = npyU16(a)
	d["sayilar/b.npy"] = npyI32(b)

	// metin/: örnek Türkçe metnin satırları belge sayılıp kodlanır, aralarına
	// <|endoftext|> konur ve iki parçaya bölünür; meta.json parça token sayılarını verir.
	p, err := pc.Ayristir(bpe)
	if err != nil {
		t.Fatal(err)
	}
	ayirici, ok := p.TokenKimligi("<|endoftext|>")
	if !ok {
		t.Fatal("<|endoftext|> yok")
	}
	var belgeler []string
	for _, s := range strings.Split(string(oku("../../parcalayici/testdata/ornek_metin.txt")), "\n") {
		if strings.TrimSpace(s) != "" {
			belgeler = append(belgeler, s)
		}
	}
	tokenlar := p.BelgeleriBirlestir(belgeler, ayirici, 1)
	yari := len(tokenlar) / 2
	parcalar := [][]int32{tokenlar[:yari], tokenlar[yari:]}
	meta := map[string]any{"tokenizer": "../parcalayici.json", "parts": map[string]any{}}
	for i, pt := range parcalar {
		u := make([]uint16, len(pt))
		for j, x := range pt {
			u[j] = uint16(x)
		}
		ad := fmt.Sprintf("metin_%03d.npy", i)
		d["metin/"+ad] = npyU16(u)
		meta["parts"].(map[string]any)[ad] = map[string]any{"tokens": len(pt)}
	}
	mj, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	d["metin/meta.json"] = append(mj, '\n')

	// Hata testleri için .npy olmayan bir dosya.
	d["bozuk.npy"] = []byte("bu bir .npy dosyası değil\n")
	return d
}

func TestOrnekVeri(t *testing.T) {
	beklenen := ornekDosyalar(t)
	if *guncelle {
		for ad, icerik := range beklenen {
			yol := filepath.Join(ornekKok, filepath.FromSlash(ad))
			if err := os.MkdirAll(filepath.Dir(yol), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(yol, icerik, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	for ad, icerik := range beklenen {
		got, err := os.ReadFile(filepath.Join(ornekKok, filepath.FromSlash(ad)))
		if err != nil {
			t.Errorf("%s okunamadı (yeniden üretmek için -guncelle): %v", ad, err)
			continue
		}
		if !bytes.Equal(got, icerik) {
			t.Errorf("%s üreteçle aynı değil (yeniden üretmek için -guncelle)", ad)
		}
	}
}
