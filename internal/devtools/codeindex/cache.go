package codeindex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Stats bir kurulumun özetidir.
type Stats struct {
	Files     int  // dizindeki dosya sayısı
	Parsed    int  // bu kurulumda yeniden ayrıştırılan dosya sayısı
	FromCache bool // hiçbir şey değişmedi, dizin önbellekten okundu
	Full      bool // önbellek yoktu ya da kullanılamadı, dizin baştan kuruldu
}

// manifestFile önbelleğin bir girdi dosyası hakkında tuttuğudur. Boyut ve değişme zamanı
// aynıysa içerik özeti dosya yeniden okunmadan kullanılır.
type manifestFile struct {
	Hash  string `json:"hash"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
}

// manifest önbelleğin hangi girdilerden kurulduğunu söyler.
type manifest struct {
	IndexVersion int                     `json:"index_version"`
	Schema       int                     `json:"schema_version"`
	Signature    string                  `json:"signature"` // seçeneklerin imzası
	Files        map[string]manifestFile `json:"files"`
}

const (
	manifestName = "manifest.json"
	indexName    = "index.json"
)

// signature dizinin içeriğini etkileyen seçeneklerin imzasıdır: farklı seçenekle kurulmuş
// önbellek kullanılmaz.
func (o Options) signature() string {
	skip := o.SkipDirs
	if skip == nil {
		skip = DefaultSkipDirs
	}
	var names []string
	for n, on := range skip {
		if on {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	lib := "-"
	if o.Library != nil {
		lib = "lib"
	}
	return strings.Join(o.Dirs, ",") + "|" + strings.Join(names, ",") + "|" + lib + "|" + strconv.FormatBool(o.Tracked)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

// cached önbellekten okunan dizin ve girdi listesidir.
type cached struct {
	man   manifest
	index *Index
}

func loadCache(root string, opts Options) *cached {
	dir := filepath.Join(root, CacheDir)
	var man manifest
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil || json.Unmarshal(data, &man) != nil {
		return nil
	}
	if man.IndexVersion != IndexVersion || man.Schema != SchemaVersion || man.Signature != opts.signature() {
		return nil
	}
	data, err = os.ReadFile(filepath.Join(dir, indexName))
	if err != nil {
		return nil
	}
	idx, err := DecodeIndex(data)
	if err != nil || idx.Version != IndexVersion {
		return nil
	}
	return &cached{man: man, index: idx}
}

// saveCache dizini (idx nil değilse) ve manifesti yazar. Önbellek dosyası sıkıştırılmış JSON'dur
// (insan okumaz; okunan biçim `tr dizin --json` ya da EncodeIndex'tir).
func saveCache(root string, man manifest, idx *Index) error {
	dir := filepath.Join(root, CacheDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if idx != nil {
		data, err := json.Marshal(idx)
		if err != nil {
			return err
		}
		if err := WriteFileAtomic(filepath.Join(dir, indexName), data); err != nil {
			return err
		}
	}
	mdata, err := json.Marshal(man)
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(dir, manifestName), mdata)
}

// writeAtomic dosyayı geçici dosyaya yazıp yerine koyar: eşzamanlı okuyan yarım dosya görmez.
func WriteFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp.Name())
		return werr
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// inputs projedeki .tr dosyalarının (köke göre yol) güncel özetleridir. Önceki manifestte boyutu ve
// değişme zamanı aynı olan dosya yeniden okunmaz.
func inputs(root string, opts Options, prev map[string]manifestFile) (map[string]manifestFile, error) {
	x := &indexer{root: root}
	skip := opts.SkipDirs
	if skip == nil {
		skip = DefaultSkipDirs
	}
	out := map[string]manifestFile{}
	for _, rel := range x.discover(opts, skip) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		st, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		cur := manifestFile{Size: st.Size(), MTime: st.ModTime().UnixNano()}
		if p, ok := prev[rel]; ok && p.Size == cur.Size && p.MTime == cur.MTime {
			cur.Hash = p.Hash
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			cur.Hash = hashBytes(data)
		}
		out[rel] = cur
	}
	return out, nil
}

// libraryBytes gömülü kitaplık modülünün (yol "tr:lib/ad.tr") içeriğidir.
func libraryBytes(opts Options, rel string) ([]byte, error) {
	name := strings.TrimSuffix(strings.TrimPrefix(rel, libraryPrefix+"lib/"), ".tr")
	if opts.Library == nil {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(opts.Library, name+".tr")
}

// libraryHash gömülü kitaplık modülünün özetidir; yoksa "".
func libraryHash(opts Options, rel string) string {
	data, err := libraryBytes(opts, rel)
	if err != nil {
		return ""
	}
	return hashBytes(data)
}

// BuildCached dizini önbellekle kurar. Önbellek yoksa ya da başka seçenekle/sürümle kurulmuşsa dizin
// baştan kurulur; hiçbir şey değişmediyse önbellekten okunur; dosyaların yalnızca içeriği
// değiştiyse yalnızca değişen dosyalar ve onları (dolaylı da) aktaran dosyalar yeniden çözülür
// (artımlı yenileme). Dosya eklenmesi/silinmesi ya da gömülü kitaplık kümesinin değişmesi baştan
// kurulumdur. Sonuç her zaman sıfırdan kurulanla aynıdır. Önbellek yazılamazsa dizin yine döner.
func BuildCached(root string, opts Options) (*Index, Stats, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, Stats{}, err
	}
	prev := loadCache(abs, opts)
	var prevFiles map[string]manifestFile
	if prev != nil {
		prevFiles = prev.man.Files
	}
	cur, err := inputs(abs, opts, prevFiles)
	if err != nil {
		return nil, Stats{}, err
	}
	if prev != nil {
		if idx, stats, ok := refresh(abs, opts, prev, cur); ok {
			return idx, stats, nil
		}
	}
	idx, err := Build(abs, opts)
	if err != nil {
		return nil, Stats{}, err
	}
	man := manifest{IndexVersion: IndexVersion, Schema: SchemaVersion, Signature: opts.signature(), Files: cur}
	for _, rel := range idx.Files {
		if strings.HasPrefix(rel, libraryPrefix) {
			man.Files[rel] = manifestFile{Hash: libraryHash(opts, rel)}
		}
	}
	_ = saveCache(abs, man, idx) // önbellek isteğe bağlıdır
	return idx, Stats{Files: len(idx.Files), Parsed: len(idx.Files), Full: true}, nil
}

// CacheStatus önbelleğin durumudur (`tr dizin --durum`).
type CacheStatus struct {
	Exists  bool     // kullanılabilir bir önbellek var
	Files   int      // önbellekteki dosya sayısı
	Changed []string // içeriği değişmiş dosyalar
	Added   []string // önbellekte olmayan dosyalar
	Removed []string // önbellekte olup artık bulunmayan dosyalar
}

// Status önbelleği okuyup güncel dosyalarla karşılaştırır; dizini kurmaz.
func Status(root string, opts Options) (CacheStatus, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return CacheStatus{}, err
	}
	prev := loadCache(abs, opts)
	if prev == nil {
		return CacheStatus{}, nil
	}
	cur, err := inputs(abs, opts, prev.man.Files)
	if err != nil {
		return CacheStatus{}, err
	}
	st := CacheStatus{Exists: true, Files: len(prev.man.Files)}
	for rel, c := range cur {
		p, ok := prev.man.Files[rel]
		switch {
		case !ok:
			st.Added = append(st.Added, rel)
		case p.Hash != c.Hash:
			st.Changed = append(st.Changed, rel)
		}
	}
	for rel := range prev.man.Files {
		if _, ok := cur[rel]; !ok && !strings.HasPrefix(rel, libraryPrefix) {
			st.Removed = append(st.Removed, rel)
		}
	}
	sort.Strings(st.Changed)
	sort.Strings(st.Added)
	sort.Strings(st.Removed)
	return st, nil
}

// LoadCached önbellekteki dizini hiçbir dosyaya bakmadan olduğu gibi okur (`--dizin-yok`: elle
// yenilemeden hızlı sorgu). Kullanılabilir önbellek yoksa ok=false döner.
func LoadCached(root string, opts Options) (*Index, bool) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, false
	}
	if c := loadCache(abs, opts); c != nil {
		return c.index, true
	}
	return nil, false
}
