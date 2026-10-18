//go:build amd64.v3

#include "textflag.h"

// func expVekCekirdek(dst, src *float32, n int, sabit *float32)
//
// dst[i] = e^src[i], i < n (n 8'in katı). sabit, 8'er kez yinelenmiş sabitler
// tablosudur (bkz. cpu_vektor_avx2.go, expSabit): 0 alt sınır, 1 üst sınır,
// 2 log2e, 3 ln2 üst, 4 ln2 alt, 5..10 polinom katsayıları, 11 bir, 12 üs
// sapması (tam sayı 127). Her tablo satırı 32 bayttır.
//
// x [−104, 89] aralığına kırpılır; n = yuvarla(x·log2e), r = x − n·ln2;
// e^r 6. derece polinomla; 2^n iki çarpanla (2^(n>>1)·2^(n−n>>1)) uygulanır,
// böylece normal altı sonuçlar ve taşma (+sonsuz) doğru çıkar. NaN girdiler
// olduğu gibi geçer.
TEXT ·expVekCekirdek(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	MOVQ sabit+24(FP), R9
	SHRQ $3, CX
	JZ   bitti

dongu:
	VMOVUPS      (SI), Y0
	VMAXPS       0(R9), Y0, Y1
	VMINPS       32(R9), Y1, Y1
	VMULPS       64(R9), Y1, Y2
	VROUNDPS     $0, Y2, Y2
	VFNMADD231PS 96(R9), Y2, Y1
	VFNMADD231PS 128(R9), Y2, Y1
	VMOVUPS      160(R9), Y3
	VFMADD213PS  192(R9), Y1, Y3
	VFMADD213PS  224(R9), Y1, Y3
	VFMADD213PS  256(R9), Y1, Y3
	VFMADD213PS  288(R9), Y1, Y3
	VFMADD213PS  320(R9), Y1, Y3
	VMULPS       Y1, Y1, Y4
	VFMADD213PS  Y1, Y4, Y3
	VADDPS       352(R9), Y3, Y3
	VCVTPS2DQ    Y2, Y5
	VPSRAD       $1, Y5, Y6
	VPSUBD       Y6, Y5, Y7
	VPADDD       384(R9), Y6, Y6
	VPADDD       384(R9), Y7, Y7
	VPSLLD       $23, Y6, Y6
	VPSLLD       $23, Y7, Y7
	VMULPS       Y6, Y3, Y3
	VMULPS       Y7, Y3, Y3
	VCMPPS       $3, Y0, Y0, Y8
	VBLENDVPS    Y8, Y0, Y3, Y3
	VMOVUPS      Y3, (DI)
	ADDQ         $32, SI
	ADDQ         $32, DI
	DECQ         CX
	JNZ          dongu

bitti:
	VZEROUPPER
	RET
