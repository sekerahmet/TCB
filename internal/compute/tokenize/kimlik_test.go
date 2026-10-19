package tokenize

import (
	"fmt"
	"runtime"
	"testing"
)

// Seyrek kimlikli küçük bir dosya, kimliklerin büyüklüğüyle orantılı bellek
// ayırtmamalı (eskiden 69 baytlık dosya 1<<24 kimlikle 288 MB, 1<<30 ile
// ~18 GB ayırıyordu).
func TestSeyrekKimlikBellegi(t *testing.T) {
	for _, id := range []int{1 << 24, 1 << 30} {
		js := fmt.Sprintf(`{"model":{"type":"WordLevel","vocab":{"a":%d,"b":3},"unk_token":"a"},`+
			`"added_tokens":[{"id":2,"content":"<s>","single_word":false,"lstrip":false,"rstrip":false,"normalized":false,"special":true}],`+
			`"pre_tokenizer":{"type":"WhitespaceSplit"}}`, id)
		runtime.GC()
		var once, sonra runtime.MemStats
		runtime.ReadMemStats(&once)
		p, err := Ayristir([]byte(js))
		runtime.ReadMemStats(&sonra)
		if err != nil {
			t.Fatal(err)
		}
		if mb := (sonra.TotalAlloc - once.TotalAlloc) >> 20; mb > 4 {
			t.Errorf("kimlik %d: yükleme %d MB ayırdı", id, mb)
		}
		if got := p.Kodla("a b <s> c"); fmt.Sprint(got) != fmt.Sprint([]int{id, 3, 2, id}) {
			t.Errorf("kimlik %d: kodlama %v", id, got)
		}
		if s, ok := p.KimlikToken(id); !ok || s != "a" {
			t.Errorf("KimlikToken(%d) = %q %v", id, s, ok)
		}
		if _, ok := p.KimlikToken(id - 1); ok {
			t.Errorf("KimlikToken(%d) olmamalı", id-1)
		}
		if got := p.Coz([]int{id, 3, 2, 7, -1, 1 << 40}, true); got != "a b" {
			t.Errorf("çözme %q", got)
		}
		if got := p.Coz([]int{2, 3}, false); got != "<s> b" || !p.OzelMi(2) || p.OzelMi(3) {
			t.Errorf("özel token: %q %v %v", got, p.OzelMi(2), p.OzelMi(3))
		}
	}
}
