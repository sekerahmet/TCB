// Package checkpoint uzun eğitimin kontrol noktasıdır: kontrol_noktası(koşu, {ad: nesne, ...})
// (T702; tip KontrolNoktası, A258) eğitimin bütün durumunu koşu klasörüne atomik kaydeder ve
// yeniden başlatılan programı kaldığı adımdan, kesintisiz eğitimle bit düzeyinde aynı
// sürecek biçimde devam ettirir. Kayıt adım sınırında yapılır: kn.güncelle(adım) (Y010)
// her_dakika süresi dolunca ya da durdurma istenince (Ctrl+C), kn.kaydet() (Y521) hemen.
//
// Klasör düzeni (biçim sürümü 2; bkz. bicim.go):
//
//	<koşu>/kontrol_noktalari/
//	  son.json              {"sürüm":2,"adım":1500,"klasör":"adim_0000001500"} (araçlar için)
//	  adim_0000001500/
//	    tensorler.trt       modellerin tensörleri (türev izlemeyenler dahil), iyileştirici durumları
//	    durum.json          adım, program, ayarlar, okuyucu durumları, düz değerler, günlük boyları
//	    ozet.json           dosyaların bayt sayısı ve SHA-256 özeti (en son yazılır)
//	  adim_0000001000/      tut: 2 → bir önceki de durur
//
// Yazma: nokta önce adim_N.tmp-<süreç> klasörüne yazılıp diske işlenir, sonra tek bir
// yeniden adlandırmayla yerine konur, son.json atomik güncellenir ve tut'u aşan eskiler
// silinir; her aşamada kesilme önceki geçerli noktayı bozmaz. Aralıkla kayıtta yazma hatası
// eğitimi durdurmaz (uyarı, sonraki aralıkta yeniden deneme; art arda 3 başarısızlıkta açık
// hata). Yükleme: en yeni nokta klasörü özetleriyle doğrulanır, bütün nesnelerin uyumu
// hiçbir şey yüklenmeden denetlenir, sonra okuyucular, parametreler (yerinde), iyileştirici
// durumları (parametrelerin cihazına), düz değerler (yerinde) yüklenir ve günlüklere devam
// kaydı yazılır. Bozuk bir nokta açık hatadır ve bir önceki geçerli noktayı önerir.
//
// Kontrol noktası kurulunca ilk kesme isteği programı hemen durdurmaz (interp.VM.RequestInterrupt):
// sıradaki adım sınırında nokta kaydedilir ve program nazikçe (çıkış kodu 3) durur.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini ve öbür motorları içe aktarır.
package checkpoint

import (
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/files"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
)

// CheckpointType KontrolNoktası değerlerinin tip adıdır.
var CheckpointType = interp.TypeName(codes.A258)

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	nameEveryMinutes = bind.NamedArg(codes.T984)
	nameKeep         = bind.NamedArg(codes.T985)
	nameSeed         = bind.NamedArg(codes.T903)
)

// Varsayılanlar ve sınırlar.
const (
	defaultMinutes = 15
	defaultKeep    = 2
	maxMinutes     = 1e6
	// maxFailures art arda bu kadar aralıklı kayıt başarısız olursa eğitim durur.
	maxFailures = 3
)

// clock şimdiki zamandır; testler aralığı sınamak için değiştirir.
var clock = time.Now

// Register kontrol_noktası yerleşiğini VM'ye kaydeder. s programın süreç durumudur (komut
// satırı ayarları noktaya yazılır); nil olabilir.
func Register(v *interp.VM, s *process.State) {
	v.RegisterBuiltinCode(codes.T702, func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Safe(codes.Name(codes.T702), func() (interp.Value, error) { return setup(m, s, args, named) })
	})
}

// Checkpoint TR'de kurulmuş bir kontrol noktasıdır (tip KontrolNoktası). Alanları
// yorumlayıcı kilidi altında değişir.
type Checkpoint struct {
	shown    string // kullanıcıya gösterilen klasör: <koşu>/kontrol_noktalari
	dir      string // çözülmüş klasör
	objects  []*object
	interval time.Duration
	keep     int
	process  *process.State

	resumed   int64     // yüklenen noktanın adımı (yeni koşuda 0)
	step      int64     // son güncelle(adım)
	lastSave  time.Time // son kayıt ya da kayıt denemesi (aralık buradan sayılır)
	failures  int       // art arda başarısız aralıklı kayıt
	lastValid int64     // son geçerli noktanın adımı (-1: yok)
}

func (kn *Checkpoint) Type() interp.ValueType { return CheckpointType }
func (kn *Checkpoint) IsTruthy() bool         { return true }
func (kn *Checkpoint) Inspect() string {
	return fmt.Sprintf("%s(%s, adım %d, %d nesne)", CheckpointType, kn.shown, kn.step, len(kn.objects))
}

// Dir noktaların çözülmüş klasörüdür (<koşu>/kontrol_noktalari).
func (kn *Checkpoint) Dir() string { return kn.dir }

func (kn *Checkpoint) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case codes.Y520:
		return interp.NewInt(kn.resumed), nil
	case codes.Y010:
		return &interp.BuiltinVal{Name: codes.Name(codes.Y010), NoNamedArgs: true, Fn: func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return bind.Safe(interp.CallName(codes.Y010), func() (interp.Value, error) { return kn.update(m, args) })
		}}, nil
	case codes.Y521:
		return &interp.BuiltinVal{Name: codes.Name(codes.Y521), NoNamedArgs: true, Fn: func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return bind.Safe(interp.CallName(codes.Y521), func() (interp.Value, error) { return kn.saveTR(m, args) })
		}}, nil
	}
	return nil, bind.MemberError(string(CheckpointType), prop, codes.Names(codes.Y010, codes.Y520, codes.Y521))
}

// setup kontrol_noktası(koşu, nesneler, her_dakika:, tut:) yerleşiğidir.
func setup(m *interp.VM, s *process.State, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.T702)
	if err := bind.CheckNamed(fn, named, nameEveryMinutes, nameKeep); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, interp.ArgCountError(codes.T702, "2", len(args), "koşu_klasörü, {model: model, op: op, ...}")
	}
	fpath, ok, err := interp.PathArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	if !ok || fpath == "" {
		return nil, interp.ArgTypeError(codes.T702, 1, args[0], codes.A202, codes.A250)
	}
	dict, ok := args[1].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.T702, 2, args[1], codes.A206)
	}
	interval, keep, err := options(fn, named)
	if err != nil {
		return nil, err
	}
	objects, err := splitObjects(fn, dict)
	if err != nil {
		return nil, err
	}
	runDir := bind.ResolvePath(m, fpath)
	if b, err := os.Stat(runDir); err == nil && !b.IsDir() {
		return nil, fmt.Errorf("%s: %q bir dosya; koşu klasörü olmalı (noktalar %s/ altına yazılır)", fn, fpath, dirName)
	}
	kn := &Checkpoint{shown: path.Join(filepath.ToSlash(fpath), dirName), dir: filepath.Join(runDir, dirName),
		objects: objects, interval: interval, keep: keep, process: s, lastValid: -1}
	if err := kn.load(m, fn); err != nil {
		return nil, err
	}
	kn.lastSave = clock()
	m.DeferInterrupts()
	setupUnseededWarning(m)
	return kn, nil
}

// options her_dakika: ve tut: adlı argümanlarını okur.
func options(fn string, named map[string]interp.Value) (time.Duration, int, error) {
	minutes := float64(defaultMinutes)
	if v, ok := bind.GetNamed(named, nameEveryMinutes); ok {
		f, number := interp.ToFloat(v)
		if !number || math.IsNaN(f) || f < 0 || f > maxMinutes {
			return 0, 0, fmt.Errorf("%s: %s 0 ile %g arasında bir sayı (dakika) olmalı, %s verildi", fn, nameEveryMinutes.TR, float64(maxMinutes), interp.TypeAndSummary(v))
		}
		minutes = f
	}
	keep := defaultKeep
	if v, ok := bind.GetNamed(named, nameKeep); ok {
		n, integer := v.(*interp.IntVal)
		if !integer || n.Val < 1 || n.Val > 1000 {
			return 0, 0, fmt.Errorf("%s: %s 1 ile 1000 arasında bir %s olmalı (saklanacak nokta sayısı), %s verildi", fn, nameKeep.TR, codes.Name(codes.A200), interp.TypeAndSummary(v))
		}
		keep = int(n.Val)
	}
	return time.Duration(minutes * float64(time.Minute)), keep, nil
}

// setupUnseededWarning tohumsuz rastgelelik kullanılınca bir kez uyarır: sürdürülen eğitim
// kesintisizle aynı olmayacaktır (paylaşılan üretecin durumu kaydedilemez).
func setupUnseededWarning(m *interp.VM) {
	var one sync.Once
	ml.TohumsuzKancasiAyarla(func() {
		one.Do(func() {
			m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %s kurulduktan sonra tohumsuz rastgelelik kullanıldı (%s ya da %s, %s: verilmeden); sürdürülen eğitim kesintisizle aynı olmayacak. Rastgeleliği %s: ile verin.\n",
				codes.Name(codes.T702), codes.Name(codes.T103), codes.Name(codes.T611), nameSeed.TR, nameSeed.TR))
		})
	})
}

// stepArg güncelle'nin tamamlanan adım argümanını okur.
func stepArg(kd codes.Code, args []interp.Value) (int64, error) {
	if len(args) != 1 {
		return 0, interp.ArgCountError(kd, "1", len(args), "tamamlanan_adım")
	}
	n, ok := args[0].(*interp.IntVal)
	if !ok || n.Val < 0 {
		return 0, fmt.Errorf("%s: tamamlanan adım sayısı 0 ya da pozitif bir %s olmalı, %s verildi", interp.CallPrefix(kd), codes.Name(codes.A200), interp.TypeAndSummary(args[0]))
	}
	return n.Val, nil
}

// update kn.güncelle(adım) metodudur: adım sınırı. Durdurma istendiyse kaydeder ve
// programı durdurur; her_dakika süresi dolduysa kaydeder (kaydettiyse doğru).
func (kn *Checkpoint) update(m *interp.VM, args []interp.Value) (interp.Value, error) {
	n, err := stepArg(codes.Y010, args)
	if err != nil {
		return nil, err
	}
	kn.step = n
	if m.StopRequested() {
		kn.halt(m)
	}
	if clock().Sub(kn.lastSave) < kn.interval {
		return interp.FALSE, nil
	}
	return kn.saveOnInterval(m)
}

// saveOnInterval aralıklı kaydı yapar: yazma hatası eğitimi durdurmaz (uyarı, sonraki
// aralıkta yeniden deneme), art arda maxFailures başarısızlık açık hatadır.
func (kn *Checkpoint) saveOnInterval(m *interp.VM) (interp.Value, error) {
	_, err := kn.save(m)
	kn.lastSave = clock()
	if err == nil {
		kn.failures = 0
		return interp.TRUE, nil
	}
	if _, collecting := err.(collectError); collecting {
		return nil, err
	}
	kn.failures++
	if kn.failures >= maxFailures {
		return nil, fmt.Errorf("%v; kontrol noktası art arda %d kez kaydedilemedi, eğitim durduruldu. %s", err, kn.failures, kn.lastValidText())
	}
	what := "sonraki " + codes.Name(codes.Y010) + "'de"
	if kn.interval > 0 {
		what = durationText(kn.interval) + " sonra"
	}
	m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %v (%d/%d; eğitim sürüyor, %s yeniden denenecek). %s\n",
		err, kn.failures, maxFailures, what, kn.lastValidText()))
	return interp.FALSE, nil
}

// saveTR kn.kaydet() metodudur: hemen kaydeder; hata açık hatadır.
func (kn *Checkpoint) saveTR(m *interp.VM, args []interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.Y521, "0", len(args))
	}
	fpath, err := kn.save(m)
	kn.lastSave = clock()
	if err != nil {
		return nil, err
	}
	kn.failures = 0
	if m.StopRequested() {
		kn.halt(m)
	}
	return files.NewPath(fpath), nil
}

// halt ertelenen kesme isteğini karşılar: noktayı kaydeder (hata uyarıdır) ve programı
// nazikçe durdurur (dönmez).
func (kn *Checkpoint) halt(m *interp.VM) {
	if fpath, err := kn.save(m); err != nil {
		m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: program durdurulurken kontrol noktası kaydedilemedi (%v). %s\n", err, kn.lastValidText()))
	} else {
		m.WriteOutput(m.Err, fmt.Sprintf("Kontrol noktası kaydedildi: adım %d (%s). Sürdürmek için programı aynı komutla yeniden çalıştırın.\n", kn.step, fpath))
	}
	m.InterruptNow()
}

// lastValidText son geçerli noktayı söyler (uyarı ve hata mesajları için).
func (kn *Checkpoint) lastValidText() string {
	if kn.lastValid < 0 {
		return "Henüz geçerli bir kontrol noktası yok"
	}
	return fmt.Sprintf("Son geçerli nokta: adım %d (%s/%s)", kn.lastValid, kn.shown, ckptName(kn.lastValid))
}

// durationText bir aralığı kısaca yazar: 15 dk, 30 sn.
func durationText(d time.Duration) string {
	if d >= time.Minute {
		return strings.Replace(strconv.FormatFloat(d.Minutes(), 'g', 3, 64), ".", ",", 1) + " dk"
	}
	return strings.Replace(strconv.FormatFloat(d.Seconds(), 'g', 3, 64), ".", ",", 1) + " sn"
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*Checkpoint)(nil)
)
