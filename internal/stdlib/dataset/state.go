package dataset

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Okuyucu durumu: ö.durum (Y502) okuyucunun kaldığı yeri düz değerli bir sözlük olarak verir,
// ö.durumu_yükle(sözlük) (Y503) aynı ayarla kurulmuş bir okuyucuyu o konuma getirir. Konum
// tüketilen son toplu işten sonraki konumdur (önden yükleme hattında bekleyen toplu işler
// sayılmaz); yüklemede önden yükleme hattı boşaltılıp okuyucu konumdan yeniden kurulur. Böylece
// sürdürülen eğitim kesintisizle aynı toplu işleri aynı sırayla alır. Kontrol noktası
// (internal/stdlib/checkpoint) okuyucuyu bu iki Go yöntemiyle kaydeder (StateDict, LoadState).

const (
	memberState     = codes.Y502
	memberLoadState = codes.Y503
	memberSkip      = codes.Y603
)

// Durum sözlüğünün anahtarları ve okuyucu türleri dosya biçiminin (kontrol noktasının
// durum.json'u) parçasıdır: yazım dilinden bağımsızdır ve çevrilmez (günlük biçimi gibi).
var stateNames = strings.Fields("tür toplu bağlam tohum küme dönem sıra pencere sayaç bitti örnekleyici sıralı")

var (
	sKind, sBatch, sContext, sSeed    = stateNames[0], stateNames[1], stateNames[2], stateNames[3]
	sDataset, sEpoch, sOrder, sWindow = stateNames[4], stateNames[5], stateNames[6], stateNames[7]
	sCounter, sDone                   = stateNames[8], stateNames[9]
	kindSampler, kindSequential       = stateNames[10], stateNames[11]
)

// datasetDigest veri kümesinin kimliğidir: veri dosyalarının adları ve parça sayılarının özeti.
// Veri klasörü taşınsa da (yerel disk → bulut sürücüsü) aynı kalır; dosya eklenir, çıkarılır
// ya da değişirse farklılaşır.
func datasetDigest(k *dataload.Dataset) string {
	h := sha256.New()
	var b [8]byte
	for _, p := range k.Shards() {
		h.Write([]byte(p.Name))
		h.Write([]byte{0})
		binary.LittleEndian.PutUint64(b[:], uint64(p.Token))
		h.Write(b[:])
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}

func (o *Reader) kindName() string {
	if o.sequential {
		return kindSequential
	}
	return kindSampler
}

// StateDict okuyucunun durumudur (TR'de ö.durum): tür, toplu, bağlam, (örnekleyicide)
// tohum, küme özeti, konum (dönem, sıra, pencere), okunan toplu iş sayısı ve (sıralıda)
// bitti. Değerler düz değerlerdir; sözlük JSON'a yazılabilir.
func (o *Reader) StateDict(*interp.VM) (*interp.DictVal, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := interp.NewDictVal()
	s.Set(sKind, &interp.StringVal{Val: o.kindName()})
	s.Set(sBatch, interp.NewInt(int64(o.b)))
	s.Set(sContext, interp.NewInt(int64(o.t)))
	if !o.sequential {
		s.Set(sSeed, interp.NewInt(int64(o.seed)))
	}
	s.Set(sDataset, &interp.StringVal{Val: datasetDigest(o.ds.k)})
	if !o.sequential {
		s.Set(sEpoch, interp.NewInt(int64(o.pos.Epoch)))
	}
	s.Set(sOrder, interp.NewInt(int64(o.pos.Pos)))
	s.Set(sWindow, interp.NewInt(o.pos.Window))
	s.Set(sCounter, interp.NewInt(int64(o.counter)))
	if o.sequential {
		s.Set(sDone, interp.BoolToVal(o.done))
	}
	return s, nil
}

// loadStateTR ö.durumu_yükle(sözlük) metodudur.
func (o *Reader) loadStateTR(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(named) > 0 {
		return nil, messages.New(messages.H5537, interp.CallPrefix(memberLoadState))
	}
	s, err := bind.SingleDictArg(memberLoadState, args, messages.Text(messages.H2315, codes.NameRef(memberState)))
	if err != nil {
		return nil, err
	}
	if err := o.LoadState(m, s); err != nil {
		return nil, err
	}
	return interp.True, nil
}

// LoadState durumu (StateDict biçiminde) yükler: tür, toplu, bağlam, tohum ve küme
// okuyucununkilerle aynı olmalıdır, değilse yol gösterici bir hata verir ve okuyucuyu
// değiştirmez. Önden yükleme hattı durdurulur, okuyucu kaydedilen konumdan yeniden kurulur.
func (o *Reader) LoadState(m *interp.VM, s *interp.DictVal) error {
	commit, _, err := o.PrepareState(m, s)
	if err != nil {
		return err
	}
	commit()
	return nil
}

// PrepareState durumu denetler ve okuyucuyu kaydedilen konumdan kuracak kaynağı hazırlar;
// okuyucuyu değiştirmez. commit yüklemeyi hatasız bitirir (önden yükleme hattını değiştirir),
// discard hazırlanan kaynağı bırakır; ikisinden biri çağrılmalıdır. Kontrol noktası bütün
// nesneleri önce hazırlar, hepsi geçerse yazar (dil tanımı 20.7).
func (o *Reader) PrepareState(m *interp.VM, s *interp.DictVal) (commit, discard func(), err error) {
	fn := interp.CallPrefix(memberLoadState)
	d := stateReader{fn: fn, s: s}
	kind := d.text(sKind)
	if d.failure == nil && kind != kindSampler && kind != kindSequential {
		// Hiçbir okuyucunun yazmadığı tür: kullanıcının programındaki fark değil, kayıt bozuk.
		d.failure = messages.New(messages.H5546, fn, sKind, kind, kindSampler+", "+kindSequential)
	}
	if d.failure == nil && kind != o.kindName() {
		d.failure = messages.New(messages.H5538, fn, kind, o.kindName())
	}
	d.same(sBatch, nameBatch, int64(o.b), true)
	d.same(sContext, nameContext, int64(o.t), true)
	if !o.sequential {
		d.same(sSeed, nameSeed, int64(o.seed), false) // tohum 0 olabilir
	}
	if k := d.text(sDataset); d.failure == nil && k != datasetDigest(o.ds.k) {
		d.failure = messages.New(messages.H5539, fn, o.ds.path)
	}
	var pos dataload.State
	if !o.sequential {
		pos.Epoch = int(d.integer(sEpoch))
	}
	pos.Pos, pos.Window = int(d.integer(sOrder)), d.integer(sWindow)
	counter := int(d.integer(sCounter))
	done := o.sequential && d.boolean(sDone)
	if d.failure != nil {
		return nil, nil, d.failure
	}
	var goSource dataload.Source
	m.Blocking(func() {
		if o.sequential {
			goSource, err = o.ds.k.Sequential(dataload.SequentialOptions{B: o.b, T: o.t, Start: &pos})
		} else {
			goSource, err = o.ds.k.Sampler(dataload.SamplerOptions{B: o.b, T: o.t, Seed: o.seed, Start: &pos})
		}
	})
	if err != nil {
		return nil, nil, messages.New(messages.H5540, fn, o.ds.path, clean(err.Error()))
	}
	commit = func() { o.seekTo(m, goSource, pos, counter, done) }
	discard = func() {
		if c, ok := goSource.(io.Closer); ok {
			_ = c.Close()
		}
	}
	return commit, discard, nil
}

// seekTo okuyucuyu hazırlanmış kaynakla kaydedilen konumdan yeniden kurar ve eski önden
// yükleme hattını kapatır.
func (o *Reader) seekTo(m *interp.VM, goSource dataload.Source, pos dataload.State, counter int, done bool) {
	o.mu.Lock()
	pf := dataload.Prefetch(context.Background(), goSource, prefetchDepth)
	old := o.resource
	o.resource = old.link(prefetcherCloser(pf))
	o.pf, o.ti, o.pos = pf, dataload.Batch{}, pos
	o.counter, o.epoch, o.done = counter, pos.Epoch, done
	fresh := o.resource
	o.mu.Unlock()
	// Eski (ve okuyucu bittiyse yeni) önden yükleyicinin kapanması süren bir disk okumasını
	// bekler: yorumlayıcı kilidi bırakılır, eşzamanlı görevler ilerler.
	m.Blocking(func() {
		_ = old.Close(0)
		if done {
			_ = fresh.Close(0)
		}
	})
}

// stateReader durum sözlüğünden alan okur; ilk hatayı tutar (sonraki okumalar boş döner).
type stateReader struct {
	fn      any // iletilerdeki ad (codes.Ref)
	s       *interp.DictVal
	failure error
}

func (d *stateReader) get(name string) (interp.Value, bool) {
	if d.failure != nil {
		return nil, false
	}
	v, ok := d.s.Pairs[name]
	if !ok {
		d.failure = messages.New(messages.H5541, d.fn, name, codes.NameRef(memberState))
	}
	return v, ok
}

func (d *stateReader) text(name string) string {
	v, ok := d.get(name)
	if !ok {
		return ""
	}
	m, ok := v.(*interp.StringVal)
	if !ok {
		d.failure = messages.New(messages.H5542, d.fn, name, codes.NameRef(codes.A202), interp.TypeAndSummary(v))
		return ""
	}
	return m.Val
}

func (d *stateReader) integer(name string) int64 {
	v, ok := d.get(name)
	if !ok {
		return 0
	}
	n, ok := v.(*interp.IntVal)
	if !ok || n.Val < 0 {
		d.failure = messages.New(messages.H5543, d.fn, name, codes.NameRef(codes.A200), interp.TypeAndSummary(v))
		return 0
	}
	return n.Val
}

func (d *stateReader) boolean(name string) bool {
	v, ok := d.get(name)
	if !ok {
		return false
	}
	b, ok := v.(*interp.BoolVal)
	if !ok {
		d.failure = messages.New(messages.H5542, d.fn, name, codes.NameRef(codes.A203), interp.TypeAndSummary(v))
		return false
	}
	return b.Val
}

// same durumdaki bir ayarın okuyucununkiyle aynı olmasını ister. positive ise (toplu, bağlam)
// pozitif olmayan değeri hiçbir okuyucu yazmaz: bu uyumsuzluk değil bozukluktur.
func (d *stateReader) same(name string, shown any, now int64, positive bool) {
	n := d.integer(name)
	if d.failure == nil && positive && n < 1 {
		d.failure = messages.New(messages.H5547, d.fn, name, codes.NameRef(codes.A200), n)
	}
	if d.failure == nil && n != now {
		d.failure = messages.New(messages.H5544, d.fn, shown, n, now)
	}
}

// skipTR ö.atla(n) metodudur (20.15): sıradaki n toplu işi önden yükleyiciden alıp tensör
// kurmadan bırakır, durumu .sonraki()'deki gibi ilerletir ve atlanan sayıyı verir; sıralı
// okuyucu biterse kalan kadar atlar.
func (o *Reader) skipTR(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(memberSkip)
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, interp.ArgCountError(memberSkip, "1", len(args))
	}
	n, ok := args[0].(*interp.IntVal)
	if !ok {
		return nil, interp.ArgTypeError(memberSkip, 1, args[0], codes.A200)
	}
	if n.Val < 0 {
		return nil, messages.New(messages.H5545, fn, n.Val)
	}
	skipped := int64(0)
	var err error
	m.Blocking(func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		for ; skipped < n.Val && !o.done; skipped++ {
			var eof bool
			if eof, err = o.takeOne(); eof || err != nil {
				return
			}
		}
	})
	if err != nil {
		return nil, dataError(fn, o.ds.path, err)
	}
	return interp.NewInt(skipped), nil
}
