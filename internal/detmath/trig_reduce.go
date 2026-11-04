// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package detmath

import (
	"math"
	"math/bits"
)

// reduceThreshold, Pi/4'ü 3 float64 parçayla kullanan indirgemenin hâlâ
// doğru sonuç verdiği en büyük x değeridir. Bu eşik, y*C çarpımının
// float64 olarak hatasız gösterilebilmesinden gelir; burada
// y = floor(x * (4 / Pi)) ve C, 4/Pi'nin baştaki kısmi terimleridir.
// Baştaki terimlerin (sin.go'daki PI4A ve PI4B) 30 ve 32 sondaki sıfır biti
// olduğundan y'nin 30'dan az anlamlı biti olmalıdır.
//
//	y < 1<<30  -> floor(x*4/Pi) < 1<<30 -> x < (1<<30 - 1) * Pi/4
//
// Dolayısıyla temkinli olarak x < 1<<29 alınabilir.
// Bu eşiğin üstünde Payne-Hanek aralık indirgemesi kullanılmalıdır.
const reduceThreshold = 1 << 29

// trigReduce, x > 0 için Pi/4 ile Payne-Hanek aralık indirgemesini
// gerçekler. x / (Pi/4)'ün 8 modülündeki tamsayı kısmını (j) ve
// kesir kısmını (z) verir.
// Gerçekleme şu çalışmaya dayanır:
// "ARGUMENT REDUCTION FOR HUGE ARGUMENTS: Good to the Last Bit"
// K. C. Ng et al, March 24, 1992
// x*B'nin benzetimle çok duyarlıklı hesabı 64 bitlik tamsayı aritmetiği kullanır.
func trigReduce(x float64) (j uint64, z float64) {
	const PI4 = math.Pi / 4
	if x < PI4 {
		return 0, x
	}
	// Tamsayıyı ve üssü şöyle ayır:
	// x = ix * 2 ** exp.
	ix := math.Float64bits(x)
	exp := int(ix>>shift&mask) - bias - shift
	ix &^= mask << shift
	ix |= 1 << shift
	// Üssü kullanarak mPi4'ten uygun 3 uint64 basamağı al,
	// B ~ (z0, z1, z2); çarpımın baştaki basamağı -61 üssünde olsun.
	// Not: x >= PI4 olduğundan exp >= -53; en büyük float64 için exp < 971.
	digit, bitshift := uint(exp+61)/64, uint(exp+61)%64
	z0 := (mPi4[digit] << bitshift) | (mPi4[digit+1] >> (64 - bitshift))
	z1 := (mPi4[digit+1] << bitshift) | (mPi4[digit+2] >> (64 - bitshift))
	z2 := (mPi4[digit+2] << bitshift) | (mPi4[digit+3] >> (64 - bitshift))
	// Mantisi basamaklarla çarp ve üstteki iki basamağı (hi, lo) al.
	z2hi, _ := bits.Mul64(z2, ix)
	z1hi, z1lo := bits.Mul64(z1, ix)
	z0lo := z0 * ix
	lo, c := bits.Add64(z1lo, z2hi, 0)
	hi, _ := bits.Add64(z0lo, z1hi, c)
	// En üstteki 3 bit j'dir.
	j = hi >> 61
	// Kesri ayır ve büyüklüğünü bul.
	hi = hi<<3 | lo>>61
	lz := uint(bits.LeadingZeros64(hi))
	e := uint64(bias - (lz + 1))
	// Örtük mantis bitini temizle ve yerine kaydır.
	hi = (hi << (lz + 1)) | (lo >> (64 - (lz + 1)))
	hi >>= 64 - shift
	// Üssü ekle ve kayan noktalıya çevir.
	hi |= e << shift
	z = math.Float64frombits(hi)
	// Sıfırları başlangıca eşle.
	if j&1 == 1 {
		j++
		j &= 7
		z--
	}
	// Kesir kısmını pi/4 ile çarp.
	return j, float64(z * PI4)
}

// mPi4, 4/pi'nin ikilik basamaklarının uint64 dizisidir,
// yani 4/pi = Sum mPi4[i]*2^(-64*i).
// 19 adet 64 bitlik basamak ve baştaki bir bit, olası en büyük float64 üssünü
// karşılamak için 1217 bit duyarlık verir.
var mPi4 = [...]uint64{
	0x0000000000000001,
	0x45f306dc9c882a53,
	0xf84eafa3ea69bb81,
	0xb6c52b3278872083,
	0xfca2c757bd778ac3,
	0x6e48dc74849ba5c0,
	0x0c925dd413a32439,
	0xfc3bd63962534e7d,
	0xd1046bea5d768909,
	0xd338e04d68befc82,
	0x7323ac7306a673e9,
	0x3908bf177bf25076,
	0x3ff12fffbc0b301f,
	0xde5e2316b414da3e,
	0xda6cfd9e4f96136e,
	0x9e8c7ecd3cbfd45a,
	0xea4f758fd7cbe2f6,
	0x7a0e73ef14a525d4,
	0xd7f6bf623f1aba10,
	0xac06608df8f6d757,
}

// float64 bit alanları (Go'nun bits.go'sundaki sabitler).
const (
	mask  = 0x7FF
	shift = 64 - 11 - 1
	bias  = 1023
)
