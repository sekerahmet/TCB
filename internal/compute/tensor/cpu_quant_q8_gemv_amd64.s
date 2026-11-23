#include "textflag.h"

// func q8FiniteAsm(x *float32, n int) bool
//
// İşaret bitini silip 32 bitlik en büyük bit kalıbını bulur; sonlu sayılarda üs alanı 0xFF olamaz.
// Yalnız ilk n &^ 7 öğeye bakar. n < 8'de döngü sayacı 0 olur, DECQ onu sarar ve döngü dizinin
// sonunu aşardı: o durumda hiç öğe okunmadan sonlu (true) döner (sarmalayıcı q8Finite kısa diziyi
// zaten Go eşine verir; DENETİM 958 N7).
TEXT ·q8FiniteAsm(SB), NOSPLIT, $0-17
	MOVQ         x+0(FP), SI
	MOVQ         n+8(FP), CX
	SHRQ         $3, CX
	JZ           q8finBos
	MOVL         $0x7FFFFFFF, AX
	VMOVD        AX, X4
	VPBROADCASTD X4, Y4
	VPXOR        Y0, Y0, Y0

q8finLoop:
	VPAND   (SI), Y4, Y1
	VPMAXSD Y1, Y0, Y0
	ADDQ    $32, SI
	DECQ    CX
	JNZ     q8finLoop

	VEXTRACTI128 $1, Y0, X1
	VPMAXSD      X1, X0, X0
	VPSHUFD      $0x4E, X0, X1
	VPMAXSD      X1, X0, X0
	VPSHUFD      $0xB1, X0, X1
	VPMAXSD      X1, X0, X0
	VMOVD        X0, AX
	ANDL         $0x7F800000, AX
	CMPL         AX, $0x7F800000
	SETNE        ret+16(FP)
	VZEROUPPER
	RET

q8finBos:
	MOVB $1, ret+16(FP)
	RET

// func quantizeQ8Asm(x *float32, q *int8, d *float32, bs *int16, nb, blk int)
//
// quantizeQ8Go'nun AVX2 eşi: blok başına en büyük |x|, ölçek amax/127, kod = çifte yuvarla(x · float32(127/amax)),
// ±127 kırpma ve 16'lık alt blok toplamları (bs). Sonuç her blokta quantizeQ8Go ile bit bit aynıdır.
TEXT ·quantizeQ8Asm(SB), NOSPLIT, $0-48
	MOVQ         x+0(FP), SI
	MOVQ         q+8(FP), DI
	MOVQ         d+16(FP), R11
	MOVQ         bs+24(FP), R12
	MOVQ         nb+32(FP), CX
	MOVQ         blk+40(FP), R8
	MOVL         $0x7FFFFFFF, AX
	VMOVD        AX, X13
	VPBROADCASTD X13, Y13
	MOVL         $0x42FE0000, AX // 127.0f
	VMOVD        AX, X14
	VBROADCASTSS X14, Y14
	MOVL         $0xC2FE0000, AX // -127.0f
	VMOVD        AX, X15
	VBROADCASTSS X15, Y15

q8qBlok:
	MOVQ  SI, R9
	MOVQ  R8, R10
	SHRQ  $3, R10
	VPXOR Y0, Y0, Y0

q8qAmax:
	VPAND   (R9), Y13, Y1
	VPMAXSD Y1, Y0, Y0
	ADDQ    $32, R9
	DECQ    R10
	JNZ     q8qAmax

	VEXTRACTI128 $1, Y0, X1
	VPMAXSD      X1, X0, X0
	VPSHUFD      $0x4E, X0, X1
	VPMAXSD      X1, X0, X0
	VPSHUFD      $0xB1, X0, X1
	VPMAXSD      X1, X0, X0
	VMOVD        X0, AX
	TESTL        AX, AX
	JZ           q8qSifir
	VDIVSS       X0, X14, X1
	VMOVD        X1, BX
	ANDL         $0x7F800000, BX
	CMPL         BX, $0x7F800000
	JE           q8qSifir

	VDIVSS       X14, X0, X2
	VMOVSS       X2, (R11)
	VBROADCASTSS X1, Y1
	MOVQ         R8, R10
	SHRQ         $4, R10

q8qAlt:
	VMULPS       (SI), Y1, Y2
	VMULPS       32(SI), Y1, Y3
	VMAXPS       Y15, Y2, Y2
	VMINPS       Y14, Y2, Y2
	VMAXPS       Y15, Y3, Y3
	VMINPS       Y14, Y3, Y3
	VROUNDPS     $0x08, Y2, Y2
	VROUNDPS     $0x08, Y3, Y3
	VCVTTPS2DQ   Y2, Y2
	VCVTTPS2DQ   Y3, Y3
	VEXTRACTI128 $1, Y2, X4
	VEXTRACTI128 $1, Y3, X5
	VPACKSSDW    X4, X2, X2
	VPACKSSDW    X5, X3, X3
	VPADDW       X3, X2, X4
	VPHADDW      X4, X4, X4
	VPHADDW      X4, X4, X4
	VPHADDW      X4, X4, X4
	VMOVD        X4, AX
	MOVW         AX, (R12)
	VPACKSSWB    X3, X2, X2
	VMOVDQU      X2, (DI)
	ADDQ         $64, SI
	ADDQ         $16, DI
	ADDQ         $2, R12
	DECQ         R10
	JNZ          q8qAlt

	ADDQ $4, R11
	DECQ CX
	JNZ  q8qBlok
	VZEROUPPER
	RET

q8qSifir:
	MOVL  $0, (R11)
	VPXOR X2, X2, X2
	MOVQ  R8, R10
	SHRQ  $4, R10

q8qSifirAlt:
	VMOVDQU X2, (DI)
	MOVW    $0, (R12)
	ADDQ    $64, SI
	ADDQ    $16, DI
	ADDQ    $2, R12
	DECQ    R10
	JNZ     q8qSifirAlt

	ADDQ $4, R11
	DECQ CX
	JNZ  q8qBlok
	VZEROUPPER
	RET

// Q4KW4OLCEK bir ağırlık satırının 12 baytlık ölçeklerini açar: sc kelimelerini off(DX)'e yazar,
// m kelimelerini X11 (bs32 toplamları) ile çarpıp dört kısmi toplamı xm'ye koyar.
#define Q4KW4OLCEK(m4, m8, m12, off, xm) \
	MOVL      m4, R8; \
	MOVL      m8, R9; \
	MOVL      m12, R10; \
	MOVL      R8, AX; \
	ANDL      $0x3f3f3f3f, AX; \
	MOVL      R10, BX; \
	ANDL      $0x0f0f0f0f, BX; \
	SHRL      $6, R8; \
	ANDL      $0x03030303, R8; \
	SHLL      $4, R8; \
	ORL       R8, BX; \
	SHLQ      $32, BX; \
	ORQ       BX, AX; \
	VMOVQ     AX, X9; \
	MOVL      R9, AX; \
	ANDL      $0x3f3f3f3f, AX; \
	SHRL      $4, R10; \
	ANDL      $0x0f0f0f0f, R10; \
	SHRL      $6, R9; \
	ANDL      $0x03030303, R9; \
	SHLL      $4, R9; \
	ORL       R9, R10; \
	SHLQ      $32, R10; \
	ORQ       R10, AX; \
	VMOVQ     AX, X10; \
	VPMOVZXBW X9, X9; \
	VPMOVZXBW X10, X10; \
	VMOVDQU   X9, off(DX); \
	VPMADDWD  X11, X10, xm

// VNNI_GEMV_DPBUSD: Y_acc += Σ_4 uint8(Y_u8) · int8(Y_i8) (256 bit VEX.256.66.0F38.W0 50 /r).
#define VNNI_GEMV_DPBUSD(u8, i8, acc) \
	BYTE $0xC4; \
	BYTE $(0x42 | ((1-((acc)>>3))<<7) | ((1-((i8)>>3))<<5)); \
	BYTE $(((15-(u8))<<3) | 0x05); \
	BYTE $0x50; \
	BYTE $(0xC0 | (((acc)&7)<<3) | ((i8)&7))

// Q4KW4GRUPVNNI bir ağırlık satırının 64 öğelik grubunu VNNI ile daraltıp tek VPMADDWD ile ölçekler.
#define Q4KW4GRUPVNNI(wadr, scoff, acc) \
	VMOVDQU      wadr, Y2; \
	VPSRLW       $4, Y2, Y3; \
	VPAND        Y4, Y2, Y2; \
	VPAND        Y4, Y3, Y3; \
	VPXOR        Y7, Y7, Y7; \
	VPXOR        Y13, Y13, Y13; \
	VNNI_GEMV_DPBUSD(2, 5, 7); \
	VNNI_GEMV_DPBUSD(3, 6, 13); \
	VPSLLD       $16, Y13, Y13; \
	VPBLENDW     $0xAA, Y13, Y7, Y7; \
	VPBROADCASTD scoff(DX)(BX*1), Y13; \
	VPMADDWD     Y13, Y7, Y7; \
	VPADDD       Y7, acc, acc

// func sdotq4kw4q8Vnni(blocks int, w *byte, rowBytes int, q *int8, d *float32, bs *int16, tmp *byte, out *float32)
TEXT ·sdotq4kw4q8Vnni(SB), NOSPLIT, $0-64
	MOVQ         blocks+0(FP), CX
	MOVQ         w+8(FP), DI
	MOVQ         rowBytes+16(FP), R13
	MOVQ         q+24(FP), SI
	MOVQ         d+32(FP), R11
	MOVQ         bs+40(FP), R12
	MOVQ         tmp+48(FP), DX
	LEAQ         (DI)(R13*2), R14
	ADDQ         R13, R14
	VXORPS       X0, X0, X0
	MOVL         $0x0F0F0F0F, AX
	VMOVD        AX, X4
	VPBROADCASTD X4, Y4
	TESTQ        CX, CX
	JZ           q4w4vSon

q4w4vBlok:
	VMOVDQU (R12), X11
	VMOVDQU 16(R12), X12
	VPHADDW X12, X11, X11
	Q4KW4OLCEK(4(DI), 8(DI), 12(DI), 0, X1)
	Q4KW4OLCEK(4(DI)(R13*1), 8(DI)(R13*1), 12(DI)(R13*1), 16, X2)
	Q4KW4OLCEK(4(DI)(R13*2), 8(DI)(R13*2), 12(DI)(R13*2), 32, X3)
	Q4KW4OLCEK(4(R14), 8(R14), 12(R14), 48, X12)
	VPHADDD X2, X1, X1
	VPHADDD X12, X3, X3
	VPHADDD X3, X1, X15
	VPXOR   Y8, Y8, Y8
	VPXOR   Y9, Y9, Y9
	VPXOR   Y10, Y10, Y10
	VPXOR   Y11, Y11, Y11
	ADDQ    $16, DI
	ADDQ    $16, R14
	XORQ    BX, BX
	MOVQ    $4, R9

q4w4vGrup:
	VMOVDQU (SI), Y5
	VMOVDQU 32(SI), Y6
	Q4KW4GRUPVNNI((DI), 0, Y8)
	Q4KW4GRUPVNNI((DI)(R13*1), 16, Y9)
	Q4KW4GRUPVNNI((DI)(R13*2), 32, Y10)
	Q4KW4GRUPVNNI((R14), 48, Y11)
	ADDQ    $32, DI
	ADDQ    $32, R14
	ADDQ    $64, SI
	ADDQ    $4, BX
	DECQ    R9
	JNZ     q4w4vGrup

	VPHADDD      Y9, Y8, Y8
	VPHADDD      Y11, Y10, Y10
	VPHADDD      Y10, Y8, Y8
	VEXTRACTI128 $1, Y8, X9
	VPADDD       X9, X8, X8
	MOVL         -144(DI), AX
	VMOVD        AX, X2
	MOVL         -144(DI)(R13*1), AX
	VPINSRD      $1, AX, X2, X2
	MOVL         -144(DI)(R13*2), AX
	VPINSRD      $2, AX, X2, X2
	MOVL         -144(R14), AX
	VPINSRD      $3, AX, X2, X2
	VCVTPH2PS    X2, Y2
	VEXTRACTF128 $1, Y2, X3
	VSHUFPS      $0x88, X3, X2, X5
	VSHUFPS      $0xDD, X3, X2, X6
	VCVTDQ2PS    X8, X8
	VMULPS       X5, X8, X8
	VCVTDQ2PS    X15, X15
	VMULPS       X6, X15, X15
	VSUBPS       X15, X8, X8
	VBROADCASTSS (R11), X7
	VMULPS       X7, X8, X8
	VADDPS       X8, X0, X0
	ADDQ         $4, R11
	ADDQ         $32, R12
	DECQ         CX
	JNZ          q4w4vBlok

q4w4vSon:
	MOVQ       out+56(FP), R8
	VMOVUPS    X0, (R8)
	VZEROUPPER
	RET

// Q6KW4YARIVNNI bir ağırlık satırının 128 öğelik yarım bloğunu (q1, q3) ve (q2, q4) çiftleriyle VNNI üzerinden
// daraltıp iki VPMADDWD ile ölçekler (|s| <= 4·63·127 = 32004 <= 32767: int16'da taşmaz).
#define Q6KW4YARIVNNI(ql0, ql1, qh, sc, acc) \
	VMOVDQU     qh, Y4; \
	VPMOVSXBW   sc, X7; \
	VINSERTI128 $1, X7, Y7, Y7; \
	VMOVDQU     ql0, Y2; \
	VPAND       Y1, Y2, Y5; \
	VPSLLW      $4, Y4, Y6; \
	VPAND       128(DX), Y6, Y6; \
	VPOR        Y6, Y5, Y5; \
	VPXOR       Y3, Y3, Y3; \
	VNNI_GEMV_DPBUSD(5, 12, 3); \
	VPSRLW      $4, Y2, Y5; \
	VPAND       Y1, Y5, Y5; \
	VPAND       128(DX), Y4, Y6; \
	VPOR        Y6, Y5, Y5; \
	VPXOR       Y6, Y6, Y6; \
	VNNI_GEMV_DPBUSD(5, 14, 6); \
	VPSLLD      $16, Y6, Y6; \
	VPBLENDW    $0xAA, Y6, Y3, Y3; \
	VPSHUFB     160(DX), Y7, Y5; \
	VPMADDWD    Y5, Y3, Y3; \
	VPADDD      Y3, acc, acc; \
	VMOVDQU     ql1, Y2; \
	VPAND       Y1, Y2, Y5; \
	VPSLLW      $2, Y4, Y6; \
	VPAND       128(DX), Y6, Y6; \
	VPOR        Y6, Y5, Y5; \
	VPXOR       Y3, Y3, Y3; \
	VNNI_GEMV_DPBUSD(5, 13, 3); \
	VPSRLW      $4, Y2, Y5; \
	VPAND       Y1, Y5, Y5; \
	VPSRLW      $2, Y4, Y6; \
	VPAND       128(DX), Y6, Y6; \
	VPOR        Y6, Y5, Y5; \
	VPXOR       Y6, Y6, Y6; \
	VNNI_GEMV_DPBUSD(5, 15, 6); \
	VPSLLD      $16, Y6, Y6; \
	VPBLENDW    $0xAA, Y6, Y3, Y3; \
	VPSHUFB     192(DX), Y7, Y5; \
	VPMADDWD    Y5, Y3, Y3; \
	VPADDD      Y3, acc, acc

// func sdotq6kw4q8Vnni(blocks int, w *byte, rowBytes int, q *int8, d *float32, bs *int16, cnst *uint32, out *float32)
TEXT ·sdotq6kw4q8Vnni(SB), NOSPLIT, $0-64
	MOVQ         blocks+0(FP), CX
	MOVQ         w+8(FP), DI
	MOVQ         rowBytes+16(FP), R13
	MOVQ         q+24(FP), SI
	MOVQ         d+32(FP), R11
	MOVQ         bs+40(FP), R12
	MOVQ         cnst+48(FP), DX
	LEAQ         (DI)(R13*2), R14
	ADDQ         R13, R14
	VXORPS       X0, X0, X0
	MOVL         $0x0F0F0F0F, AX
	VMOVD        AX, X1
	VPBROADCASTD X1, Y1
	TESTQ        CX, CX
	JZ           q6w4vSon

q6w4vBlok:
	VPXOR   Y8, Y8, Y8
	VPXOR   Y9, Y9, Y9
	VPXOR   Y10, Y10, Y10
	VPXOR   Y11, Y11, Y11
	VMOVDQU 0(SI), Y12
	VMOVDQU 32(SI), Y13
	VMOVDQU 64(SI), Y14
	VMOVDQU 96(SI), Y15
	Q6KW4YARIVNNI(0(DI), 32(DI), 128(DI), 192(DI), Y8)
	Q6KW4YARIVNNI(0(DI)(R13*1), 32(DI)(R13*1), 128(DI)(R13*1), 192(DI)(R13*1), Y9)
	Q6KW4YARIVNNI(0(DI)(R13*2), 32(DI)(R13*2), 128(DI)(R13*2), 192(DI)(R13*2), Y10)
	Q6KW4YARIVNNI(0(R14), 32(R14), 128(R14), 192(R14), Y11)
	VMOVDQU 128(SI), Y12
	VMOVDQU 160(SI), Y13
	VMOVDQU 192(SI), Y14
	VMOVDQU 224(SI), Y15
	Q6KW4YARIVNNI(64(DI), 96(DI), 160(DI), 200(DI), Y8)
	Q6KW4YARIVNNI(64(DI)(R13*1), 96(DI)(R13*1), 160(DI)(R13*1), 200(DI)(R13*1), Y9)
	Q6KW4YARIVNNI(64(DI)(R13*2), 96(DI)(R13*2), 160(DI)(R13*2), 200(DI)(R13*2), Y10)
	Q6KW4YARIVNNI(64(R14), 96(R14), 160(R14), 200(R14), Y11)

	VPHADDD      Y9, Y8, Y8
	VPHADDD      Y11, Y10, Y10
	VPHADDD      Y10, Y8, Y8
	VEXTRACTI128 $1, Y8, X9
	VPADDD       X9, X8, X8

	VMOVDQU      (R12), Y5
	VPMOVSXBW    192(DI), Y2
	VPMADDWD     Y5, Y2, Y2
	VPMOVSXBW    192(DI)(R13*1), Y3
	VPMADDWD     Y5, Y3, Y3
	VPMOVSXBW    192(DI)(R13*2), Y4
	VPMADDWD     Y5, Y4, Y4
	VPMOVSXBW    192(R14), Y6
	VPMADDWD     Y5, Y6, Y6
	VPHADDD      Y3, Y2, Y2
	VPHADDD      Y6, Y4, Y4
	VPHADDD      Y4, Y2, Y2
	VEXTRACTI128 $1, Y2, X3
	VPADDD       X3, X2, X2
	VPSLLD       $5, X2, X2
	VPSUBD       X2, X8, X8
	VCVTDQ2PS    X8, X8

	VPXOR        X2, X2, X2
	MOVWLZX      208(DI), AX
	VPINSRW      $0, AX, X2, X2
	MOVWLZX      208(DI)(R13*1), AX
	VPINSRW      $1, AX, X2, X2
	MOVWLZX      208(DI)(R13*2), AX
	VPINSRW      $2, AX, X2, X2
	MOVWLZX      208(R14), AX
	VPINSRW      $3, AX, X2, X2
	VCVTPH2PS    X2, X2
	VMULPS       X2, X8, X8
	VBROADCASTSS (R11), X7
	VMULPS       X7, X8, X8
	VADDPS       X8, X0, X0

	ADDQ $210, DI
	ADDQ $210, R14
	ADDQ $256, SI
	ADDQ $4, R11
	ADDQ $32, R12
	DECQ CX
	JNZ  q6w4vBlok

q6w4vSon:
	MOVQ       out+56(FP), R8
	VMOVUPS    X0, (R8)
	VZEROUPPER
	RET
