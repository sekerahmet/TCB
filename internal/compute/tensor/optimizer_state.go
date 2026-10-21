package tensor

import (
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// İyileştirici durumunun denetimi ve yüklenmesi: adım sayaçları, durum anahtarları ve şekilleri.
// LoadState'ler önce CheckState ile her şeyi denetler, sonra yazar (yükleme ya hiç olmaz ya tam
// olur; kontrol noktası, dil tanımı 20.7).

// readCounter counterTensor'ın yazdığı sayacı okur. İki parça da 0 ile 2^24 arasında tam
// sayı olmalıdır: bozuk bir durumdaki negatif, kesirli ya da sonlu olmayan sayaç sapma
// düzeltmesini sıfıra böldürüp ağırlıkları NaN yapardı.
func readCounter(ad string, t *Tensor) int {
	if t == nil || !sameShape(t.shape, []int{2}) {
		panic(messages.New(messages.H3056, ad))
	}
	d := t.Data()
	for _, x := range d {
		if !(x >= 0 && x < 1<<24) || x != float32(math.Trunc(float64(x))) {
			panic(messages.New(messages.H3057, ad, d))
		}
	}
	return int(d[0])<<24 | int(d[1])
}

// loadState st[ad]'ı p'nin şeklinde, p'nin cihazında yeni bir float32 depolamaya
// kopyalar; yoksa nil.
func loadState(st map[string]*Tensor, ad string, p *Tensor) Storage {
	t, ok := st[ad]
	if !ok {
		return nil
	}
	if !sameShape(t.shape, p.shape) {
		panic(messages.New(messages.H3058, ad, t.shape, p.shape))
	}
	sameDevice("LoadState", p, t)
	c := p.be.Alloc(p.Numel(), Float32)
	p.be.Copy(c, 0, t.view())
	return c
}

// checkStateKeys durumdaki her anahtarın bilinen biçimde ve parametre sayısıyla
// uyumlu olduğunu denetler (başka bir modelin durumu sessizce kısmen yüklenmesin).
func checkStateKeys(st map[string]*Tensor, n int, prefixes ...string) {
	for ad := range st {
		if ad == "adim" {
			continue
		}
		valid := false
		for _, on := range prefixes {
			var k int
			if c, err := fmt.Sscanf(ad, on+".%d", &k); err == nil && c == 1 && fmt.Sprintf("%s.%d", on, k) == ad {
				if k < 0 || k >= n {
					panic(messages.New(messages.H3059, ad, n))
				}
				valid = true
			}
		}
		if !valid {
			panic(messages.New(messages.H3060, ad))
		}
	}
}

// CheckState durumu yüklemeden, LoadState'in bütün kurallarıyla denetler: anahtarlar, şekiller,
// adım sayaçları, moment çiftleri ve momentli parametrenin sayacı. Uymazsa LoadState gibi
// paniğe geçer. LoadState önce bunu çağırır, böylece yükleme ya hiç olmaz ya tam olur; kontrol
// noktası da bütün nesneleri yazmadan önce çağırır (dil tanımı 20.7).
func (o *AdamW) CheckState(st map[string]*Tensor) {
	checkStateKeys(st, len(o.Params), "m", "v", "adim", "ana")
	if t, ok := st["adim"]; ok {
		readCounter("adim", t)
	}
	for k, p := range o.Params {
		m := checkStateShape(st, fmt.Sprintf("m.%d", k), p)
		v := checkStateShape(st, fmt.Sprintf("v.%d", k), p)
		checkStateShape(st, fmt.Sprintf("ana.%d", k), p)
		if m != v {
			panic(messages.New(messages.H3061, k))
		}
		steps := 0
		if t, ok := st[fmt.Sprintf("adim.%d", k)]; ok {
			steps = readCounter(fmt.Sprintf("adim.%d", k), t)
		}
		if m && steps == 0 {
			panic(messages.New(messages.H3062, k))
		}
	}
}

// checkStateShape durumda ad varsa şeklinin p'ninkiyle aynı olduğunu denetler ve var mı
// sorusunu yanıtlar.
func checkStateShape(st map[string]*Tensor, ad string, p *Tensor) bool {
	t, ok := st[ad]
	if ok && !sameShape(t.shape, p.shape) {
		panic(messages.New(messages.H3058, ad, t.shape, p.shape))
	}
	return ok
}

// CheckState durumu yüklemeden denetler (anahtarlar ve şekiller; bkz. AdamW.CheckState).
func (o *SGD) CheckState(st map[string]*Tensor) {
	checkStateKeys(st, len(o.Params), "hiz", "ana")
	for k, p := range o.Params {
		checkStateShape(st, fmt.Sprintf("hiz.%d", k), p)
		checkStateShape(st, fmt.Sprintf("ana.%d", k), p)
	}
}
