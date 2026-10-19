package profil

import (
	"sync"
	"time"
	"unsafe"

	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Yorumlayıcı örneklemesi: hangi TR satırı ve fonksiyonu zaman harcıyor
// ============================================================================
//
// Örnekleyici yorumlayıcının hata ayıklayıcı yuvasına (VM.Debugger) bağlanır; yuva boşken
// yorumlayıcının her deyimdeki maliyeti tek bir nil denetimidir, yani örnekleme kapalıyken
// sıfır maliyet. Açıkken:
//
//   - BeforeStatement (kilidi tutan görevde, her deyimden önce) o anki konumu tek bir 64
//     bitlik değer olarak, çağrı derinliğinin yuvasına yazar: fonksiyon numarası ve satır.
//     Fonksiyon numarası çağrı yığınının tepesi değişince bir kez çözülür (değişmeyen
//     deyimde üç karşılaştırma).
//   - Ayrı bir goroutine her aralıkta (1 ms) son deyimi çalıştıran görevin çağrı derinliğini
//     okur ve o derinliğin yuvasındaki konuma geçen süreyi yazar (tensör katmanında bir işlem
//     çalışıyorsa ayrıca tensör payına). Derinlikten okumak, bir fonksiyon döndükten sonra
//     çağıran satırın kalan işini (ör. kayıp = çapraz_entropi(ileri(x), y) satırında
//     ileri döndükten sonraki çapraz entropi) doğru satıra yazar.
//
// Yorumlayıcı kilidi aynı anda tek görevin TR kodu çalıştırmasına izin verdiğinden tek bir
// konum yeter. Bütün görevler beklerken (bekle, dosya, kilit) konum son deyimde kalır: o
// satırın süresi bekleme olarak görünür, doğrusu da budur. kesme_noktası profil altında
// yok sayılır.

// ornekAraligi örnekleme sıklığıdır.
const ornekAraligi = time.Millisecond

// enUzunOrnek bir örneğin taşıyabileceği en uzun süredir (örnekleyici goroutine'i uzun süre
// bekletilirse geçen süre tek konuma yığılmasın).
const enUzunOrnek = 20 * time.Millisecond

// yiginSiniri konumu yuvada tutulan en büyük çağrı derinliğidir; daha derinde son deyimin
// konumu kullanılır.
const yiginSiniri = 256

// konumBilgisi bir konumda biriken örnek süreleridir.
type konumBilgisi struct {
	sure, tensor time.Duration
}

// ornekleyici satır/fonksiyon örnekleyicisidir (bkz. dosya başı).
type ornekleyici struct {
	// Konumlar (fonksiyon<<32 | satır) konum_*.go ile yazılıp okunur: son deyimin konumu,
	// derinlik başına son konum ve son deyimi çalıştıran görev.
	konum uint64
	yigin [yiginSiniri]uint64
	gorev unsafe.Pointer // *yurut.VM

	// Fonksiyon numarası önbelleği: yalnızca kilidi tutan görev yazar ve okur.
	sonVM       *yurut.VM
	sonDerinlik int
	sonAd       *byte
	sonNo       uint64

	mu       sync.Mutex
	adlar    []string          // fonksiyon numarası → ad (0: üst düzey)
	adNo     map[string]uint64 // ad → numara
	konumlar map[uint64]*konumBilgisi
	toplam   time.Duration // örneklenen toplam süre
	ornek    int64

	dur   chan struct{}
	bitti sync.WaitGroup
}

func yeniOrnekleyici() *ornekleyici {
	return &ornekleyici{
		adlar:    []string{""},
		adNo:     map[string]uint64{"": 0},
		konumlar: map[uint64]*konumBilgisi{},
		dur:      make(chan struct{}),
	}
}

// BeforeStatement her deyimden önce o anki konumu yazar (hata ayıklayıcı yuvası).
func (o *ornekleyici) BeforeStatement(v *yurut.VM, stmt agac.Statement, _ *yurut.Environment) int {
	if stmt == nil {
		return 0
	}
	fno := uint64(0)
	if n := len(v.CallStack); n > 0 {
		ad := v.CallStack[n-1].FuncName
		if p := unsafe.StringData(ad); v != o.sonVM || n != o.sonDerinlik || p != o.sonAd {
			o.sonVM, o.sonDerinlik, o.sonAd, o.sonNo = v, n, p, o.fonksiyonNo(ad)
		}
		fno = o.sonNo
	}
	k := fno<<32 | uint64(uint32(stmt.Line()))
	if n := len(v.CallStack); n < yiginSiniri {
		konumYaz(&o.yigin[n], k)
	}
	konumYaz(&o.konum, k)
	gorevYaz(&o.gorev, unsafe.Pointer(v))
	return 0
}

// KesmeNoktasi profil altında koddaki kesme noktası yok sayılır.
func (o *ornekleyici) KesmeNoktasi(string) {}

// fonksiyonNo adın numarasını verir (yoksa ekler).
func (o *ornekleyici) fonksiyonNo(ad string) uint64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	if n, ok := o.adNo[ad]; ok {
		return n
	}
	n := uint64(len(o.adlar))
	o.adlar = append(o.adlar, ad)
	o.adNo[ad] = n
	return n
}

// baslat örnekleme goroutine'ini başlatır.
func (o *ornekleyici) baslat() {
	o.bitti.Add(1)
	go o.dongu()
}

// durdur örneklemeyi durdurur ve goroutine'in bitmesini bekler.
func (o *ornekleyici) durdur() {
	select {
	case <-o.dur:
		return
	default:
	}
	close(o.dur)
	o.bitti.Wait()
}

func (o *ornekleyici) dongu() {
	defer o.bitti.Done()
	t := time.NewTicker(ornekAraligi)
	defer t.Stop()
	son := time.Now()
	for {
		select {
		case <-o.dur:
			return
		case simdi := <-t.C:
			d := min(simdi.Sub(son), enUzunOrnek)
			son = simdi
			o.ornekle(d)
		}
	}
}

// ornekle geçen d süresini o anki konuma yazar.
func (o *ornekleyici) ornekle(d time.Duration) {
	k := konumOku(&o.konum)
	if v := (*yurut.VM)(gorevOku(&o.gorev)); v != nil {
		if n := derinlikOku(v); n >= 0 && n < yiginSiniri {
			if y := konumOku(&o.yigin[n]); y != 0 {
				k = y
			}
		}
	}
	if k == 0 || d <= 0 {
		return // program henüz ilk deyime gelmedi
	}
	_, _, tensorde := tensor.EtkinIslem()
	o.mu.Lock()
	b := o.konumlar[k]
	if b == nil {
		b = &konumBilgisi{}
		o.konumlar[k] = b
	}
	b.sure += d
	if tensorde {
		b.tensor += d
	}
	o.toplam += d
	o.ornek++
	o.mu.Unlock()
}

// konumOrnegi bir konumun anlık görüntüsüdür.
type konumOrnegi struct {
	fonksiyon    string
	satir        int
	sure, tensor time.Duration
}

// goruntu biriken örneklerin kopyasını verir.
func (o *ornekleyici) goruntu() (konumlar []konumOrnegi, toplam time.Duration, ornek int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	konumlar = make([]konumOrnegi, 0, len(o.konumlar))
	for k, b := range o.konumlar {
		konumlar = append(konumlar, konumOrnegi{
			fonksiyon: o.adlar[k>>32], satir: int(uint32(k)), sure: b.sure, tensor: b.tensor,
		})
	}
	return konumlar, o.toplam, o.ornek
}
