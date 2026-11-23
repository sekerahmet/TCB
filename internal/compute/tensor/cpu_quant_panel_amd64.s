#include "textflag.h"

// VNNI_DPBUSD: Y_acc += Σ_4 uint8(Y_u8) · int8(Y_i8) (256 bit VEX.256.66.0F38.W0 50 /r).
#define VNNI_DPBUSD(u8, i8, acc) \
	BYTE $0xC4; \
	BYTE $(0x42 | ((1-((acc)>>3))<<7) | ((1-((i8)>>3))<<5)); \
	BYTE $(((15-(u8))<<3) | 0x05); \
	BYTE $0x50; \
	BYTE $(0xC0 | (((acc)&7)<<3) | ((i8)&7))

// TRANS8X8 Y0..Y7'deki sekiz satırın sekizer dword'ünü çevirip dst_off(target)'a 256 bayt yazar.
// Y8..Y14 geçici kullanılır; Y15 korunur.
#define TRANS8X8(base, off) \
	VPUNPCKLDQ  Y1, Y0, Y8; \
	VPUNPCKHDQ  Y1, Y0, Y9; \
	VPUNPCKLDQ  Y3, Y2, Y0; \
	VPUNPCKHDQ  Y3, Y2, Y1; \
	VPUNPCKLDQ  Y5, Y4, Y2; \
	VPUNPCKHDQ  Y5, Y4, Y3; \
	VPUNPCKLDQ  Y7, Y6, Y4; \
	VPUNPCKHDQ  Y7, Y6, Y5; \
	VPUNPCKLQDQ Y0, Y8, Y6; \
	VPUNPCKHQDQ Y0, Y8, Y7; \
	VPUNPCKLQDQ Y1, Y9, Y10; \
	VPUNPCKHQDQ Y1, Y9, Y11; \
	VPUNPCKLQDQ Y4, Y2, Y12; \
	VPUNPCKHQDQ Y4, Y2, Y13; \
	VPUNPCKLQDQ Y5, Y3, Y14; \
	VPUNPCKHQDQ Y5, Y3, Y0; \
	VPERM2I128  $0x20, Y12, Y6, Y1; \
	VMOVDQU     Y1, (off+0)(base); \
	VPERM2I128  $0x20, Y13, Y7, Y1; \
	VMOVDQU     Y1, (off+32)(base); \
	VPERM2I128  $0x20, Y14, Y10, Y1; \
	VMOVDQU     Y1, (off+64)(base); \
	VPERM2I128  $0x20, Y0, Y11, Y1; \
	VMOVDQU     Y1, (off+96)(base); \
	VPERM2I128  $0x31, Y12, Y6, Y1; \
	VMOVDQU     Y1, (off+128)(base); \
	VPERM2I128  $0x31, Y13, Y7, Y1; \
	VMOVDQU     Y1, (off+160)(base); \
	VPERM2I128  $0x31, Y14, Y10, Y1; \
	VMOVDQU     Y1, (off+192)(base); \
	VPERM2I128  $0x31, Y0, Y11, Y1; \
	VMOVDQU     Y1, (off+224)(base)

// TRANS4X8 Y0..Y7'nin alt 128 bitlerindeki dörder dword'ü çevirip (off+0..96)(base)'e 128 bayt yazar.
#define TRANS4X8(base, off) \
	VPUNPCKLDQ  X1, X0, X8; \
	VPUNPCKHDQ  X1, X0, X9; \
	VPUNPCKLDQ  X3, X2, X0; \
	VPUNPCKHDQ  X3, X2, X1; \
	VPUNPCKLDQ  X5, X4, X2; \
	VPUNPCKHDQ  X5, X4, X3; \
	VPUNPCKLDQ  X7, X6, X4; \
	VPUNPCKHDQ  X7, X6, X5; \
	VPUNPCKLQDQ X0, X8, X6; \
	VPUNPCKHQDQ X0, X8, X7; \
	VPUNPCKLQDQ X1, X9, X10; \
	VPUNPCKHQDQ X1, X9, X11; \
	VPUNPCKLQDQ X4, X2, X12; \
	VPUNPCKHQDQ X4, X2, X13; \
	VPUNPCKLQDQ X5, X3, X14; \
	VPUNPCKHQDQ X5, X3, X0; \
	VINSERTI128 $1, X12, Y6, Y1; \
	VMOVDQU     Y1, (off+0)(base); \
	VINSERTI128 $1, X13, Y7, Y1; \
	VMOVDQU     Y1, (off+32)(base); \
	VINSERTI128 $1, X14, Y10, Y1; \
	VMOVDQU     Y1, (off+64)(base); \
	VINSERTI128 $1, X0, Y11, Y1; \
	VMOVDQU     Y1, (off+96)(base)

// Q4KFP16 sekiz ağırlık satırının off bayttaki fp16 ölçeğini Y1'e float32 olarak açar.
#define Q4KFP16(off) \
	VPXOR   X1, X1, X1; \
	MOVWLZX off(DI), AX; \
	VPINSRW $0, AX, X1, X1; \
	MOVWLZX off(DI)(BX*1), AX; \
	VPINSRW $1, AX, X1, X1; \
	MOVWLZX off(DI)(BX*2), AX; \
	VPINSRW $2, AX, X1, X1; \
	MOVWLZX off(R10), AX; \
	VPINSRW $3, AX, X1, X1; \
	MOVWLZX off(DI)(BX*4), AX; \
	VPINSRW $4, AX, X1, X1; \
	MOVWLZX off(R10)(BX*2), AX; \
	VPINSRW $5, AX, X1, X1; \
	MOVWLZX off(R11), AX; \
	VPINSRW $6, AX, X1, X1; \
	MOVWLZX off(R11)(BX*1), AX; \
	VPINSRW $7, AX, X1, X1; \
	VCVTPH2PS X1, Y1

// Q4KSC bir ağırlık satırının sekiz alt blok ölçeğini (sc0..sc7) int32 olarak yd yazmacına açar.
#define Q4KSC(m4, m8, m12, xd, yd) \
	MOVL       m4, R12; \
	MOVL       m12, R13; \
	MOVL       R12, AX; \
	ANDL       $0x3f3f3f3f, AX; \
	MOVL       R13, R14; \
	ANDL       $0x0f0f0f0f, R14; \
	SHRL       $6, R12; \
	ANDL       $0x03030303, R12; \
	SHLL       $4, R12; \
	ORL        R12, R14; \
	SHLQ       $32, R14; \
	ORQ        R14, AX; \
	VMOVQ      AX, xd; \
	VPMOVZXBD  xd, yd

// Q4KMIN bir ağırlık satırının sekiz alt blok minimumunu (m0..m7) çiftler halinde int16 olarak xd'ye açar.
#define Q4KMIN(m8, m12, xd) \
	MOVL       m8, R12; \
	MOVL       m12, R13; \
	MOVL       R12, AX; \
	ANDL       $0x3f3f3f3f, AX; \
	SHRL       $4, R13; \
	ANDL       $0x0f0f0f0f, R13; \
	SHRL       $6, R12; \
	ANDL       $0x03030303, R12; \
	SHLL       $4, R12; \
	ORL        R12, R13; \
	SHLQ       $32, R13; \
	ORQ        R13, AX; \
	VMOVQ      AX, xd; \
	VPMOVZXBW  xd, xd

// Q4KKODLO sekiz satırın off bayttaki 32 alt nibble kodunu Y0..Y7'ye yükler (Y15 = 0x0F).
#define Q4KKODLO(off) \
	VPAND off(DI), Y15, Y0; \
	VPAND off(DI)(BX*1), Y15, Y1; \
	VPAND off(DI)(BX*2), Y15, Y2; \
	VPAND off(R10), Y15, Y3; \
	VPAND off(DI)(BX*4), Y15, Y4; \
	VPAND off(R10)(BX*2), Y15, Y5; \
	VPAND off(R11), Y15, Y6; \
	VPAND off(R11)(BX*1), Y15, Y7

// Q4KKODHI sekiz satırın off bayttaki 32 üst nibble kodunu Y0..Y7'ye yükler (Y15 = 0x0F).
#define Q4KKODHI(off) \
	VMOVDQU off(DI), Y0; VPSRLW $4, Y0, Y0; VPAND Y15, Y0, Y0; \
	VMOVDQU off(DI)(BX*1), Y1; VPSRLW $4, Y1, Y1; VPAND Y15, Y1, Y1; \
	VMOVDQU off(DI)(BX*2), Y2; VPSRLW $4, Y2, Y2; VPAND Y15, Y2, Y2; \
	VMOVDQU off(R10), Y3; VPSRLW $4, Y3, Y3; VPAND Y15, Y3, Y3; \
	VMOVDQU off(DI)(BX*4), Y4; VPSRLW $4, Y4, Y4; VPAND Y15, Y4, Y4; \
	VMOVDQU off(R10)(BX*2), Y5; VPSRLW $4, Y5, Y5; VPAND Y15, Y5, Y5; \
	VMOVDQU off(R11), Y6; VPSRLW $4, Y6, Y6; VPAND Y15, Y6, Y6; \
	VMOVDQU off(R11)(BX*1), Y7; VPSRLW $4, Y7, Y7; VPAND Y15, Y7, Y7

// func packq4kx8(blocks int, w *byte, rowBytes int, pan *byte, pmeta *byte)
TEXT ·packq4kx8(SB), NOSPLIT, $0-40
	MOVQ         blocks+0(FP), CX
	MOVQ         w+8(FP), DI
	MOVQ         rowBytes+16(FP), BX
	MOVQ         pan+24(FP), DX
	MOVQ         pmeta+32(FP), R9
	LEAQ         (DI)(BX*2), R10
	ADDQ         BX, R10
	LEAQ         (R10)(BX*2), R11
	ADDQ         BX, R11
	MOVL         $0x0F0F0F0F, AX
	VMOVD        AX, X15
	VPBROADCASTD X15, Y15
	TESTQ        CX, CX
	JZ           pk4son

pk4blok:
	Q4KFP16(0)
	VMOVUPS Y1, 0(R9)
	Q4KFP16(2)
	VMOVUPS Y1, 32(R9)

	Q4KSC(4(DI), 8(DI), 12(DI), X0, Y0)
	Q4KSC(4(DI)(BX*1), 8(DI)(BX*1), 12(DI)(BX*1), X1, Y1)
	Q4KSC(4(DI)(BX*2), 8(DI)(BX*2), 12(DI)(BX*2), X2, Y2)
	Q4KSC(4(R10), 8(R10), 12(R10), X3, Y3)
	Q4KSC(4(DI)(BX*4), 8(DI)(BX*4), 12(DI)(BX*4), X4, Y4)
	Q4KSC(4(R10)(BX*2), 8(R10)(BX*2), 12(R10)(BX*2), X5, Y5)
	Q4KSC(4(R11), 8(R11), 12(R11), X6, Y6)
	Q4KSC(4(R11)(BX*1), 8(R11)(BX*1), 12(R11)(BX*1), X7, Y7)
	TRANS8X8(R9, 64)

	Q4KMIN(8(DI), 12(DI), X0)
	Q4KMIN(8(DI)(BX*1), 12(DI)(BX*1), X1)
	Q4KMIN(8(DI)(BX*2), 12(DI)(BX*2), X2)
	Q4KMIN(8(R10), 12(R10), X3)
	Q4KMIN(8(DI)(BX*4), 12(DI)(BX*4), X4)
	Q4KMIN(8(R10)(BX*2), 12(R10)(BX*2), X5)
	Q4KMIN(8(R11), 12(R11), X6)
	Q4KMIN(8(R11)(BX*1), 12(R11)(BX*1), X7)
	TRANS4X8(R9, 320)

	Q4KKODLO(16)
	TRANS8X8(DX, 0)
	Q4KKODHI(16)
	TRANS8X8(DX, 256)
	Q4KKODLO(48)
	TRANS8X8(DX, 512)
	Q4KKODHI(48)
	TRANS8X8(DX, 768)
	Q4KKODLO(80)
	TRANS8X8(DX, 1024)
	Q4KKODHI(80)
	TRANS8X8(DX, 1280)
	Q4KKODLO(112)
	TRANS8X8(DX, 1536)
	Q4KKODHI(112)
	TRANS8X8(DX, 1792)

	ADDQ $144, DI
	ADDQ $144, R10
	ADDQ $144, R11
	ADDQ $2048, DX
	ADDQ $448, R9
	DECQ CX
	JNZ  pk4blok

pk4son:
	VZEROUPPER
	RET

// Q4KVNNIGRUP bir 4-bayt grubu dört etkinleştirme satırıyla VNNI üzerinden Y8..Y11'e biriktirir.
#define Q4KVNNIGRUP(g) \
	VMOVDQU      (g*32)(DI), Y12; \
	VPBROADCASTD (g*4)(SI), Y13; \
	VNNI_DPBUSD(12, 13, 8); \
	VPBROADCASTD (g*4)(SI)(BX*1), Y14; \
	VNNI_DPBUSD(12, 14, 9); \
	VPBROADCASTD (g*4)(SI)(BX*2), Y13; \
	VNNI_DPBUSD(12, 13, 10); \
	VPBROADCASTD (g*4)(SI)(DX*1), Y14; \
	VNNI_DPBUSD(12, 14, 11)

// Q4KAVX2GRUP bir 4-bayt grubu dört etkinleştirme satırıyla AVX2 üzerinden Y8..Y11'e (int16) biriktirir.
#define Q4KAVX2GRUP(g) \
	VMOVDQU      (g*32)(DI), Y12; \
	VPBROADCASTD (g*4)(SI), Y13; \
	VPMADDUBSW   Y13, Y12, Y13; \
	VPADDW       Y13, Y8, Y8; \
	VPBROADCASTD (g*4)(SI)(BX*1), Y13; \
	VPMADDUBSW   Y13, Y12, Y13; \
	VPADDW       Y13, Y9, Y9; \
	VPBROADCASTD (g*4)(SI)(BX*2), Y13; \
	VPMADDUBSW   Y13, Y12, Y13; \
	VPADDW       Y13, Y10, Y10; \
	VPBROADCASTD (g*4)(SI)(DX*1), Y13; \
	VPMADDUBSW   Y13, Y12, Y13; \
	VPADDW       Y13, Y11, Y11

// Q4KPANELSON bir etkinleştirme satırının Q4_K blok sonucunu float32 acc'ye ekler (dotQ4KQ8Go sırası).
#define Q4KPANELSON(isum, b32ptr, dxm, acc) \
	VPBROADCASTD 0(b32ptr), Y8; \
	VPMADDWD     320(R9), Y8, Y8; \
	VPBROADCASTD 4(b32ptr), Y9; \
	VPMADDWD     352(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VPBROADCASTD 8(b32ptr), Y9; \
	VPMADDWD     384(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VPBROADCASTD 12(b32ptr), Y9; \
	VPMADDWD     416(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VCVTDQ2PS    isum, isum; \
	VMULPS       0(R9), isum, isum; \
	VCVTDQ2PS    Y8, Y8; \
	VMULPS       32(R9), Y8, Y8; \
	VSUBPS       Y8, isum, isum; \
	VBROADCASTSS dxm, Y9; \
	VMULPS       Y9, isum, isum; \
	VADDPS       isum, acc, acc

// func sdotq4kx8q8x4Vnni(blocks int, pan *byte, pmeta *byte, q *int8, d *float32, bs32 *int16, out *float32, sq, sd, sbs int)
TEXT ·sdotq4kx8q8x4Vnni(SB), NOSPLIT, $0-80
	MOVQ   blocks+0(FP), CX
	MOVQ   pan+8(FP), DI
	MOVQ   pmeta+16(FP), R9
	MOVQ   q+24(FP), SI
	MOVQ   d+32(FP), R11
	MOVQ   bs32+40(FP), R12
	MOVQ   sq+56(FP), BX
	MOVQ   sd+64(FP), R13
	LEAQ   (BX)(BX*2), DX
	VXORPS Y0, Y0, Y0
	VXORPS Y1, Y1, Y1
	VXORPS Y2, Y2, Y2
	VXORPS Y3, Y3, Y3
	TESTQ  CX, CX
	JZ     q4kvSon

q4kvBlok:
	VPXOR Y4, Y4, Y4
	VPXOR Y5, Y5, Y5
	VPXOR Y6, Y6, Y6
	VPXOR Y7, Y7, Y7
	LEAQ  64(R9), R10
	MOVQ  $8, R8

q4kvAlt:
	VPXOR Y8, Y8, Y8
	VPXOR Y9, Y9, Y9
	VPXOR Y10, Y10, Y10
	VPXOR Y11, Y11, Y11
	Q4KVNNIGRUP(0)
	Q4KVNNIGRUP(1)
	Q4KVNNIGRUP(2)
	Q4KVNNIGRUP(3)
	Q4KVNNIGRUP(4)
	Q4KVNNIGRUP(5)
	Q4KVNNIGRUP(6)
	Q4KVNNIGRUP(7)
	VMOVDQU (R10), Y15
	VPMULLD Y15, Y8, Y8
	VPADDD  Y8, Y4, Y4
	VPMULLD Y15, Y9, Y9
	VPADDD  Y9, Y5, Y5
	VPMULLD Y15, Y10, Y10
	VPADDD  Y10, Y6, Y6
	VPMULLD Y15, Y11, Y11
	VPADDD  Y11, Y7, Y7
	ADDQ    $256, DI
	ADDQ    $32, SI
	ADDQ    $32, R10
	DECQ    R8
	JNZ     q4kvAlt

	MOVQ R12, AX
	MOVQ sbs+72(FP), R14
	Q4KPANELSON(Y4, AX, (R11), Y0)
	ADDQ R14, AX
	Q4KPANELSON(Y5, AX, (R11)(R13*1), Y1)
	ADDQ R14, AX
	Q4KPANELSON(Y6, AX, (R11)(R13*2), Y2)
	ADDQ R14, AX
	LEAQ (R11)(R13*2), R10
	ADDQ R13, R10
	Q4KPANELSON(Y7, AX, (R10), Y3)

	ADDQ $448, R9
	ADDQ $4, R11
	ADDQ $16, R12
	DECQ CX
	JNZ  q4kvBlok

q4kvSon:
	MOVQ       out+48(FP), R8
	VMOVUPS    Y0, 0(R8)
	VMOVUPS    Y1, 32(R8)
	VMOVUPS    Y2, 64(R8)
	VMOVUPS    Y3, 96(R8)
	VZEROUPPER
	RET

// func sdotq4kx8q8x4Avx2(blocks int, pan *byte, pmeta *byte, q *int8, d *float32, bs32 *int16, out *float32, sq, sd, sbs int)
TEXT ·sdotq4kx8q8x4Avx2(SB), NOSPLIT, $0-80
	MOVQ         blocks+0(FP), CX
	MOVQ         pan+8(FP), DI
	MOVQ         pmeta+16(FP), R9
	MOVQ         q+24(FP), SI
	MOVQ         d+32(FP), R11
	MOVQ         bs32+40(FP), R12
	MOVQ         sq+56(FP), BX
	MOVQ         sd+64(FP), R13
	LEAQ         (BX)(BX*2), DX
	MOVL         $0x00010001, AX
	VMOVD        AX, X14
	VPBROADCASTD X14, Y14
	VXORPS       Y0, Y0, Y0
	VXORPS       Y1, Y1, Y1
	VXORPS       Y2, Y2, Y2
	VXORPS       Y3, Y3, Y3
	TESTQ        CX, CX
	JZ           q4kaSon

q4kaBlok:
	VPXOR Y4, Y4, Y4
	VPXOR Y5, Y5, Y5
	VPXOR Y6, Y6, Y6
	VPXOR Y7, Y7, Y7
	LEAQ  64(R9), R10
	MOVQ  $8, R8

q4kaAlt:
	VPXOR Y8, Y8, Y8
	VPXOR Y9, Y9, Y9
	VPXOR Y10, Y10, Y10
	VPXOR Y11, Y11, Y11
	Q4KAVX2GRUP(0)
	Q4KAVX2GRUP(1)
	Q4KAVX2GRUP(2)
	Q4KAVX2GRUP(3)
	Q4KAVX2GRUP(4)
	Q4KAVX2GRUP(5)
	Q4KAVX2GRUP(6)
	Q4KAVX2GRUP(7)
	VPMADDWD Y14, Y8, Y8
	VPMADDWD Y14, Y9, Y9
	VPMADDWD Y14, Y10, Y10
	VPMADDWD Y14, Y11, Y11
	VMOVDQU  (R10), Y15
	VPMULLD  Y15, Y8, Y8
	VPADDD   Y8, Y4, Y4
	VPMULLD  Y15, Y9, Y9
	VPADDD   Y9, Y5, Y5
	VPMULLD  Y15, Y10, Y10
	VPADDD   Y10, Y6, Y6
	VPMULLD  Y15, Y11, Y11
	VPADDD   Y11, Y7, Y7
	ADDQ     $256, DI
	ADDQ     $32, SI
	ADDQ     $32, R10
	DECQ     R8
	JNZ      q4kaAlt

	MOVQ R12, AX
	MOVQ sbs+72(FP), R14
	Q4KPANELSON(Y4, AX, (R11), Y0)
	ADDQ R14, AX
	Q4KPANELSON(Y5, AX, (R11)(R13*1), Y1)
	ADDQ R14, AX
	Q4KPANELSON(Y6, AX, (R11)(R13*2), Y2)
	ADDQ R14, AX
	LEAQ (R11)(R13*2), R10
	ADDQ R13, R10
	Q4KPANELSON(Y7, AX, (R10), Y3)

	MOVL         $0x00010001, AX
	VMOVD        AX, X14
	VPBROADCASTD X14, Y14
	ADDQ         $448, R9
	ADDQ         $4, R11
	ADDQ         $16, R12
	DECQ         CX
	JNZ          q4kaBlok

q4kaSon:
	MOVQ       out+48(FP), R8
	VMOVUPS    Y0, 0(R8)
	VMOVUPS    Y1, 32(R8)
	VMOVUPS    Y2, 64(R8)
	VMOVUPS    Y3, 96(R8)
	VZEROUPPER
	RET

// Q6KPARCA1 bir satırın q1 parçasını (ql&0xF | (qh&3)<<4) dst'ye açar (Y15=0x0F, Y14=0x30, Y8 geçici).
#define Q6KP1(ql, qh, dst) \
	VPAND  ql, Y15, dst; \
	VMOVDQU qh, Y8; \
	VPSLLW $4, Y8, Y8; \
	VPAND  Y14, Y8, Y8; \
	VPOR   Y8, dst, dst

// Q6KP2 bir satırın q2 parçasını (ql&0xF | ((qh>>2)&3)<<4) dst'ye açar.
#define Q6KP2(ql, qh, dst) \
	VPAND  ql, Y15, dst; \
	VMOVDQU qh, Y8; \
	VPSLLW $2, Y8, Y8; \
	VPAND  Y14, Y8, Y8; \
	VPOR   Y8, dst, dst

// Q6KP3 bir satırın q3 parçasını ((ql>>4) | ((qh>>4)&3)<<4) dst'ye açar.
#define Q6KP3(ql, qh, dst) \
	VMOVDQU ql, dst; \
	VPSRLW $4, dst, dst; \
	VPAND  Y15, dst, dst; \
	VPAND  qh, Y14, Y8; \
	VPOR   Y8, dst, dst

// Q6KP4 bir satırın q4 parçasını ((ql>>4) | ((qh>>6)&3)<<4) dst'ye açar.
#define Q6KP4(ql, qh, dst) \
	VMOVDQU ql, dst; \
	VPSRLW $4, dst, dst; \
	VPAND  Y15, dst, dst; \
	VMOVDQU qh, Y8; \
	VPSRLW $2, Y8, Y8; \
	VPAND  Y14, Y8, Y8; \
	VPOR   Y8, dst, dst

// Q6KYARI sekiz satırın 128 öğelik yarım bloğunu (qlo, qlo+32, qho) panele (poff) yazar.
#define Q6KYARI(qlo, qho, poff) \
	MOVL $0x30303030, AX; VMOVD AX, X14; VPBROADCASTD X14, Y14; \
	Q6KP1(qlo(DI), qho(DI), Y0); \
	Q6KP1(qlo(DI)(BX*1), qho(DI)(BX*1), Y1); \
	Q6KP1(qlo(DI)(BX*2), qho(DI)(BX*2), Y2); \
	Q6KP1(qlo(R10), qho(R10), Y3); \
	Q6KP1(qlo(DI)(BX*4), qho(DI)(BX*4), Y4); \
	Q6KP1(qlo(R10)(BX*2), qho(R10)(BX*2), Y5); \
	Q6KP1(qlo(R11), qho(R11), Y6); \
	Q6KP1(qlo(R11)(BX*1), qho(R11)(BX*1), Y7); \
	TRANS8X8(DX, (poff+0)); \
	MOVL $0x30303030, AX; VMOVD AX, X14; VPBROADCASTD X14, Y14; \
	Q6KP2((qlo+32)(DI), qho(DI), Y0); \
	Q6KP2((qlo+32)(DI)(BX*1), qho(DI)(BX*1), Y1); \
	Q6KP2((qlo+32)(DI)(BX*2), qho(DI)(BX*2), Y2); \
	Q6KP2((qlo+32)(R10), qho(R10), Y3); \
	Q6KP2((qlo+32)(DI)(BX*4), qho(DI)(BX*4), Y4); \
	Q6KP2((qlo+32)(R10)(BX*2), qho(R10)(BX*2), Y5); \
	Q6KP2((qlo+32)(R11), qho(R11), Y6); \
	Q6KP2((qlo+32)(R11)(BX*1), qho(R11)(BX*1), Y7); \
	TRANS8X8(DX, (poff+256)); \
	MOVL $0x30303030, AX; VMOVD AX, X14; VPBROADCASTD X14, Y14; \
	Q6KP3(qlo(DI), qho(DI), Y0); \
	Q6KP3(qlo(DI)(BX*1), qho(DI)(BX*1), Y1); \
	Q6KP3(qlo(DI)(BX*2), qho(DI)(BX*2), Y2); \
	Q6KP3(qlo(R10), qho(R10), Y3); \
	Q6KP3(qlo(DI)(BX*4), qho(DI)(BX*4), Y4); \
	Q6KP3(qlo(R10)(BX*2), qho(R10)(BX*2), Y5); \
	Q6KP3(qlo(R11), qho(R11), Y6); \
	Q6KP3(qlo(R11)(BX*1), qho(R11)(BX*1), Y7); \
	TRANS8X8(DX, (poff+512)); \
	MOVL $0x30303030, AX; VMOVD AX, X14; VPBROADCASTD X14, Y14; \
	Q6KP4((qlo+32)(DI), qho(DI), Y0); \
	Q6KP4((qlo+32)(DI)(BX*1), qho(DI)(BX*1), Y1); \
	Q6KP4((qlo+32)(DI)(BX*2), qho(DI)(BX*2), Y2); \
	Q6KP4((qlo+32)(R10), qho(R10), Y3); \
	Q6KP4((qlo+32)(DI)(BX*4), qho(DI)(BX*4), Y4); \
	Q6KP4((qlo+32)(R10)(BX*2), qho(R10)(BX*2), Y5); \
	Q6KP4((qlo+32)(R11), qho(R11), Y6); \
	Q6KP4((qlo+32)(R11)(BX*1), qho(R11)(BX*1), Y7); \
	TRANS8X8(DX, (poff+768))

// func packq6kx8(blocks int, w *byte, rowBytes int, pan *byte, pmeta *byte)
TEXT ·packq6kx8(SB), NOSPLIT, $0-40
	MOVQ         blocks+0(FP), CX
	MOVQ         w+8(FP), DI
	MOVQ         rowBytes+16(FP), BX
	MOVQ         pan+24(FP), DX
	MOVQ         pmeta+32(FP), R9
	LEAQ         (DI)(BX*2), R10
	ADDQ         BX, R10
	LEAQ         (R10)(BX*2), R11
	ADDQ         BX, R11
	MOVL         $0x0F0F0F0F, AX
	VMOVD        AX, X15
	VPBROADCASTD X15, Y15
	TESTQ        CX, CX
	JZ           pk6son

pk6blok:
	Q4KFP16(208)
	VMOVUPS Y1, 0(R9)

	VPMOVSXBW 192(DI), Y0
	VPMOVSXBW 192(DI)(BX*1), Y1
	VPMOVSXBW 192(DI)(BX*2), Y2
	VPMOVSXBW 192(R10), Y3
	VPMOVSXBW 192(DI)(BX*4), Y4
	VPMOVSXBW 192(R10)(BX*2), Y5
	VPMOVSXBW 192(R11), Y6
	VPMOVSXBW 192(R11)(BX*1), Y7
	TRANS8X8(R9, 544)

	XORQ R12, R12
	XORQ R13, R13

pk6sc:
	VMOVDQU 544(R9)(R12*1), Y0
	VPSLLD  $16, Y0, Y1
	VPSRAD  $16, Y1, Y1
	VPSRAD  $16, Y0, Y2
	VMOVDQU Y1, 32(R9)(R13*1)
	VMOVDQU Y2, 64(R9)(R13*1)
	ADDQ    $32, R12
	ADDQ    $64, R13
	CMPQ    R12, $256
	JNE     pk6sc

	Q6KYARI(0, 128, 0)
	Q6KYARI(64, 160, 1024)

	ADDQ $210, DI
	ADDQ $210, R10
	ADDQ $210, R11
	ADDQ $2048, DX
	ADDQ $800, R9
	DECQ CX
	JNZ  pk6blok

pk6son:
	VZEROUPPER
	RET

// Q6KPANELSON bir etkinleştirme satırının Q6_K blok sonucunu float32 acc'ye ekler (dotQ6KQ8Go sırası).
#define Q6KPANELSON(isum, bsptr, dxm, acc) \
	VPBROADCASTD 0(bsptr), Y8; \
	VPMADDWD     544(R9), Y8, Y8; \
	VPBROADCASTD 4(bsptr), Y9; \
	VPMADDWD     576(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VPBROADCASTD 8(bsptr), Y9; \
	VPMADDWD     608(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VPBROADCASTD 12(bsptr), Y9; \
	VPMADDWD     640(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VPBROADCASTD 16(bsptr), Y9; \
	VPMADDWD     672(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VPBROADCASTD 20(bsptr), Y9; \
	VPMADDWD     704(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VPBROADCASTD 24(bsptr), Y9; \
	VPMADDWD     736(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VPBROADCASTD 28(bsptr), Y9; \
	VPMADDWD     768(R9), Y9, Y9; \
	VPADDD       Y9, Y8, Y8; \
	VPSLLD       $5, Y8, Y8; \
	VPSUBD       Y8, isum, isum; \
	VCVTDQ2PS    isum, isum; \
	VMULPS       0(R9), isum, isum; \
	VBROADCASTSS dxm, Y9; \
	VMULPS       Y9, isum, isum; \
	VADDPS       isum, acc, acc

// func sdotq6kx8q8x4Vnni(blocks int, pan *byte, pmeta *byte, q *int8, d *float32, bs *int16, out *float32, sq, sd, sbs int)
TEXT ·sdotq6kx8q8x4Vnni(SB), NOSPLIT, $0-80
	MOVQ   blocks+0(FP), CX
	MOVQ   pan+8(FP), DI
	MOVQ   pmeta+16(FP), R9
	MOVQ   q+24(FP), SI
	MOVQ   d+32(FP), R11
	MOVQ   bs+40(FP), R12
	MOVQ   sq+56(FP), BX
	MOVQ   sd+64(FP), R13
	LEAQ   (BX)(BX*2), DX
	VXORPS Y0, Y0, Y0
	VXORPS Y1, Y1, Y1
	VXORPS Y2, Y2, Y2
	VXORPS Y3, Y3, Y3
	TESTQ  CX, CX
	JZ     q6kvSon

q6kvBlok:
	VPXOR Y4, Y4, Y4
	VPXOR Y5, Y5, Y5
	VPXOR Y6, Y6, Y6
	VPXOR Y7, Y7, Y7
	LEAQ  32(R9), R10
	MOVQ  $16, R8

q6kvAlt:
	VPXOR Y8, Y8, Y8
	VPXOR Y9, Y9, Y9
	VPXOR Y10, Y10, Y10
	VPXOR Y11, Y11, Y11
	Q4KVNNIGRUP(0)
	Q4KVNNIGRUP(1)
	Q4KVNNIGRUP(2)
	Q4KVNNIGRUP(3)
	VMOVDQU (R10), Y15
	VPMULLD Y15, Y8, Y8
	VPADDD  Y8, Y4, Y4
	VPMULLD Y15, Y9, Y9
	VPADDD  Y9, Y5, Y5
	VPMULLD Y15, Y10, Y10
	VPADDD  Y10, Y6, Y6
	VPMULLD Y15, Y11, Y11
	VPADDD  Y11, Y7, Y7
	ADDQ    $128, DI
	ADDQ    $16, SI
	ADDQ    $32, R10
	DECQ    R8
	JNZ     q6kvAlt

	MOVQ R12, AX
	MOVQ sbs+72(FP), R14
	Q6KPANELSON(Y4, AX, (R11), Y0)
	ADDQ R14, AX
	Q6KPANELSON(Y5, AX, (R11)(R13*1), Y1)
	ADDQ R14, AX
	Q6KPANELSON(Y6, AX, (R11)(R13*2), Y2)
	ADDQ R14, AX
	LEAQ (R11)(R13*2), R10
	ADDQ R13, R10
	Q6KPANELSON(Y7, AX, (R10), Y3)

	ADDQ $800, R9
	ADDQ $4, R11
	ADDQ $32, R12
	DECQ CX
	JNZ  q6kvBlok

q6kvSon:
	MOVQ       out+48(FP), R8
	VMOVUPS    Y0, 0(R8)
	VMOVUPS    Y1, 32(R8)
	VMOVUPS    Y2, 64(R8)
	VMOVUPS    Y3, 96(R8)
	VZEROUPPER
	RET

// Q6KAVX2GRUP bir 4-bayt grubu dört etkinleştirme satırıyla AVX2 üzerinden Y8..Y11'e (int32) biriktirir.
#define Q6KAVX2GRUP(g) \
	VMOVDQU      (g*32)(DI), Y12; \
	VPBROADCASTD (g*4)(SI), Y13; \
	VPMADDUBSW   Y13, Y12, Y13; \
	VPMADDWD     Y14, Y13, Y13; \
	VPADDD       Y13, Y8, Y8; \
	VPBROADCASTD (g*4)(SI)(BX*1), Y13; \
	VPMADDUBSW   Y13, Y12, Y13; \
	VPMADDWD     Y14, Y13, Y13; \
	VPADDD       Y13, Y9, Y9; \
	VPBROADCASTD (g*4)(SI)(BX*2), Y13; \
	VPMADDUBSW   Y13, Y12, Y13; \
	VPMADDWD     Y14, Y13, Y13; \
	VPADDD       Y13, Y10, Y10; \
	VPBROADCASTD (g*4)(SI)(DX*1), Y13; \
	VPMADDUBSW   Y13, Y12, Y13; \
	VPMADDWD     Y14, Y13, Y13; \
	VPADDD       Y13, Y11, Y11

// func sdotq6kx8q8x4Avx2(blocks int, pan *byte, pmeta *byte, q *int8, d *float32, bs *int16, out *float32, sq, sd, sbs int)
TEXT ·sdotq6kx8q8x4Avx2(SB), NOSPLIT, $0-80
	MOVQ         blocks+0(FP), CX
	MOVQ         pan+8(FP), DI
	MOVQ         pmeta+16(FP), R9
	MOVQ         q+24(FP), SI
	MOVQ         d+32(FP), R11
	MOVQ         bs+40(FP), R12
	MOVQ         sq+56(FP), BX
	MOVQ         sd+64(FP), R13
	LEAQ         (BX)(BX*2), DX
	MOVL         $0x00010001, AX
	VMOVD        AX, X14
	VPBROADCASTD X14, Y14
	VXORPS       Y0, Y0, Y0
	VXORPS       Y1, Y1, Y1
	VXORPS       Y2, Y2, Y2
	VXORPS       Y3, Y3, Y3
	TESTQ        CX, CX
	JZ           q6kaSon

q6kaBlok:
	VPXOR Y4, Y4, Y4
	VPXOR Y5, Y5, Y5
	VPXOR Y6, Y6, Y6
	VPXOR Y7, Y7, Y7
	LEAQ  32(R9), R10
	MOVQ  $16, R8

q6kaAlt:
	VPXOR Y8, Y8, Y8
	VPXOR Y9, Y9, Y9
	VPXOR Y10, Y10, Y10
	VPXOR Y11, Y11, Y11
	Q6KAVX2GRUP(0)
	Q6KAVX2GRUP(1)
	Q6KAVX2GRUP(2)
	Q6KAVX2GRUP(3)
	VMOVDQU (R10), Y15
	VPMULLD Y15, Y8, Y8
	VPADDD  Y8, Y4, Y4
	VPMULLD Y15, Y9, Y9
	VPADDD  Y9, Y5, Y5
	VPMULLD Y15, Y10, Y10
	VPADDD  Y10, Y6, Y6
	VPMULLD Y15, Y11, Y11
	VPADDD  Y11, Y7, Y7
	ADDQ    $128, DI
	ADDQ    $16, SI
	ADDQ    $32, R10
	DECQ    R8
	JNZ     q6kaAlt

	MOVQ R12, AX
	MOVQ sbs+72(FP), R14
	Q6KPANELSON(Y4, AX, (R11), Y0)
	ADDQ R14, AX
	Q6KPANELSON(Y5, AX, (R11)(R13*1), Y1)
	ADDQ R14, AX
	Q6KPANELSON(Y6, AX, (R11)(R13*2), Y2)
	ADDQ R14, AX
	LEAQ (R11)(R13*2), R10
	ADDQ R13, R10
	Q6KPANELSON(Y7, AX, (R10), Y3)

	MOVL         $0x00010001, AX
	VMOVD        AX, X14
	VPBROADCASTD X14, Y14
	ADDQ         $800, R9
	ADDQ         $4, R11
	ADDQ         $32, R12
	DECQ         CX
	JNZ          q6kaBlok

q6kaSon:
	MOVQ       out+48(FP), R8
	VMOVUPS    Y0, 0(R8)
	VMOVUPS    Y1, 32(R8)
	VMOVUPS    Y2, 64(R8)
	VMOVUPS    Y3, 96(R8)
	VZEROUPPER
	RET
