package dataload

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

var (
	// ErrClosed, kapatılmış bir kaynağın kullanılmaya çalışıldığını bildirir.
	ErrClosed error = messages.New(messages.H5080)

	errNoMmap       error = messages.New(messages.H5081)
	errMmapEmpty    error = messages.New(messages.H5082)
	errMmapTooLarge error = messages.New(messages.H5083)
)

// openShardCount, açık (kapatılmamış) Parca sayısıdır; sızıntı testleri için.
var openShardCount atomic.Int64

// ShardOptions, bir parçanın nasıl açılacağını belirler.
type ShardOptions struct {
	// NoMmap true ise bellek eşleme hiç denenmez, her okuma ReadAt
	// ile yapılır (ör. ağ sürücülerinde eşleme sorun çıkarıyorsa).
	NoMmap bool
}

// Shard, tek boyutlu tek bir .npy dosyasıdır. Veri belleğe kopyalanmaz:
// mümkünse dosya belleğe eşlenir, değilse her okuma ReadAt ile yapılır.
//
// Oku yöntemleri birden çok gorutinden aynı anda çağrılabilir. Kapat,
// süren okumaların bitmesini bekler.
type Shard struct {
	path   string
	header Header

	mu       sync.RWMutex
	closed   bool
	f        *os.File     // ReadAt yolunda açık; eşlemeli yolda nil
	table    []byte       // tüm dosyanın eşlemesi (eşlemeli yolda)
	data     []byte       // harita içindeki veri bölümü
	closeErr func() error // eşlemeyi serbest bırakır
	release  func()       // önbellek iğnesini bırakır (varsa)
	mmapErr  error        // eşleme denendiyse ve başarısız olduysa nedeni
	buffer   *sync.Pool   // ReadAt yolu için bayt tamponları
}

// OpenShard, yol'daki .npy dosyasını açar ve başlığını doğrular.
func OpenShard(path string, s ShardOptions) (*Shard, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, messages.New(messages.H5024, err)
	}
	p, err := buildShard(path, f, s)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	openShardCount.Add(1)
	return p, nil
}

func buildShard(path string, f *os.File, s ShardOptions) (*Shard, error) {
	bs, err := ReadHeader(f)
	if err != nil {
		var bh *FormatError
		if errors.As(err, &bh) {
			bh.Path = path
		}
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		return nil, messages.New(messages.H5022, path, err)
	}
	needed := bs.DataOffset + bs.DataSize()
	if st.Size() < needed {
		return nil, &FormatError{Path: path, Reason: messages.New(messages.H5084, bs.Length, needed, st.Size())}
	}
	p := &Shard{path: path, header: bs, f: f}
	if !s.NoMmap && mmapSupported && bs.Length > 0 {
		table, closeFn, err := mmapFile(f, needed)
		if err == nil {
			p.table, p.closeErr = table, closeFn
			p.data = table[bs.DataOffset:needed]
			// Eşleme dosya tutamağından bağımsızdır; tutamağı hemen bırak.
			_ = f.Close()
			p.f = nil
		} else {
			p.mmapErr = err
		}
	}
	if p.f != nil {
		p.buffer = &sync.Pool{New: func() any { b := make([]byte, 0, 64<<10); return &b }}
	}
	return p, nil
}

// Path, parçanın dosya yoludur.
func (p *Shard) Path() string { return p.path }

// Header, çözümlenmiş .npy başlığıdır.
func (p *Shard) Header() Header { return p.header }

// Length, parçadaki token sayısıdır.
func (p *Shard) Length() int64 { return p.header.Length }

// DType, eleman türüdür.
func (p *Shard) DType() DType { return p.header.DType }

// Mmapped, parçanın belleğe eşlenerek okunup okunmadığıdır.
func (p *Shard) Mmapped() bool { return p.data != nil }

// MmapError, eşleme denenip başarısız olduysa nedenidir (yoksa nil).
func (p *Shard) MmapError() error { return p.mmapErr }

func (p *Shard) checkRange(n int, begin int64) error {
	if begin < 0 || int64(n) > p.header.Length-begin {
		return messages.New(messages.H5085, p.path, begin, begin+int64(n), p.header.Length)
	}
	return nil
}

// Read, bas konumundan başlayarak len(dst) tokenı dst'ye int32 olarak okur.
// Aralık parça dışına taşarsa hata döner. '<u4' değerleri int32'ye sığmazsa
// (2^31 ve üstü) da hata döner: eksiye dönen bir token çapraz entropide yok
// sayılan hedef olur ve bozuk veri sessizce eğitimden düşerdi.
func (p *Shard) Read(dst []int32, begin int64) error {
	if err := p.checkRange(len(dst), begin); err != nil {
		return err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	if len(dst) == 0 {
		return nil
	}
	es := p.header.DType.Size()
	fits := true
	var err error
	if p.data != nil {
		err = p.mmapRead(func() {
			fits = decodeI32(dst, p.data[begin*int64(es):(begin+int64(len(dst)))*int64(es)], p.header.DType)
		})
	} else {
		err = p.readAtRead(len(dst), begin, func(i int, b []byte, k int) { fits = decodeI32(dst[i:i+k], b, p.header.DType) && fits })
	}
	if err == nil && !fits {
		err = messages.New(messages.H5086, p.path, begin, begin+int64(len(dst)))
	}
	return err
}

// ReadU16, bas konumundan başlayarak len(dst) tokenı dst'ye uint16 olarak
// okur. Yalnızca '<u2' parçalarda kayıpsızdır; diğer türlerde değerler
// 16 bite kırpılmaz, hata döner.
func (p *Shard) ReadU16(dst []uint16, begin int64) error {
	if p.header.DType != DTypeU16 {
		return messages.New(messages.H5087, p.path, p.header.DType)
	}
	if err := p.checkRange(len(dst), begin); err != nil {
		return err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	if len(dst) == 0 {
		return nil
	}
	if p.data != nil {
		return p.mmapRead(func() { decodeU16(dst, p.data[begin*2:(begin+int64(len(dst)))*2]) })
	}
	return p.readAtRead(len(dst), begin, func(i int, b []byte, k int) { decodeU16(dst[i:i+k], b) })
}

// mmapRead, eşlenmiş bellekten okuyan f'yi korumalı çalıştırır. Eşlenmiş
// dosya dışarıdan kısaltılırsa okunan sayfa artık dosyada yoktur ve Unix'te
// erişim SIGBUS ile sonuçlanır; normalde bu süreci kurtarılamaz biçimde
// çökertir (okuma çoğunlukla önden yükleme gorutininde yapıldığından daha
// da). debug.SetPanicOnFault ile bu hata bu gorutinde yakalanabilir bir
// paniğe çevrilir ve açık bir hata olarak döner. Önbellek klasörü
// süreçler arası kilitli olduğundan (bkz. Onbellek) bu yalnızca dışarıdan
// (TR dışı bir programla) dosyaya dokunulursa olur.
func (p *Shard) mmapRead(f func()) (err error) {
	old := debug.SetPanicOnFault(true)
	defer func() {
		debug.SetPanicOnFault(old)
		if r := recover(); r != nil {
			if _, ok := r.(interface{ Addr() uintptr }); ok {
				err = messages.New(messages.H5088, p.path, r)
				return
			}
			panic(r)
		}
	}()
	f()
	return nil
}

// readAtRead, n elemanı parçalar hâlinde ReadAt ile okur ve her parçayı cevir'e
// verir. Kilit çağıran tarafından tutulur.
func (p *Shard) readAtRead(n int, begin int64, convert func(i int, b []byte, k int)) error {
	es := p.header.DType.Size()
	tp := p.buffer.Get().(*[]byte)
	defer p.buffer.Put(tp)
	shardElem := cap(*tp) / es
	for i := 0; i < n; i += shardElem {
		k := min(shardElem, n-i)
		b := (*tp)[:k*es]
		ofs := p.header.DataOffset + (begin+int64(i))*int64(es)
		m, err := p.f.ReadAt(b, ofs)
		if m < len(b) {
			if err == nil || errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return messages.New(messages.H5089, p.path, err)
		}
		convert(i, b, k)
	}
	return nil
}

// Close, eşlemeyi ve dosyayı serbest bırakır. Birden çok kez çağrılabilir.
func (p *Shard) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	var errs []error
	if p.closeErr != nil {
		errs = append(errs, p.closeErr())
		p.closeErr = nil
	}
	p.table, p.data = nil, nil
	if p.f != nil {
		errs = append(errs, p.f.Close())
		p.f = nil
	}
	release := p.release
	p.release = nil
	p.mu.Unlock()
	// Önbellek iğnesi, eşleme kaldırıldıktan sonra bırakılmalı: bazı
	// platformlarda eşlenmiş dosya silinemez.
	if release != nil {
		release()
	}
	openShardCount.Add(-1)
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Küçük uçlu çözme döngüleri
// ---------------------------------------------------------------------------

// decodeI32 b'yi dst'ye çözer; '<u4' değerlerinin hepsi int32'ye sığıyorsa doğru verir
// (öbür türlerde her zaman doğru).
func decodeI32(dst []int32, b []byte, t DType) bool {
	switch t {
	case DTypeU16:
		b = b[:2*len(dst)]
		for i := range dst {
			dst[i] = int32(binary.LittleEndian.Uint16(b[2*i:]))
		}
	case DTypeU32:
		b = b[:4*len(dst)]
		var all uint32
		for i := range dst {
			v := binary.LittleEndian.Uint32(b[4*i:])
			all |= v
			dst[i] = int32(v)
		}
		return all <= math.MaxInt32
	case DTypeI32:
		b = b[:4*len(dst)]
		for i := range dst {
			dst[i] = int32(binary.LittleEndian.Uint32(b[4*i:]))
		}
	}
	return true
}

func decodeU16(dst []uint16, b []byte) {
	b = b[:2*len(dst)]
	for i := range dst {
		dst[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
}
