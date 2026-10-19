package profil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Durum dosyasının (profil.json) yazıcısı. Dosya önce aynı klasörde geçici bir dosyaya
// yazılıp yerine konur: okuyan (işi izleyen araç, düzenleyici) hiçbir zaman yarım dosya
// görmez. Yazma hatası eğitimi ASLA durdurmaz:
//
//   - Yerine koyma geçici olarak başarısız olabilir: Windows'ta dosyayı o an okuyan biri
//     varken yeniden adlandırma "erişim reddedildi" verir. Bu yüzden kısa aralıklarla birkaç
//     kez yeniden denenir (en çok ~250 ms; yalnızca profilin goroutine'i bekler).
//   - Yine olmazsa o aralıkta dosya doğrudan yazılır (okuyan nadiren yarım dosya görebilir;
//     okuyucu bir sonraki saniyede yeniden okur) ve bir kez uyarılır. Bulut sürücüleri (ör. not
//     defterinin bağladığı sürücü) yeniden adlandırmayı hiç desteklemeyebilir: art arda
//     enCokYerineKoymaHatasi aralıkta olmazsa yerine koyma bırakılır, hep doğrudan yazılır.
//   - Doğrudan yazma da başarısızsa bir kez uyarılır (başarılı bir yazmaya kadar), sonraki
//     aralıkta yeniden denenir.
//
// Yazıcı yalnızca profilin saniyelik döngüsünden ve sonda çağrılır; aynı anda iki yazma olmaz.

// RaporDosyasiAdi 'tr profil --json klasör/' ile yazılan raporun adıdır: koşu klasörünün
// (config.json, metrics.jsonl, results.json ...) hız raporu.
const RaporDosyasiAdi = "speed.json"

// enCokYerineKoymaHatasi yerine koymanın bırakıldığı art arda başarısız aralık sayısıdır.
const enCokYerineKoymaHatasi = 3

// yerineKoymaBeklemeleri yeniden adlandırma denemeleri arasındaki beklemelerdir.
// Toplam ~250 ms: dosyayı o an okuyan bir araç ya da yeni dosyayı tarayan bir virüs tarayıcısı
// bu kadar sürede bırakır.
var yerineKoymaBeklemeleri = []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond,
	8 * time.Millisecond, 16 * time.Millisecond, 32 * time.Millisecond, 64 * time.Millisecond, 128 * time.Millisecond}

// DosyayaYaz görüntüyü yola bir kez yazar (atomik; yerine koyma çalışmazsa doğrudan).
// Uyarılar uyari'ye gider; doğrudan yazma da başarısızsa hatayı döndürür.
func DosyayaYaz(yol string, g *Goruntu, uyari func(string)) error {
	y := &durumYazici{yol: yol, uyari: uyari}
	return y.yaz(g)
}

// yerineKoy geçici dosyayı hedefin yerine koyar (testler başarısız bir sürücüyü benzetir).
var yerineKoy = os.Rename

// durumYazici bir durum dosyasının yazıcısıdır.
type durumYazici struct {
	yol      string
	uyari    func(string)
	dogrudan bool // yerine koyma bırakıldı: hep doğrudan yaz
	hatali   int  // yerine koymanın art arda başarısız olduğu aralık sayısı
	// koymaUyarildi yerine koyma sorunu bildirildi (bir kez); yazmaUyarildi doğrudan yazma
	// sorunu bildirildi (başarılı yazma sıfırlar).
	koymaUyarildi, yazmaUyarildi bool
}

// yaz görüntüyü JSON olarak yazar. Hatayı bildirir ve döndürür; çağıran (profilin
// saniyelik döngüsü) yok sayar: program sürer (bkz. dosya başı).
func (y *durumYazici) yaz(g *Goruntu) error {
	veri, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		y.bildir(&y.yazmaUyarildi, fmt.Sprintf("profil durumu kodlanamadı: %v", err))
		return err
	}
	veri = append(veri, '\n')
	if !y.dogrudan {
		err := atomikYaz(y.yol, veri)
		if err == nil {
			y.hatali, y.yazmaUyarildi = 0, false
			return nil
		}
		y.hatali++
		y.dogrudan = y.hatali >= enCokYerineKoymaHatasi
		y.bildir(&y.koymaUyarildi, fmt.Sprintf("%s yerine konamadı (%v); dosya doğrudan yazılacak (okuyan nadiren yarım dosya görebilir)", y.yol, err))
	}
	if err := os.WriteFile(y.yol, veri, 0o644); err != nil {
		y.bildir(&y.yazmaUyarildi, fmt.Sprintf("%s yazılamadı (%v); program sürüyor, sonraki aralıkta yeniden denenecek", y.yol, err))
		return err
	}
	y.yazmaUyarildi = false
	return nil
}

// bildir bir yazma sorununu uyarı olarak bildirir; bayrak doluysa (aynı sorun bildirildi)
// sessiz kalır.
func (y *durumYazici) bildir(uyarildi *bool, m string) {
	if *uyarildi || y.uyari == nil {
		return
	}
	*uyarildi = true
	y.uyari("⚠️ profil: " + m)
}

// atomikYaz veriyi aynı klasörde geçici bir dosyaya yazar ve hedefin yerine koyar; yerine
// koyma başarısızsa kısa aralıklarla yeniden dener (bkz. dosya başı).
func atomikYaz(yol string, veri []byte) error {
	f, err := os.CreateTemp(filepath.Dir(yol), "."+filepath.Base(yol)+".*.yaziliyor")
	if err != nil {
		return err
	}
	gecici := f.Name()
	_, err = f.Write(veri)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = yerineKoy(gecici, yol)
		for _, d := range yerineKoymaBeklemeleri {
			if err == nil {
				break
			}
			time.Sleep(d)
			err = yerineKoy(gecici, yol)
		}
	}
	if err != nil {
		os.Remove(gecici)
	}
	return err
}
