package tokenize

import (
	"reflect"
	"testing"
)

// Kural: SafeEncodePairBatch ve SafeDecodeBatch her öğeyi tek tek SafeEncodePair / SafeDecode ile
// aynı sonuçla işler (sıra korunur, hata öğe başınadır); boş girdide iş yapmaz. Stdlib bunları
// yorumlayıcı kilidini bırakarak çağırır.
func TestSafeBatchesMatchTheSingleCalls(t *testing.T) {
	p, err := Parse([]byte(`{"model":{"type":"WordLevel","vocab":{"a":0,"b":1,"c":2},"unk_token":"a"},"pre_tokenizer":{"type":"Whitespace"}}`))
	if err != nil {
		t.Fatal(err)
	}
	texts, seconds := []string{"a b", "c", "b b c"}, []string{"c", "a b", "a"}
	ids, errs := p.SafeEncodePairBatch(texts, seconds, Options{}, 0)
	for i := range texts {
		k, err := p.SafeEncodePair(texts[i], &seconds[i], Options{})
		if err != nil || errs[i] != nil || !reflect.DeepEqual(ids[i], ToInt(nil, k.IDs)) {
			t.Fatalf("çift %d: toplu %v (%v), tek %v (%v)", i, ids[i], errs[i], ToInt(nil, k.IDs), err)
		}
	}
	seqs := [][]int{{0, 1}, {2}, {}}
	out, derrs := p.SafeDecodeBatch(seqs, false, 2)
	for i, s := range seqs {
		want, err := p.SafeDecode(s, false)
		if err != nil || derrs[i] != nil || out[i] != want {
			t.Fatalf("dizi %d: toplu %q (%v), tek %q (%v)", i, out[i], derrs[i], want, err)
		}
	}
	if o, e := p.SafeDecodeBatch(nil, false, 0); len(o) != 0 || len(e) != 0 {
		t.Fatalf("boş girdi iş yaptı: %v %v", o, e)
	}
}
