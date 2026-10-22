package dataload

import (
	"context"
	"errors"
	"time"
)

// Önbelleğin arka plan işçileri: Prepare'in listesini sırayla önbelleğe alırlar; Wait
// listenin bitmesini bekler.

func (o *cacheCore) run() {
	defer o.wg.Done()
	for {
		select {
		case <-o.ctx.Done():
			return
		case <-o.warning:
		}
		o.mu.Lock()
		list, generation := o.requested, o.generation
		o.active++
		o.mu.Unlock()
		completed := o.fetchList(list, generation)
		o.mu.Lock()
		o.active--
		if completed && generation > o.handled {
			o.handled = generation
		}
		if o.handled == o.generation && o.active == 0 {
			o.pending = false
		}
		o.mu.Unlock()
		if o.ctx.Err() != nil {
			return
		}
	}
}

// fetchList kuşağın listesini önbelleğe alır; liste sonuna kadar işlendiyse (ya da kota
// yetmediği için bırakıldıysa) doğru, yeni bir liste geldiği ya da önbellek kapandığı için
// yarıda kaldıysa yanlış döner.
func (o *cacheCore) fetchList(list []string, generation uint64) bool {
	for _, k := range list {
		if o.ctx.Err() != nil {
			return false
		}
		o.mu.Lock()
		changed := o.generation != generation
		o.mu.Unlock()
		if changed {
			return false // uyarı kanalında yeni liste bekliyor
		}
		_, _, err := o.fetch(o.ctx, k, true)
		if errors.Is(err, ErrCacheInsufficient) {
			return true
		}
	}
	return true
}

func (o *cacheCore) wait(ctx context.Context) error {
	for {
		o.mu.Lock()
		finished := !o.pending || o.closed
		o.mu.Unlock()
		if finished {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}
