package tokenize

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// kucukBPE bulanıklaştırma tohumlarında bileşenlerin yanına konan küçük bir BPE modelidir.
const kucukBPE = `{"type":"BPE","vocab":{"a":0,"b":1,"ab":2,"<unk>":3,"Ġ":4},"merges":["a b"],"unk_token":"<unk>","byte_fallback":false}`

// FuzzParcalayici: tokenizer.json güvenilmeyen girdidir (indirilen model). Yükleyici
// hiçbir dosyada çökmemeli; yüklenen parçalayıcı her metni ya kodlamalı ya da
// *AdimSiniriHatasi döndürmeli; kodlamanın verdiği her kimlik sözlükte olmalı ve kimlikler
// (geçersizleri de) hata vermeden çözülebilmeli. Derin arama:
//
//	go test ./internal/compute/tokenize -run '^$' -fuzz FuzzParcalayici -fuzztime 5m
func FuzzParcalayici(f *testing.F) {
	metin := "Bugün hava çok güzel; İstanbul'da 3,14 ab  aab\t😀 <unk>"
	f.Add([]byte(`{"model":`+kucukBPE+`}`), metin)
	f.Add([]byte(`{"added_tokens":[{"id":5,"content":"<s>","special":true,"normalized":false}],"model":`+kucukBPE+`,"post_processor":{"type":"TemplateProcessing","single":[{"SpecialToken":{"id":"<s>","type_id":0}},{"Sequence":{"id":"A","type_id":0}}],"pair":[{"Sequence":{"id":"A","type_id":0}},{"Sequence":{"id":"B","type_id":1}}],"special_tokens":{"<s>":{"id":"<s>","ids":[5],"tokens":["<s>"]}}}}`), metin)
	f.Add([]byte(`{"model":{"type":"Unigram","unk_id":0,"vocab":[["<unk>",0],["a",-1.5],["b",-2],["ab",-1]],"byte_fallback":true}}`), metin)
	f.Add([]byte(`{"model":{"type":"WordPiece","unk_token":"[UNK]","continuing_subword_prefix":"##","max_input_chars_per_word":100,"vocab":{"[UNK]":0,"a":1,"##b":2}}}`), metin)
	f.Add([]byte(`{"model":{"type":"WordLevel","unk_token":"?","vocab":{"?":0,"a":1}}}`), metin)

	// Bileşen kayıtları (normalleştirici, ön bölücü, çözücü) küçük modelle birleştirilir.
	if veri, err := os.ReadFile("testdata/bilesenler.json"); err == nil {
		var kayitlar [][]json.RawMessage
		if json.Unmarshal(veri, &kayitlar) == nil {
			for _, k := range kayitlar {
				var tur string
				if len(k) < 2 || json.Unmarshal(k[0], &tur) != nil {
					continue
				}
				f.Add([]byte(`{"`+tur+`":`+string(k[1])+`,"model":`+kucukBPE+`}`), metin)
			}
		}
	}
	// Gerçek biçimdeki küçük parçalayıcılar.
	if yollar, err := filepath.Glob("testdata/p_*.json"); err == nil {
		for _, y := range yollar {
			if veri, err := os.ReadFile(y); err == nil {
				f.Add(veri, metin)
			}
		}
	}

	f.Fuzz(func(t *testing.T, dosya []byte, metin string) {
		if len(dosya) > 256<<10 || len(metin) > 4<<10 {
			return
		}
		p, err := Ayristir(dosya)
		if err != nil {
			return
		}
		var h *AdimSiniriHatasi
		for _, s := range []Secenekler{{}, {OzelTaninmasin: true, OzelEklenmesin: true}} {
			ids, err := p.GuvenliKodla(metin, s)
			if err != nil {
				if !errors.As(err, &h) {
					t.Fatalf("kodlamada beklenmeyen hata: %v", err)
				}
				continue
			}
			for _, id := range ids {
				if _, ok := p.KimlikToken(id); !ok {
					t.Fatalf("kodlama sözlükte olmayan %d kimliğini verdi (%v)", id, ids)
				}
			}
			for _, atla := range []bool{false, true} {
				if _, err := p.GuvenliCoz(ids, atla); err != nil && !errors.As(err, &h) {
					t.Fatalf("çözmede beklenmeyen hata: %v", err)
				}
			}
		}
		// Model çıktısının dışında kalan kimlikler de çökertmemeli.
		if _, err := p.GuvenliCoz([]int{-1, 1<<31 - 1, p.SozlukBoyutu(), 0, -1 << 40}, false); err != nil && !errors.As(err, &h) {
			t.Fatalf("geçersiz kimlik çözmede beklenmeyen hata: %v", err)
		}
	})
}
