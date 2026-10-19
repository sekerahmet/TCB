package logging

import (
	"os"
	"path/filepath"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// Arka plan işi: program bir iş olarak çalışırken (tr başlat) TR_İŞ ortam değişkeni iş
// klasörünü gösterir. Çubuk o zaman terminal kipine geçmez (çıktı dosyaya gider, düz satırlar
// yazılır) ve iş klasörüne saniyede en çok bir kez ilerleme.json anlık görüntüsünü yazar;
// tr izle ve düzenleyici çubuğun aynı satırını bu dosyadan çizer:
//
//	{"adım":1200,"toplam":5000,"ölçüler":{"kayıp":2.31},"hız":85.2,"kalan_sn":840,"zaman":"...","bitti":false}
//
// Dosya geçici bir dosyaya yazılıp yerine konur: okuyan hiçbir zaman yarım dosya görmez.

// isOrtamDegiskeni arka plan işinin klasörünü gösteren ortam değişkenidir.
const isOrtamDegiskeni = "TR_İŞ"

// isDosyasi iş klasöründeki ilerleme anlık görüntüsünün adıdır.
const isDosyasi = "ilerleme.json"

// isAraligi anlık görüntünün en sık yazılma aralığıdır.
const isAraligi = time.Second

// isDurumuYolu ilerleme.json'un yoludur; program bir arka plan işi değilse boş.
func isDurumuYolu() string {
	k := os.Getenv(isOrtamDegiskeni)
	if k == "" {
		return ""
	}
	return filepath.Join(k, isDosyasi)
}

// isDurumunuYaz anlık görüntüyü (son değilse en çok isAraligi'nde bir) yazar. Yazılamazsa
// sessiz kalır: izleme eğitimi durdurmamalı.
func (c *Ilerleme) isDurumunuYaz(simdi time.Time, son bool) {
	if c.isYolu == "" || (!son && simdi.Sub(c.sonIs) < isAraligi) {
		return
	}
	c.sonIs = simdi
	c.mu.Lock()
	k := interp.NewDictVal()
	k.Set(aAdim, interp.NewInt(c.adim))
	if c.toplam > 0 {
		k.Set(aToplam, interp.NewInt(c.toplam))
	} else {
		k.Set(aToplam, interp.NULL)
	}
	olculer := interp.NewDictVal()
	for _, o := range c.olculer {
		olculer.Set(o.ad, o.deger)
	}
	k.Set(aOlculer, olculer)
	if c.hiz > 0 {
		k.Set(aHiz, &interp.FloatVal{Val: yuvarla(c.hiz)})
		if c.toplam > 0 {
			k.Set(aKalan, &interp.FloatVal{Val: yuvarla(float64(c.toplam-c.adim) / c.hiz)})
		}
	}
	k.Set(aZaman, zamanDegeri(simdi))
	k.Set(aBitti, interp.BoolToVal(c.durdu))
	c.mu.Unlock()
	b, err := jsonwrite.WriteOptions(k, jsonwrite.Options{NonFiniteAsText: true, TextFilter: c.m.Mask})
	if err != nil {
		return
	}
	_ = atomikYaz(c.isYolu, b)
}

// atomikYaz veriyi aynı klasörde geçici bir dosyaya yazar ve hedefin yerine koyar.
func atomikYaz(yol string, veri []byte) error {
	f, err := os.CreateTemp(filepath.Dir(yol), "."+isDosyasi+".*")
	if err != nil {
		return err
	}
	gecici := f.Name()
	_, err = f.Write(veri)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(gecici, yol)
	}
	if err != nil {
		os.Remove(gecici)
	}
	return err
}
