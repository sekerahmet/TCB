package logging

import (
	"os"
	"path/filepath"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// Arka plan işi: program bir iş olarak çalışırken (tr başlat) TR_IS_KLASORU ortam değişkeni iş
// klasörünü gösterir. Çubuk o zaman terminal kipine geçmez (çıktı dosyaya gider, düz satırlar
// yazılır) ve iş klasörüne saniyede en çok bir kez ilerleme.json anlık görüntüsünü yazar;
// tr izle ve düzenleyici çubuğun aynı satırını bu dosyadan çizer:
//
//	{"adım":1200,"toplam":5000,"ölçüler":{"kayıp":2.31},"hız":85.2,"kalan_sn":840,"zaman":"...","bitti":false}
//
// Dosya geçici bir dosyaya yazılıp yerine konur: okuyan hiçbir zaman yarım dosya görmez.

// jobEnvVar arka plan işinin klasörünü gösteren ortam değişkenidir.
const jobEnvVar = "TR_IS_KLASORU"

// jobFile iş klasöründeki ilerleme anlık görüntüsünün adıdır.
const jobFile = "ilerleme.json"

// jobInterval anlık görüntünün en sık yazılma aralığıdır.
const jobInterval = time.Second

// jobStatusPath ilerleme.json'un yoludur; program bir arka plan işi değilse boş.
func jobStatusPath() string {
	k := os.Getenv(jobEnvVar)
	if k == "" {
		return ""
	}
	return filepath.Join(k, jobFile)
}

// writeJobStatus anlık görüntüyü (son değilse en çok jobInterval'nde bir) yazar. Yazılamazsa
// sessiz kalır: izleme eğitimi durdurmamalı.
func (c *Progress) writeJobStatus(now time.Time, last bool) {
	if c.jobPath == "" || (!last && now.Sub(c.lastJob) < jobInterval) {
		return
	}
	c.lastJob = now
	c.mu.Lock()
	k := interp.NewDictVal()
	k.Set(keyStep, interp.NewInt(c.step))
	if c.total > 0 {
		k.Set(keyTotal, interp.NewInt(c.total))
	} else {
		k.Set(keyTotal, interp.NULL)
	}
	metrics := interp.NewDictVal()
	for _, o := range c.metrics {
		metrics.Set(o.name, o.value)
	}
	k.Set(keyMetrics, metrics)
	if c.rate > 0 {
		k.Set(keyRate, &interp.FloatVal{Val: round(c.rate)})
		if c.total > 0 {
			k.Set(keyRemaining, &interp.FloatVal{Val: round(float64(c.total-c.step) / c.rate)})
		}
	}
	k.Set(keyTime, timeValue(now))
	k.Set(keyDone, interp.BoolToVal(c.stopped))
	c.mu.Unlock()
	b, err := jsonwrite.WriteOptions(k, jsonwrite.Options{NonFiniteAsText: true, TextFilter: c.m.Mask})
	if err != nil {
		return
	}
	_ = atomicWrite(c.jobPath, b)
}

// atomicWrite veriyi aynı klasörde geçici bir dosyaya yazar ve hedefin yerine koyar.
func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+jobFile+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cErr := f.Close(); err == nil {
		err = cErr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}
