#include "textflag.h"

// Q4_0 ağırlık × 32'lik Q8 satır (dotQ4Q8Go'nun AVX2 eşleri). Blok 18 bayt: fp16 ölçek, 16 bayt kod; j. baytın alt
// yarısı j., üst yarısı j+16. öğedir. Y4: 0x0F baytları, Y5: 8 baytları, Y6: 1 int16'ları.

// Q4Q8SABIT sabitleri kurar.
#define Q4Q8SABIT \
	MOVL         $0x0F0F0F0F, AX; \
	VMOVD        AX, X4; \
	VPBROADCASTD X4, Y4; \
	MOVL         $0x08080808, AX; \
	VMOVD        AX, X5; \
	VPBROADCASTD X5, Y5; \
	MOVL         $0x00010001, AX; \
	VMOVD        AX, X6; \
	VPBROADCASTD X6, Y6

// Q4Q8KOD DI'deki bloğu açar: X10 ölçek (float32), Y2 işaretli kodlar (kod − 8, öğe sırasıyla), Y7 |kod − 8|.
#define Q4Q8KOD \
	MOVWLZX     (DI), AX; \
	VMOVD       AX, X10; \
	VCVTPH2PS   X10, X10; \
	VMOVDQU     2(DI), X2; \
	VPSRLW      $4, X2, X3; \
	VPAND       X4, X2, X2; \
	VPAND       X4, X3, X3; \
	VINSERTI128 $1, X3, Y2, Y2; \
	VPSUBB      Y5, Y2, Y2; \
	VPSIGNB     Y2, Y2, Y7

// Q4Q8SATIR bir satırın bloğunu acc'ye ekler: isum = Σ |w|·(x·işaret(w)) = Σ (kod − 8)·x tamsayıda (ikişer
// int16'ya, sonra int32'ye ve yatay), blok başına bir kez float32'ye; t = ölçek·d8, acc += t·isum (dotQ4Q8Go'nun
// sırası, kaynaştırma yok).
#define Q4Q8SATIR(xq, dm, acc) \
	VMOVDQU      xq, Y8; \
	VPSIGNB      Y2, Y8, Y8; \
	VPMADDUBSW   Y8, Y7, Y8; \
	VPMADDWD     Y6, Y8, Y8; \
	VEXTRACTI128 $1, Y8, X9; \
	VPADDD       X9, X8, X8; \
	VPSHUFD      $0x4E, X8, X9; \
	VPADDD       X9, X8, X8; \
	VPSHUFD      $0xB1, X8, X9; \
	VPADDD       X9, X8, X8; \
	VCVTDQ2PS    X8, X8; \
	VMULSS       dm, X10, X9; \
	VMULSS       X8, X9, X9; \
	VADDSS       X9, acc, acc

// func sdotq4q8(blocks int, w *byte, q *int8, d *float32) float32
TEXT ·sdotq4q8(SB), NOSPLIT, $0-36
	MOVQ   blocks+0(FP), CX
	MOVQ   w+8(FP), DI
	MOVQ   q+16(FP), SI
	MOVQ   d+24(FP), R11
	Q4Q8SABIT
	VXORPS X0, X0, X0
	TESTQ  CX, CX
	JZ     q4q8son

q4q8blok:
	Q4Q8KOD
	Q4Q8SATIR((SI), (R11), X0)
	ADDQ $18, DI
	ADDQ $32, SI
	ADDQ $4, R11
	DECQ CX
	JNZ  q4q8blok

q4q8son:
	VMOVSS     X0, ret+32(FP)
	VZEROUPPER
	RET

// func sdotq4q8x4(blocks int, w *byte, q *int8, d *float32, out *float32, sq, sd int)
//
// Dört Q8 satırı aynı Q4_0 satırıyla: blok bir kez açılır. Satır r'nin kodları q + r·sq, ölçekleri d + r·sd
// bayttadır; out[r] r'inci satırın sonucudur (yazılır, eklenmez), sdotq4q8'inkiyle bit bit aynı.
TEXT ·sdotq4q8x4(SB), NOSPLIT, $0-56
	MOVQ   blocks+0(FP), CX
	MOVQ   w+8(FP), DI
	MOVQ   q+16(FP), SI
	MOVQ   d+24(FP), R11
	MOVQ   out+32(FP), R8
	MOVQ   sq+40(FP), BX
	MOVQ   sd+48(FP), R12
	LEAQ   (BX)(BX*2), DX
	LEAQ   (R12)(R12*2), R13
	Q4Q8SABIT
	VXORPS X0, X0, X0
	VXORPS X1, X1, X1
	VXORPS X12, X12, X12
	VXORPS X13, X13, X13
	TESTQ  CX, CX
	JZ     q4q8x4son

q4q8x4blok:
	Q4Q8KOD
	Q4Q8SATIR((SI), (R11), X0)
	Q4Q8SATIR((SI)(BX*1), (R11)(R12*1), X1)
	Q4Q8SATIR((SI)(BX*2), (R11)(R12*2), X12)
	Q4Q8SATIR((SI)(DX*1), (R11)(R13*1), X13)
	ADDQ $18, DI
	ADDQ $32, SI
	ADDQ $4, R11
	DECQ CX
	JNZ  q4q8x4blok

q4q8x4son:
	VMOVSS X0, (R8)
	VMOVSS X1, 4(R8)
	VMOVSS X12, 8(R8)
	VMOVSS X13, 12(R8)
	VZEROUPPER
	RET

// Q4Q8X8SATIR bir satırın bloğundaki sekiz int32 kısmi toplamı r'ye yazar: |w|·(x·işaret(w)), ikişer int16, int32.
#define Q4Q8X8SATIR(xq, r) \
	VMOVDQU    xq, r; \
	VPSIGNB    Y2, r, r; \
	VPMADDUBSW r, Y7, r; \
	VPMADDWD   Y6, r, r

// func sdotq4q8x8(blocks int, w *byte, q *int8, dT *float32, out *float32, sq int)
//
// Sekiz Q8 satırını (kodlar sq bayt arayla) aynı Q4_0 satırıyla çarpar: blok bir kez açılır; sekiz satırın isum'u
// tek vektöre iner (şerit r = r'inci satırın sekiz int32'sinin toplamı; tamsayı, sırası serbest; eşlemesi
// sdotq4kq8x8'inki), float adımları şeritte dotQ4Q8Go'nun sırasıyla: t = ölçek·d8, acc += t·isum (kaynaştırma
// yok). dT blok başına sekiz satırın d8'i (32 bayt, q8x8Scales). out[r] r'inci satırın sonucudur (yazılır).
TEXT ·sdotq4q8x8(SB), NOSPLIT, $0-48
	MOVQ   blocks+0(FP), CX
	MOVQ   w+8(FP), DI
	MOVQ   q+16(FP), SI
	MOVQ   dT+24(FP), R11
	MOVQ   out+32(FP), R8
	MOVQ   sq+40(FP), R13
	Q4Q8SABIT
	VXORPS Y15, Y15, Y15
	TESTQ  CX, CX
	JZ     q4q8x8son

q4q8x8blok:
	Q4Q8KOD
	VBROADCASTSS X10, Y3
	LEAQ         (SI)(R13*2), R14
	ADDQ         R13, R14
	LEAQ         (R14)(R13*2), AX
	ADDQ         R13, AX
	Q4Q8X8SATIR((SI), Y0)
	Q4Q8X8SATIR((SI)(R13*1), Y1)
	Q4Q8X8SATIR((SI)(R13*2), Y9)
	Q4Q8X8SATIR((R14), Y10)
	Q4Q8X8SATIR((SI)(R13*4), Y11)
	Q4Q8X8SATIR((R14)(R13*2), Y12)
	Q4Q8X8SATIR((AX), Y13)
	Q4Q8X8SATIR((AX)(R13*1), Y14)
	VPHADDD      Y1, Y0, Y0
	VPHADDD      Y10, Y9, Y9
	VPHADDD      Y12, Y11, Y11
	VPHADDD      Y14, Y13, Y13
	VPHADDD      Y9, Y0, Y0
	VPHADDD      Y13, Y11, Y11
	VPERM2I128   $0x20, Y11, Y0, Y13
	VPERM2I128   $0x31, Y11, Y0, Y14
	VPADDD       Y14, Y13, Y13
	VCVTDQ2PS    Y13, Y13
	VMULPS       (R11), Y3, Y8
	VMULPS       Y13, Y8, Y8
	VADDPS       Y8, Y15, Y15
	ADDQ         $18, DI
	ADDQ         $32, SI
	ADDQ         $32, R11
	DECQ         CX
	JNZ          q4q8x8blok

q4q8x8son:
	VMOVUPS Y15, (R8)
	VZEROUPPER
	RET
