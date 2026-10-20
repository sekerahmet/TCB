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

// Kontrol noktası dosya biçimi (sürüm 2; sürüm 1 de okunur: o sürümde modellerin yalnızca
// türev izleyen tensörleri kaydedilirdi, sürüm 2'de türev izlemeyenler de kaydedilir; dosya
// düzeni aynıdır). Dosyalar güvenilmeyen girdidir (başkasından
// gelen ya da yarım kopyalanmış bir koşu klasörü): her dosyanın boyu sınırlıdır, JSON katı
// çözülür (bilinmeyen alan, fazladan veri hata), sürüm, adım ve dosya adları birbiriyle
// karşılaştırılır, her dosyanın SHA-256 özeti tutmalıdır. Biçimde yalnızca veri (tensörler,
// sayılar, metinler) vardır; kod çalıştıran ya da yol çözen hiçbir alan yoktur.

const (
	formatVersion = 2 // yazılan sürüm
	oldestVersion = 1 // okunabilen en eski sürüm

	dirName    = "kontrol_noktalari" // <koşu>/kontrol_noktalari/
	latestFile = "son.json"          // en son kaydedilen noktayı gösterir (araçlar için)
	digestFile = "ozet.json"         // noktanın dosyalarının bayt sayısı ve SHA-256 özeti (en son yazılır)
	stateFile  = "durum.json"        // adım, program, ayarlar ve nesnelerin kaydı
	tensorFile = "tensorler.trt"     // parametreler ve iyileştirici durumları (tensör_kaydet biçimi)
	ckptPrefix = "adim_"             // adim_0000001500
	tmpSuffix  = ".tmp-"             // yazılmakta olan nokta: adim_0000001500.tmp-<süreç>
	oldSuffix  = ".eski-"            // yerine yenisi konan aynı adımlı nokta: adim_...eski-<süreç>

	// Boy sınırları (SECURITY.md): JSON dosyaları belleğe okunmadan önce sınanır.
	maxSmallJSON  = 64 << 10 // son.json, ozet.json
	maxStateJSON  = 64 << 20 // durum.json (düz değerler ve okuyucu durumları)
	maxObjects    = 1000     // kontrol_noktası sözlüğündeki nesne sayısı
	maxObjectName = 200      // nesne adının bayt uzunluğu
)

// ckptPattern bir noktanın klasör adıdır: en az 10 basamaklı adım (sıralama sözlük sırasıyla
// da doğru olsun diye sıfırla doldurulur).
var ckptPattern = regexp.MustCompile(`^adim_([0-9]{10,19})$`)

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

// latestRecord son.json'dur.
type latestRecord struct {
	Version int    `json:"sürüm"`
	Step    int64  `json:"adım"`
	Dir     string `json:"klasör"`
}

// digestRecord ozet.json'dur: noktadaki her dosyanın boyu ve özeti.
type digestRecord struct {
	Version int                   `json:"sürüm"`
	Step    int64                 `json:"adım"`
	Files   map[string]fileDigest `json:"dosyalar"`
}

type fileDigest struct {
	Bytes  int64  `json:"bayt"`
	SHA256 string `json:"sha256"`
}

// stateRecord durum.json'dur.
type stateRecord struct {
	Version int    `json:"sürüm"`
	Step    int64  `json:"adım"`
	Time    string `json:"zaman"`
	TR      string `json:"tr"`      // TR sürümü (bilgi için)
	Program string `json:"program"` // programın dosya adı (bilgi için)
	Workers int    `json:"işçi"`    // GOMAXPROCS (bilgi için)
	// Settings programın komut_satırı sonucudur (gizli değerler maskeli); sürdürmede
	// farklıysa uyarı verilir.
	Settings json.RawMessage `json:"ayarlar,omitempty"`
	Objects  []objectRecord  `json:"nesneler"`
}

// objectRecord kontrol_noktası sözlüğündeki bir nesnenin kaydıdır. Tensörleri (parametreler,
// iyileştirici durumu) tensorler.trt'de "<ad>/<tensör adı>" adıyla durur.
type objectRecord struct {
	Name       string          `json:"ad"`
	Kind       string          `json:"tür"`
	Optimizer  string          `json:"iyileştirici,omitempty"`     // AdamW, SGD
	ParamCount int             `json:"parametre_sayısı,omitempty"` // iyileştiricinin
	Length     int64           `json:"uzunluk,omitempty"`          // günlüğün bayt uzunluğu
	Value      json.RawMessage `json:"değer,omitempty"`            // düz değer ya da okuyucu durumu (duz.go)
}

// Nesne türlerinin dosyadaki adları (biçimin parçası; çevrilmez).
var kindNames = strings.Fields("parametreler iyileştirici durumlu günlük değerler")

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
	case version >= oldestVersion && version <= formatVersion:
		return nil
	case version > formatVersion:
		return messages.New(messages.H7421, file, version, oldestVersion, formatVersion)
	}
	return messages.New(messages.H7422, file, version)
}

// decodeDigest ozet.json'u çözer ve noktanın adımıyla karşılaştırır.
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

// decodeState durum.json'u çözer ve noktanın adımıyla karşılaştırır.
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
