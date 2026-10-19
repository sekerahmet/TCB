package tokenize

import (
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// ============================================================================
// Kodlama (metin -> kimlik) ve çözme (kimlik -> metin)
// ============================================================================

// Secenekler kodlama seçenekleridir. Sıfır değer, başvuru uygulamasının
// varsayılan davranışıdır: metindeki özel tokenlar tanınır ve son işleyici
// (varsa) özel tokenları (ör. BOS/EOS) ekler.
type Secenekler struct {
	// OzelTaninmasin doğruysa metindeki "<|end|>" gibi özel tokenlar
	// tanınmaz, sıradan metin gibi parçalanır (özel olmayan ek tokenlar yine
	// tanınır). Kullanıcıdan gelen metni güvenle kodlamak için uygundur.
	OzelTaninmasin bool
	// OzelEklenmesin doğruysa son işleyici özel token eklemez.
	OzelEklenmesin bool
}

// Kodla metni token kimliklerine çevirir (varsayılan seçeneklerle).
// Parçalayıcının bir düzenli ifadesi adım sınırını aşarsa *AdimSiniriHatasi
// ile panikler; kullanıcı girdisi işleyen sunucular GuvenliKodla kullanmalıdır.
func (p *Parcalayici) Kodla(metin string) []int {
	return p.KodlaSecenekli(metin, Secenekler{})
}

// KodlaDuz metni, içindeki özel tokenları TANIMADAN kodlar.
func (p *Parcalayici) KodlaDuz(metin string) []int {
	return p.KodlaSecenekli(metin, Secenekler{OzelTaninmasin: true})
}

// KodlaSecenekli metni verilen seçeneklerle kodlar.
func (p *Parcalayici) KodlaSecenekli(metin string, s Secenekler) []int {
	return intCevir(nil, p.Kodla32(nil, metin, s))
}

// KodlaEkle metnin kimliklerini dst'nin sonuna ekler ve dst'yi döndürür.
// ozelleriTani yanlışsa özel tokenlar metin içinde tanınmaz.
func (p *Parcalayici) KodlaEkle(dst []int, metin string, ozelleriTani bool) []int {
	return intCevir(dst, p.Kodla32(nil, metin, Secenekler{OzelTaninmasin: !ozelleriTani}))
}

func intCevir(dst []int, ids []int32) []int {
	if dst == nil {
		dst = make([]int, 0, len(ids))
	}
	for _, id := range ids {
		dst = append(dst, int(id))
	}
	return dst
}

// Kodla32 metni 32 bitlik kimliklere çevirip dst'ye ekler (büyük derlemler
// için bellek dostu biçim).
func (p *Parcalayici) Kodla32(dst []int32, metin string, s Secenekler) []int32 {
	if p.sonIsleyici == nil {
		return p.dizi(dst, metin, s.OzelTaninmasin)
	}
	k := p.sonIsleyici.Isle(&Kodlama{Idler: p.dizi(nil, metin, s.OzelTaninmasin)}, nil, !s.OzelEklenmesin)
	return append(dst, k.Idler...)
}

// KodlaAyrintili tek bir metni kodlar; tür kimliklerini ve özel token
// maskesini de verir.
func (p *Parcalayici) KodlaAyrintili(metin string, s Secenekler) *Kodlama {
	return p.KodlaCift(metin, nil, s)
}

// KodlaCift bir metin çiftini (ör. soru ve bağlam) kodlar; ikinci nil ise
// tek metin kodlanır.
func (p *Parcalayici) KodlaCift(birinci string, ikinci *string, s Secenekler) *Kodlama {
	a := &Kodlama{Idler: p.dizi(nil, birinci, s.OzelTaninmasin)}
	var b *Kodlama
	if ikinci != nil {
		b = &Kodlama{Idler: p.dizi(nil, *ikinci, s.OzelTaninmasin)}
	}
	if p.sonIsleyici == nil {
		k := varsayilanIsle(a, b)
		if b == nil {
			k = &Kodlama{}
			k.ekle(a, 0, false)
		}
		return k
	}
	return p.sonIsleyici.Isle(a, b, !s.OzelEklenmesin)
}

// dizi tek bir metnin (son işleme öncesi) kimliklerini dst'ye ekler.
func (p *Parcalayici) dizi(dst []int32, metin string, ozelleriAtla bool) []int32 {
	metin = gecerliMetin(metin)
	modelKodla := func(x Parca) { dst = p.model.Kodla(dst, x) }
	// Bütün düzenli ifade aramaları tek bir bütçeyi paylaşır: her biri kendi
	// sınırında kalan çok sayıda arama da toplamda metin uzunluğuyla orantılı
	// işte durdurulur.
	butce := p.kodlaButcesi(len(metin))
	p.hamEslesme.bol(metin, ozelleriAtla, func(a ekParca) {
		if a.id >= 0 {
			dst = append(dst, a.id)
			return
		}
		bolum := metin[a.bas:a.son]
		ilk := a.bas == 0
		if p.normallestirici != nil {
			bolum = normallestirButceli(p.normallestirici, bolum, butce)
			if butce != nil {
				butce.ekle(p.kodlaIfade * len(bolum))
			}
		}
		p.nrmEslesme.bol(bolum, ozelleriAtla, func(b ekParca) {
			if b.id >= 0 {
				dst = append(dst, b.id)
				return
			}
			if b.son <= b.bas {
				return
			}
			parca := Parca{Metin: bolum[b.bas:b.son], Ilk: ilk && b.bas == 0, butce: butce}
			if p.onBolucu == nil {
				dst = p.model.Kodla(dst, parca)
				return
			}
			onBolAkis(p.onBolucu, parca, modelKodla)
		})
	})
	return dst
}

// Tokenlar metnin token metinlerini (modelin gösterimiyle) verir.
func (p *Parcalayici) Tokenlar(metin string) []string {
	ids := p.Kodla(metin)
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i], _ = p.KimlikToken(id)
	}
	return out
}

// Coz kimlik dizisini metne çevirir. ozelleriAtla doğruysa özel tokenlar
// çıktıya yazılmaz. Sözlükte olmayan kimlikler sessizce atlanır. Bayt düzeyi
// çözücüde tokenların birleşimi geçerli UTF-8 değilse (ör. çok baytlı bir
// karakterin yarısı) bozuk diziler U+FFFD ile değiştirilir.
func (p *Parcalayici) Coz(idler []int, ozelleriAtla bool) string {
	if p.baytCozucu {
		buf := make([]byte, 0, len(idler)*4)
		for _, id := range idler {
			buf = p.baytEkle(buf, id, ozelleriAtla)
		}
		return kayipliUTF8(buf)
	}
	tokenlar := make([]string, 0, len(idler))
	for _, id := range idler {
		k := p.kimlik(id)
		if k == nil || (ozelleriAtla && k.ozel) {
			continue
		}
		tokenlar = append(tokenlar, k.token)
	}
	if p.cozucu == nil {
		return strings.Join(tokenlar, " ")
	}
	var butce *dfButce
	if p.cozIfade > 0 {
		n := 0
		for _, t := range tokenlar {
			n += len(t)
		}
		butce = yeniButce(p.cozIfade * n)
	}
	return strings.Join(cozButceli(p.cozucu, tokenlar, butce), "")
}

// Coz32 32 bitlik kimlik dizisini metne çevirir (bkz. Coz).
func (p *Parcalayici) Coz32(idler []int32, ozelleriAtla bool) string {
	return p.Coz(intCevir(nil, idler), ozelleriAtla)
}

// CozBayt kimlik dizisinin ham baytlarını verir. Bayt düzeyi çözücüde UTF-8
// onarımı yapılmaz; akış hâlinde üretimde yarım karakterleri biriktirmek
// için kullanışlıdır. Diğer çözücülerde Coz'un sonucunun baytlarıdır.
func (p *Parcalayici) CozBayt(idler []int, ozelleriAtla bool) []byte {
	if !p.baytCozucu {
		return []byte(p.Coz(idler, ozelleriAtla))
	}
	buf := make([]byte, 0, len(idler)*4)
	for _, id := range idler {
		buf = p.baytEkle(buf, id, ozelleriAtla)
	}
	return buf
}

func (p *Parcalayici) baytEkle(buf []byte, id int, ozelleriAtla bool) []byte {
	k := p.kimlik(id)
	if k == nil || (ozelleriAtla && k.ozel) {
		return buf
	}
	return append(buf, k.bayt...)
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
	// Bir işçideki panik (ör. adım sınırı) süreci çökertmesin; çağıran
	// goroutine'de yeniden panik olarak yükseltilir.
	var panikBir sync.Once
	var panikDeger any
	wg.Add(isci)
	for w := 0; w < isci; w++ {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panikBir.Do(func() { panikDeger = r })
					sayac.Store(int64(is)) // kalan işleri bırak
				}
			}()
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
	if panikDeger != nil {
		panic(panikDeger)
	}
}

// TopluKodla metinleri paralel olarak kodlar; sonuç sırası girdi sırasıyla
// aynıdır. isci <= 0 ise tüm işlemciler kullanılır.
func (p *Parcalayici) TopluKodla(metinler []string, s Secenekler, isci int) [][]int {
	out := make([][]int, len(metinler))
	paralel(len(metinler), isci, func(i int) {
		out[i] = p.KodlaSecenekli(metinler[i], s)
	})
	return out
}

// TopluKodla32 TopluKodla'nın 32 bitlik kimlik üreten biçimidir.
func (p *Parcalayici) TopluKodla32(metinler []string, s Secenekler, isci int) [][]int32 {
	out := make([][]int32, len(metinler))
	paralel(len(metinler), isci, func(i int) {
		out[i] = p.Kodla32(nil, metinler[i], s)
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

// BelgeleriBirlestir belgeleri paralel kodlar ve her belgenin sonuna ayirici
// kimliğini (ör. <|endoftext|>) koyarak tek bir eğitim dizisi hâlinde
// birleştirir. ayirici < 0 ise ayırıcı eklenmez.
func (p *Parcalayici) BelgeleriBirlestir(belgeler []string, ayirici int, isci int) []int32 {
	parcalar := p.TopluKodla32(belgeler, Secenekler{}, isci)
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
