package checkpoint

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Noktanın yazılması (bkz. paket belgesi). Bütün yazma yorumlayıcı kilidi tutularak yapılır:
// tensörler tek tek konağa indirilip yazılır (bellekte modelin ikinci bir kopyası
// toplanmaz) ve kayıt sürerken başka bir görev modeli değiştiremez. Eğitim döngüsü kayıt
// boyunca zaten bekler.

// rename klasör ve dosyaları yerine koyar; testler ağ sürücülerindeki (FUSE)
// yeniden adlandırma hatasını benzetmek için değiştirir.
var rename = os.Rename

// collectError kaydedilecek değerlerin toplanamamasıdır (ör. düz değerli bir sözlüğe tensör
// konmuş): yazma hatası değildir, yeniden denemekle geçmez; hemen bildirilir.
type collectError struct{ error }

// save noktayı kn.adim adımıyla yazar ve kullanıcıya gösterilecek yolunu verir.
func (kn *Checkpoint) save(m *interp.VM) (string, error) {
	fn := codes.Name(codes.T702)
	state, plan, err := kn.collect(m)
	if err != nil {
		return "", collectError{fmt.Errorf("%s: %v", fn, err)}
	}
	name := ckptName(kn.step)
	shown := kn.shown + "/" + name
	var writeErr error
	if _, err = bind.Recover(func() bool {
		writeErr = kn.writeCkpt(name, state, plan)
		return true
	}, func(r any) error { return fmt.Errorf("%v", r) }); err == nil {
		err = writeErr
	}
	if err != nil {
		var extras []string
		var le *os.LinkError
		if errors.As(err, &le) {
			extras = append(extras, "ağ ve bulut sürücülerinde (ör. bağlanmış Drive) yeniden adlandırma başarısız olabilir: noktaları yerel diske kaydedip koşu klasörünü ara ara oraya kopyalayın")
		}
		return "", turkish.FileError(fn, shown, "kaydedilemedi", err, extras...)
	}
	kn.lastValid = kn.step
	if err := interp.WriteAtomic(filepath.Join(kn.dir, latestFile), func(w io.Writer) error {
		return json.NewEncoder(w).Encode(latestRecord{Version: formatVersion, Step: kn.step, Dir: name})
	}); err != nil {
		m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %s: %s güncellenemedi (nokta kaydedildi; yükleme klasörlere bakar): %v\n", fn, latestFile, turkish.FileErrorReason(latestFile, err)))
	}
	if err := kn.deleteOld(name); err != nil {
		m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %s: eski noktalar silinemedi: %v\n", fn, turkish.FileErrorReason(kn.shown, err)))
	}
	return shown, nil
}

// collect durum kaydını ve tensör planını yorumlayıcı kilidi altında hazırlar.
func (kn *Checkpoint) collect(m *interp.VM) ([]byte, []tensorPlan, error) {
	d := stateRecord{Version: formatVersion, Step: kn.step, Time: clock().Format(time.RFC3339), TR: trVersion(),
		Program: filepath.Base(m.SourceFile), Workers: runtime.GOMAXPROCS(0)}
	if kn.process != nil {
		d.Settings = settingsText(m, kn.process.Settings())
	}
	var plan []tensorPlan
	for _, n := range kn.objects {
		k, err := n.record(m)
		if err != nil {
			return nil, nil, err
		}
		d.Objects = append(d.Objects, k)
		plan = append(plan, n.tensors()...)
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return b, plan, nil
}

// trVersion çalışan TR'nin sürümüdür (bilgi için; geliştirme derlemesinde "(devel)").
func trVersion() string {
	if b, ok := debug.ReadBuildInfo(); ok {
		return b.Main.Version
	}
	return ""
}

// writeCkpt noktayı geçici klasöre yazar, diske işler ve yerine koyar.
func (kn *Checkpoint) writeCkpt(name string, state []byte, plan []tensorPlan) (err error) {
	if b, err := os.Stat(kn.dir); err == nil && !b.IsDir() {
		return turkish.Reason(dirName + " bir dosya; noktaların klasörü olmalı")
	}
	if err := os.MkdirAll(kn.dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(kn.dir, name+tmpSuffix+strconv.Itoa(os.Getpid()))
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tmp)
		}
	}()
	digest := digestRecord{Version: formatVersion, Step: kn.step, Files: map[string]fileDigest{}}
	if digest.Files[tensorFile], err = writeFile(filepath.Join(tmp, tensorFile), func(w io.Writer) error { return writeTensors(w, plan) }); err != nil {
		return err
	}
	if digest.Files[stateFile], err = writeFile(filepath.Join(tmp, stateFile), func(w io.Writer) error {
		_, err := w.Write(state)
		return err
	}); err != nil {
		return err
	}
	cache, _ := json.MarshalIndent(digest, "", "  ")
	if _, err = writeFile(filepath.Join(tmp, digestFile), func(w io.Writer) error {
		_, err := w.Write(cache)
		return err
	}); err != nil {
		return err
	}
	syncDir(tmp)
	if err = replaceFile(tmp, filepath.Join(kn.dir, name)); err != nil {
		return err
	}
	syncDir(kn.dir)
	return nil
}

// writeTensors planı tensör dosyası biçiminde yazar; her tensör yazılırken konağa iner.
func writeTensors(w io.Writer, plan []tensorPlan) error {
	y := ml.YeniTensorYazici(w, len(plan))
	for _, p := range plan {
		if err := y.Yaz(p.name, p.t.Shape(), p.t.DType(), p.t.Data()); err != nil {
			return err
		}
	}
	return y.Bitir()
}

// writeFile yeni bir dosya yazar, diske işler ve boyunu ve SHA-256 özetini verir.
func writeFile(fpath string, write func(io.Writer) error) (fileDigest, error) {
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fileDigest{}, err
	}
	h := sha256.New()
	s := &counter{}
	if err := write(io.MultiWriter(f, h, s)); err != nil {
		f.Close()
		return fileDigest{}, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fileDigest{}, err
	}
	if err := f.Close(); err != nil {
		return fileDigest{}, err
	}
	return fileDigest{Bytes: s.n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// counter yazılan bayt sayısını sayar.
type counter struct{ n int64 }

func (s *counter) Write(p []byte) (int, error) { s.n += int64(len(p)); return len(p), nil }

// replaceFile geçici klasörü hedefin yerine koyar. Aynı adımlı bir nokta varsa (ör. eğitim
// sonunda save(), son aralıklı kayıtla aynı adımda) önce kenara alınır, yenisi yerine
// konunca silinir; arada kesilirse kenardaki yükleme sırasında geri konur.
func replaceFile(tmp, target string) error {
	if _, err := os.Lstat(target); err != nil {
		return rename(tmp, target)
	}
	old := target + oldSuffix + strconv.Itoa(os.Getpid())
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if err := rename(target, old); err != nil {
		return err
	}
	if err := rename(tmp, target); err != nil {
		_ = rename(old, target)
		return err
	}
	return os.RemoveAll(old)
}

// syncDir klasörün girdilerini diske işler (yeniden adlandırma elektrik kesintisinde
// kaybolmasın); desteklenmeyen sistemlerde (Windows) bir şey yapmaz.
func syncDir(fpath string) {
	if f, err := os.Open(fpath); err == nil {
		_ = f.Sync()
		f.Close()
	}
}

// deleteOld en yeni tut noktayı (ve yeni yazılanı) bırakıp eskileri siler.
func (kn *Checkpoint) deleteOld(fresh string) error {
	ckpts, err := listCkpts(kn.dir)
	if err != nil {
		return err
	}
	var errs []error
	for i, n := range ckpts {
		if i < kn.keep || n.name == fresh {
			continue
		}
		errs = append(errs, os.RemoveAll(filepath.Join(kn.dir, n.name)))
	}
	return errors.Join(errs...)
}

// ckptEntry klasördeki bir noktadır.
type ckptEntry struct {
	name string
	step int64
}

// listCkpts klasördeki nokta klasörlerini en yeniden eskiye verir (geçici ve kenara
// alınmış klasörler dışında).
func listCkpts(dir string) ([]ckptEntry, error) {
	inputs, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []ckptEntry
	for _, g := range inputs {
		if step, ok := ckptStep(g.Name()); ok && g.IsDir() {
			out = append(out, ckptEntry{name: g.Name(), step: step})
		}
	}
	slices.SortFunc(out, func(a, b ckptEntry) int { return cmp.Compare(b.step, a.step) })
	return out, nil
}

// readLimited dosyayı en çok sinir bayt olarak okur; daha büyükse hata verir.
func readLimited(fpath string, limit int64) ([]byte, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var b bytes.Buffer
	n, err := io.Copy(&b, io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if n > limit {
		return nil, fmt.Errorf("%s çok büyük (en çok %d bayt)", filepath.Base(fpath), limit)
	}
	return b.Bytes(), nil
}
