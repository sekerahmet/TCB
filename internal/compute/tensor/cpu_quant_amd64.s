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

// Sekiz kod (yığında, off) öğe xoff'tan acc'ye; scale: d·a'nın baytı (DX).
#define Q6STEP(off, scale, xoff, acc) \
	VPMOVSXBD    off(SP), Y11; \
	VCVTDQ2PS    Y11, Y11; \
	VBROADCASTSS scale(DX), Y15; \
	VMULPS       Y15, Y11, Y11; \
	VFMADD231PS  xoff(SI), Y11, acc

// Sekiz öğelik dört kod. off: 8·c, s0: alt bloğun ölçeği. Bir kulvarın sırası değişmez.
#define Q6LANE(off, s0, x0, x1, x2, x3, acc) \
	Q6STEP(off, s0, x0, acc); \
	Q6STEP(off+32, s0+8, x1, acc); \
	Q6STEP(off+64, s0+16, x2, acc); \
	Q6STEP(off+96, s0+24, x3, acc)

// Yarım bloğun 128 kodu: ql iki yarısı Y8, Y9, qh Y10. Bayt işlemleriyle 32 öğe birden; kod − 32
// işaretli bayta sığar. Y4: 0x0F, Y5: 0x30, Y6: 32.
#define Q6CODES \
	VMOVDQU (DI), Y8; \
	VMOVDQU 32(DI), Y9; \
	VMOVDQU (R8), Y10; \
	VPAND   Y4, Y8, Y11; \
	VPSLLW  $4, Y10, Y12; \
	VPAND   Y5, Y12, Y12; \
	VPOR    Y12, Y11, Y11; \
	VPSUBB  Y6, Y11, Y11; \
	VMOVDQU Y11, 0(SP); \
	VPAND   Y4, Y9, Y11; \
	VPSLLW  $2, Y10, Y12; \
	VPAND   Y5, Y12, Y12; \
	VPOR    Y12, Y11, Y11; \
	VPSUBB  Y6, Y11, Y11; \
	VMOVDQU Y11, 32(SP); \
	VPSRLW  $4, Y8, Y11; \
	VPAND   Y4, Y11, Y11; \
	VPAND   Y5, Y10, Y12; \
	VPOR    Y12, Y11, Y11; \
	VPSUBB  Y6, Y11, Y11; \
	VMOVDQU Y11, 64(SP); \
	VPSRLW  $4, Y9, Y11; \
	VPAND   Y4, Y11, Y11; \
	VPSRLW  $2, Y10, Y12; \
	VPAND   Y5, Y12, Y12; \
	VPOR    Y12, Y11, Y11; \
	VPSUBB  Y6, Y11, Y11; \
	VMOVDQU Y11, 96(SP)

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
// d·a ve m·b'si; Y12, Y13: üst yarının, d·a 16'ya bölünmüş. Üst yarı kaydırılmaz, kodun 16 katıdır:
// çarpım yine kesindir, kaydırma 0 ve 1 kapılarını boşa tutmaz. Y4: 15; Y5: 0xF0. Çıkarma tek yuvarlamadır.
#define Q4LANE(codes, xlo, xhi, acc) \
	VPMOVZXBD   codes(DI), Y9; \
	VPAND       Y4, Y9, Y14; \
	VPAND       Y5, Y9, Y15; \
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
	MOVL         $0xF0, AX
	VMOVD        AX, X5
	VPBROADCASTD X5, Y5
	MOVQ         $0x3D8000003F800000, AX
	VMOVQ        AX, X6
	VPBROADCASTQ X6, Y6
	TESTQ        CX, CX
	JZ           q4son

q4blok:
	MOVWLZX      (DI), AX
	VMOVD        AX, X7
	MOVWLZX      2(DI), AX
	VMOVD        AX, X8
	VCVTPH2PS    X7, X7
	VCVTPH2PS    X8, X8
	VBROADCASTSS X7, Y7
	VBROADCASTSS X8, Y8
	VMULPS       Y6, Y7, Y7
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
	VMOVQ        AX, X9
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
	VMOVQ        AX, X10
	VPMOVZXBD    X9, Y9
	VPMOVZXBD    X10, Y10
	VCVTDQ2PS    Y9, Y9
	VCVTDQ2PS    Y10, Y10
	VMULPS       Y7, Y9, Y9
	VMULPS       Y8, Y10, Y10
	VUNPCKLPS    Y10, Y9, Y14
	VUNPCKHPS    Y10, Y9, Y15
	VMOVUPS      X14, (DX)
	VMOVUPS      X15, 16(DX)
	VEXTRACTF128 $1, Y14, 32(DX)
	VEXTRACTF128 $1, Y15, 48(DX)
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

// func sdotq6k(blocks int, x *float32, w *byte, scales *float32) float32
//
// Blok 210 bayt: ql 128, qh 64, ölçek 16, d 2. Yarım blok 128 öğedir: ql 64, qh 32, ölçek 8.
TEXT ·sdotq6k(SB), NOSPLIT, $128-36
	MOVQ blocks+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ w+16(FP), DI
	MOVQ scales+24(FP), DX
	ZERO4
	MOVL         $0x0F, AX
	VMOVD        AX, X4
	VPBROADCASTB X4, Y4
	MOVL         $0x30, AX
	VMOVD        AX, X5
	VPBROADCASTB X5, Y5
	MOVL         $32, AX
	VMOVD        AX, X6
	VPBROADCASTB X6, Y6
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
	Q6CODES
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
