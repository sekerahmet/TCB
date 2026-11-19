#include "textflag.h"

// Tam GELU çekirdekleri (yöntem ve Go eşi: cpu_gelu_exact.go). R10 = ·gelu64Const; satır k'nin
// uzaklığı 32·k: 0 |·| maskesi, 32 1/√2, 64 ½, 96 −½, 128..320 erf serisi c0..c6, 352 bir,
// 384 üç, 416 eksi bir, 448..832 Chebyshev c0..c12, 864 −200, 896 log2e, 928 ln2 üst, 960 ln2 alt,
// 992..1312 üstel c0..c10, 1344 üs sapması (tamsayı 1023), 1376 yirmi, 1408 √π, 1440 1/√(2π).
// İşlemler Go eşindekiyle aynı sırada ve aynı yuvarlamayla yapılır (sonuç bit bit aynı).

// CLEN: Chebyshev'in bir Clenshaw adımı, B = (2s·A + c) − B (Y3 = 2s, Y10 bozulur).
#define CLEN(off, A, B) \
	VMOVUPD     off(R10), Y10; \
	VFMADD231PD A, Y3, Y10;    \
	VSUBPD      B, Y10, B

// PHI64: Y0 = x (4 float64) için Y1 = Φ(x), Y8 = e^{−x²/2} (|x| > 20'de 0). Y2-Y10 bozulur;
// Y6 = x ≥ 0 maskesi, Y2 = t kalır.
#define PHI64 \
	VANDPD       0(R10), Y0, Y1;      \
	VMULPD       32(R10), Y1, Y2;     \
	VMULPD       Y2, Y2, Y3;          \
	VMOVUPD      320(R10), Y4;        \
	VFMADD213PD  288(R10), Y3, Y4;    \
	VFMADD213PD  256(R10), Y3, Y4;    \
	VFMADD213PD  224(R10), Y3, Y4;    \
	VFMADD213PD  192(R10), Y3, Y4;    \
	VFMADD213PD  160(R10), Y3, Y4;    \
	VFMADD213PD  128(R10), Y3, Y4;    \
	VMULPD       Y4, Y2, Y4;          \
	VXORPD       Y9, Y9, Y9;          \
	VCMPPD       $13, Y9, Y0, Y6;     \
	VMOVUPD      96(R10), Y5;         \
	VMOVUPD      64(R10), Y7;         \
	VBLENDVPD    Y6, Y7, Y5, Y5;      \
	VFMADD213PD  64(R10), Y4, Y5;     \
	VMOVUPD      352(R10), Y7;        \
	VADDPD       Y7, Y2, Y3;          \
	VDIVPD       Y3, Y7, Y3;          \
	VMOVUPD      384(R10), Y7;        \
	VFMADD213PD  416(R10), Y3, Y7;    \
	VADDPD       Y7, Y7, Y3;          \
	VMOVUPD      832(R10), Y8;        \
	VXORPD       Y9, Y9, Y9;          \
	CLEN(800, Y8, Y9);                \
	CLEN(768, Y9, Y8);                \
	CLEN(736, Y8, Y9);                \
	CLEN(704, Y9, Y8);                \
	CLEN(672, Y8, Y9);                \
	CLEN(640, Y9, Y8);                \
	CLEN(608, Y8, Y9);                \
	CLEN(576, Y9, Y8);                \
	CLEN(544, Y8, Y9);                \
	CLEN(512, Y9, Y8);                \
	CLEN(480, Y8, Y9);                \
	VMOVUPD      448(R10), Y10;       \
	VFMADD231PD  Y9, Y7, Y10;         \
	VSUBPD       Y8, Y10, Y10;        \
	VMULPD       Y0, Y0, Y3;          \
	VMULPD       96(R10), Y3, Y3;     \
	VMOVUPD      864(R10), Y7;        \
	VMAXPD       Y3, Y7, Y3;          \
	VMULPD       896(R10), Y3, Y7;    \
	VROUNDPD     $0, Y7, Y7;          \
	VFNMADD231PD 928(R10), Y7, Y3;    \
	VFNMADD231PD 960(R10), Y7, Y3;    \
	VMOVUPD      1312(R10), Y9;       \
	VFMADD213PD  1280(R10), Y3, Y9;   \
	VFMADD213PD  1248(R10), Y3, Y9;   \
	VFMADD213PD  1216(R10), Y3, Y9;   \
	VFMADD213PD  1184(R10), Y3, Y9;   \
	VFMADD213PD  1152(R10), Y3, Y9;   \
	VFMADD213PD  1120(R10), Y3, Y9;   \
	VFMADD213PD  1088(R10), Y3, Y9;   \
	VFMADD213PD  1056(R10), Y3, Y9;   \
	VFMADD213PD  1024(R10), Y3, Y9;   \
	VFMADD213PD  992(R10), Y3, Y9;    \
	VCVTPD2DQY   Y7, X7;              \
	VPMOVSXDQ    X7, Y7;              \
	VPADDQ       1344(R10), Y7, Y7;   \
	VPSLLQ       $52, Y7, Y7;         \
	VMULPD       Y7, Y9, Y8;          \
	VCMPPD       $14, 1376(R10), Y1, Y9; \
	VANDNPD      Y8, Y9, Y8;          \
	VMULPD       1408(R10), Y2, Y9;   \
	VMULPD       Y10, Y8, Y3;         \
	VDIVPD       Y9, Y3, Y3;          \
	VMULPD       64(R10), Y3, Y3;     \
	VMOVUPD      352(R10), Y7;        \
	VSUBPD       Y3, Y7, Y7;          \
	VBLENDVPD    Y6, Y7, Y3, Y3;      \
	VCMPPD       $1, 64(R10), Y2, Y9; \
	VBLENDVPD    Y9, Y5, Y3, Y1

// YF4: Y0 = x, Y1 = Φ için dst = float32(x·Φ); x·Φ NaN iken (−∞·0) +0. Negatif sonlu x'te
// Φ = 0 ise x·Φ = −0 kalır (geluExactForwardGo).
#define YF4(dst) \
	VMULPD     Y1, Y0, Y3;     \
	VCMPPD     $3, Y3, Y3, Y4; \
	VANDNPD    Y3, Y4, Y3;     \
	VCVTPD2PSY Y3, dst

// DF4: Y0 = x, Y1 = Φ, Y8 = e^{−x²/2} için dst = float32(d), d = Φ + x·φ (φ = 0 iken Φ); d = 0
// iken x·0 (NaN ise +0) alınır: negatif sonlu x'te −0 (geluExactSlopeGo, geluSignedZero).
#define DF4(dst) \
	VMULPD      1440(R10), Y8, Y4;  \
	VMOVAPD     Y1, Y3;             \
	VFMADD231PD Y4, Y0, Y3;         \
	VXORPD      Y5, Y5, Y5;         \
	VCMPPD      $0, Y5, Y4, Y7;     \
	VBLENDVPD   Y7, Y1, Y3, Y3;     \
	VMULPD      Y5, Y0, Y9;         \
	VCMPPD      $3, Y9, Y9, Y10;    \
	VANDNPD     Y9, Y10, Y9;        \
	VCMPPD      $0, Y5, Y3, Y10;    \
	VBLENDVPD   Y10, Y9, Y3, Y3;    \
	VCVTPD2PSY  Y3, dst

// SEKIZ_Y: Y11'deki 8 float32 için Y12 = ileri değer; NaN girdi olduğu gibi geçer.
// Y0-Y10 ve Y13 bozulur.
#define SEKIZ_Y \
	VCVTPS2PD    X11, Y0;           \
	PHI64;                          \
	YF4(X12);                       \
	VEXTRACTF128 $1, Y11, X0;       \
	VCVTPS2PD    X0, Y0;            \
	PHI64;                          \
	YF4(X13);                       \
	VINSERTF128  $1, X13, Y12, Y12; \
	VCMPPS       $3, Y11, Y11, Y13; \
	VBLENDVPS    Y13, Y11, Y12, Y12

// SEKIZ_D: Y11'deki 8 float32 için Y12 = türev çarpanı d; NaN girdi olduğu gibi geçer.
// Y0-Y10 ve Y13 bozulur.
#define SEKIZ_D \
	VCVTPS2PD    X11, Y0;           \
	PHI64;                          \
	DF4(X12);                       \
	VEXTRACTF128 $1, Y11, X0;       \
	VCVTPS2PD    X0, Y0;            \
	PHI64;                          \
	DF4(X13);                       \
	VINSERTF128  $1, X13, Y12, Y12; \
	VCMPPS       $3, Y11, Y11, Y13; \
	VBLENDVPS    Y13, Y11, Y12, Y12

// KUYRUK: DX kuyruk eleman sayısı (0-7) için Y14 kuyruk maskesi (CX bozulur).
#define KUYRUK \
	LEAQ    ·vecMask(SB), CX;  \
	NEGQ    DX;                \
	VMOVDQU 32(CX)(DX*4), Y14; \
	NEGQ    DX

// func geluExactForwardKernel(y, x *float32, n int)
TEXT ·geluExactForwardKernel(SB), NOSPLIT, $0-24
	MOVQ y+0(FP), DI
	MOVQ x+8(FP), SI
	MOVQ n+16(FP), BX
	LEAQ ·gelu64Const(SB), R10
	MOVQ BX, DX
	SHRQ $3, BX
	ANDQ $7, DX
	KUYRUK
	XORQ  AX, AX
	TESTQ BX, BX
	JZ    gf_k

gf_d:
	VMOVUPS (SI)(AX*1), Y11
	SEKIZ_Y
	VMOVUPS Y12, (DI)(AX*1)
	ADDQ    $32, AX
	DECQ    BX
	JNZ     gf_d

gf_k:
	TESTQ      DX, DX
	JZ         gf_s
	VMASKMOVPS (SI)(AX*1), Y14, Y11
	SEKIZ_Y
	VMASKMOVPS Y12, Y14, (DI)(AX*1)

gf_s:
	VZEROUPPER
	RET

// func geluExactBackwardKernel(gx, gr, x *float32, n int)
//
// gx += float32(gr·d): çarpım ayrı yuvarlanır, sonra eklenir (Go eşiyle aynı). `g` Go
// assembly'sinde yazmaç adı olduğu için bağımsız değişken gr'dir.
TEXT ·geluExactBackwardKernel(SB), NOSPLIT, $0-32
	MOVQ gx+0(FP), DI
	MOVQ gr+8(FP), SI
	MOVQ x+16(FP), R8
	MOVQ n+24(FP), BX
	LEAQ ·gelu64Const(SB), R10
	MOVQ BX, DX
	SHRQ $3, BX
	ANDQ $7, DX
	KUYRUK
	XORQ  AX, AX
	TESTQ BX, BX
	JZ    gb_k

gb_d:
	VMOVUPS (R8)(AX*1), Y11
	SEKIZ_D
	VMOVUPS (SI)(AX*1), Y13
	VMULPS  Y12, Y13, Y13
	VMOVUPS (DI)(AX*1), Y15
	VADDPS  Y13, Y15, Y13
	VMOVUPS Y13, (DI)(AX*1)
	ADDQ    $32, AX
	DECQ    BX
	JNZ     gb_d

gb_k:
	TESTQ      DX, DX
	JZ         gb_s
	VMASKMOVPS (R8)(AX*1), Y14, Y11
	SEKIZ_D
	VMASKMOVPS (SI)(AX*1), Y14, Y13
	VMULPS     Y12, Y13, Y13
	VMASKMOVPS (DI)(AX*1), Y14, Y15
	VADDPS     Y13, Y15, Y13
	VMASKMOVPS Y13, Y14, (DI)(AX*1)

gb_s:
	VZEROUPPER
	RET
