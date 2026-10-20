package tokenize

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// smallBPE bulanıklaştırma tohumlarında bileşenlerin yanına konan küçük bir BPE modelidir.
const smallBPE = `{"type":"BPE","vocab":{"a":0,"b":1,"ab":2,"<unk>":3,"Ġ":4},"merges":["a b"],"unk_token":"<unk>","byte_fallback":false}`

// FuzzTokenizer: tokenizer.json güvenilmeyen girdidir (indirilen model). Yükleyici
// hiçbir dosyada çökmemeli; yüklenen parçalayıcı her metni ya kodlamalı ya da
// *AdimSiniriHatasi döndürmeli; kodlamanın verdiği her kimlik sözlükte olmalı ve kimlikler
// (geçersizleri de) hata vermeden çözülebilmeli. Derin arama:
//
//	go test ./internal/compute/tokenize -run '^$' -fuzz FuzzParcalayici -fuzztime 5m
func FuzzTokenizer(f *testing.F) {
	text := "Bugün hava çok güzel; İstanbul'da 3,14 ab  aab\t😀 <unk>"
	f.Add([]byte(`{"model":`+smallBPE+`}`), text)
	f.Add([]byte(`{"added_tokens":[{"id":5,"content":"<s>","special":true,"normalized":false}],"model":`+smallBPE+`,"post_processor":{"type":"TemplateProcessing","single":[{"SpecialToken":{"id":"<s>","type_id":0}},{"Sequence":{"id":"A","type_id":0}}],"pair":[{"Sequence":{"id":"A","type_id":0}},{"Sequence":{"id":"B","type_id":1}}],"special_tokens":{"<s>":{"id":"<s>","ids":[5],"tokens":["<s>"]}}}}`), text)
	f.Add([]byte(`{"model":{"type":"Unigram","unk_id":0,"vocab":[["<unk>",0],["a",-1.5],["b",-2],["ab",-1]],"byte_fallback":true}}`), text)
	f.Add([]byte(`{"model":{"type":"WordPiece","unk_token":"[UNK]","continuing_subword_prefix":"##","max_input_chars_per_word":100,"vocab":{"[UNK]":0,"a":1,"##b":2}}}`), text)
	f.Add([]byte(`{"model":{"type":"WordLevel","unk_token":"?","vocab":{"?":0,"a":1}}}`), text)

	// Bileşen kayıtları (normalleştirici, ön bölücü, çözücü) küçük modelle birleştirilir.
	if data, err := os.ReadFile("testdata/components.json"); err == nil {
		var records [][]json.RawMessage
		if json.Unmarshal(data, &records) == nil {
			for _, k := range records {
				var kind string
				if len(k) < 2 || json.Unmarshal(k[0], &kind) != nil {
					continue
				}
				f.Add([]byte(`{"`+kind+`":`+string(k[1])+`,"model":`+smallBPE+`}`), text)
			}
		}
	}
	// Gerçek biçimdeki küçük parçalayıcılar.
	if paths, err := filepath.Glob("testdata/tokenizer_*.json"); err == nil {
		for _, y := range paths {
			if data, err := os.ReadFile(y); err == nil {
				f.Add(data, text)
			}
		}
	}

	f.Fuzz(func(t *testing.T, file []byte, text string) {
		if len(file) > 256<<10 || len(text) > 4<<10 {
			return
		}
		p, err := Parse(file)
		if err != nil {
			return
		}
		var h *StepLimitError
		for _, s := range []Options{{}, {DontParseSpecial: true, DontAddSpecial: true}} {
			ids, err := p.SafeEncode(text, s)
			if err != nil {
				if !errors.As(err, &h) {
					t.Fatalf("kodlamada beklenmeyen hata: %v", err)
				}
				continue
			}
			for _, id := range ids {
				if _, ok := p.IDToToken(id); !ok {
					t.Fatalf("kodlama sözlükte olmayan %d kimliğini verdi (%v)", id, ids)
				}
			}
			for _, skip := range []bool{false, true} {
				if _, err := p.SafeDecode(ids, skip); err != nil && !errors.As(err, &h) {
					t.Fatalf("çözmede beklenmeyen hata: %v", err)
				}
			}
		}
		// Model çıktısının dışında kalan kimlikler de çökertmemeli.
		if _, err := p.SafeDecode([]int{-1, 1<<31 - 1, p.VocabSize(), 0, -1 << 40}, false); err != nil && !errors.As(err, &h) {
			t.Fatalf("geçersiz kimlik çözmede beklenmeyen hata: %v", err)
		}
	})
}
