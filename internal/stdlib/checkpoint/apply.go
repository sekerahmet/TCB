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

// apply doğrulanmış ve uyumu denetlenmiş noktayı nesnelere iki aşamada yükler (dil tanımı 20.7):
// önce hiçbir nesne değişmeden iyileştirici durumları toplanıp bütün kurallarıyla denetlenir ve
// okuyucular hazırlanır; hepsi geçerse parametreler (yerinde), okuyucular, iyileştiriciler, düz
// değerler yazılır ve günlüklere devam kaydı düşülür. Parametreler iyileştiricilerden önce yazılır:
// 16 bitlik parametrelerin ana kopyaları parametrenin yüklemeden sonraki sürümüne bağlanır.
// Noktanın içeriği bozuksa (ör. geçersiz adım sayacı) hata corruptContent'tir: çağıran bir önceki
// geçerli noktayı önerir.
func (kn *Checkpoint) apply(m *interp.VM, fn string, o *loadedCkpt) error {
	states, err := kn.readStates(o)
	if err != nil {
		return messages.New(messages.H5620, fn, kn.shown+"/"+o.name, err)
	}
	for _, n := range kn.objects {
		if n.kind == kindOptimizer {
			if err := n.op.CheckLoad(messages.Text(messages.H7441, fn, n.name), states[n.name]); err != nil {
				return corruptContent{err}
			}
		}
	}
	if err := kn.checkParamsLive(); err != nil {
		return err
	}
	var commits, discards []func()
	release := func() {
		for _, d := range discards {
			d()
		}
	}
	for _, n := range kn.objects {
		if n.kind == kindStateful {
			commit, discard, err := n.stateful.PrepareState(m, o.objects[n.name].value.(*interp.DictVal))
			if err != nil {
				release()
				err = messages.New(messages.H6626, fn, n.name, err)
				if readerMismatch(err) {
					return err
				}
				return corruptContent{err}
			}
			commits, discards = append(commits, commit), append(discards, discard)
		}
	}
	// Buradan sonra yalnızca kurtarılamayan hatalar kalır (cihaz belleğinin bitmesi, dosyanın
	// doğrulandıktan sonra değişmesi): parametreler okuyuculardan önce yazılır, yarıda kalırsa
	// okuyucular hiç değişmez (20.7).
	if err := kn.writeParams(o); err != nil {
		release()
		return messages.New(messages.H5620, fn, kn.shown+"/"+o.name, err)
	}
	for _, c := range commits {
		c()
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

// checkParamsLive modellerin tensörlerinin yazılabilir (bırakılmamış) olduğunu yazmadan önce
// denetler. TR kodu bırakılmış bir tensöre ulaşamaz; olursa motorda bir iç hatadır ve yükleme
// hiçbir nesneye dokunmadan durur.
func (kn *Checkpoint) checkParamsLive() error {
	for _, n := range kn.objects {
		if n.kind != kindModel {
			continue
		}
		for _, p := range n.params {
			if p.Tensor().IsDropped() {
				return messages.Internal("a released tensor was used")
			}
		}
	}
	return nil
}

// readerMismatch okuyucu durumunun hatası kullanıcının programındaki farkdan mı (başka okuyucu
// türü, veri kümesi, toplu, bağlam ya da tohum) geliyor? Öbür hatalar (eksik ya da geçersiz alan,
// bu veride geçersiz konum) noktanın içeriğinin bozukluğudur.
func readerMismatch(err error) bool {
	return messages.Has(err, messages.H5538) || messages.Has(err, messages.H5539) || messages.Has(err, messages.H5544)
}

// corruptContent noktanın dosyaları sağlam (özetler tutuyor) ama içeriği kurallara uymuyor
// demektir; yükleme hiçbir nesneyi değiştirmeden durmuştur.
type corruptContent struct{ err error }

func (c corruptContent) Error() string { return c.err.Error() }
func (c corruptContent) Unwrap() error { return c.err }

// readStates tensör dosyasını girdi girdi okur ve yalnızca iyileştirici durumlarını nesneye
// göre toplar; model parametrelerine dokunmaz. Çekirdek panikleri (bellek) hataya çevrilir.
func (kn *Checkpoint) readStates(o *loadedCkpt) (map[string]map[string]*tensor.Tensor, error) {
	states := map[string]map[string]*tensor.Tensor{}
	for _, n := range kn.objects {
		if n.kind == kindOptimizer {
			states[n.name] = map[string]*tensor.Tensor{}
		}
	}
	err := kn.walk(o, func(objectName, tensorName string, g ml.TensorEntry) {
		if d, ok := states[objectName]; ok {
			d[tensorName] = tensor.NewOn(tensor.CPU, g.Veri, g.Sekil...)
		}
	})
	return states, err
}

// writeParams tensör dosyasını yeniden, girdi girdi okuyup model parametrelerine yerinde
// yazar (parametrenin cihazına taşıyarak); büyük modelde bellekte ikinci kopya toplanmaz.
func (kn *Checkpoint) writeParams(o *loadedCkpt) error {
	models := map[string]map[string]*ml.TensorValue{}
	for _, n := range kn.objects {
		if n.kind == kindModel {
			p := make(map[string]*ml.TensorValue, len(n.names))
			for i, name := range n.names {
				p[name] = n.params[i]
			}
			models[n.name] = p
		}
	}
	return kn.walk(o, func(objectName, tensorName string, g ml.TensorEntry) {
		if p, ok := models[objectName][tensorName]; ok {
			ml.CopyInto(p.Tensor(), tensor.NewOn(tensor.CPU, g.Veri, g.Sekil...))
		}
	})
}

// walk tensör dosyasının girdilerini sırayla f'ye verir; çekirdek panikleri hataya çevrilir.
func (kn *Checkpoint) walk(o *loadedCkpt, f func(objectName, tensorName string, g ml.TensorEntry)) error {
	var walkErr error
	_, err := bind.Recover(func() bool {
		walkErr = o.walkTensors(func(objectName, tensorName string, g ml.TensorEntry) error {
			f(objectName, tensorName, g)
			return nil
		})
		return true
	}, panicError)
	if err == nil {
		err = walkErr
	}
	return err
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
