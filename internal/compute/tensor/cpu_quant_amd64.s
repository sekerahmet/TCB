#include "textflag.h"

// Nicemlenmiş ağırlığın nokta çarpımı: sdot32'nin (cpu_gemv_amd64.s) eşleri. Bloklar yazmaçta açılır;
// açılan değer float32'de kesin ya da tek yuvarlamalıdır, dequantize_row_* ile aynıdır. i. öğe
// (i/8)%4. biriktiricinin i%8. kulvarına, artan sırayla ve kaynaştırılmış çarpma-toplamayla girer:
// sonuç "aç + sdot32" ile bit düzeyinde aynıdır.

// X0 = (Y0+Y1)+(Y2+Y3), sonra kulvarlar sdot32'nin sırasıyla.
#define SUM32 \
	VADDPS       Y1, Y0, Y0; \
	VADDPS       Y3, Y2, Y2; \
	VADDPS       Y2, Y0, Y0; \
	VEXTRACTF128 $1, Y0, X1; \
	VADDPS       X1, X0, X0; \
	VMOVHLPS     X0, X1, X1; \
	VADDPS       X1, X0, X0; \
	VMOVSHDUP    X0, X1; \
	VADDSS       X1, X0, X0

#define ZERO4 \
	VXORPS Y0, Y0, Y0; \
	VXORPS Y1, Y1, Y1; \
	VXORPS Y2, Y2, Y2; \
	VXORPS Y3, Y3, Y3

// Sekiz "q8" kodu: acc += x · (d · kod). Y8: d.
#define Q8LANE(codes, xoff, acc) \
	VPMOVSXBD   codes(DI), Y9; \
	VCVTDQ2PS   Y9, Y9; \
	VMULPS      Y8, Y9, Y9; \
	VFMADD231PS xoff(SI), Y9, acc

// func sdotq8(n int, x *float32, w *byte) float32
//
// Blok 34 bayt: fp16 ölçek, 32 işaretli kod.
TEXT ·sdotq8(SB), NOSPLIT, $0-28
	MOVQ n+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ w+16(FP), DI
	ZERO4
	SHRQ $5, CX
	JZ   q8son

q8dongu:
	MOVWLZX      (DI), AX
	VMOVD        AX, X8
	VCVTPH2PS    X8, X8
	VBROADCASTSS X8, Y8
	Q8LANE(2, 0, Y0)
	Q8LANE(10, 32, Y1)
	Q8LANE(18, 64, Y2)
	Q8LANE(26, 96, Y3)
	ADDQ         $34, DI
	ADDQ         $128, SI
	DECQ         CX
	JNZ          q8dongu

q8son:
	SUM32
	VMOVSS X0, ret+24(FP)
	VZEROUPPER
	RET

// Sekiz "q4_k" baytı: alt yarılar acc'ye öğe xlo'dan, üst yarılar öğe xhi'den. Y10, Y11: alt yarının
// d·a ve m·b'si; Y12, Y13: üst yarının. Y4: 15. Çarpım kesindir, çıkarma tek yuvarlamadır.
#define Q4LANE(codes, xlo, xhi, acc) \
	VPMOVZXBD   codes(DI), Y9; \
	VPAND       Y4, Y9, Y14; \
	VPSRLD      $4, Y9, Y15; \
	VCVTDQ2PS   Y14, Y14; \
	VCVTDQ2PS   Y15, Y15; \
	VFMSUB213PS Y11, Y10, Y14; \
	VFMSUB213PS Y13, Y12, Y15; \
	VFMADD231PS xlo(SI), Y14, acc; \
	VFMADD231PS xhi(SI), Y15, acc

// func sdotq4k(blocks int, x *float32, w *byte, scales *float32) float32
//
// Blok 144 bayt: 16 bayt ölçek, dört grup 32 bayt. Grup 64 öğedir: 32 alt yarı, 32 üst yarı.
TEXT ·sdotq4k(SB), NOSPLIT, $0-36
	MOVQ blocks+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ w+16(FP), DI
	MOVQ scales+24(FP), DX
	ZERO4
	MOVL         $15, AX
	VMOVD        AX, X4
	VPBROADCASTD X4, Y4
	TESTQ        CX, CX
	JZ           q4son

q4blok:
	MOVWLZX      (DI), AX
	VMOVD        AX, X5
	MOVWLZX      2(DI), AX
	VMOVD        AX, X6
	VCVTPH2PS    X5, X5
	VCVTPH2PS    X6, X6
	VBROADCASTSS X5, Y5
	VBROADCASTSS X6, Y6
	MOVL         4(DI), R8
	MOVL         8(DI), R9
	MOVL         12(DI), R10
	MOVL         R8, AX
	ANDL         $0x3f3f3f3f, AX
	MOVL         R10, BX
	ANDL         $0x0f0f0f0f, BX
	SHRL         $6, R8
	ANDL         $0x03030303, R8
	SHLL         $4, R8
	ORL          R8, BX
	SHLQ         $32, BX
	ORQ          BX, AX
	VMOVQ        AX, X7
	MOVL         R9, AX
	ANDL         $0x3f3f3f3f, AX
	SHRL         $4, R10
	ANDL         $0x0f0f0f0f, R10
	SHRL         $6, R9
	ANDL         $0x03030303, R9
	SHLL         $4, R9
	ORL          R9, R10
	SHLQ         $32, R10
	ORQ          R10, AX
	VMOVQ        AX, X8
	VPMOVZXBD    X7, Y7
	VPMOVZXBD    X8, Y8
	VCVTDQ2PS    Y7, Y7
	VCVTDQ2PS    Y8, Y8
	VMULPS       Y5, Y7, Y7
	VMULPS       Y6, Y8, Y8
	VUNPCKLPS    Y8, Y7, Y9
	VUNPCKHPS    Y8, Y7, Y10
	VMOVUPS      X9, (DX)
	VMOVUPS      X10, 16(DX)
	VEXTRACTF128 $1, Y9, 32(DX)
	VEXTRACTF128 $1, Y10, 48(DX)
	ADDQ         $16, DI
	MOVQ         $4, BX

q4grup:
	VBROADCASTSS (DX), Y10
	VBROADCASTSS 4(DX), Y11
	VBROADCASTSS 8(DX), Y12
	VBROADCASTSS 12(DX), Y13
	Q4LANE(0, 0, 128, Y0)
	Q4LANE(8, 32, 160, Y1)
	Q4LANE(16, 64, 192, Y2)
	Q4LANE(24, 96, 224, Y3)
	ADDQ         $32, DI
	ADDQ         $256, SI
	ADDQ         $16, DX
	DECQ         BX
	JNZ          q4grup
	DECQ         CX
	JNZ          q4blok

q4son:
	SUM32
	VMOVSS X0, ret+32(FP)
	VZEROUPPER
	RET

// Bir kod: öğe xoff'tan acc'ye. r kodun 6 bitidir; Y6: 32, scale: d·a'nın baytı (DX).
#define Q6STEP(r, scale, xoff, acc) \
	VPSUBD       Y6, r, r; \
	VCVTDQ2PS    r, r; \
	VBROADCASTSS scale(DX), Y15; \
	VMULPS       Y15, r, r; \
	VFMADD231PS  xoff(SI), r, acc

// Sekiz öğelik dört kod: ql alt ve üst yarıları, qh iki biti. off: 8·c, s0: alt blok (c/2) ölçeği.
#define Q6LANE(off, s0, x0, x1, x2, x3, acc) \
	VPMOVZXBD off(DI), Y8; \
	VPMOVZXBD off+32(DI), Y9; \
	VPMOVZXBD off(R8), Y10; \
	VPAND     Y4, Y8, Y11; \
	VPAND     Y5, Y10, Y12; \
	VPSLLD    $4, Y12, Y12; \
	VPOR      Y12, Y11, Y11; \
	Q6STEP(Y11, s0, x0, acc); \
	VPAND     Y4, Y9, Y11; \
	VPSRLD    $2, Y10, Y12; \
	VPAND     Y5, Y12, Y12; \
	VPSLLD    $4, Y12, Y12; \
	VPOR      Y12, Y11, Y11; \
	Q6STEP(Y11, s0+8, x1, acc); \
	VPSRLD    $4, Y8, Y11; \
	VPSRLD    $4, Y10, Y12; \
	VPAND     Y5, Y12, Y12; \
	VPSLLD    $4, Y12, Y12; \
	VPOR      Y12, Y11, Y11; \
	Q6STEP(Y11, s0+16, x2, acc); \
	VPSRLD    $4, Y9, Y11; \
	VPSRLD    $6, Y10, Y12; \
	VPSLLD    $4, Y12, Y12; \
	VPOR      Y12, Y11, Y11; \
	Q6STEP(Y11, s0+24, x3, acc)

// func sdotq6k(blocks int, x *float32, w *byte, scales *float32) float32
//
// Blok 210 bayt: ql 128, qh 64, ölçek 16, d 2. Yarım blok 128 öğedir: ql 64, qh 32, ölçek 8.
TEXT ·sdotq6k(SB), NOSPLIT, $0-36
	MOVQ blocks+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ w+16(FP), DI
	MOVQ scales+24(FP), DX
	ZERO4
	MOVL         $15, AX
	VMOVD        AX, X4
	VPBROADCASTD X4, Y4
	MOVL         $3, AX
	VMOVD        AX, X5
	VPBROADCASTD X5, Y5
	MOVL         $32, AX
	VMOVD        AX, X6
	VPBROADCASTD X6, Y6
	TESTQ        CX, CX
	JZ           q6son

q6blok:
	MOVWLZX      208(DI), AX
	VMOVD        AX, X7
	VCVTPH2PS    X7, X7
	VBROADCASTSS X7, Y7
	VPMOVSXBD    192(DI), Y8
	VPMOVSXBD    200(DI), Y9
	VCVTDQ2PS    Y8, Y8
	VCVTDQ2PS    Y9, Y9
	VMULPS       Y7, Y8, Y8
	VMULPS       Y7, Y9, Y9
	VMOVUPS      Y8, (DX)
	VMOVUPS      Y9, 32(DX)
	LEAQ         128(DI), R8
	MOVQ         $2, BX

q6yari:
	Q6LANE(0, 0, 0, 128, 256, 384, Y0)
	Q6LANE(8, 0, 32, 160, 288, 416, Y1)
	Q6LANE(16, 4, 64, 192, 320, 448, Y2)
	Q6LANE(24, 4, 96, 224, 352, 480, Y3)
	ADDQ $64, DI
	ADDQ $32, R8
	ADDQ $512, SI
	ADDQ $32, DX
	DECQ BX
	JNZ  q6yari
	ADDQ $82, DI
	DECQ CX
	JNZ  q6blok

q6son:
	SUM32
	VMOVSS X0, ret+32(FP)
	VZEROUPPER
	RET
