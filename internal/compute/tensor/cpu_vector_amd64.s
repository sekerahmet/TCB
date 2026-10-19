#include "textflag.h"

// UST: Y3 = e^Y0 (8 eleman; Y0 korunur, Y1, Y2, Y4-Y8 bozulur). R9 üstel sabit
// tablosunu gösterir (bkz. cpu_vektor_avx2.go, expSabit): 0 alt sınır, 1 üst
// sınır, 2 log2e, 3 ln2 üst, 4 ln2 alt, 5..10 polinom katsayıları, 11 bir, 12 üs
// sapması (tam sayı 127). Her tablo satırı 32 bayttır.
//
// x [−104, 89] aralığına kırpılır; n = yuvarla(x·log2e), r = x − n·ln2;
// e^r 6. derece polinomla; 2^n iki çarpanla (2^(n>>1)·2^(n−n>>1)) uygulanır,
// böylece normal altı sonuçlar ve taşma (+sonsuz) doğru çıkar. NaN girdiler
// olduğu gibi geçer.
#define UST \
	VMAXPS       0(R9), Y0, Y1;    \
	VMINPS       32(R9), Y1, Y1;   \
	VMULPS       64(R9), Y1, Y2;   \
	VROUNDPS     $0, Y2, Y2;       \
	VFNMADD231PS 96(R9), Y2, Y1;   \
	VFNMADD231PS 128(R9), Y2, Y1;  \
	VMOVUPS      160(R9), Y3;      \
	VFMADD213PS  192(R9), Y1, Y3;  \
	VFMADD213PS  224(R9), Y1, Y3;  \
	VFMADD213PS  256(R9), Y1, Y3;  \
	VFMADD213PS  288(R9), Y1, Y3;  \
	VFMADD213PS  320(R9), Y1, Y3;  \
	VMULPS       Y1, Y1, Y4;       \
	VFMADD213PS  Y1, Y4, Y3;       \
	VADDPS       352(R9), Y3, Y3;  \
	VCVTPS2DQ    Y2, Y5;           \
	VPSRAD       $1, Y5, Y6;       \
	VPSUBD       Y6, Y5, Y7;       \
	VPADDD       384(R9), Y6, Y6;  \
	VPADDD       384(R9), Y7, Y7;  \
	VPSLLD       $23, Y6, Y6;      \
	VPSLLD       $23, Y7, Y7;      \
	VMULPS       Y6, Y3, Y3;       \
	VMULPS       Y7, Y3, Y3;       \
	VCMPPS       $3, Y0, Y0, Y8;   \
	VBLENDVPS    Y8, Y0, Y3, Y3

// TOPLA8: xa[0] = 8 float64 kulvarın toplamı; kulvarlar ya (0-3) ve yb (4-7)
// yazmaçlarındadır. Sıra sabittir: tk = k + (k+4), sonra (t0+t2) + (t1+t3)
// (Go eşi: topla8). xt geçici; ya'nın üst yarısı sıfırlanır.
#define TOPLA8(ya, yb, xa, xt) \
	VADDPD       yb, ya, ya;       \
	VEXTRACTF128 $1, ya, xt;       \
	VADDPD       xt, xa, xa;       \
	VUNPCKHPD    xa, xa, xt;       \
	VADDSD       xt, xa, xa

// IKIYE64: 8 float32 (ys, xs) iki 4'lü float64 yazmacına (ylo, yhi) genişletilir.
#define IKIYE64(ys, xs, ylo, yhi, xhi) \
	VCVTPS2PD    xs, ylo;          \
	VEXTRACTF128 $1, ys, xhi;      \
	VCVTPS2PD    xhi, yhi

// MASKE: DX kuyruk eleman sayısı (0-7) için Y14 float32 kuyruk maskesi, Y13 ve
// Y12 aynı maskenin float64 alt ve üst yarıları. CX bozulur.
#define MASKE \
	LEAQ      ·vecMask(SB), CX;   \
	NEGQ      DX;                  \
	VMOVDQU   32(CX)(DX*4), Y14;   \
	NEGQ      DX;                  \
	VPMOVSXDQ X14, Y13;            \
	VEXTRACTI128 $1, Y14, X12;     \
	VPMOVSXDQ X12, Y12

// func expVekCekirdek(dst, src *float32, n int, sabit *float32)
//
// dst[i] = e^src[i], i < n (n 8'in katı).
TEXT ·expVecKernel(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	MOVQ constant+24(FP), R9
	SHRQ $3, CX
	JZ   bitti

dongu:
	VMOVUPS (SI), Y0
	UST
	VMOVUPS Y3, (DI)
	ADDQ    $32, SI
	ADDQ    $32, DI
	DECQ    CX
	JNZ     dongu

bitti:
	VZEROUPPER
	RET

// GELUARG: Y10 = x için Y11 = x², Y0 = −2c·(x + a·x³) (R10 = geluSabit: 0 a,
// 1 −2c, 2 bir, 3 c, 4 3a). Y12 bozulur.
#define GELUARG \
	VMULPS Y10, Y10, Y11;      \
	VMULPS Y10, Y11, Y12;      \
	VMULPS 0(R10), Y12, Y12;   \
	VADDPS Y10, Y12, Y12;      \
	VMULPS 32(R10), Y12, Y0

// GELUTUREV: Y10 = x, Y11 = x², Y3 = e^{−2u} için Y7 = GELU'nun türevi:
// s = 1/(1+e), q = s(1−s), d = s + ((2x·q)·du), du = c·(1 + 3a·x²); q = 0 iken
// (|x| büyük) d = s (ikinci terim 0·sonsuz NaN üretmesin). Y4-Y6, Y8, Y9 bozulur.
#define GELUTUREV \
	VADDPS    64(R10), Y3, Y3;   \
	VMOVUPS   64(R10), Y4;       \
	VDIVPS    Y3, Y4, Y4;        \
	VMOVUPS   64(R10), Y5;       \
	VSUBPS    Y4, Y5, Y5;        \
	VMULPS    Y5, Y4, Y5;        \
	VMULPS    128(R10), Y11, Y6; \
	VADDPS    64(R10), Y6, Y6;   \
	VMULPS    96(R10), Y6, Y6;   \
	VADDPS    Y10, Y10, Y7;      \
	VMULPS    Y5, Y7, Y7;        \
	VMULPS    Y6, Y7, Y7;        \
	VADDPS    Y4, Y7, Y7;        \
	VXORPS    Y8, Y8, Y8;        \
	VCMPPS    $0, Y8, Y5, Y9;    \
	VBLENDVPS Y9, Y4, Y7, Y7

// func geluIleriCekirdek(y, x *float32, n int)
//
// y = x / (1 + e^{−2u}), u = c·(x + a·x³) (tanh yaklaşımlı GELU; Go eşi geluIleriGo).
TEXT ·geluForwardKernel(SB), NOSPLIT, $0-24
	MOVQ y+0(FP), DI
	MOVQ x+8(FP), SI
	MOVQ n+16(FP), BX
	LEAQ ·expConst(SB), R9
	LEAQ ·geluConst(SB), R10
	MOVQ BX, DX
	SHRQ $3, BX
	ANDQ $7, DX
	MASKE
	XORQ  AX, AX
	TESTQ BX, BX
	JZ    gi_k

gi_d:
	VMOVUPS (SI)(AX*1), Y10
	GELUARG
	UST
	VADDPS  64(R10), Y3, Y3
	VDIVPS  Y3, Y10, Y3
	VMOVUPS Y3, (DI)(AX*1)
	ADDQ    $32, AX
	DECQ    BX
	JNZ     gi_d

gi_k:
	TESTQ      DX, DX
	JZ         gi_s
	VMASKMOVPS (SI)(AX*1), Y14, Y10
	GELUARG
	UST
	VADDPS     64(R10), Y3, Y3
	VDIVPS     Y3, Y10, Y3
	VMASKMOVPS Y3, Y14, (DI)(AX*1)

gi_s:
	VZEROUPPER
	RET

// func geluGeriCekirdek(gx, gr, x *float32, n int)
//
// gx += gr·GELU'(x) (Go eşi geluGeriGo).
TEXT ·geluBackwardKernel(SB), NOSPLIT, $0-32
	MOVQ gx+0(FP), DI
	MOVQ gr+8(FP), R8
	MOVQ x+16(FP), SI
	MOVQ n+24(FP), BX
	LEAQ ·expConst(SB), R9
	LEAQ ·geluConst(SB), R10
	MOVQ BX, DX
	SHRQ $3, BX
	ANDQ $7, DX
	MASKE
	XORQ  AX, AX
	TESTQ BX, BX
	JZ    gg_k

gg_d:
	VMOVUPS (SI)(AX*1), Y10
	GELUARG
	UST
	GELUTUREV
	VMULPS  (R8)(AX*1), Y7, Y7
	VADDPS  (DI)(AX*1), Y7, Y7
	VMOVUPS Y7, (DI)(AX*1)
	ADDQ    $32, AX
	DECQ    BX
	JNZ     gg_d

gg_k:
	TESTQ      DX, DX
	JZ         gg_s
	VMASKMOVPS (SI)(AX*1), Y14, Y10
	GELUARG
	UST
	GELUTUREV
	VMASKMOVPS (R8)(AX*1), Y14, Y8
	VMULPS     Y8, Y7, Y7
	VMASKMOVPS (DI)(AX*1), Y14, Y8
	VADDPS     Y8, Y7, Y7
	VMASKMOVPS Y7, Y14, (DI)(AX*1)

gg_s:
	VZEROUPPER
	RET

// func ceSatirCekirdek(satir *float32, n int) (m float32, s float64)
//
// Çapraz entropinin satır işi: m = satırın en büyüğü (NaN'lar yok sayılır),
// s = Σ e^{x−m} (float64, 8 kulvar; Go eşi ceSatirGo). n > 0.
TEXT ·ceRowKernel(SB), NOSPLIT, $0-32
	MOVQ row+0(FP), SI
	MOVQ n+8(FP), BX
	LEAQ ·expConst(SB), R9
	MOVQ BX, DX
	SHRQ $3, BX
	ANDQ $7, DX
	MASKE
	MOVL         $0xFF800000, CX
	VMOVD        CX, X13
	VBROADCASTSS X13, Y13
	VMOVAPS      Y13, Y11
	XORQ         AX, AX
	MOVQ         BX, CX
	TESTQ        CX, CX
	JZ           ce_mk

ce_m:
	VMOVUPS (SI)(AX*1), Y0
	VMAXPS  Y11, Y0, Y11
	ADDQ    $32, AX
	DECQ    CX
	JNZ     ce_m

ce_mk:
	TESTQ      DX, DX
	JZ         ce_ms
	VMASKMOVPS (SI)(AX*1), Y14, Y0
	VBLENDVPS  Y14, Y0, Y13, Y0
	VMAXPS     Y11, Y0, Y11

ce_ms:
	VEXTRACTF128 $1, Y11, X0
	VMAXPS       X11, X0, X11
	VPERMILPS    $0x4E, X11, X0
	VMAXPS       X11, X0, X11
	VPERMILPS    $0xB1, X11, X0
	VMAXPS       X11, X0, X11
	VXORPS       X0, X0, X0
	VADDSS       X0, X11, X11 // −0 + 0 = +0: kulvar sırası sıfırın işaretini değiştirmesin
	VMOVSS       X11, m+16(FP)
	VBROADCASTSS X11, Y11

	VXORPD Y9, Y9, Y9
	VXORPD Y10, Y10, Y10
	XORQ   AX, AX
	MOVQ   BX, CX
	TESTQ  CX, CX
	JZ     ce_tk

ce_t:
	VMOVUPS (SI)(AX*1), Y0
	VSUBPS  Y11, Y0, Y0
	UST
	IKIYE64(Y3, X3, Y1, Y2, X2)
	VADDPD  Y1, Y9, Y9
	VADDPD  Y2, Y10, Y10
	ADDQ    $32, AX
	DECQ    CX
	JNZ     ce_t

ce_tk:
	TESTQ      DX, DX
	JZ         ce_ts
	VMASKMOVPS (SI)(AX*1), Y14, Y0
	VSUBPS     Y11, Y0, Y0
	UST
	VANDPS     Y14, Y3, Y3
	IKIYE64(Y3, X3, Y1, Y2, X2)
	VADDPD     Y1, Y9, Y9
	VADDPD     Y2, Y10, Y10

ce_ts:
	TOPLA8(Y9, Y10, X9, X1)
	VMOVSD X9, s+24(FP)
	VZEROUPPER
	RET

// func ceBackwardKernel(target, row *float32, n int, l, k float32)
//
// Çapraz entropinin satır gradyanı: hedef += k·e^{x−l} (Go eşi ceBackwardGo).
TEXT ·ceBackwardKernel(SB), NOSPLIT, $0-32
	MOVQ         target+0(FP), DI
	MOVQ         row+8(FP), SI
	MOVQ         n+16(FP), BX
	VBROADCASTSS l+24(FP), Y11
	VBROADCASTSS k+28(FP), Y10
	LEAQ         ·expConst(SB), R9
	MOVQ         BX, DX
	SHRQ         $3, BX
	ANDQ         $7, DX
	MASKE
	XORQ         AX, AX
	TESTQ        BX, BX
	JZ           ceg_k

ceg_d:
	VMOVUPS (SI)(AX*1), Y0
	VSUBPS  Y11, Y0, Y0
	UST
	VMULPS  Y10, Y3, Y3
	VADDPS  (DI)(AX*1), Y3, Y3
	VMOVUPS Y3, (DI)(AX*1)
	ADDQ    $32, AX
	DECQ    BX
	JNZ     ceg_d

ceg_k:
	TESTQ      DX, DX
	JZ         ceg_s
	VMASKMOVPS (SI)(AX*1), Y14, Y0
	VSUBPS     Y11, Y0, Y0
	UST
	VMULPS     Y10, Y3, Y3
	VMASKMOVPS (DI)(AX*1), Y14, Y9
	VADDPS     Y9, Y3, Y3
	VMASKMOVPS Y3, Y14, (DI)(AX*1)

ceg_s:
	VZEROUPPER
	RET

// func lnIleriCekirdek(y, x, gam, bet, mean, rstd *float32, satir, boy int, dD, eps float64)
//
// R satırlık (satır uzunluğu D) katman ya da RMS normu. Satır başına:
//   mu = Σx / D (mean nil ise RMS: mu = 0, geçiş atlanır),
//   v  = Σ(x − mu)² / D, rs = 1 / √(v + eps)   (toplamlar float64, 8 kulvar),
//   y  = ((x − m32)·r32)·g + b                 (float32; g, b nil olabilir).
// mean (varsa) ve rstd satır başına float32 yazılır. Kaynaştırılmış çarpma-
// toplama kullanılmaz: sonuç Go eşiyle (lnIleriBlokGo) bit düzeyinde aynıdır.
TEXT ·lnForwardKernel(SB), NOSPLIT, $0-80
	MOVQ y+0(FP), DI
	MOVQ x+8(FP), SI
	MOVQ gammaPtr+16(FP), R8
	MOVQ betaPtr+24(FP), R9
	MOVQ mean+32(FP), R10
	MOVQ rstd+40(FP), R11
	MOVQ row+48(FP), R12
	MOVQ size+56(FP), R13
	TESTQ R12, R12
	JZ    lni_bitti
	MOVQ  R13, BX
	SHRQ  $3, BX
	MOVQ  R13, DX
	ANDQ  $7, DX
	MASKE

lni_satir:
	// Geçiş 1: ortalama.
	VXORPD Y0, Y0, Y0
	TESTQ  R10, R10
	JZ     lni_ortk
	VXORPD Y1, Y1, Y1
	XORQ   AX, AX
	MOVQ   BX, CX
	TESTQ  CX, CX
	JZ     lni_t1k

lni_t1:
	VCVTPS2PD (SI)(AX*1), Y2
	VCVTPS2PD 16(SI)(AX*1), Y3
	VADDPD    Y2, Y0, Y0
	VADDPD    Y3, Y1, Y1
	ADDQ      $32, AX
	DECQ      CX
	JNZ       lni_t1

lni_t1k:
	TESTQ      DX, DX
	JZ         lni_t1s
	VMASKMOVPS (SI)(AX*1), Y14, Y4
	IKIYE64(Y4, X4, Y2, Y3, X5)
	VADDPD     Y2, Y0, Y0
	VADDPD     Y3, Y1, Y1

lni_t1s:
	TOPLA8(Y0, Y1, X0, X1)
	VDIVSD dD+64(FP), X0, X0

lni_ortk:
	VBROADCASTSD X0, Y6
	VCVTSD2SS    X0, X0, X7
	VBROADCASTSS X7, Y7

	// Geçiş 2: kare farkların toplamı.
	VXORPD Y0, Y0, Y0
	VXORPD Y1, Y1, Y1
	XORQ   AX, AX
	MOVQ   BX, CX
	TESTQ  CX, CX
	JZ     lni_t2k

lni_t2:
	VCVTPS2PD (SI)(AX*1), Y2
	VCVTPS2PD 16(SI)(AX*1), Y3
	VSUBPD    Y6, Y2, Y2
	VSUBPD    Y6, Y3, Y3
	VMULPD    Y2, Y2, Y2
	VMULPD    Y3, Y3, Y3
	VADDPD    Y2, Y0, Y0
	VADDPD    Y3, Y1, Y1
	ADDQ      $32, AX
	DECQ      CX
	JNZ       lni_t2

lni_t2k:
	TESTQ      DX, DX
	JZ         lni_t2s
	VMASKMOVPS (SI)(AX*1), Y14, Y4
	IKIYE64(Y4, X4, Y2, Y3, X5)
	VSUBPD     Y6, Y2, Y2
	VSUBPD     Y6, Y3, Y3
	VANDPD     Y13, Y2, Y2
	VANDPD     Y12, Y3, Y3
	VMULPD     Y2, Y2, Y2
	VMULPD     Y3, Y3, Y3
	VADDPD     Y2, Y0, Y0
	VADDPD     Y3, Y1, Y1

lni_t2s:
	TOPLA8(Y0, Y1, X0, X1)
	VDIVSD    dD+64(FP), X0, X0
	VADDSD    eps+72(FP), X0, X0
	VSQRTSD   X0, X0, X0
	MOVQ      $0x3FF0000000000000, CX
	VMOVQ     CX, X1
	VDIVSD    X0, X1, X0
	VCVTSD2SS X0, X0, X8
	VMOVSS    X8, (R11)
	ADDQ      $4, R11
	TESTQ     R10, R10
	JZ        lni_g3
	VMOVSS    X7, (R10)
	ADDQ      $4, R10

lni_g3:
	VBROADCASTSS X8, Y8

	// Geçiş 3: normalleştirme.
	XORQ  AX, AX
	MOVQ  BX, CX
	TESTQ CX, CX
	JZ    lni_t3k

lni_t3:
	VMOVUPS (SI)(AX*1), Y2
	VSUBPS  Y7, Y2, Y2
	VMULPS  Y8, Y2, Y2
	TESTQ   R8, R8
	JZ      lni_t3g
	VMULPS  (R8)(AX*1), Y2, Y2

lni_t3g:
	TESTQ   R9, R9
	JZ      lni_t3b
	VADDPS  (R9)(AX*1), Y2, Y2

lni_t3b:
	VMOVUPS Y2, (DI)(AX*1)
	ADDQ    $32, AX
	DECQ    CX
	JNZ     lni_t3

lni_t3k:
	TESTQ      DX, DX
	JZ         lni_t3s
	VMASKMOVPS (SI)(AX*1), Y14, Y2
	VSUBPS     Y7, Y2, Y2
	VMULPS     Y8, Y2, Y2
	TESTQ      R8, R8
	JZ         lni_t3kg
	VMASKMOVPS (R8)(AX*1), Y14, Y3
	VMULPS     Y3, Y2, Y2

lni_t3kg:
	TESTQ      R9, R9
	JZ         lni_t3kb
	VMASKMOVPS (R9)(AX*1), Y14, Y3
	VADDPS     Y3, Y2, Y2

lni_t3kb:
	VMASKMOVPS Y2, Y14, (DI)(AX*1)

lni_t3s:
	LEAQ (SI)(R13*4), SI
	LEAQ (DI)(R13*4), DI
	DECQ R12
	JNZ  lni_satir

lni_bitti:
	VZEROUPPER
	RET

// func lnGeriCekirdek(xg *float32, pg, pb *float64, x, gr, gam, mean, rstd *float32, satir, boy int, dD float64)
//
// R satırlık katman ya da RMS normu geri yönü (mean nil ise RMS: m = 0, m1 = 0).
// Satır başına h = (x − m)·rs, dh = gr·gam (gam nil ise gr):
//   pg[j] += gr·h, pb[j] += gr            (float64; pg, pb nil olabilir),
//   s1 = Σdh, s2 = Σdh·h                  (float64, 8 kulvar),
//   xg   += rs·((dh − m1) − h·m2), m1 = s1/D, m2 = s2/D (xg nil olabilir).
// Kaynaştırılmış çarpma-toplama yoktur: sonuç lnGeriBlokGo ile bit düzeyinde aynıdır.
TEXT ·lnBackwardKernel(SB), NOSPLIT, $0-88
	MOVQ xg+0(FP), DI
	MOVQ pg+8(FP), R8
	MOVQ pb+16(FP), R9
	MOVQ x+24(FP), SI
	MOVQ gr+32(FP), R10
	MOVQ gammaPtr+40(FP), R11
	MOVQ mean+48(FP), R12
	MOVQ rstd+56(FP), R13
	MOVQ row+64(FP), AX
	TESTQ AX, AX
	JZ    lng_bitti
	MOVQ  size+72(FP), BX
	MOVQ  BX, DX
	SHRQ  $3, BX
	ANDQ  $7, DX
	MASKE

lng_satir:
	VXORPS       Y7, Y7, Y7
	TESTQ        R12, R12
	JZ           lng_m0
	VBROADCASTSS (R12), Y7
	ADDQ         $4, R12

lng_m0:
	VBROADCASTSS (R13), Y8
	ADDQ         $4, R13
	VXORPD       Y0, Y0, Y0
	VXORPD       Y1, Y1, Y1
	VXORPD       Y2, Y2, Y2
	VXORPD       Y3, Y3, Y3

	// Geçiş A: ölçek/sapma gradyanları ve s1, s2.
	XORQ  AX, AX
	MOVQ  BX, CX
	TESTQ CX, CX
	JZ    lng_ak

lng_a:
	VMOVUPS (SI)(AX*1), Y4
	VSUBPS  Y7, Y4, Y4
	VMULPS  Y8, Y4, Y4
	VMOVUPS (R10)(AX*1), Y5
	TESTQ   R8, R8
	JZ      lng_a1
	IKIYE64(Y5, X5, Y9, Y10, X10)
	IKIYE64(Y4, X4, Y11, Y15, X15)
	VMULPD  Y11, Y9, Y11
	VMULPD  Y15, Y10, Y15
	VADDPD  (R8)(AX*2), Y11, Y11
	VADDPD  32(R8)(AX*2), Y15, Y15
	VMOVUPD Y11, (R8)(AX*2)
	VMOVUPD Y15, 32(R8)(AX*2)

lng_a1:
	TESTQ   R9, R9
	JZ      lng_a2
	IKIYE64(Y5, X5, Y9, Y10, X10)
	VADDPD  (R9)(AX*2), Y9, Y9
	VADDPD  32(R9)(AX*2), Y10, Y10
	VMOVUPD Y9, (R9)(AX*2)
	VMOVUPD Y10, 32(R9)(AX*2)

lng_a2:
	TESTQ  DI, DI
	JZ     lng_a3
	TESTQ  R11, R11
	JZ     lng_a4
	VMULPS (R11)(AX*1), Y5, Y5

lng_a4:
	IKIYE64(Y5, X5, Y9, Y10, X10)
	VADDPD Y9, Y0, Y0
	VADDPD Y10, Y1, Y1
	IKIYE64(Y4, X4, Y11, Y15, X15)
	VMULPD Y11, Y9, Y9
	VMULPD Y15, Y10, Y10
	VADDPD Y9, Y2, Y2
	VADDPD Y10, Y3, Y3

lng_a3:
	ADDQ $32, AX
	DECQ CX
	JNZ  lng_a

lng_ak:
	TESTQ      DX, DX
	JZ         lng_as
	VMASKMOVPS (SI)(AX*1), Y14, Y4
	VSUBPS     Y7, Y4, Y4
	VMULPS     Y8, Y4, Y4
	VMASKMOVPS (R10)(AX*1), Y14, Y5
	TESTQ      R8, R8
	JZ         lng_ak1
	IKIYE64(Y5, X5, Y9, Y10, X10)
	IKIYE64(Y4, X4, Y11, Y15, X15)
	VMULPD     Y11, Y9, Y11
	VMULPD     Y15, Y10, Y15
	VMASKMOVPD (R8)(AX*2), Y13, Y9
	VMASKMOVPD 32(R8)(AX*2), Y12, Y10
	VADDPD     Y9, Y11, Y11
	VADDPD     Y10, Y15, Y15
	VMASKMOVPD Y11, Y13, (R8)(AX*2)
	VMASKMOVPD Y15, Y12, 32(R8)(AX*2)

lng_ak1:
	TESTQ      R9, R9
	JZ         lng_ak2
	IKIYE64(Y5, X5, Y9, Y10, X10)
	VMASKMOVPD (R9)(AX*2), Y13, Y11
	VMASKMOVPD 32(R9)(AX*2), Y12, Y15
	VADDPD     Y11, Y9, Y9
	VADDPD     Y15, Y10, Y10
	VMASKMOVPD Y9, Y13, (R9)(AX*2)
	VMASKMOVPD Y10, Y12, 32(R9)(AX*2)

lng_ak2:
	TESTQ      DI, DI
	JZ         lng_as
	TESTQ      R11, R11
	JZ         lng_ak4
	VMASKMOVPS (R11)(AX*1), Y14, Y9
	VMULPS     Y9, Y5, Y5

lng_ak4:
	IKIYE64(Y5, X5, Y9, Y10, X10)
	VADDPD Y9, Y0, Y0
	VADDPD Y10, Y1, Y1
	IKIYE64(Y4, X4, Y11, Y15, X15)
	VMULPD Y11, Y9, Y9
	VMULPD Y15, Y10, Y10
	VADDPD Y9, Y2, Y2
	VADDPD Y10, Y3, Y3

lng_as:
	TESTQ DI, DI
	JZ    lng_sonraki
	TOPLA8(Y0, Y1, X0, X9)
	TOPLA8(Y2, Y3, X2, X9)
	VDIVSD    dD+80(FP), X0, X0
	VDIVSD    dD+80(FP), X2, X2
	VCVTSD2SS X0, X0, X0
	VCVTSD2SS X2, X2, X2
	CMPQ      mean+48(FP), $0
	JNE       lng_m1
	VXORPS    X0, X0, X0

lng_m1:
	VBROADCASTSS X0, Y9
	VBROADCASTSS X2, Y10

	// Geçiş B: girdi gradyanı.
	XORQ  AX, AX
	MOVQ  BX, CX
	TESTQ CX, CX
	JZ    lng_bk

lng_b:
	VMOVUPS (SI)(AX*1), Y4
	VSUBPS  Y7, Y4, Y4
	VMULPS  Y8, Y4, Y4
	VMOVUPS (R10)(AX*1), Y5
	TESTQ   R11, R11
	JZ      lng_b1
	VMULPS  (R11)(AX*1), Y5, Y5

lng_b1:
	VSUBPS  Y9, Y5, Y5
	VMULPS  Y10, Y4, Y4
	VSUBPS  Y4, Y5, Y5
	VMULPS  Y8, Y5, Y5
	VADDPS  (DI)(AX*1), Y5, Y5
	VMOVUPS Y5, (DI)(AX*1)
	ADDQ    $32, AX
	DECQ    CX
	JNZ     lng_b

lng_bk:
	TESTQ      DX, DX
	JZ         lng_sonraki
	VMASKMOVPS (SI)(AX*1), Y14, Y4
	VSUBPS     Y7, Y4, Y4
	VMULPS     Y8, Y4, Y4
	VMASKMOVPS (R10)(AX*1), Y14, Y5
	TESTQ      R11, R11
	JZ         lng_bk1
	VMASKMOVPS (R11)(AX*1), Y14, Y11
	VMULPS     Y11, Y5, Y5

lng_bk1:
	VSUBPS     Y9, Y5, Y5
	VMULPS     Y10, Y4, Y4
	VSUBPS     Y4, Y5, Y5
	VMULPS     Y8, Y5, Y5
	VMASKMOVPS (DI)(AX*1), Y14, Y11
	VADDPS     Y11, Y5, Y5
	VMASKMOVPS Y5, Y14, (DI)(AX*1)

lng_sonraki:
	MOVQ size+72(FP), CX
	SHLQ $2, CX
	ADDQ CX, SI
	ADDQ CX, R10
	TESTQ DI, DI
	JZ    lng_d
	ADDQ CX, DI

lng_d:
	DECQ row+64(FP)
	JNZ  lng_satir

lng_bitti:
	VZEROUPPER
	RET
