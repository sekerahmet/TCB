package process

import (
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// job çalışan bir dış programdır. İptal, süre sınırı ve durdurma onu aynı yolla durdurur (21.5): nazik
// istek, stopGrace, öldürme. Program kendi süreç grubunda başlar, istek ve öldürme başlattığı süreçlere
// de gider. İşletim sistemine bağlı kısım processGroup'tadır.
type job struct {
	cmd      *exec.Cmd
	group    processGroup
	done     chan struct{} // program bitti, çıktıların okunması da bitti
	waitErr  error         // done kapanınca okunur
	stopOnce sync.Once
	stopping atomic.Bool // TR programı durdurdu: çıkışı hata değildir (21.4)
	timedOut atomic.Bool
}

// startJob programı kendi grubunda başlatır.
func startJob(c *exec.Cmd) (*job, error) {
	j := &job{cmd: c, done: make(chan struct{})}
	j.group.prepare(c)
	if err := c.Start(); err != nil {
		return nil, err
	}
	if err := j.group.started(c); err != nil {
		_ = c.Process.Kill()
		_ = c.Wait()
		return nil, err
	}
	go func() {
		defer close(j.done)
		j.waitErr = c.Wait()
	}()
	return j, nil
}

// stop programı durdurur ve bitmesini bekler; bitmiş programda etkisizdir. Yorumlayıcı kilidi
// tutulurken çağrılmaz: en çok stopGrace bekler.
func (j *job) stop() { j.stopFor(nil) }

// stopFor stop gibidir; durdurmayı bu çağrı başlatırsa önce mark çalışır: süre sınırı program
// kendiliğinden bittiyse dolmuş sayılmaz. Program istekle durunca grupta kalan torun da öldürülür.
func (j *job) stopFor(mark *atomic.Bool) {
	j.stopOnce.Do(func() {
		select {
		case <-j.done:
			return
		default:
		}
		if mark != nil {
			mark.Store(true)
		}
		j.stopping.Store(true)
		if j.group.request() {
			timer := time.NewTimer(stopGrace)
			defer timer.Stop()
			select {
			case <-j.done:
			case <-timer.C:
			}
		}
		j.group.kill()
	})
	<-j.done
}

// kill grubu beklemeden öldürür: TR hemen biterken (1.7).
func (j *job) kill() {
	j.stopping.Store(true)
	j.group.kill()
}

// release çağrı biterken çalışır (Go defer'inde). Süren programı durdurur: yığın panikle çözülse de
// geride yetim süreç kalmaz. Sonra grubun kaynakları bırakılır; program kendiliğinden bittiyse torunu
// öldürülmez.
func (j *job) release() {
	j.stop()
	j.group.release(j.stopping.Load())
}
