package ml

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/detmath"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Mel süzgeçleri (23.4): formül librosa'nın filters.mel'idir (0.11.0). Hesap float64'tedir, sonuç
// float32'ye bir kez yuvarlanır. Slaney ölçeği 1000 Hz'e kadar doğrusal, sonra logaritmiktir.

var (
	nameSampleRate = bind.NamedArg(codes.ArgSampleRate)
	nameNFFT       = bind.NamedArg(codes.ArgNFft)
	nameNMels      = bind.NamedArg(codes.ArgNMels)
	nameNorm       = bind.NamedArg(codes.ArgNorm)
	nameMelScale   = bind.NamedArg(codes.ArgMelScale)
	nameFMin       = bind.NamedArg(codes.ArgFMin)
	nameFMax       = bind.NamedArg(codes.ArgFMax)
)

const (
	slaneyStep   = 200.0 / 3 // doğrusal bölgede mel başına Hz
	slaneyKnee   = 1000.0    // logaritmik bölgenin başı (Hz)
	slaneyKneeMs = slaneyKnee / slaneyStep
)

// slaneyLogStep logaritmik bölgenin adımıdır: log(6.4) / 27.
func slaneyLogStep() float64 { return detmath.Log(6.4) / 27 }

// hzToMel frekansı mel'e çevirir.
func hzToMel(hz float64, htk bool) float64 {
	if htk {
		return 2595 * detmath.Log10(1+hz/700)
	}
	if hz >= slaneyKnee {
		return slaneyKneeMs + detmath.Log(hz/slaneyKnee)/slaneyLogStep()
	}
	return hz / slaneyStep
}

// melToHz mel'i frekansa çevirir.
func melToHz(mel float64, htk bool) float64 {
	if htk {
		return 700 * (detmath.Pow(10, mel/2595) - 1)
	}
	if mel >= slaneyKneeMs {
		return slaneyKnee * detmath.Exp(slaneyLogStep()*(mel-slaneyKneeMs))
	}
	return slaneyStep * mel
}

// melPoints süzgeçlerin m + 2 köşesinin frekansıdır (np.linspace'in adımıyla).
func melPoints(m int, fMin, fMax float64, htk bool) []float64 {
	lo, hi := hzToMel(fMin, htk), hzToMel(fMax, htk)
	step := (hi - lo) / float64(m+1)
	out := make([]float64, m+2)
	for i := range out {
		out[i] = melToHz(float64(i)*step+lo, htk)
	}
	out[m+1] = melToHz(hi, htk)
	return out
}

// melSpec TR_mel_filterbank'ın ayarlarıdır.
type melSpec struct {
	rate, nFFT, nMels int
	slaney, htk       bool
	fMin, fMax        float64
}

// melWeights üçgen süzgeçleri kurar; empty boş süzgecin sırasıdır, yoksa -1.
func melWeights(s melSpec) (w []float32, empty int) {
	bins := s.nFFT/2 + 1
	freq := make([]float64, bins)
	for i := range freq {
		freq[i] = float64(i) * float64(s.rate) / float64(s.nFFT)
	}
	f := melPoints(s.nMels, s.fMin, s.fMax, s.htk)
	w = make([]float32, s.nMels*bins)
	empty = -1
	for i := range s.nMels {
		scale := 1.0
		if s.slaney {
			scale = 2 / (f[i+2] - f[i])
		}
		any := false
		for b := range bins {
			lower := (freq[b] - f[i]) / (f[i+1] - f[i])
			upper := (f[i+2] - freq[b]) / (f[i+2] - f[i+1])
			v := min(lower, upper)
			if v <= 0 {
				continue
			}
			any = true
			w[i*bins+b] = float32(v * scale)
		}
		if !any && empty < 0 {
			empty = i
		}
	}
	return w, empty
}

// melArgs adlı argümanları okur ve denetler.
func melArgs(fn any, named map[string]interp.Value) (melSpec, error) {
	var s melSpec
	for _, a := range []struct {
		name bind.Named
		to   *int
	}{{nameSampleRate, &s.rate}, {nameNFFT, &s.nFFT}, {nameNMels, &s.nMels}} {
		v, given := bind.GetNamed(named, a.name)
		if !given {
			return s, messages.New(messages.H7052, fn, a.name, "23.4")
		}
		n, err := positiveInt(fn, a.name, v)
		if err != nil {
			return s, err
		}
		*a.to = n
	}
	norm, given := bind.GetNamed(named, nameNorm)
	if !given {
		return s, messages.New(messages.H7052, fn, nameNorm, "23.4")
	}
	if t, isText := norm.(*interp.StringVal); isText && t.Val == "slaney" {
		s.slaney = true
	} else if norm != interp.Null {
		return s, messages.New(messages.H7077, fn, nameNorm, `"slaney", `+codes.Spelling(codes.KwNone), interp.Shown(norm), "23.4")
	}
	scale, given := bind.GetNamed(named, nameMelScale)
	if !given {
		return s, messages.New(messages.H7052, fn, nameMelScale, "23.4")
	}
	t, isText := scale.(*interp.StringVal)
	if !isText || t.Val != "slaney" && t.Val != "htk" {
		return s, messages.New(messages.H7077, fn, nameMelScale, `"slaney", "htk"`, interp.Shown(scale), "23.4")
	}
	s.htk = t.Val == "htk"
	return s, melRange(fn, named, &s)
}

// melRange frekans aralığını okur: 0 ≤ f_min < f_max ≤ r / 2.
func melRange(fn any, named map[string]interp.Value, s *melSpec) error {
	top := float64(s.rate) / 2
	s.fMin, s.fMax = 0, top
	for _, a := range []struct {
		name bind.Named
		to   *float64
	}{{nameFMin, &s.fMin}, {nameFMax, &s.fMax}} {
		if v, given := bind.GetNamed(named, a.name); given {
			f, ok := interp.ToFloat(v)
			if !ok {
				return messages.New(messages.H6517, fn, a.name, typeName(v))
			}
			*a.to = f
		}
	}
	if !(0 <= s.fMin && s.fMin < s.fMax && s.fMax <= top) {
		return messages.New(messages.H7079, fn, "range", fmt.Sprintf("[%g, %g]", s.fMin, s.fMax), "", "")
	}
	return nil
}

// melBuiltin TR_mel_filterbank(...).
func melBuiltin(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	kd := codes.FnMelFilterbank
	fn := codes.NameRef(kd)
	if err := bind.CheckNamed(fn.Called(), named, nameSampleRate, nameNFFT, nameNMels, nameNorm, nameMelScale, nameFMin, nameFMax); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, interp.ArgCountError(kd, 0, len(args))
	}
	s, err := melArgs(fn, named)
	if err != nil {
		return nil, err
	}
	var weights []float32
	var empty int
	r, err := safe(func() *tensor.Tensor {
		tensor.CheckSize([]int{s.nMels, s.nFFT/2 + 1}, tensor.Float32)
		weights, empty = melWeights(s)
		if empty >= 0 {
			return nil
		}
		return tensor.New(weights, s.nMels, s.nFFT/2+1)
	})
	if err != nil {
		return nil, err
	}
	if empty >= 0 {
		return nil, messages.New(messages.H7079, fn, "empty_filter", empty, "", "")
	}
	return New(r), nil
}
