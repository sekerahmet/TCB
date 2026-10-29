package logging

import (
	"io"
	"path/filepath"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// Arka plan işi: program bir iş olarak çalışırken (tr başlat) TR_JOB_DIR ortam değişkeni iş
// klasörünü gösterir. Çubuk o zaman terminal kipine geçmez (çıktı dosyaya gider, düz satırlar
// yazılır) ve iş klasörüne saniyede en çok bir kez progress.json anlık görüntüsünü yazar;
// tr watch ve düzenleyici çubuğun aynı satırını bu dosyadan çizer:
//
//	{"step":1200,"total":5000,"metrics":{"kayıp":2.31},"rate":85.2,"remaining_seconds":840,"time":"...","done":false}
//
// Dosya geçici bir dosyaya yazılıp yerine konur: okuyan hiçbir zaman yarım dosya görmez.

// jobDir arka plan işinin klasörüdür (ortam değişkeni TR_env_JOB_DIR, her yazım dilinin adıyla:
// TR_JOB_DIR, TR_JOB_DIR); iş değilse boş.
func jobDir() string {
	k, _ := codes.EnvValue(codes.EnvJobDir)
	return k
}

// jobFile iş klasöründeki ilerleme anlık görüntüsünün adıdır.
const jobFile = "progress.json"

// jobInterval anlık görüntünün en sık yazılma aralığıdır.
const jobInterval = time.Second

// jobStatusPath progress.json'un yoludur; program bir arka plan işi değilse boş.
func jobStatusPath() string {
	k := jobDir()
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
		k.Set(keyTotal, interp.Null)
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
	_ = interp.WriteAtomic(c.jobPath, func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	})
}
