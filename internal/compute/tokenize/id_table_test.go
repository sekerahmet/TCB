package tokenize

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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
			t.Fatalf("IDToToken(%d) TR_none", id)
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

// Kural (16.1): sözlük boyu kimliklerin sınırıdır: eklenmiş parçalar dahil en büyük kimlik + 1.
// Kimlikler aralıksızsa parça sayısıdır; aradaki boş kimlik IDToToken'da yoktur.
func TestVocabSizeIsTheIDBound(t *testing.T) {
	const head = `{"version":"1.0","truncation":null,"padding":null,"normalizer":null,"pre_tokenizer":null,"post_processor":null,"decoder":null,`
	const wordLevel = `"model":{"type":"WordLevel","vocab":{"[UNK]":0,"a":1,"b":5},"unk_token":"[UNK]"}}`
	const added = `{"id":3,"content":"<s>","single_word":false,"lstrip":false,"rstrip":false,"normalized":false,"special":true}`
	const dense = `"model":{"type":"WordLevel","vocab":{"[UNK]":0,"a":1,"b":2},"unk_token":"[UNK]"}}`
	for _, c := range []struct {
		name, js string
		want     int
	}{
		{"seyrek", head + `"added_tokens":[],` + wordLevel, 6},
		{"eklenmiş parça boşluğa girer", head + `"added_tokens":[` + added + `],` + wordLevel, 6},
		{"eklenmiş parça sınırı büyütür", head + `"added_tokens":[` + added + `],` + dense, 4},
		{"aralıksız", head + `"added_tokens":[],` + dense, 3},
		{"kimlik listeli model", head + `"added_tokens":[],"model":{"type":"Unigram","unk_id":0,"vocab":[["<unk>",0.0],["a",-1.0]]}}`, 2},
	} {
		p, err := Parse([]byte(c.js))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if n := p.VocabSize(); n != c.want {
			t.Errorf("%s: sınır %d, beklenen %d", c.name, n, c.want)
		}
	}
	p, err := Parse([]byte(head + `"added_tokens":[],` + wordLevel))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.IDToToken(3); ok {
		t.Fatal("aradaki boş kimliğin metni var")
	}
}

// Kural (16.1): ek parçanın dosyadaki kimliği görülen bütün kimliklerin üstündeyse geçerlidir:
// metin o kimliğe kodlanır, sınır onunla büyür. Başka bir kimlik tutarsızdır (H5277).
func TestAddedTokenAboveAllIDsKeepsItsID(t *testing.T) {
	const head = `{"version":"1.0","truncation":null,"padding":null,"normalizer":null,"pre_tokenizer":{"type":"WhitespaceSplit"},"post_processor":null,"decoder":null,`
	const wordLevel = `"model":{"type":"WordLevel","vocab":{"[UNK]":0,"a":1,"b":5},"unk_token":"[UNK]"}}`
	added := func(id int, content string) string {
		return fmt.Sprintf(`{"id":%d,"content":%q,"single_word":false,"lstrip":false,"rstrip":false,"normalized":false,"special":true}`, id, content)
	}
	for _, c := range []struct {
		name, tokens string
		bound        int
		encoded      string
	}{
		{"üstte", added(9, "<s>"), 10, "[1 9 5]"},
		{"sınırda", added(6, "<s>"), 7, "[1 6 5]"},
		{"boşlukta sıradaki", added(3, "<s>"), 6, "[1 3 5]"},
		{"ikisi üstte", added(9, "<s>") + "," + added(12, "</s>"), 13, "[1 9 5]"},
		{"üstten sonra sıradaki", added(9, "<s>") + "," + added(10, "</s>"), 11, "[1 9 5]"},
	} {
		p, err := Parse([]byte(head + `"added_tokens":[` + c.tokens + `],` + wordLevel))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := fmt.Sprint(p.Encode("a <s> b")); got != c.encoded {
			t.Errorf("%s: kodlama %s, beklenen %s", c.name, got, c.encoded)
		}
		if n := p.VocabSize(); n != c.bound {
			t.Errorf("%s: sınır %d, beklenen %d", c.name, n, c.bound)
		}
	}
	for name, tokens := range map[string]string{
		"dolu kimlik":      added(5, "<s>"),
		"boşlukta sırasız": added(4, "<s>"),
		"azalan":           added(12, "</s>") + "," + added(9, "<s>"),
		"sözlükteki metin": added(9, "b"),
		"sınırı aşan":      added(1<<30+1, "<s>"),
	} {
		if _, err := Parse([]byte(head + `"added_tokens":[` + tokens + `],` + wordLevel)); !messages.Has(err, messages.H5277) {
			t.Errorf("%s: H5277 bekleniyordu: %v", name, err)
		}
	}
}
