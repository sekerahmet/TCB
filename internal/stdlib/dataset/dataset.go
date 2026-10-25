// Package dataset, .npy token parçaları için veri yükleyiciyi (github.com/sekerahmet/TCB/internal/compute/dataload) TR diline
// bağlar: veri_kümesi(dizin, önbellek:, kota_gb:), kümenin .örnekleyici(...) ve
// .sıralı(...) okuyucuları ve okuyucuların .sonraki() metodu.
//
// .sonraki() [girdi, hedef] listesi döndürür; ikisi de [toplu, bağlam] şeklinde, tam sayı
// değerli tensörlerdir ve gömme/çapraz_entropi'ye doğrudan verilebilir. Önden yükleme her
// zaman açıktır: sonraki toplu işler arka plandaki bir Go gorutininde hazırlanır. Bu
// gorutin TR kodu çalıştırmaz, yorumlayıcı kilidini hiç tutmaz; .sonraki() beklerken
// kilit bırakılır.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package dataset

import (
	"context"
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"io"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Tip adları.
var (
	DatasetType    = string(interp.TypeName(codes.A231)) // VeriKümesi
	SamplerType    = string(interp.TypeName(codes.A232)) // Örnekleyici
	SequentialType = string(interp.TypeName(codes.A233)) // SıralıOkuyucu
)

// prefetchDepth arka planda hazır tutulan toplu iş sayısıdır.
const prefetchDepth = 4

// maxExactFloat32 float32'nin tam gösterebildiği en büyük tam sayıdır (2^24).
const maxExactFloat32 = 1 << 24

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	nameCache   = bind.NamedArg(codes.V900)
	nameQuota   = bind.NamedArg(codes.V901)
	nameBatch   = bind.NamedArg(codes.V902)
	nameContext = bind.NamedArg(codes.V903)
	nameSeed    = bind.NamedArg(codes.T903)
)

// Register veri yerleşiklerini VM'ye kaydeder.
func Register(v *interp.VM) {
	v.RegisterBuiltinCode(codes.V100, bind.SafeBuiltin(codes.Name(codes.V100), openDataset))
}

// ----------------------------------------------------------------------------
// Küme
// ----------------------------------------------------------------------------

// Dataset TR'de açılmış bir veri kümesidir.
type Dataset struct {
	k        *dataload.Dataset
	path     string // kullanıcının verdiği yol
	cache    *dataload.Cache
	resource *resource // yalnızca önbellek varsa
}

func (k *Dataset) Type() interp.ValueType { return interp.ValueType(DatasetType) }
func (k *Dataset) IsTruthy() bool         { return true }

func (k *Dataset) summary(lang *codes.Language) string {
	return messages.TextIn(lang.Code, messages.H5500, k.k.ShardCount(), bind.ShortNumber(k.k.TotalTokens()))
}

// Inspect is the value's display text, in the person's writing language (TR_DIL).
func (k *Dataset) Inspect() string { return k.Text(codes.Display()) }

var (
	_ interp.LanguageText = (*Dataset)(nil)
	_ interp.LanguageText = (*Reader)(nil)
)

// Text is the value's text in lang: str() and yazdır give it in the language of the file they
// are written in (docs/spec/03_tipler.md).
func (k *Dataset) Text(lang *codes.Language) string {
	if k.cache != nil {
		return messages.TextIn(lang.Code, messages.H5501, lang.Name(codes.A231), k.summary(lang), bind.FormatBytes(k.cache.Quota()))
	}
	return fmt.Sprintf("%s(%s)", lang.Name(codes.A231), k.summary(lang))
}

// WatchKind İzleme Masası'nda ikon seçimi için türdür.
func (k *Dataset) WatchKind() string { return "diger" }

// WatchLabel tip sütununda gösterilen özettir.
func (k *Dataset) WatchLabel() string {
	return messages.Text(messages.H5503, k.summary(codes.Display()))
}

// TableData parçaları (ad, token, boyut) tablo olarak verir.
func (k *Dataset) TableData() ([]string, [][]string, error) {
	const limit = 10000
	var lines [][]string
	for i, p := range k.k.Shards() {
		if i >= limit {
			break
		}
		lines = append(lines, []string{p.Name, fmt.Sprint(p.Token), messages.Decimal(bind.FormatBytes(p.Bytes))})
	}
	return []string{messages.Text(messages.H5504), messages.Text(messages.H5505), messages.Text(messages.H5506)}, lines, nil
}

// Veri kümesi ve okuyucu üyeleri (adları kod listesinde, internal/lang/codes Y60x).
const (
	memberSampler    = codes.Y600
	memberSequential = codes.Y601
	memberNext       = codes.Y602
)

var datasetMembers = codes.Names(memberSampler, memberSequential)

func (k *Dataset) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case memberSampler:
		return bind.Method(codes.Name(memberSampler), k.sampler), nil
	case memberSequential:
		return bind.Method(codes.Name(memberSequential), k.sequentialTR), nil
	}
	return nil, bind.MemberError(messages.Text(messages.H5502), prop, datasetMembers)
}

// resolveCacheDir önbellek: adlı argümanının (verildiyse) çözülmüş klasör yoludur.
func resolveCacheDir(m *interp.VM, fn string, named map[string]interp.Value) (string, error) {
	v, ok := bind.GetNamed(named, nameCache)
	if !ok {
		return "", nil
	}
	d, ok, err := interp.PathArg(fn, v)
	if !ok || d == "" {
		return "", messages.New(messages.H5507, fn, nameCache.TR)
	}
	if err != nil {
		return "", err
	}
	// Göreli yol, diğer yollar gibi tek kuralla çözülür (16.8): önce çalışma
	// klasörü, orada yoksa programın klasörü.
	return bind.ResolvePath(m, d), nil
}

func openDataset(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.V100)
	if err := bind.CheckNamed(fn+"()", named, nameCache, nameQuota); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, messages.New(messages.H5508, fn, len(args))
	}
	raw, ok, pathErr := interp.PathArg(fn, args[0])
	if !ok {
		return nil, messages.New(messages.H5509, fn, args[0].Type())
	}
	if pathErr != nil {
		return nil, pathErr
	}
	path, tried := bind.ResolvePathTried(m, raw)

	cacheDir, cacheErr := resolveCacheDir(m, fn, named)
	if cacheErr != nil {
		return nil, cacheErr
	}
	var quota int64
	quotaV, hasQuota := bind.GetNamed(named, nameQuota)
	if hasQuota {
		if cacheDir == "" {
			return nil, messages.New(messages.H5510, fn, nameQuota.TR, nameCache.TR)
		}
		gb, ok := interp.ToFloat(quotaV)
		if !ok || gb <= 0 || math.IsInf(gb, 0) || math.IsNaN(gb) {
			return nil, messages.New(messages.H5511, fn, nameQuota.TR, interp.Shown(quotaV))
		}
		// int64 bayt sayısına sığmayan kota (≈ 8,6 milyar GB) taşıp negatif olmasın.
		if gb >= float64(math.MaxInt64)/(1<<30) {
			return nil, messages.New(messages.H5512, fn, nameQuota.TR, interp.Shown(quotaV), int64(math.MaxInt64>>30))
		}
		quota = max(int64(gb*(1<<30)), 1)
	}

	var k *dataload.Dataset
	var cache *dataload.Cache
	var err error
	m.Blocking(func() {
		// Önbelleksiz okumada parçalar belleğe eşlenmez: her pencere ReadAt ile okunur.
		// Ağ ya da akışlı bulut sürücülerinde yalnızca istenen baytlar okunur; yerel
		// diskte de bir toplu iş başına birkaç düzine küçük okuma eğitim adımının yanında
		// önemsizdir. Önbellekteki yerel kopyalar eşlenerek okunur.
		pick := dataload.DatasetOptions{Shard: dataload.ShardOptions{NoMmap: cacheDir == ""}}
		if cacheDir != "" && hasQuota {
			cache, err = dataload.OpenCache(dataload.CacheOptions{Dir: cacheDir, Quota: quota})
			if err != nil {
				return
			}
			pick.Cache = cache
		}
		k, err = dataload.OpenDataset(path, pick)
		if err != nil && cache != nil {
			_ = cache.Close()
			cache = nil
		}
	})
	if err != nil {
		return nil, dataError(fn, raw, err, interp.SearchedPlaces(tried))
	}
	if cacheDir != "" && !hasQuota {
		needed := largestShard(k) * int64(dataload.DefaultGroupSize+dataload.DefaultLookahead)
		return nil, messages.New(messages.H5513,
			fn, nameCache.TR, nameQuota.TR, dataload.DefaultGroupSize+dataload.DefaultLookahead, bind.FormatBytes(largestShard(k)), int64(math.Ceil(float64(needed)/(1<<30))))
	}
	tk := &Dataset{k: k, path: raw, cache: cache}
	if cache != nil {
		tk.resource = addResource(tk, levelCache, func(time.Duration) error { return cache.Close() })
	}
	return tk, nil
}

func largestShard(k *dataload.Dataset) int64 {
	var b int64
	for _, p := range k.Shards() {
		b = max(b, p.Bytes)
	}
	return b
}

// dataError Go veri paketinin hatasını yol gösterici bir TR hatasına çevirir; ekler
// küme bulunamadığında mesaja eklenir (ör. bakılan yerler).
func dataError(fn, path string, err error, extras ...string) error {
	var bh *dataload.FormatError
	switch {
	case errors.As(err, &bh):
		file := bh.Path
		if file == "" {
			file = path
		}
		return messages.New(messages.H5514,
			fn, filepath.ToSlash(file), bh.Reason)
	case errors.Is(err, dataload.ErrCacheInsufficient):
		return messages.New(messages.H5515, fn, sentinelDetail(err))
	case errors.Is(err, dataload.ErrClosed):
		return messages.New(messages.H5516, fn)
	case errors.Is(err, dataload.ErrCacheLocked):
		return messages.New(messages.H2000, fn, sentinelDetail(err))
	case bind.IsNotExist(err) && messages.Has(err, messages.H5019):
		return turkish.FileError(fn, path, messages.H2103, err,
			append([]string{messages.Text(messages.H5517)}, extras...)...)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		// İzin, kilitli dosya, disk hatası...: işletim sisteminin metni yerine Türkçe neden
		return turkish.FileError(fn, filepath.ToSlash(pe.Path), messages.H2100, pe)
	}
	msg := clean(err.Error())
	if messages.Has(err, messages.H5023) {
		return messages.New(messages.H5518, fn, path)
	}
	return messages.New(messages.H2000, fn, msg)
}

// clean Go paketinin "veri: " öneklerini kaldırır.
func clean(s string) string { return strings.ReplaceAll(s, "veri: ", "") }

// ----------------------------------------------------------------------------
// Okuyucular (örnekleyici ve sıralı)
// ----------------------------------------------------------------------------

// Reader .sonraki() ile [girdi, hedef] tensörleri veren örnekleyici ya da sıralı
// okuyucudur. Önden yükleyici her zaman açıktır.
type Reader struct {
	sequential bool
	ds         *Dataset // önbelleğin okuyucudan önce kapatılmaması için canlı tutulur
	b, t       int
	seed       uint64

	mu       sync.Mutex
	pf       *dataload.Prefetcher
	ti       dataload.Batch
	pos      dataload.State // tüketilen son toplu işten sonraki konum (durum, state.go)
	counter  int
	epoch    int
	done     bool
	resource *resource
}

func (o *Reader) Type() interp.ValueType {
	if o.sequential {
		return interp.ValueType(SequentialType)
	}
	return interp.ValueType(SamplerType)
}

func (o *Reader) IsTruthy() bool { return true }

// Inspect is the value's display text, in the person's writing language (TR_DIL).
func (o *Reader) Inspect() string { return o.Text(codes.Display()) }

// Text is the value's text in lang: str() and yazdır give it in the language of the file they
// are written in (docs/spec/03_tipler.md).
func (o *Reader) Text(lang *codes.Language) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	batch, context := lang.Name(nameBatch.Code), lang.Name(nameContext.Code)
	if o.sequential {
		state := messages.TextIn(lang.Code, messages.H5519, o.counter)
		if o.done {
			state = messages.TextIn(lang.Code, messages.H5520, o.counter)
		}
		return fmt.Sprintf("%s(%s: %d, %s: %d, %s)", lang.Name(codes.A233), batch, o.b, context, o.t, state)
	}
	return messages.TextIn(lang.Code, messages.H5521, lang.Name(codes.A232), batch, o.b, context, o.t, lang.Name(nameSeed.Code), o.seed, o.epoch, o.counter)
}

// WatchKind İzleme Masası'nda ikon seçimi için türdür.
func (o *Reader) WatchKind() string { return "diger" }

// WatchLabel tip sütununda gösterilen özettir.
func (o *Reader) WatchLabel() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.sequential {
		if o.done {
			return messages.Text(messages.H5522, o.b, o.t)
		}
		return messages.Text(messages.H5523, o.b, o.t, o.counter)
	}
	return messages.Text(messages.H5524, SamplerType, o.b, o.t, o.epoch, o.counter)
}

// TableData okuyucuyu tablo olarak açmaz; toplu işler .sonraki() ile alınır.
func (o *Reader) TableData() ([]string, [][]string, error) {
	return nil, nil, messages.New(messages.H5525, codes.DisplayName(memberNext))
}

func (o *Reader) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case memberNext:
		return bind.Method(codes.Name(memberNext), o.nextTR), nil
	case memberState:
		return o.StateDict(nil)
	case memberLoadState:
		return bind.Method(codes.Name(memberLoadState), o.loadStateTR), nil
	case memberSkip:
		return bind.Method(codes.Name(memberSkip), o.skipTR), nil
	}
	what := SamplerType
	if o.sequential {
		what = messages.Text(messages.H5526)
	}
	return nil, bind.MemberError(what, prop, codes.Names(memberNext, memberSkip, memberState, memberLoadState))
}

// readerArgs toplu, bağlam ve (örnekleyicide) tohum argümanlarını okur. Konumlu
// olarak da aynı sırayla verilebilirler.
func readerArgs(fn string, args []interp.Value, named map[string]interp.Value, seeded bool) (b, t int, seed uint64, err error) {
	names := []bind.Named{nameBatch, nameContext}
	if seeded {
		names = append(names, nameSeed)
	}
	if err = bind.CheckNamed(fn+"()", named, names...); err != nil {
		return
	}
	if len(args) > len(names) {
		err = messages.New(messages.H5527, fn, len(names), len(args))
		return
	}
	values := make([]interp.Value, len(names))
	for i, a := range names {
		v, given := bind.GetNamed(named, a)
		if i < len(args) {
			if given {
				err = messages.New(messages.H5528, fn, a.TR)
				return
			}
			v = args[i]
		}
		values[i] = v
	}
	sample := fmt.Sprintf("%s(%s: 32, %s: 1024)", fn, nameBatch.TR, nameContext.TR)
	if seeded {
		sample = fmt.Sprintf("%s(%s: 32, %s: 1024, %s: 1)", fn, nameBatch.TR, nameContext.TR, nameSeed.TR)
	}
	positive := func(name string, v interp.Value) (int, error) {
		if v == nil {
			return 0, messages.New(messages.H5529, fn, name, sample)
		}
		n, ok := bind.Int(v)
		if !ok || n <= 0 || n > 1<<24 {
			return 0, messages.New(messages.H5530, fn, name, interp.Shown(v))
		}
		return int(n), nil
	}
	if b, err = positive(nameBatch.TR, values[0]); err != nil {
		return
	}
	if t, err = positive(nameContext.TR, values[1]); err != nil {
		return
	}
	if int64(b)*int64(t+1) > int64(interp.MaxCollectionSize) {
		err = messages.New(messages.H5531, fn, nameBatch.TR, nameContext.TR, b, t, interp.MaxCollectionSize)
		return
	}
	if seeded && values[2] != nil {
		n, ok := bind.Int(values[2])
		if !ok || n < 0 {
			err = messages.New(messages.H5532, fn, nameSeed.TR, interp.Shown(values[2]))
			return
		}
		seed = uint64(n)
	}
	return
}

func (k *Dataset) sampler(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(memberSampler)
	b, t, seed, err := readerArgs(fn, args, named, true)
	if err != nil {
		return nil, err
	}
	var ok *dataload.Sampler
	m.Blocking(func() {
		ok, err = k.k.Sampler(dataload.SamplerOptions{B: b, T: t, Seed: seed})
	})
	if err != nil {
		return nil, readerError(fn, k, t, err)
	}
	return k.newReader(false, b, t, seed, ok), nil
}

func (k *Dataset) sequentialTR(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(memberSequential)
	b, t, _, err := readerArgs(fn, args, named, false)
	if err != nil {
		return nil, err
	}
	var so *dataload.SequentialReader
	m.Blocking(func() {
		so, err = k.k.Sequential(dataload.SequentialOptions{B: b, T: t})
	})
	if err != nil {
		return nil, readerError(fn, k, t, err)
	}
	return k.newReader(true, b, t, 0, so), nil
}

func readerError(fn string, k *Dataset, t int, err error) error {
	if messages.Has(err, messages.H5076) {
		var longest int64
		for _, p := range k.k.Shards() {
			longest = max(longest, p.Token)
		}
		return messages.New(messages.H5533, fn, nameContext.TR, t, t+1, longest)
	}
	if errors.Is(err, dataload.ErrCacheInsufficient) {
		return messages.New(messages.H5515, fn, sentinelDetail(err))
	}
	return dataError(fn, k.path, err)
}

func (k *Dataset) newReader(sequential bool, b, t int, seed uint64, goSource dataload.Source) *Reader {
	pf := dataload.Prefetch(context.Background(), goSource, prefetchDepth)
	o := &Reader{sequential: sequential, ds: k, b: b, t: t, seed: seed, pf: pf}
	o.resource = addResource(o, levelReader, prefetcherCloser(pf))
	return o
}

// prefetcherCloser önden yükleyicinin kaynak kaydındaki kapatıcısıdır: önden yükleyici,
// gorutini durduktan sonra örnekleyiciyi/okuyucuyu (ve açık parçaları) kapatır.
func prefetcherCloser(pf *dataload.Prefetcher) func(time.Duration) error {
	return func(timeout time.Duration) error {
		if timeout > 0 {
			return pf.CloseTimeout(timeout)
		}
		return pf.Close()
	}
}

// takeOne önden yükleyiciden bir toplu iş çeker (o.mu tutulur) ve sayaç, dönem ve konumu günceller.
// Veri bitmişse okuyucuyu kapatır ve eof döner (hata nil).
func (o *Reader) takeOne() (eof bool, err error) {
	err = o.pf.Take(&o.ti)
	if errors.Is(err, io.EOF) {
		o.done = true
		_ = o.resource.Close(0)
		return true, nil
	}
	if err != nil {
		return false, err
	}
	o.counter++
	o.epoch = o.ti.State.Epoch
	o.pos = o.ti.State
	return false, nil
}

// nextTR bir sonraki toplu işi [girdi, hedef] olarak verir; sıralı okuyucu bitince yok.
func (o *Reader) nextTR(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(memberNext)
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, messages.New(messages.H2014, fn, len(args))
	}
	var input, target []float32
	var b, t int
	var err error
	last := false
	// Bekleme ve dönüşüm yorumlayıcı kilidi bırakılarak yapılır; aynı okuyucuyu kullanan
	// eşzamanlı görevler okuyucunun kendi kilidiyle sıralanır.
	m.Blocking(func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if o.done {
			last = true
			return
		}
		var eof bool
		if eof, err = o.takeOne(); eof || err != nil {
			last = eof
			return
		}
		b, t = o.ti.B, o.ti.T
		input, target, err = split(&o.ti)
	})
	if err != nil {
		return nil, dataError(fn, o.ds.path, err)
	}
	if last {
		return interp.Null, nil
	}
	gt := ml.NewInt(tensor.New(input, b, t))
	ht := ml.NewInt(tensor.New(target, b, t))
	return &interp.ListVal{Elements: []interp.Value{gt, ht}}, nil
}

var _ interp.Stream = (*Reader)(nil)

// Next sıralı okuyucuyu 'döngü iş içinde okuyucu { ... }' ile gezdirir (interp.Stream):
// her tur .sonraki()'nin verdiği [girdi, hedef] listesidir, veri bitince döngü biter.
// Örnekleyici sonsuz ürettiği için döngüyle gezilmez.
func (o *Reader) Next(m *interp.VM) (interp.Value, bool, error) {
	if !o.sequential {
		return nil, false, messages.New(messages.H5535, codes.DisplayName(memberNext), codes.DisplayName(memberSequential))
	}
	d, err := o.nextTR(m, nil, nil)
	if err != nil || d == interp.Null {
		return nil, false, err
	}
	return d, true, nil
}

// split [B, T+1] toplu işini [B, T] girdi ve bir kaydırılmış [B, T] hedef dizilerine
// (float32) böler.
func split(ti *dataload.Batch) ([]float32, []float32, error) {
	n := ti.B * ti.T
	input := make([]float32, n)
	target := make([]float32, n)
	for i := 0; i < ti.B; i++ {
		line := ti.Row(i)
		for j := 0; j < ti.T; j++ {
			a, h := line[j], line[j+1]
			if a < 0 || a >= maxExactFloat32 || h < 0 || h >= maxExactFloat32 {
				x := a
				if a >= 0 && a < maxExactFloat32 {
					x = h
				}
				return nil, nil, messages.New(messages.H5536, x, maxExactFloat32-1)
			}
			input[i*ti.T+j] = float32(a)
			target[i*ti.T+j] = float32(h)
		}
	}
	return input, target, nil
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*Dataset)(nil)
	_ interp.PropertyGetter = (*Reader)(nil)
)
