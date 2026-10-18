package parcalayici

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// ============================================================================
// Kodlama (metin -> kimlik) ve çözme (kimlik -> metin)
// ============================================================================

// Kodla metni token kimliklerine çevirir. Metindeki ek/özel tokenlar
// (ör. "<|endoftext|>") tek bir kimlik olarak tanınır.
func (p *Parcalayici) Kodla(metin string) []int {
	return p.KodlaEkle(nil, metin, true)
}

// KodlaDuz metni, içindeki özel tokenları TANIMADAN kodlar: "<|end|>" gibi
// bir dizgi sıradan metin gibi parçalanır. Özel olmayan ek tokenlar yine
// tanınır. Kullanıcıdan gelen metni güvenle kodlamak için uygundur.
func (p *Parcalayici) KodlaDuz(metin string) []int {
	return p.KodlaEkle(nil, metin, false)
}

// KodlaEkle metnin kimliklerini dst'nin sonuna ekler ve dst'yi döndürür.
// ozelleriTani yanlışsa özel tokenlar metin içinde tanınmaz.
func (p *Parcalayici) KodlaEkle(dst []int, metin string, ozelleriTani bool) []int {
	var tampon [64]int32
	ids := p.kodla32(tampon[:0], metin, ozelleriTani)
	for _, id := range ids {
		dst = append(dst, int(id))
	}
	return dst
}

// Kodla32 metni 32 bitlik kimliklere çevirip dst'ye ekler (büyük derlemler
// için bellek dostu biçim).
func (p *Parcalayici) Kodla32(dst []int32, metin string, ozelleriTani bool) []int32 {
	return p.kodla32(dst, metin, ozelleriTani)
}

func (p *Parcalayici) kodla32(dst []int32, metin string, ozelleriTani bool) []int32 {
	metin = gecerliMetin(metin)
	ozelleriAtla := !ozelleriTani
	p.hamEslesme.bol(metin, ozelleriAtla, func(a ekParca) {
		if a.id >= 0 {
			dst = append(dst, a.id)
			return
		}
		parca := metin[a.bas:a.son]
		// Normalleştirici olmadığından normalleştirilmiş metin aynıdır.
		p.nrmEslesme.bol(parca, ozelleriAtla, func(b ekParca) {
			if b.id >= 0 {
				dst = append(dst, b.id)
				return
			}
			onBol(parca[b.bas:b.son], func(sozcuk string) {
				dst = p.sozcukKodla(dst, sozcuk)
			})
		})
	})
	return dst
}

// sozcukKodla bir ön bölme parçasını (önbellek yardımıyla) kodlar.
func (p *Parcalayici) sozcukKodla(dst []int32, sozcuk string) []int32 {
	o := p.onbellek
	if o == nil || len(sozcuk) > onbellekAzamiUzunluk {
		return p.bpeKodla(dst, sozcuk)
	}
	if v, ok := o.al(sozcuk); ok {
		return append(dst, v...)
	}
	n := len(dst)
	dst = p.bpeKodla(dst, sozcuk)
	o.koy(sozcuk, dst[n:])
	return dst
}

// Tokenlar metnin token metinlerini (bayt düzeyi gösterimiyle) verir.
func (p *Parcalayici) Tokenlar(metin string) []string {
	ids := p.Kodla(metin)
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i], _ = p.KimlikToken(id)
	}
	return out
}

// Coz kimlik dizisini metne çevirir. ozelleriAtla doğruysa özel tokenlar
// çıktıya yazılmaz. Sözlükte olmayan kimlikler sessizce atlanır. Tokenların
// birleşimi geçerli UTF-8 değilse (ör. çok baytlı bir karakterin yarısı)
// bozuk diziler U+FFFD ile değiştirilir.
func (p *Parcalayici) Coz(idler []int, ozelleriAtla bool) string {
	buf := make([]byte, 0, len(idler)*4)
	for _, id := range idler {
		buf = p.cozEkle(buf, id, ozelleriAtla)
	}
	return kayipliUTF8(buf)
}

// Coz32 32 bitlik kimlik dizisini metne çevirir (bkz. Coz).
func (p *Parcalayici) Coz32(idler []int32, ozelleriAtla bool) string {
	buf := make([]byte, 0, len(idler)*4)
	for _, id := range idler {
		buf = p.cozEkle(buf, int(id), ozelleriAtla)
	}
	return kayipliUTF8(buf)
}

// CozBayt kimlik dizisinin ham baytlarını verir (UTF-8 onarımı yapılmaz);
// akış hâlinde üretimde yarım karakterleri biriktirmek için kullanışlıdır.
func (p *Parcalayici) CozBayt(idler []int, ozelleriAtla bool) []byte {
	buf := make([]byte, 0, len(idler)*4)
	for _, id := range idler {
		buf = p.cozEkle(buf, id, ozelleriAtla)
	}
	return buf
}

func (p *Parcalayici) cozEkle(buf []byte, id int, ozelleriAtla bool) []byte {
	if id < 0 || id >= len(p.idVar) || !p.idVar[id] {
		return buf
	}
	if ozelleriAtla && p.idOzel[id] {
		return buf
	}
	return append(buf, p.idBayt[id]...)
}

// ============================================================================
// Toplu ve paralel işlem
// ============================================================================

// isciSayisi 0 ya da negatifse kullanılabilir işlemci sayısını verir.
func isciSayisi(n, is int) int {
	if n <= 0 {
		n = runtime.GOMAXPROCS(0)
	}
	if n > is {
		n = is
	}
	if n < 1 {
		n = 1
	}
	return n
}

// paralel 0..is-1 işlerini isci goroutine'e dağıtır.
func paralel(is, isci int, f func(i int)) {
	isci = isciSayisi(isci, is)
	if isci == 1 {
		for i := 0; i < is; i++ {
			f(i)
		}
		return
	}
	var sayac atomic.Int64
	var wg sync.WaitGroup
	wg.Add(isci)
	for w := 0; w < isci; w++ {
		go func() {
			defer wg.Done()
			for {
				i := int(sayac.Add(1)) - 1
				if i >= is {
					return
				}
				f(i)
			}
		}()
	}
	wg.Wait()
}

// TopluKodla metinleri paralel olarak kodlar; sonuç sırası girdi sırasıyla
// aynıdır. isci <= 0 ise tüm işlemciler kullanılır.
func (p *Parcalayici) TopluKodla(metinler []string, ozelleriTani bool, isci int) [][]int {
	out := make([][]int, len(metinler))
	paralel(len(metinler), isci, func(i int) {
		out[i] = p.KodlaEkle(nil, metinler[i], ozelleriTani)
	})
	return out
}

// TopluKodla32 TopluKodla'nın 32 bitlik kimlik üreten biçimidir.
func (p *Parcalayici) TopluKodla32(metinler []string, ozelleriTani bool, isci int) [][]int32 {
	out := make([][]int32, len(metinler))
	paralel(len(metinler), isci, func(i int) {
		out[i] = p.kodla32(nil, metinler[i], ozelleriTani)
	})
	return out
}

// TopluCoz kimlik dizilerini paralel olarak çözer.
func (p *Parcalayici) TopluCoz(diziler [][]int, ozelleriAtla bool, isci int) []string {
	out := make([]string, len(diziler))
	paralel(len(diziler), isci, func(i int) {
		out[i] = p.Coz(diziler[i], ozelleriAtla)
	})
	return out
}

// BelgeleriBirlestir belgeleri paralel kodlar ve aralarına ayirici kimliğini
// (ör. <|endoftext|>) koyarak tek bir eğitim dizisi hâlinde birleştirir.
// ayirici < 0 ise ayırıcı eklenmez. Her belgenin sonuna ayırıcı konur.
func (p *Parcalayici) BelgeleriBirlestir(belgeler []string, ayirici int, isci int) []int32 {
	parcalar := p.TopluKodla32(belgeler, true, isci)
	toplam := 0
	for _, d := range parcalar {
		toplam += len(d) + 1
	}
	out := make([]int32, 0, toplam)
	for _, d := range parcalar {
		out = append(out, d...)
		if ayirici >= 0 {
			out = append(out, int32(ayirici))
		}
	}
	return out
}
