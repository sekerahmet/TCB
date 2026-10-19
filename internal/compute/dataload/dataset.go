package dataload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Meta, veri kökündeki meta.json dosyasıdır. Yalnızca yükleyicinin
// kullandığı alanlar türlüdür; geri kalanı ham tutulur.
type Meta struct {
	Tokenizer json.RawMessage            `json:"tokenizer"`
	Files     map[string]json.RawMessage `json:"files"`
	Parts     map[string]MetaPart        `json:"parts"`
	Valid     *MetaValid                 `json:"valid"`
}

// MetaParca, meta.json içindeki bir parça kaydıdır.
type MetaPart struct {
	Tokens int64 `json:"tokens"`
}

// MetaDogrulama, meta.json içindeki doğrulama kümesi kaydıdır.
type MetaValid struct {
	Tokens int64 `json:"tokens"`
	Docs   int64 `json:"docs"`
}

// TokenizerYolu, 'tokenizer' alanı bir metinse onu döner.
func (m *Meta) TokenizerPath() string {
	var s string
	if m == nil || json.Unmarshal(m.Tokenizer, &s) != nil {
		return ""
	}
	return s
}

// MetaOku, verilen meta.json dosyasını okur.
func ReadMeta(path string) (*Meta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("veri: meta okunamadı: %w", err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("veri: %s: meta.json çözümlenemedi: %w", path, err)
	}
	return &m, nil
}

// ParcaBilgisi, kümedeki bir parçanın adı, yolu ve token sayısıdır.
type ShardInfo struct {
	Name  string
	Path  string
	Token int64
	Bytes int64 // dosya boyutu (dizin listesinden)
}

// KumeSecenekleri, KumeAc'ın davranışını belirler.
type DatasetOptions struct {
	// Desen, parça dosyalarının adı için kalıptır (varsayılan "*.npy").
	Pattern string
	// MetaYok true ise meta.json aranmaz; tüm parçaların başlığı okunur.
	NoMeta bool
	// Parca, parçaların açılış seçenekleridir.
	Shard ShardOptions
	// Onbellek verilirse parçalar açılmadan önce yerel önbelleğe alınır;
	// yerel kopya alınamazsa parça açma hata verir (kaynağa geri dönülmez).
	// Varsayılan (nil): önbellek kapalı, parçalar doğrudan kaynaktan
	// okunur. UYARI: kaynak yavaşsa (ağ ya da akışlı bulut sürücüsü)
	// rastgele pencere okuması eğitimi kat kat yavaşlatır; böyle
	// kaynaklarda önbellek kullanın.
	Cache *Cache
}

// Kume, bir dizindeki .npy parçalarının sıralı listesidir. Parçalar ancak
// okunacakları zaman açılır.
type Dataset struct {
	dir    string
	shards []ShardInfo
	total  int64
	meta   *Meta
	pick   DatasetOptions
}

// KumeAc, dizindeki parçaları tanır. dizin tek bir .npy dosyası da olabilir.
// Dizinde ya da üst dizinde meta.json varsa, parça token sayıları oradan
// alınır ve dosya başlıkları açılışta okunmaz (yavaş kaynaklarda önemli);
// sayılar parça açıldığında başlıkla karşılaştırılır.
func OpenDataset(dir string, s DatasetOptions) (*Dataset, error) {
	if s.Pattern == "" {
		s.Pattern = "*.npy"
	}
	st, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("veri: küme açılamadı: %w", err)
	}
	var paths []string
	byteBuf := map[string]int64{}
	metaDirs := []string{dir, filepath.Dir(dir)}
	if st.IsDir() {
		if _, err := filepath.Match(s.Pattern, ""); err != nil {
			return nil, fmt.Errorf("veri: geçersiz desen %q: %w", s.Pattern, err)
		}
		// Dizin listesi boyutları da verir; parça başına ayrı sorgu gerekmez.
		inputs, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("veri: küme dizini okunamadı: %w", err)
		}
		for _, g := range inputs {
			if g.IsDir() {
				continue
			}
			if ok, _ := filepath.Match(s.Pattern, g.Name()); !ok {
				continue
			}
			info, err := g.Info()
			if err != nil {
				return nil, fmt.Errorf("veri: %s: %w", g.Name(), err)
			}
			y := filepath.Join(dir, g.Name())
			paths = append(paths, y)
			byteBuf[y] = info.Size()
		}
	} else {
		paths = []string{dir}
		byteBuf[dir] = st.Size()
		metaDirs = []string{filepath.Dir(dir), filepath.Dir(filepath.Dir(dir))}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("veri: %s içinde %q desenine uyan veri dosyası yok", dir, s.Pattern)
	}

	k := &Dataset{dir: dir, pick: s}
	if !s.NoMeta {
		for _, d := range metaDirs {
			my := filepath.Join(d, "meta.json")
			if _, err := os.Stat(my); err == nil {
				m, err := ReadMeta(my)
				if err != nil {
					return nil, err
				}
				k.meta = m
				break
			}
		}
	}
	for _, y := range paths {
		ad := filepath.Base(y)
		pb := ShardInfo{Name: ad, Path: y, Token: -1, Bytes: byteBuf[y]}
		if k.meta != nil {
			if mp, ok := k.meta.Parts[ad]; ok {
				pb.Token = mp.Tokens
			}
		}
		if pb.Token < 0 {
			n, err := headerLen(y)
			if err != nil {
				return nil, err
			}
			pb.Token = n
		}
		k.shards = append(k.shards, pb)
		k.total += pb.Token
	}
	return k, nil
}

func headerLen(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("veri: veri dosyası açılamadı: %w", err)
	}
	defer f.Close()
	bs, err := ReadHeader(f)
	if err != nil {
		var bh *FormatError
		if errors.As(err, &bh) {
			bh.Path = path
		}
		return 0, err
	}
	return bs.Length, nil
}

// Dizin, kümenin açıldığı yoldur.
func (k *Dataset) Dir() string { return k.dir }

// Meta, bulunduysa meta.json içeriğidir (yoksa nil).
func (k *Dataset) Meta() *Meta { return k.meta }

// Parcalar, parçaların ada göre sıralı listesidir (kopya).
func (k *Dataset) Shards() []ShardInfo { return append([]ShardInfo(nil), k.shards...) }

// ParcaSayisi, parça sayısıdır.
func (k *Dataset) ShardCount() int { return len(k.shards) }

// ToplamToken, tüm parçalardaki token sayısıdır.
func (k *Dataset) TotalTokens() int64 { return k.total }

// ParcaAc, i'inci parçayı açar. Önbellek etkinse parça önce yerel
// önbelleğe alınır (gerekirse kopyalanmasını bekler) ve Kapat'a kadar
// önbellekten silinmez.
func (k *Dataset) OpenShard(ctx context.Context, i int) (*Shard, error) {
	if i < 0 || i >= len(k.shards) {
		return nil, fmt.Errorf("veri: veri dosyası sırası %d geçersiz (0..%d)", i, len(k.shards)-1)
	}
	pb := k.shards[i]
	path, release := pb.Path, func() {}
	if k.pick.Cache != nil {
		y, b, err := k.pick.Cache.Fetch(ctx, pb.Path)
		if err != nil {
			return nil, err
		}
		path, release = y, b
	}
	p, err := OpenShard(path, k.pick.Shard)
	if err != nil {
		release()
		return nil, err
	}
	p.release = release
	if p.Length() != pb.Token {
		_ = p.Close()
		return nil, fmt.Errorf("veri: %s: başlık %d token bildiriyor, meta.json %d diyor", pb.Path, p.Length(), pb.Token)
	}
	return p, nil
}

// onbellekKotaDenetle, önbellek etkinse kotanın aynı anda gereken parça
// sayısına (açık parçalar + ileri bakış) en büyük parça boyutuyla yettiğini
// denetler. enKisa tokendan kısa parçalar hiç açılmadığı için sayılmaz.
func (k *Dataset) checkCacheQuota(count int, shortest int64) error {
	ob := k.pick.Cache
	if ob == nil {
		return nil
	}
	var largest int64
	fits := 0
	for _, p := range k.shards {
		if p.Token >= shortest {
			fits++
			largest = max(largest, p.Bytes)
		}
	}
	count = min(count, fits)
	needed := int64(count) * largest
	if ob.Quota() < needed {
		return fmt.Errorf("%w: kota %d bayt, en az %d veri dosyası × en büyük veri dosyası %d bayt = %d bayt gerekli "+
			"(kotayı büyütün ya da ileri bakışı/grup boyunu küçültün)",
			ErrCacheInsufficient, ob.Quota(), count, largest, needed)
	}
	// Klasörü paylaşan diğer kümelerin ayırdığı yerle birlikte de sığmalı.
	return ob.Reserve(needed)
}

// onbellekHazirla, önbellek etkinse verilen parçaların arka planda
// kopyalanmasını ister.
func (k *Dataset) prepareCache(orders []int) {
	if k.pick.Cache == nil {
		return
	}
	paths := make([]string, len(orders))
	for i, s := range orders {
		paths[i] = k.shards[s].Path
	}
	k.pick.Cache.Prepare(paths)
}
