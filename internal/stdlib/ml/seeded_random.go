package ml

import (
	"math/rand"
	randv2 "math/rand/v2"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// Tohumlu rastgelelik (dil tanımı 15.2, 15.26, 15.26a). Tohum üretece doğrudan verilmez, bütün 64
// bitiyle karıştırılır: ardışık tohumların akışları da birbirinden bağımsız olsun. Go'nun
// math/rand kaynağında değildi (tohumu 2³¹−1'e göre indirger, komşu tohumların akışları sabit
// farklıdır), ham PCG(tohum, 0) ise bunu sağlayan bir karışım sözü vermez.

// Akışların etiketleri: aynı tohumla ayrı işlerin akışları da ayrıdır. Değerleri tohumlu sayıların
// parçasıdır, değişmez.
const (
	randnLabel  uint64 = 1
	sampleLabel uint64 = 2
	foldLabel   uint64 = 3
)

// mixSeed değerleri sırayla tek bir 64 bitlik sözcüğe katlar (splitmix64 adımları). Her adım son
// değere göre bire birdir.
func mixSeed(values ...uint64) uint64 {
	h := uint64(0x6A09E667F3BCC909)
	for _, v := range values {
		z := (h ^ v) + 0x9E3779B97F4A7C15
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		h = z ^ (z >> 31)
	}
	return h
}

// seededRandom tohumun ve etiketin akışıdır: paylaşılan üretecin PCG'si (shared_random.go), durumu
// tohumdan ve etiketten karıştırılmış iki sözcük.
func seededRandom(seed int64, label uint64) *rand.Rand {
	s := uint64(seed)
	return rand.New(pcgSource{randv2.NewPCG(mixSeed(s, label), mixSeed(label, s))})
}

// foldIn TR_fold_in(tohum, i): tohumdan ve bir tam sayıdan yeni bir tohum (15.26a).
func foldIn(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if err := bind.CheckNamed(codes.NameRef(codes.FnFoldIn), named); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, interp.ArgCountErrorWriting(codes.FnFoldIn, 2, len(args), "tohum, i")
	}
	var v [2]uint64
	for i, a := range args {
		n, ok := a.(*interp.IntVal)
		if !ok {
			return nil, interp.ArgTypeError(codes.FnFoldIn, i+1, a, codes.TypeInt)
		}
		v[i] = uint64(n.Val)
	}
	return interp.NewInt(int64(mixSeed(foldLabel, v[0], v[1]))), nil
}
