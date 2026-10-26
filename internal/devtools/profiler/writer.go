package profiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Durum dosyasının (profile.json) yazıcısı. Dosya önce aynı klasörde geçici bir dosyaya
// yazılıp yerine konur: okuyan (işi izleyen araç, düzenleyici) hiçbir zaman yarım dosya
// görmez. Yazma hatası eğitimi ASLA durdurmaz:
//
//   - Yerine koyma geçici olarak başarısız olabilir: Windows'ta dosyayı o an okuyan biri
//     varken yeniden adlandırma "erişim reddedildi" verir. Bu yüzden kısa aralıklarla birkaç
//     kez yeniden denenir (en çok ~250 ms; yalnızca profilin goroutine'i bekler).
//   - Yine olmazsa o aralık atlanır: önceki tam dosya yerinde kalır, okuyan yarım dosya
//     görmez; bir sonraki aralık yeniden dener. Son rapor (Finish, WriteFile) atlanmaz,
//     doğrudan yazılır. Bulut sürücüleri (ör. not defterinin bağladığı sürücü) yeniden
//     adlandırmayı hiç desteklemeyebilir: art arda maxReplaceErrors aralıkta olmazsa yerine
//     koyma bırakılır, bir kez uyarılır ve hep doğrudan yazılır (okuyan nadiren yarım dosya
//     görebilir; bir sonraki saniyede yeniden okur).
//   - Doğrudan yazma da başarısızsa bir kez uyarılır (başarılı bir yazmaya kadar), sonraki
//     aralıkta yeniden denenir.
//
// Yazıcı yalnızca profilin saniyelik döngüsünden ve sonda çağrılır; aynı anda iki yazma olmaz.

// ReportFileName 'tr profil --json klasör/' ile yazılan raporun adıdır: koşu klasörünün
// (config.json, metrics.jsonl, results.json ...) hız raporu.
const ReportFileName = "speed.json"

// maxReplaceErrors yerine koymanın bırakıldığı art arda başarısız aralık sayısıdır.
const maxReplaceErrors = 3

// replaceWaits yeniden adlandırma denemeleri arasındaki beklemelerdir.
// Toplam ~250 ms: dosyayı o an okuyan bir araç ya da yeni dosyayı tarayan bir virüs tarayıcısı
// bu kadar sürede bırakır.
var replaceWaits = []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond,
	8 * time.Millisecond, 16 * time.Millisecond, 32 * time.Millisecond, 64 * time.Millisecond, 128 * time.Millisecond}

// WriteFile görüntüyü yola bir kez yazar (atomik; yerine koyma çalışmazsa doğrudan).
// Uyarılar uyari'ye gider; doğrudan yazma da başarısızsa hatayı döndürür.
func WriteFile(fpath string, g *Snapshot, warning func(string)) error {
	y := &statusWriter{fpath: fpath, warning: warning}
	return y.writeFinal(g)
}

// replace geçici dosyayı hedefin yerine koyar (testler başarısız bir sürücüyü benzetir).
var replace = os.Rename

// statusWriter bir durum dosyasının yazıcısıdır.
type statusWriter struct {
	fpath   string
	warning func(string)
	direct  bool // yerine koyma bırakıldı: hep doğrudan yaz
	faulty  int  // yerine koymanın art arda başarısız olduğu aralık sayısı
	// replaceWarned yerine koyma sorunu bildirildi (bir kez); writeWarned doğrudan yazma
	// sorunu bildirildi (başarılı yazma sıfırlar).
	replaceWarned, writeWarned bool
}

// write saniyelik görüntüyü JSON olarak yazar; yerine koyma geçici olarak olmazsa aralığı
// atlar. Hatayı bildirir ve döndürür; çağıran (profilin saniyelik döngüsü) yok sayar:
// program sürer (bkz. dosya başı).
func (y *statusWriter) write(g *Snapshot) error { return y.put(g, false) }

// writeFinal son raporu yazar: yerine koyma olmazsa atlamaz, doğrudan yazar.
func (y *statusWriter) writeFinal(g *Snapshot) error { return y.put(g, true) }

func (y *statusWriter) put(g *Snapshot, final bool) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		y.report(&y.writeWarned, messages.Text(messages.H8220, err))
		return err
	}
	data = append(data, '\n')
	if !y.direct {
		err := atomicWrite(y.fpath, data)
		if err == nil {
			y.faulty, y.writeWarned = 0, false
			return nil
		}
		y.faulty++
		y.direct = y.faulty >= maxReplaceErrors
		if !y.direct && !final {
			return nil // geçici: önceki tam dosya kalır, sonraki aralık yeniden dener
		}
		y.report(&y.replaceWarned, messages.Text(messages.H8221, y.fpath, err))
	}
	if err := os.WriteFile(y.fpath, data, 0o644); err != nil {
		y.report(&y.writeWarned, messages.Text(messages.H8222, y.fpath, err))
		return err
	}
	y.writeWarned = false
	return nil
}

// report bir yazma sorununu uyarı olarak bildirir; bayrak doluysa (aynı sorun bildirildi)
// sessiz kalır.
func (y *statusWriter) report(warned *bool, m string) {
	if *warned || y.warning == nil {
		return
	}
	*warned = true
	y.warning("⚠️ " + messages.Text(messages.H8219, m))
}

// atomicWrite veriyi aynı klasörde geçici bir dosyaya yazar ve hedefin yerine koyar; yerine
// koyma başarısızsa kısa aralıklarla yeniden dener (bkz. dosya başı).
func atomicWrite(fpath string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(fpath), "."+filepath.Base(fpath)+".*.yaziliyor")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = replace(tmp, fpath)
		for _, d := range replaceWaits {
			if err == nil {
				break
			}
			time.Sleep(d)
			err = replace(tmp, fpath)
		}
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}
