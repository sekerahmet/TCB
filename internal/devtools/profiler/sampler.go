package profiler

import (
	"sync"
	"time"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/interp"
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
//     Fonksiyon numarası (fonksiyonun adı ve dosyası: 'aktar' ile yüklenen modülün satırları
//     kendi dosyasındadır) çağrı yığınının tepesi ya da çalışan dosya değişince bir kez
//     çözülür (değişmeyen deyimde dört karşılaştırma).
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

// sampleInterval örnekleme sıklığıdır.
const sampleInterval = time.Millisecond

// longestSample bir örneğin taşıyabileceği en uzun süredir (örnekleyici goroutine'i uzun süre
// bekletilirse geçen süre tek konuma yığılmasın).
const longestSample = 20 * time.Millisecond

// stackLimit konumu yuvada tutulan en büyük çağrı derinliğidir; daha derinde son deyimin
// konumu kullanılır.
const stackLimit = 256

// funcKey bir fonksiyonun kimliğidir: adı (üst düzey kodda boş) ve çalıştığı dosya (nil: ana
// program). Başka dosyada aynı adlı fonksiyon başka bir fonksiyondur.
type funcKey struct {
	name string
	src  *interp.Source
}

// posInfo bir konumda biriken örnek süreleridir.
type posInfo struct {
	dur, tensor time.Duration
}

// sampler satır/fonksiyon örnekleyicisidir (bkz. dosya başı).
type sampler struct {
	// Konumlar (fonksiyon<<32 | satır) konum_*.go ile yazılıp okunur: son deyimin konumu,
	// derinlik başına son konum ve son deyimi çalıştıran görev.
	pos   uint64
	stack [stackLimit]uint64
	task  unsafe.Pointer // *interp.VM

	// Fonksiyon numarası önbelleği: yalnızca kilidi tutan görev yazar ve okur.
	lastVM    *interp.VM
	lastDepth int
	lastName  *byte
	lastSrc   *interp.Source
	lastNo    uint64

	mu        sync.Mutex
	names     []funcKey          // fonksiyon numarası → ad ve dosya (0: ana programın üst düzeyi)
	nameNo    map[funcKey]uint64 // ad ve dosya → numara
	positions map[uint64]*posInfo
	total     time.Duration // örneklenen toplam süre
	sample    int64

	stop chan struct{}
	done sync.WaitGroup
}

func newSampler() *sampler {
	return &sampler{
		names:     []funcKey{{}},
		nameNo:    map[funcKey]uint64{{}: 0},
		positions: map[uint64]*posInfo{},
		stop:      make(chan struct{}),
	}
}

// BeforeStatement her deyimden önce o anki konumu yazar (hata ayıklayıcı yuvası).
func (o *sampler) BeforeStatement(v *interp.VM, stmt ast.Statement, _ *interp.Environment) int {
	if stmt == nil {
		return 0
	}
	fno := uint64(0)
	src := v.CurrentSource()
	if n := len(v.CallStack); n > 0 || src != nil {
		ad := ""
		if n > 0 {
			ad = v.CallStack[n-1].FuncName
		}
		if p := unsafe.StringData(ad); v != o.lastVM || n != o.lastDepth || p != o.lastName || src != o.lastSrc {
			o.lastVM, o.lastDepth, o.lastName, o.lastSrc, o.lastNo = v, n, p, src, o.functionNo(funcKey{ad, src})
		}
		fno = o.lastNo
	}
	k := fno<<32 | uint64(uint32(stmt.Line()))
	if n := len(v.CallStack); n < stackLimit {
		storePos(&o.stack[n], k)
	}
	storePos(&o.pos, k)
	storeTask(&o.task, unsafe.Pointer(v))
	return 0
}

// Breakpoint profil altında koddaki kesme noktası yok sayılır.
func (o *sampler) Breakpoint(string) {}

// functionNo fonksiyonun numarasını verir (yoksa ekler).
func (o *sampler) functionNo(f funcKey) uint64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	if n, ok := o.nameNo[f]; ok {
		return n
	}
	n := uint64(len(o.names))
	o.names = append(o.names, f)
	o.nameNo[f] = n
	return n
}

// start örnekleme goroutine'ini başlatır.
func (o *sampler) start() {
	o.done.Add(1)
	go o.loop()
}

// halt örneklemeyi durdurur ve goroutine'in bitmesini bekler.
func (o *sampler) halt() {
	select {
	case <-o.stop:
		return
	default:
	}
	close(o.stop)
	o.done.Wait()
}

func (o *sampler) loop() {
	defer o.done.Done()
	t := time.NewTicker(sampleInterval)
	defer t.Stop()
	last := time.Now()
	for {
		select {
		case <-o.stop:
			return
		case now := <-t.C:
			d := min(now.Sub(last), longestSample)
			last = now
			o.sample2(d)
		}
	}
}

// sample2 geçen d süresini o anki konuma yazar.
func (o *sampler) sample2(d time.Duration) {
	k := loadPos(&o.pos)
	if v := (*interp.VM)(loadTask(&o.task)); v != nil {
		if n := readDepth(v); n >= 0 && n < stackLimit {
			if y := loadPos(&o.stack[n]); y != 0 {
				k = y
			}
		}
	}
	if k == 0 || d <= 0 {
		return // program henüz ilk deyime gelmedi
	}
	_, _, inTensor := tensor.ActiveOp()
	o.mu.Lock()
	b := o.positions[k]
	if b == nil {
		b = &posInfo{}
		o.positions[k] = b
	}
	b.dur += d
	if inTensor {
		b.tensor += d
	}
	o.total += d
	o.sample++
	o.mu.Unlock()
}

// posSample bir konumun anlık görüntüsüdür.
type posSample struct {
	function    string
	src         *interp.Source // satırın dosyası; nil ana programdır
	line        int
	dur, tensor time.Duration
}

// snapshot biriken örneklerin kopyasını verir.
func (o *sampler) snapshot() (positions []posSample, total time.Duration, sample int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	positions = make([]posSample, 0, len(o.positions))
	for k, b := range o.positions {
		f := o.names[k>>32]
		positions = append(positions, posSample{
			function: f.name, src: f.src, line: int(uint32(k)), dur: b.dur, tensor: b.tensor,
		})
	}
	return positions, o.total, o.sample
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.Debugger = (*sampler)(nil)
)
