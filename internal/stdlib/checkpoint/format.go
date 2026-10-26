package checkpoint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kontrol noktası dosya biçimi (sürüm 3). Dosya ve klasör adları, JSON anahtarları ve kayıt
// türleri dilden bağımsız İngilizce tanımlayıcılardır (dil tanımı 17.3); değeri bir dil öğesi
// olan alan (iyileştiricinin adı) kodunu taşır. Eski sürümler okunmaz. Dosyalar güvenilmeyen
// girdidir (başkasından gelen ya da yarım kopyalanmış bir koşu klasörü): her dosyanın boyu
// sınırlıdır, JSON katı çözülür (bilinmeyen alan, fazladan veri hata), sürüm, adım ve dosya
// adları birbiriyle karşılaştırılır, her dosyanın SHA-256 özeti tutmalıdır. Biçimde yalnızca
// veri (tensörler, sayılar, metinler) vardır; kod çalıştıran ya da yol çözen hiçbir alan yoktur.

const (
	formatVersion = 3 // yazılan ve okunan tek sürüm

	dirName    = "checkpoints" // <koşu>/checkpoints/
	latestFile = "latest.json" // en son kaydedilen noktayı gösterir (araçlar için)
	digestFile = "digest.json" // noktanın dosyalarının bayt sayısı ve SHA-256 özeti (en son yazılır)
	stateFile  = "state.json"  // adım, program, ayarlar ve nesnelerin kaydı
	tensorFile = "tensors.trt" // parametreler ve iyileştirici durumları (tensör_kaydet biçimi)
	ckptPrefix = "step_"       // step_0000001500
	tmpSuffix  = ".tmp-"       // yazılmakta olan nokta: step_0000001500.tmp-<süreç>
	oldSuffix  = ".old-"       // yerine yenisi konan aynı adımlı nokta: step_...old-<süreç>

	// Boy sınırları (SECURITY.md): JSON dosyaları belleğe okunmadan önce sınanır.
	maxSmallJSON  = 64 << 10 // latest.json, digest.json
	maxStateJSON  = 64 << 20 // state.json (düz değerler ve okuyucu durumları)
	maxObjects    = 1000     // kontrol_noktası sözlüğündeki nesne sayısı
	maxObjectName = 200      // nesne adının bayt uzunluğu
)

// ckptPattern bir noktanın klasör adıdır: en az 10 basamaklı adım (sıralama sözlük sırasıyla
// da doğru olsun diye sıfırla doldurulur).
var ckptPattern = regexp.MustCompile(`^step_([0-9]{10,19})$`)

// ckptName adımın klasör adıdır.
func ckptName(step int64) string { return fmt.Sprintf("%s%010d", ckptPrefix, step) }

// ckptStep klasör adından adımı okur; ad bir nokta adı değilse ok yanlış.
func ckptStep(name string) (int64, bool) {
	e := ckptPattern.FindStringSubmatch(name)
	if e == nil {
		return 0, false
	}
	n, err := strconv.ParseInt(e[1], 10, 64)
	return n, err == nil && ckptName(n) == name
}

// latestRecord latest.json'dur.
type latestRecord struct {
	Version int    `json:"version"`
	Step    int64  `json:"step"`
	Dir     string `json:"dir"`
}

// digestRecord digest.json'dur: noktadaki her dosyanın boyu ve özeti.
type digestRecord struct {
	Version int                   `json:"version"`
	Step    int64                 `json:"step"`
	Files   map[string]fileDigest `json:"files"`
}

type fileDigest struct {
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// stateRecord state.json'dur.
type stateRecord struct {
	Version int    `json:"version"`
	Step    int64  `json:"step"`
	Time    string `json:"time"`
	TR      string `json:"tr"`      // TR sürümü (bilgi için)
	Program string `json:"program"` // programın dosya adı (bilgi için)
	Workers int    `json:"workers"` // GOMAXPROCS (bilgi için)
	// Settings programın komut_satırı sonucudur (gizli değerler maskeli); sürdürmede
	// farklıysa uyarı verilir.
	Settings json.RawMessage `json:"settings,omitempty"`
	Objects  []objectRecord  `json:"objects"`
}

// objectRecord kontrol_noktası sözlüğündeki bir nesnenin kaydıdır. Tensörleri (parametreler,
// iyileştirici durumu) tensors.trt'de "<ad>/<tensör adı>" adıyla durur.
type objectRecord struct {
	Name       string          `json:"name"`
	Kind       string          `json:"kind"`
	Optimizer  string          `json:"optimizer,omitempty"`   // iyileştiricinin kodu: TR_fn_AdamW, TR_fn_SGD
	ParamCount int             `json:"param_count,omitempty"` // iyileştiricinin
	Length     int64           `json:"length,omitempty"`      // günlüğün bayt uzunluğu
	Value      json.RawMessage `json:"value,omitempty"`       // düz değer ya da okuyucu durumu (plain.go)
}

// Nesne türlerinin dosyadaki adları (biçimin parçası; çevrilmez).
var kindNames = strings.Fields("parameters optimizer stateful log values")

const (
	kindModel objectKind = iota
	kindOptimizer
	kindStateful
	kindLog
	kindPlain
)

type objectKind int

func (t objectKind) String() string { return kindNames[t] }

func decodeKind(name string) (objectKind, bool) {
	for i, a := range kindNames {
		if a == name {
			return objectKind(i), true
		}
	}
	return 0, false
}

// decodeJSON b'yi katı çözer: bilinmeyen alan ve değerden sonra fazladan veri hatadır.
func decodeJSON(b []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return messages.New(messages.H7420)
	}
	return nil
}

// checkVersion dosyanın biçim sürümünü denetler.
func checkVersion(file string, version int) error {
	switch {
	case version == formatVersion:
		return nil
	case version > formatVersion:
		return messages.New(messages.H7421, file, version, formatVersion, formatVersion)
	}
	return messages.New(messages.H7422, file, version)
}

// decodeDigest digest.json'u çözer ve noktanın adımıyla karşılaştırır.
func decodeDigest(b []byte, step int64) (*digestRecord, error) {
	var o digestRecord
	if err := decodeJSON(b, &o); err != nil {
		return nil, messages.New(messages.H7423, digestFile, err)
	}
	if err := checkVersion(digestFile, o.Version); err != nil {
		return nil, err
	}
	if o.Step != step {
		return nil, messages.New(messages.H7424, digestFile, o.Step, step)
	}
	if len(o.Files) != 2 {
		return nil, messages.New(messages.H7425, digestFile, len(o.Files), stateFile, tensorFile)
	}
	for _, name := range []string{stateFile, tensorFile} {
		d, ok := o.Files[name]
		if !ok {
			return nil, messages.New(messages.H7426, digestFile, name)
		}
		if d.Bytes < 0 || len(d.SHA256) != 64 || strings.Trim(d.SHA256, "0123456789abcdef") != "" {
			return nil, messages.New(messages.H7427, digestFile, name)
		}
	}
	return &o, nil
}

// decodeState state.json'u çözer ve noktanın adımıyla karşılaştırır.
func decodeState(b []byte, step int64) (*stateRecord, error) {
	var d stateRecord
	if err := decodeJSON(b, &d); err != nil {
		return nil, messages.New(messages.H7423, stateFile, err)
	}
	if err := checkVersion(stateFile, d.Version); err != nil {
		return nil, err
	}
	if d.Step != step {
		return nil, messages.New(messages.H7424, stateFile, d.Step, step)
	}
	if len(d.Objects) == 0 || len(d.Objects) > maxObjects {
		return nil, messages.New(messages.H7428, stateFile, len(d.Objects), maxObjects)
	}
	seen := map[string]bool{}
	for _, n := range d.Objects {
		if err := checkObjectName(n.Name); err != nil {
			return nil, messages.New(messages.H2000, stateFile, err)
		}
		if seen[n.Name] {
			return nil, messages.New(messages.H7429, stateFile, n.Name)
		}
		seen[n.Name] = true
		if _, ok := decodeKind(n.Kind); !ok {
			return nil, messages.New(messages.H7430, stateFile, n.Name, n.Kind)
		}
		if n.Length < 0 || n.ParamCount < 0 {
			return nil, messages.New(messages.H7431, stateFile, n.Name)
		}
	}
	return &d, nil
}

// checkObjectName bir nesne adının (sözlük anahtarı) dosya biçimine uygun olduğunu
// denetler: boş değil, en çok maxObjectName bayt, '/' ve denetim karakteri içermez
// (tensör adları "<ad>/<tensör adı>" biçimindedir).
func checkObjectName(name string) error {
	if name == "" || len(name) > maxObjectName {
		return messages.New(messages.H7432, maxObjectName)
	}
	for _, r := range name {
		if r == '/' || r < 0x20 || r == 0x7f {
			return messages.New(messages.H7433, name)
		}
	}
	return nil
}
