package checkpoint

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// apply doğrulanmış ve uyumu denetlenmiş noktayı nesnelere yükler. Sıra önemlidir:
// okuyucular önce (kendi denetimleri hatayla dönerse parametrelere dokunulmamış olur),
// parametreler iyileştiricilerden önce (16 bitlik parametrelerin ana kopyaları parametrenin
// yüklemeden sonraki sürümüne bağlanır).
func (kn *Checkpoint) apply(m *interp.VM, fn string, o *loadedCkpt) error {
	for _, n := range kn.objects {
		if n.kind == kindStateful {
			if err := n.stateful.LoadState(m, o.objects[n.name].value.(*interp.DictVal)); err != nil {
				return messages.New(messages.H6626, fn, n.name, err)
			}
		}
	}
	states, err := kn.loadTensors(o)
	if err != nil {
		return messages.New(messages.H5620, fn, kn.shown+"/"+o.name, err)
	}
	for _, n := range kn.objects {
		switch n.kind {
		case kindOptimizer:
			if err := n.op.LoadState(messages.Text(messages.H7441, fn, n.name), states[n.name]); err != nil {
				return err
			}
		case kindPlain:
			if err := refreshInPlace(n.name, n.value, o.objects[n.name].value); err != nil {
				return messages.New(messages.H2000, fn, err)
			}
		case kindLog:
			kn.writeLogResume(m, fn, n, o)
		}
	}
	return nil
}

// loadTensors tensör dosyasını ikinci kez, girdi girdi okur: model parametrelerine
// yerinde yazar (parametrenin cihazına taşıyarak), iyileştirici durumlarını nesneye göre
// toplar. Çekirdek panikleri (cihaz belleği) hataya çevrilir.
func (kn *Checkpoint) loadTensors(o *loadedCkpt) (map[string]map[string]*tensor.Tensor, error) {
	models := map[string]map[string]*ml.TensorValue{}
	states := map[string]map[string]*tensor.Tensor{}
	for _, n := range kn.objects {
		switch n.kind {
		case kindModel:
			p := make(map[string]*ml.TensorValue, len(n.names))
			for i, name := range n.names {
				p[name] = n.params[i]
			}
			models[n.name] = p
		case kindOptimizer:
			states[n.name] = map[string]*tensor.Tensor{}
		}
	}
	var walkErr error
	_, err := bind.Recover(func() bool {
		walkErr = o.walkTensors(func(objectName, tensorName string, g ml.TensorEntry) error {
			src := tensor.NewOn(tensor.CPU, g.Veri, g.Sekil...)
			if p, ok := models[objectName][tensorName]; ok {
				ml.CopyInto(p.Tensor(), src)
			} else if d, ok := states[objectName]; ok {
				d[tensorName] = src
			}
			return nil
		})
		return true
	}, panicError)
	if err == nil {
		err = walkErr
	}
	return states, err
}

// writeLogResume günlüğe devam kaydını yazar; günlük kayıttakinden kısaysa (silinmiş ya da
// kesilmiş) uyarır.
func (kn *Checkpoint) writeLogResume(m *interp.VM, fn string, n *object, o *loadedCkpt) {
	if u, err := n.log.Len(); err == nil && u < o.objects[n.name].record.Length {
		m.WriteOutput(m.Err, messages.Text(messages.H7403,
			fn, n.name, u, o.objects[n.name].record.Length)+"\n")
	}
	if err := n.log.WriteResume(o.step); err != nil {
		m.WriteOutput(m.Err, messages.Text(messages.H7404, fn, n.name, err)+"\n")
	}
}

// compareSettings komut satırı ayarları kayıttakinden farklıysa not düşer (adım sayısını
// ya da öğrenme oranını bilerek değiştirmek meşrudur, en sık durum budur; uyarı değildir).
func (kn *Checkpoint) compareSettings(m *interp.VM, d *stateRecord) {
	if kn.process == nil || len(d.Settings) == 0 {
		return
	}
	now := settingsText(m, kn.process.Settings())
	if now == nil || bytes.Equal(now, d.Settings) {
		return
	}
	oldV, err1 := readPlain(d.Settings)
	freshV, err2 := readPlain(now)
	old, ok1 := oldV.(*interp.DictVal)
	fresh, ok2 := freshV.(*interp.DictVal)
	if err1 != nil || err2 != nil || !ok1 || !ok2 {
		return
	}
	var diffs []string
	for _, k := range fresh.Order {
		e, var_ := old.Pairs[k]
		if !var_ || e.Inspect() != fresh.Pairs[k].Inspect() {
			oldText := codes.Name(codes.A042)
			if var_ {
				oldText = e.Inspect()
			}
			diffs = append(diffs, fmt.Sprintf("%s: %s → %s", k, oldText, fresh.Pairs[k].Inspect()))
		}
	}
	for _, k := range old.Order {
		if _, ok := fresh.Pairs[k]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: %s → %s", k, old.Pairs[k].Inspect(), codes.Name(codes.A042)))
		}
	}
	if len(diffs) == 0 {
		return
	}
	slices.Sort(diffs)
	m.WriteOutput(m.Err, messages.Text(messages.H7405,
		codes.Name(codes.T702), strings.Join(diffs, ", "))+"\n")
}
