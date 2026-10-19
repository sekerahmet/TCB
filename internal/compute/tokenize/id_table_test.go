package tokenize

import (
	"fmt"
	"runtime"
	"testing"
)

// Seyrek kimlikli küçük bir dosya, kimliklerin büyüklüğüyle orantılı bellek
// ayırtmamalı (eskiden 69 baytlık dosya 1<<24 kimlikle 288 MB, 1<<30 ile
// ~18 GB ayırıyordu).
func TestSparseIDMemory(t *testing.T) {
	for _, id := range []int{1 << 24, 1 << 30} {
		js := fmt.Sprintf(`{"model":{"type":"WordLevel","vocab":{"a":%d,"b":3},"unk_token":"a"},`+
			`"added_tokens":[{"id":2,"content":"<s>","single_word":false,"lstrip":false,"rstrip":false,"normalized":false,"special":true}],`+
			`"pre_tokenizer":{"type":"WhitespaceSplit"}}`, id)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		p, err := Parse([]byte(js))
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		if mb := (after.TotalAlloc - before.TotalAlloc) >> 20; mb > 4 {
			t.Errorf("kimlik %d: yükleme %d MB ayırdı", id, mb)
		}
		if got := p.Encode("a b <s> c"); fmt.Sprint(got) != fmt.Sprint([]int{id, 3, 2, id}) {
			t.Errorf("kimlik %d: kodlama %v", id, got)
		}
		if s, ok := p.IDToToken(id); !ok || s != "a" {
			t.Errorf("KimlikToken(%d) = %q %v", id, s, ok)
		}
		if _, ok := p.IDToToken(id - 1); ok {
			t.Errorf("KimlikToken(%d) olmamalı", id-1)
		}
		if got := p.Decode([]int{id, 3, 2, 7, -1, 1 << 40}, true); got != "a b" {
			t.Errorf("çözme %q", got)
		}
		if got := p.Decode([]int{2, 3}, false); got != "<s> b" || !p.IsSpecial(2) || p.IsSpecial(3) {
			t.Errorf("özel token: %q %v %v", got, p.IsSpecial(2), p.IsSpecial(3))
		}
	}
}

// Unigram dağarcığında aynı metin iki kimlikte geçse de her kimliğin metni bilinir (tablo
// dosyadaki listeden kurulur): kodlamanın ürettiği bilinmeyen kimliği (unk_id 0) çözülür.
func TestIDTableUnigramDuplicateToken(t *testing.T) {
	js := `{"version":"1.0","truncation":null,"padding":null,"added_tokens":[],
"normalizer":null,"pre_tokenizer":null,"post_processor":null,"decoder":null,
"model":{"type":"Unigram","unk_id":0,"vocab":[["<unk>",0.0],["<unk>",0.0],["a",-1.0]]}}`
	p, err := Parse([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	for id := 0; id < 3; id++ {
		if _, ok := p.IDToToken(id); !ok {
			t.Fatalf("IDToToken(%d) yok", id)
		}
	}
	if n := p.VocabSize(); n != 3 {
		t.Fatalf("sözlük boyu %d, 3 olmalı", n)
	}
}

// Aynı kimliği paylaşan tokenlardan her yüklemede aynısı (bayt sırasıyla en küçüğü) seçilir;
// harita sırasına bağlı değildir.
func TestIDTableSharedIDDeterministic(t *testing.T) {
	js := `{"version":"1.0","truncation":null,"padding":null,"added_tokens":[],
"normalizer":null,"pre_tokenizer":null,"post_processor":null,"decoder":null,
"model":{"type":"WordLevel","unk_token":"<unk>","vocab":{"<unk>":0,"d":1,"b":1,"c":1,"a":1}}}`
	for i := 0; i < 100; i++ {
		p, err := Parse([]byte(js))
		if err != nil {
			t.Fatal(err)
		}
		if s, _ := p.IDToToken(1); s != "a" {
			t.Fatalf("%d. yükleme: IDToToken(1) = %q, \"a\" olmalı", i, s)
		}
	}
}
