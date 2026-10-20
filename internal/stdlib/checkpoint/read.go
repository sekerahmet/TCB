package checkpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Noktanın yüklenmesi (bkz. paket belgesi). Sıra: (1) en yeni nokta klasörü doğrulanır
// (ozet.json, dosyaların boyu ve SHA-256 özeti, durum.json, tensör dosyasının biçimi ve
// sağlaması); (2) her nesnenin kayda uyumu hiçbir şey yüklenmeden denetlenir; (3) okuyucular,
// parametreler (tensör dosyası ikinci kez, girdi girdi okunarak; bellekte modelin ikinci bir
// kopyası toplanmaz), iyileştirici durumları, düz değerler yüklenir, günlüklere devam kaydı
// yazılır.

// loadedCkpt doğrulanmış bir noktadır.
type loadedCkpt struct {
	name, dir string
	step      int64
	digest    *digestRecord
	state     *stateRecord
	objects   map[string]*pending
}

// load klasörde nokta varsa en yenisini yükler; yoksa bir şey yapmaz (yeni koşu).
func (kn *Checkpoint) load(m *interp.VM, fn string) error {
	if _, err := os.Stat(kn.dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	cleanLeftovers(kn.dir)
	ckpts, err := listCkpts(kn.dir)
	if err != nil {
		return turkish.FileError(fn, kn.shown, messages.H2100, err)
	}
	if len(ckpts) == 0 {
		return nil
	}
	o, err := verifyCkpt(kn.dir, ckpts[0])
	if err != nil {
		return kn.brokenCheckpointError(fn, ckpts, err)
	}
	if err := kn.checkMatch(fn, o); err != nil {
		return err
	}
	if err := kn.apply(m, fn, o); err != nil {
		return err
	}
	kn.resumed, kn.step, kn.lastValid = o.step, o.step, o.step
	kn.compareSettings(m, o.state)
	m.WriteOutput(m.Err, fmt.Sprintf("Kaldığı yerden devam: adım %d (%s/%s)\n", o.step, kn.shown, o.name))
	return nil
}

// brokenCheckpointError en yeni nokta bozuksa açık hatadır; bir önceki geçerli nokta varsa onu
// önerir (bozuk nokta kendiliğinden atlanmaz: kullanıcı neyi kaybettiğini bilerek seçer).
func (kn *Checkpoint) brokenCheckpointError(fn string, ckpts []ckptEntry, reason error) error {
	broken := kn.shown + "/" + ckpts[0].name
	hint := fmt.Sprintf("Başka geçerli nokta yok; baştan başlamak için %q klasörünü silin ya da başka bir koşu klasörü verin", kn.shown)
	for _, n := range ckpts[1:] {
		if _, err := verifyCkpt(kn.dir, n); err == nil {
			hint = fmt.Sprintf("Bir önceki geçerli nokta: %s (adım %d); ondan sürdürmek için bozuk klasörü silin ya da koşu klasörünün dışına taşıyın", n.name, n.step)
			break
		}
	}
	return fmt.Errorf("%s: %q bozuk: %v. %s", fn, broken, reason, hint)
}

// cleanLeftovers yarıda kalmış kayıtların klasörlerini siler: geçici klasörler atılır;
// kenara alınmış (aynı adımlı yenisi yerine konamamış) bir nokta yerine geri konur.
func cleanLeftovers(dir string) {
	inputs, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, g := range inputs {
		name := g.Name()
		if !strings.HasPrefix(name, ckptPrefix) {
			continue
		}
		if i := strings.Index(name, tmpSuffix); i > 0 {
			_ = os.RemoveAll(filepath.Join(dir, name))
		} else if i := strings.Index(name, oldSuffix); i > 0 {
			actual := filepath.Join(dir, name[:i])
			if _, err := os.Lstat(actual); errors.Is(err, fs.ErrNotExist) {
				_ = rename(filepath.Join(dir, name), actual)
			} else {
				_ = os.RemoveAll(filepath.Join(dir, name))
			}
		}
	}
}

// verifyCkpt bir noktanın dosyalarını doğrular ve kayıtlarını okur (hiçbir şey yüklemez).
func verifyCkpt(dir string, n ckptEntry) (*loadedCkpt, error) {
	o := &loadedCkpt{name: n.name, dir: filepath.Join(dir, n.name), step: n.step}
	cache, err := readLimited(filepath.Join(o.dir, digestFile), maxSmallJSON)
	if err != nil {
		return nil, readReason(digestFile, err)
	}
	if o.digest, err = decodeDigest(cache, n.step); err != nil {
		return nil, err
	}
	db, err := readLimited(filepath.Join(o.dir, stateFile), maxStateJSON)
	if err != nil {
		return nil, readReason(stateFile, err)
	}
	if err := digestMatches(stateFile, o.digest.Files[stateFile], int64(len(db)), sha256Text(db)); err != nil {
		return nil, err
	}
	if o.state, err = decodeState(db, n.step); err != nil {
		return nil, err
	}
	if o.state.Version != o.digest.Version {
		return nil, fmt.Errorf("%s biçim sürümü %d, %s %d diyor", stateFile, o.state.Version, digestFile, o.digest.Version)
	}
	o.objects = map[string]*pending{}
	for _, k := range o.state.Objects {
		o.objects[k.Name] = &pending{record: k, shapes: map[string][]int{}, kinds: map[string]tensor.DType{}}
	}
	err = o.walkTensors(func(objectName, tensorName string, g ml.TensorEntry) error {
		y := o.objects[objectName]
		y.shapes[tensorName], y.kinds[tensorName] = g.Sekil, g.Tur
		y.order = append(y.order, tensorName)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return o, nil
}

// walkTensors tensör dosyasını girdi girdi okur; her girdinin adı "<nesne>/<tensör>"
// olmalı ve nesne tensör tutan bir türden olmalıdır. Dosyanın boyu ve SHA-256 özeti
// ozet.json'la, sağlaması kendi biçimiyle denetlenir (gez yalnızca bunlar tutarsa güvenilir).
func (o *loadedCkpt) walkTensors(walk func(objectName, tensorName string, g ml.TensorEntry) error) error {
	fpath := filepath.Join(o.dir, tensorFile)
	f, err := os.Open(fpath)
	if err != nil {
		return readReason(tensorFile, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return readReason(tensorFile, err)
	}
	want := o.digest.Files[tensorFile]
	if info.Size() != want.Bytes {
		return fmt.Errorf("%s %d bayt, %s %d bayt diyor (dosya yarım kopyalanmış ya da değişmiş)", tensorFile, info.Size(), digestFile, want.Bytes)
	}
	h := sha256.New()
	err = ml.WalkTensorFile(io.TeeReader(f, h), info.Size(), func(g ml.TensorEntry) error {
		objectName, tensorName, ok := strings.Cut(g.Ad, "/")
		y := o.objects[objectName]
		if !ok || y == nil || (y.record.Kind != kindModel.String() && y.record.Kind != kindOptimizer.String()) {
			return fmt.Errorf("%q girdisi kayıttaki bir modele ya da iyileştiriciye ait değil", g.Ad)
		}
		return walk(objectName, tensorName, g)
	})
	if err != nil {
		return fmt.Errorf("%s: %v", tensorFile, err)
	}
	return digestMatches(tensorFile, want, info.Size(), hashText(h))
}

func sha256Text(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hashText(h hash.Hash) string { return hex.EncodeToString(h.Sum(nil)) }

// digestMatches dosyanın boyunu ve özetini ozet.json'daki kayıtla karşılaştırır.
func digestMatches(name string, want fileDigest, nBytes int64, digest string) error {
	if nBytes != want.Bytes || digest != want.SHA256 {
		return fmt.Errorf("%s özeti tutmuyor (dosya değişmiş, bozulmuş ya da yarım kopyalanmış)", name)
	}
	return nil
}

// readReason bir dosyanın okunamama nedenini Türkçe söyler.
func readReason(name string, err error) error {
	return fmt.Errorf("%s okunamadı: %s", name, turkish.FileErrorReason(name, err))
}

// checkMatch şimdiki nesnelerin kayıttakilerle aynı olduğunu ve her birinin kaydına
// uyduğunu, hiçbir şey yüklemeden denetler.
func (kn *Checkpoint) checkMatch(fn string, o *loadedCkpt) error {
	for _, n := range kn.objects {
		y, ok := o.objects[n.name]
		if !ok {
			return fmt.Errorf("%s: %q kayıtta yok (kayıttaki nesneler: %s); aynı nesnelerle sürdürün ya da yeni bir koşu klasörü verin", fn, n.name, o.objectNames())
		}
		if err := n.checkMatch(fn, y, o.state.Version); err != nil {
			return err
		}
	}
	if len(o.objects) != len(kn.objects) {
		for _, k := range o.state.Objects {
			if !kn.hasObject(k.Name) {
				return fmt.Errorf("%s: kayıttaki %q (%s) verilmedi; aynı nesnelerle sürdürün ya da yeni bir koşu klasörü verin", fn, k.Name, k.Kind)
			}
		}
	}
	return nil
}

func (kn *Checkpoint) hasObject(name string) bool {
	for _, n := range kn.objects {
		if n.name == name {
			return true
		}
	}
	return false
}

func (o *loadedCkpt) objectNames() string {
	names := make([]string, len(o.state.Objects))
	for i, k := range o.state.Objects {
		names[i] = k.Name
	}
	return strings.Join(names, ", ")
}
