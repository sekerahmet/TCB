package profiler

import (
	"sort"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// GPU süresinin sahibi: GPU arka ucu her çekirdeğin cihaz süresini, onu başlatan tensör
// işlemine ve TR satırına yazar (gpu.OwnerSamples; satır, örnekleme açıkken). Konakta ölçülen
// süre GPU'da yalnızca başlatma ve beklemedir; GPU'yu meşgul eden işlem ve satır buradan
// okunur. Geri yöndeki çekirdekler, düğümü üreten ileri satıra yazılır (bkz. tensor/position.go).

// maxGPUOwners rapordaki ve dosyadaki GPU işlem ve satır sayısıdır.
const maxGPUOwners = 15

// gpuOwners a ile before arasındaki çekirdek süresini işlem (tür, yön) ve satıra göre verir.
func (p *Profile) gpuOwners(a, before []gpu.OwnerSample) (ops []GPUOpInfo, lines []GPULineInfo) {
	prev := map[tensor.Owner]gpu.OwnerSample{}
	for _, o := range before {
		prev[o.Owner] = o
	}
	type opKey struct {
		k    tensor.OpKind
		back bool
	}
	opSum := map[opKey]*GPUOpInfo{}
	type lineSum struct{ dur, back time.Duration }
	lineAt := map[uint64]*lineSum{}
	var total time.Duration
	for _, o := range a {
		e := prev[o.Owner]
		d, calls := o.GPUTime-e.GPUTime, o.Calls-e.Calls
		if d <= 0 {
			continue
		}
		total += d
		k := opKey{o.Owner.Kind, o.Owner.Backward}
		s := opSum[k]
		if s == nil {
			s = &GPUOpInfo{Code: k.k.Code(), Name: fullName(k.k, k.back, 1), Direction: "forward"}
			if k.back {
				s.Direction = "backward"
			}
			opSum[k] = s
		}
		s.Calls += calls
		s.Seconds += sn(d)
		if o.Owner.Pos != 0 && p.smp != nil {
			l := lineAt[o.Owner.Pos]
			if l == nil {
				l = &lineSum{}
				lineAt[o.Owner.Pos] = l
			}
			l.dur += d
			if o.Owner.Backward {
				l.back += d
			}
		}
	}
	for _, s := range opSum {
		s.Share = ratio(s.Seconds, sn(total))
		ops = append(ops, *s)
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Seconds != ops[j].Seconds {
			return ops[i].Seconds > ops[j].Seconds
		}
		return ops[i].Name < ops[j].Name
	})
	for pos, l := range lineAt {
		li := p.lineInfo(p.smp.resolve(pos), 0)
		lines = append(lines, GPULineInfo{File: li.File, Line: li.Line, Function: li.Function, Text: li.Text,
			Seconds: sn(l.dur), Share: ratio(sn(l.dur), sn(total)), BackwardShare: ratio(sn(l.back), sn(l.dur))})
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Seconds != lines[j].Seconds {
			return lines[i].Seconds > lines[j].Seconds
		}
		return lines[i].Line < lines[j].Line
	})
	return ops[:min(len(ops), maxGPUOwners)], lines[:min(len(lines), maxGPUOwners)]
}
