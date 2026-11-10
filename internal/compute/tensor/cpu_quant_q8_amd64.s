#include "textflag.h"

// X4SATIR bir satırın blok sonucunu out'a ekler: isum (yacc'nin 8 int32'si) toplanır; t = d·isum − dmin·msum,
// t·d8, out += t (dotQ4KQ8Go'nun sırası). X14 d, X15 dmin; msum moff(DX)'te, d8 (R11)(r·sd)'de, out ooff(R8)'de.
#define X4SATIR(yacc, xacc, moff, ooff, r) \
	VEXTRACTI128 $1, yacc, X2; \
	VPADDD       X2, xacc, X2; \
	VPSHUFD      $0x4E, X2, X3; \
	VPADDD       X3, X2, X2; \
	VPSHUFD      $0xB1, X2, X3; \
	VPADDD       X3, X2, X2; \
	VCVTDQ2PS    X2, X2; \
	VMULSS       X2, X14, X2; \
	VMOVD        moff(DX), X3; \
	VCVTDQ2PS    X3, X3; \
	VMULSS       X3, X15, X3; \
	VSUBSS       X3, X2, X2; \
	MOVQ         sd+64(FP), AX; \
	IMULQ        $r, AX; \
	VMULSS       (R11)(AX*1), X2, X2; \
	VADDSS       ooff(R8), X2, X2; \
	VMOVSS       X2, ooff(R8)

// func sdotq4kq8(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte) float32
//
// Q4_K blok 144 bayt (d, dmin, 12 bayt ölçek, 128 bayt kod); Q8 satırı blok başına 256 kod, bir d, 16 alt toplam.
// Alt blok j (32 öğe): ölçek sc_j ve minimum m_j; tamsayı isum = Σ sc_j·Σ q4·q8, msum = Σ m_j·Σ q8.
// Float sırası dotQ4KQ8Go'nunki: t = d·isum − dmin·msum; acc += t·d8 (kaynaştırma yok).
TEXT ·sdotq4kq8(SB), NOSPLIT, $0-52
	MOVQ         blocks+0(FP), CX
	MOVQ         w+8(FP), DI
	MOVQ         q+16(FP), SI
	MOVQ         d+24(FP), R11
	MOVQ         bs+32(FP), R12
	MOVQ         tmp+40(FP), DX
	VXORPS       X0, X0, X0
	MOVL         $0x0F0F0F0F, AX
	VMOVD        AX, X4
	VPBROADCASTD X4, Y4
	TESTQ        CX, CX
	JZ           q8son

q8blok:
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
	VPMOVZXBW    X9, X9
	VPMOVZXBW    X10, X10
	VMOVDQU      X9, (DX)
	VMOVDQU      (R12), X11
	VMOVDQU      16(R12), X12
	VPHADDW      X12, X11, X11
	VPMADDWD     X11, X10, X10
	VPSHUFD      $0x4E, X10, X13
	VPADDD       X13, X10, X10
	VPSHUFD      $0xB1, X10, X13
	VPADDD       X13, X10, X10
	VMOVD        X10, R13
	VPXOR        Y1, Y1, Y1
	ADDQ         $16, DI
	XORQ         BX, BX
	MOVQ         $4, R9

q8grup:
	VMOVDQU      (DI), Y2
	VPSRLW       $4, Y2, Y3
	VPAND        Y4, Y2, Y2
	VPAND        Y4, Y3, Y3
	VMOVDQU      (SI), Y5
	VMOVDQU      32(SI), Y6
	VPMADDUBSW   Y5, Y2, Y2
	VPMADDUBSW   Y6, Y3, Y3
	VPBROADCASTW (DX)(BX*1), Y7
	VPBROADCASTW 2(DX)(BX*1), Y8
	VPMADDWD     Y7, Y2, Y2
	VPMADDWD     Y8, Y3, Y3
	VPADDD       Y2, Y1, Y1
	VPADDD       Y3, Y1, Y1
	ADDQ         $32, DI
	ADDQ         $64, SI
	ADDQ         $4, BX
	DECQ         R9
	JNZ          q8grup

	VEXTRACTI128 $1, Y1, X2
	VPADDD       X2, X1, X1
	VPSHUFD      $0x4E, X1, X2
	VPADDD       X2, X1, X1
	VPSHUFD      $0xB1, X1, X2
	VPADDD       X2, X1, X1
	VCVTDQ2PS    X1, X1
	MOVWLZX      -144(DI), AX
	VMOVD        AX, X2
	VCVTPH2PS    X2, X2
	MOVWLZX      -142(DI), AX
	VMOVD        AX, X3
	VCVTPH2PS    X3, X3
	VMULSS       X1, X2, X2
	VMOVD        R13, X5
	VCVTDQ2PS    X5, X5
	VMULSS       X5, X3, X3
	VSUBSS       X3, X2, X2
	VMULSS       (R11), X2, X2
	VADDSS       X2, X0, X0
	ADDQ         $4, R11
	ADDQ         $32, R12
	DECQ         CX
	JNZ          q8blok

q8son:
	VMOVSS       X0, ret+48(FP)
	VZEROUPPER
	RET

// func sdotq4kq8x4(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte, out *float32, sq, sd, sb int)
//
// Dört Q8 satırı aynı Q4_K satırıyla: kodlar blok başına bir kez yüklenip açılır. Satır r'nin q'su q + r·sq,
// d'si d + r·sd, bs'si bs + r·sb bayttadır; tmp 32 bayt. Her bloğun t·d8'i blok sırasıyla doğrudan out[r]'ye
// eklenir: sonuç sdotq4kq8'inkiyle bit bit aynıdır yalnız out sıfırla başlarsa; sarmalayıcı (dotQ4KQ8x4) out'u
// önce sıfırlar.
TEXT ·sdotq4kq8x4(SB), NOSPLIT, $0-80
	MOVQ         blocks+0(FP), CX
	MOVQ         w+8(FP), DI
	MOVQ         q+16(FP), SI
	MOVQ         d+24(FP), R11
	MOVQ         bs+32(FP), R12
	MOVQ         tmp+40(FP), DX
	MOVQ         sq+56(FP), R13
	MOVL         $0x0F0F0F0F, AX
	VMOVD        AX, X4
	VPBROADCASTD X4, Y4
	TESTQ        CX, CX
	JZ           x4son

x4blok:
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
	VPMOVZXBW    X9, X9
	VPMOVZXBW    X10, X10
	VMOVDQU      X9, (DX)
	MOVQ         out+48(FP), R8
	MOVQ         R12, BX
	XORQ         R9, R9

x4msum:
	VMOVDQU      (BX), X11
	VMOVDQU      16(BX), X12
	VPHADDW      X12, X11, X11
	VPMADDWD     X11, X10, X11
	VPSHUFD      $0x4E, X11, X13
	VPADDD       X13, X11, X11
	VPSHUFD      $0xB1, X11, X13
	VPADDD       X13, X11, X11
	VMOVD        X11, 16(DX)(R9*4)
	ADDQ         sb+72(FP), BX
	INCQ         R9
	CMPQ         R9, $4
	JNE          x4msum
	VPXOR        Y0, Y0, Y0
	VPXOR        Y1, Y1, Y1
	VPXOR        Y11, Y11, Y11
	VPXOR        Y12, Y12, Y12
	ADDQ         $16, DI
	XORQ         BX, BX
	MOVQ         SI, R10
	MOVQ         $4, R9

x4grup:
	VMOVDQU      (DI), Y2
	VPSRLW       $4, Y2, Y3
	VPAND        Y4, Y2, Y2
	VPAND        Y4, Y3, Y3
	VPBROADCASTW (DX)(BX*1), Y7
	VPBROADCASTW 2(DX)(BX*1), Y8
	MOVQ         R10, AX
	VMOVDQU      (AX), Y5
	VMOVDQU      32(AX), Y6
	VPMADDUBSW   Y5, Y2, Y5
	VPMADDUBSW   Y6, Y3, Y6
	VPMADDWD     Y7, Y5, Y5
	VPMADDWD     Y8, Y6, Y6
	VPADDD       Y5, Y0, Y0
	VPADDD       Y6, Y0, Y0
	ADDQ         R13, AX
	VMOVDQU      (AX), Y5
	VMOVDQU      32(AX), Y6
	VPMADDUBSW   Y5, Y2, Y5
	VPMADDUBSW   Y6, Y3, Y6
	VPMADDWD     Y7, Y5, Y5
	VPMADDWD     Y8, Y6, Y6
	VPADDD       Y5, Y1, Y1
	VPADDD       Y6, Y1, Y1
	ADDQ         R13, AX
	VMOVDQU      (AX), Y5
	VMOVDQU      32(AX), Y6
	VPMADDUBSW   Y5, Y2, Y5
	VPMADDUBSW   Y6, Y3, Y6
	VPMADDWD     Y7, Y5, Y5
	VPMADDWD     Y8, Y6, Y6
	VPADDD       Y5, Y11, Y11
	VPADDD       Y6, Y11, Y11
	ADDQ         R13, AX
	VMOVDQU      (AX), Y5
	VMOVDQU      32(AX), Y6
	VPMADDUBSW   Y5, Y2, Y5
	VPMADDUBSW   Y6, Y3, Y6
	VPMADDWD     Y7, Y5, Y5
	VPMADDWD     Y8, Y6, Y6
	VPADDD       Y5, Y12, Y12
	VPADDD       Y6, Y12, Y12
	ADDQ         $32, DI
	ADDQ         $64, R10
	ADDQ         $4, BX
	DECQ         R9
	JNZ          x4grup

	MOVWLZX      -144(DI), AX
	VMOVD        AX, X14
	VCVTPH2PS    X14, X14
	MOVWLZX      -142(DI), AX
	VMOVD        AX, X15
	VCVTPH2PS    X15, X15
	X4SATIR(Y0, X0, 16, 0, 0)
	X4SATIR(Y1, X1, 20, 4, 1)
	X4SATIR(Y11, X11, 24, 8, 2)
	X4SATIR(Y12, X12, 28, 12, 3)
	ADDQ         $256, SI
	ADDQ         $4, R11
	ADDQ         $32, R12
	DECQ         CX
	JNZ          x4blok

x4son:
	VZEROUPPER
	RET

// func sdotq6kq8(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte) float32
//
// Q6_K blok 210 bayt (ql 128, qh 64, 16 int8 ölçek, d). Yarım blok 128 öğe: dört 32'lik parça, her parça iki
// ölçek (16'şar öğe). Tamsayı isum = Σ sc_j·Σ q6·q8, msum = Σ sc_j·bs_j; t = d·(isum − 32·msum), acc += t·d8.
TEXT ·sdotq6kq8(SB), NOSPLIT, $0-52
	MOVQ         blocks+0(FP), CX
	MOVQ         w+8(FP), DI
	MOVQ         q+16(FP), SI
	MOVQ         d+24(FP), R11
	MOVQ         bs+32(FP), R12
	MOVQ         tmp+40(FP), DX
	VXORPS       X0, X0, X0
	MOVL         $0x0F0F0F0F, AX
	VMOVD        AX, X4
	VPBROADCASTD X4, Y4
	MOVL         $0x30303030, AX
	VMOVD        AX, X9
	VPBROADCASTD X9, Y9
	TESTQ        CX, CX
	JZ           q6son

q6blok:
	VPMOVSXBW    192(DI), Y7
	VMOVDQU      Y7, (DX)
	VPMADDWD     (R12), Y7, Y7
	VEXTRACTI128 $1, Y7, X8
	VPADDD       X8, X7, X7
	VPSHUFD      $0x4E, X7, X8
	VPADDD       X8, X7, X7
	VPSHUFD      $0xB1, X7, X8
	VPADDD       X8, X7, X7
	VMOVD        X7, R13
	VPXOR        Y1, Y1, Y1
	VMOVDQU      0(DI), Y2
	VMOVDQU      32(DI), Y3
	VMOVDQU      128(DI), Y10
	VPAND        Y4, Y2, Y5
	VPSLLW       $4, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VMOVDQU      0(SI), Y11
	VPMADDUBSW   Y11, Y5, Y5
	VPBROADCASTW 0(DX), X7
	VPBROADCASTW 2(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDWD     Y7, Y5, Y5
	VPADDD       Y5, Y1, Y1
	VPAND        Y4, Y3, Y5
	VPSLLW       $2, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VMOVDQU      32(SI), Y11
	VPMADDUBSW   Y11, Y5, Y5
	VPBROADCASTW 4(DX), X7
	VPBROADCASTW 6(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDWD     Y7, Y5, Y5
	VPADDD       Y5, Y1, Y1
	VPSRLW       $4, Y2, Y5
	VPAND        Y4, Y5, Y5
	VMOVDQA      Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VMOVDQU      64(SI), Y11
	VPMADDUBSW   Y11, Y5, Y5
	VPBROADCASTW 8(DX), X7
	VPBROADCASTW 10(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDWD     Y7, Y5, Y5
	VPADDD       Y5, Y1, Y1
	VPSRLW       $4, Y3, Y5
	VPAND        Y4, Y5, Y5
	VPSRLW       $2, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VMOVDQU      96(SI), Y11
	VPMADDUBSW   Y11, Y5, Y5
	VPBROADCASTW 12(DX), X7
	VPBROADCASTW 14(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDWD     Y7, Y5, Y5
	VPADDD       Y5, Y1, Y1
	VMOVDQU      64(DI), Y2
	VMOVDQU      96(DI), Y3
	VMOVDQU      160(DI), Y10
	VPAND        Y4, Y2, Y5
	VPSLLW       $4, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VMOVDQU      128(SI), Y11
	VPMADDUBSW   Y11, Y5, Y5
	VPBROADCASTW 16(DX), X7
	VPBROADCASTW 18(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDWD     Y7, Y5, Y5
	VPADDD       Y5, Y1, Y1
	VPAND        Y4, Y3, Y5
	VPSLLW       $2, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VMOVDQU      160(SI), Y11
	VPMADDUBSW   Y11, Y5, Y5
	VPBROADCASTW 20(DX), X7
	VPBROADCASTW 22(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDWD     Y7, Y5, Y5
	VPADDD       Y5, Y1, Y1
	VPSRLW       $4, Y2, Y5
	VPAND        Y4, Y5, Y5
	VMOVDQA      Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VMOVDQU      192(SI), Y11
	VPMADDUBSW   Y11, Y5, Y5
	VPBROADCASTW 24(DX), X7
	VPBROADCASTW 26(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDWD     Y7, Y5, Y5
	VPADDD       Y5, Y1, Y1
	VPSRLW       $4, Y3, Y5
	VPAND        Y4, Y5, Y5
	VPSRLW       $2, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VMOVDQU      224(SI), Y11
	VPMADDUBSW   Y11, Y5, Y5
	VPBROADCASTW 28(DX), X7
	VPBROADCASTW 30(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDWD     Y7, Y5, Y5
	VPADDD       Y5, Y1, Y1
	VEXTRACTI128 $1, Y1, X2
	VPADDD       X2, X1, X1
	VPSHUFD      $0x4E, X1, X2
	VPADDD       X2, X1, X1
	VPSHUFD      $0xB1, X1, X2
	VPADDD       X2, X1, X1
	VMOVD        X1, AX
	SHLL         $5, R13
	SUBL         R13, AX
	VMOVD        AX, X2
	VCVTDQ2PS    X2, X2
	MOVWLZX      208(DI), AX
	VMOVD        AX, X3
	VCVTPH2PS    X3, X3
	VMULSS       X2, X3, X2
	VMULSS       (R11), X2, X2
	VADDSS       X2, X0, X0
	ADDQ         $210, DI
	ADDQ         $256, SI
	ADDQ         $4, R11
	ADDQ         $32, R12
	DECQ         CX
	JNZ          q6blok

q6son:
	VMOVSS       X0, ret+48(FP)
	VZEROUPPER
	RET

// func sdotq6kq8x4(blocks int, w *byte, q *int8, d *float32, bs *int16, tmp *byte, out *float32, sq, sd, sb int)
//
// Dört Q8 satırı aynı Q6_K satırıyla: kodlar ve ölçekler blok başına bir kez açılır. Satır r'nin q'su q + r·sq,
// d'si d + r·sd, bs'si bs + r·sb bayttadır; tmp 48 bayt (16 ölçek, dört msum). Her bloğun t·d8'i blok
// sırasıyla doğrudan out[r]'ye eklenir: sonuç sdotq6kq8'inkiyle bit bit aynıdır yalnız out sıfırla başlarsa;
// sarmalayıcı (dotQ6KQ8x4) out'u önce sıfırlar.
//
// Elle bakılır: gövde 2026-11-10'da bir kez bir üreteçle yazıldı, üreteç depoda değil. Her 32'lik parçanın
// açılışı (ql, qh, maske, kaydırma, ölçek yayını) sdotq6kq8'deki parçanınkiyle komut komut aynıdır; dört satır
// için yalnız çarpım ve toplama tekrarlanır (satır r'nin kodu q + r·sq: SI, SI+R13, SI+2·R13, SI+R10). Değişiklik
// iki çekirdekte birlikte yapılır; TestQ8FourRowsMatchOneRow iki çekirdeği bit bit karşılaştırır.
TEXT ·sdotq6kq8x4(SB), NOSPLIT, $0-80
	MOVQ         blocks+0(FP), CX
	MOVQ         w+8(FP), DI
	MOVQ         q+16(FP), SI
	MOVQ         d+24(FP), R11
	MOVQ         bs+32(FP), R12
	MOVQ         tmp+40(FP), DX
	MOVQ         out+48(FP), R8
	MOVQ         sq+56(FP), R13
	LEAQ         (R13)(R13*2), R10
	MOVL         $0x0F0F0F0F, AX
	VMOVD        AX, X4
	VPBROADCASTD X4, Y4
	MOVL         $0x30303030, AX
	VMOVD        AX, X9
	VPBROADCASTD X9, Y9
	TESTQ        CX, CX
	JZ           q6x4son

q6x4blok:
	VPMOVSXBW    192(DI), Y6
	VMOVDQU      Y6, (DX)
	MOVQ         R12, BX
	VPMADDWD     (BX), Y6, Y11
	VEXTRACTI128 $1, Y11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0x4E, X11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0xB1, X11, X15
	VPADDD       X15, X11, X11
	VMOVD        X11, 32(DX)
	ADDQ         sb+72(FP), BX
	VPMADDWD     (BX), Y6, Y11
	VEXTRACTI128 $1, Y11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0x4E, X11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0xB1, X11, X15
	VPADDD       X15, X11, X11
	VMOVD        X11, 36(DX)
	ADDQ         sb+72(FP), BX
	VPMADDWD     (BX), Y6, Y11
	VEXTRACTI128 $1, Y11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0x4E, X11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0xB1, X11, X15
	VPADDD       X15, X11, X11
	VMOVD        X11, 40(DX)
	ADDQ         sb+72(FP), BX
	VPMADDWD     (BX), Y6, Y11
	VEXTRACTI128 $1, Y11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0x4E, X11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0xB1, X11, X15
	VPADDD       X15, X11, X11
	VMOVD        X11, 44(DX)
	VPXOR        Y0, Y0, Y0
	VPXOR        Y1, Y1, Y1
	VPXOR        Y12, Y12, Y12
	VPXOR        Y13, Y13, Y13
	VMOVDQU      0(DI), Y2
	VMOVDQU      32(DI), Y3
	VMOVDQU      128(DI), Y10
	VPAND        Y4, Y2, Y5
	VPSLLW       $4, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VPBROADCASTW 0(DX), X7
	VPBROADCASTW 2(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDUBSW   0(SI), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y0, Y0
	VPMADDUBSW   0(SI)(R13*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y1, Y1
	VPMADDUBSW   0(SI)(R13*2), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y12, Y12
	VPMADDUBSW   0(SI)(R10*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y13, Y13
	VPAND        Y4, Y3, Y5
	VPSLLW       $2, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VPBROADCASTW 4(DX), X7
	VPBROADCASTW 6(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDUBSW   32(SI), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y0, Y0
	VPMADDUBSW   32(SI)(R13*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y1, Y1
	VPMADDUBSW   32(SI)(R13*2), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y12, Y12
	VPMADDUBSW   32(SI)(R10*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y13, Y13
	VPSRLW       $4, Y2, Y5
	VPAND        Y4, Y5, Y5
	VMOVDQA      Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VPBROADCASTW 8(DX), X7
	VPBROADCASTW 10(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDUBSW   64(SI), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y0, Y0
	VPMADDUBSW   64(SI)(R13*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y1, Y1
	VPMADDUBSW   64(SI)(R13*2), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y12, Y12
	VPMADDUBSW   64(SI)(R10*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y13, Y13
	VPSRLW       $4, Y3, Y5
	VPAND        Y4, Y5, Y5
	VPSRLW       $2, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VPBROADCASTW 12(DX), X7
	VPBROADCASTW 14(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDUBSW   96(SI), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y0, Y0
	VPMADDUBSW   96(SI)(R13*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y1, Y1
	VPMADDUBSW   96(SI)(R13*2), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y12, Y12
	VPMADDUBSW   96(SI)(R10*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y13, Y13
	VMOVDQU      64(DI), Y2
	VMOVDQU      96(DI), Y3
	VMOVDQU      160(DI), Y10
	VPAND        Y4, Y2, Y5
	VPSLLW       $4, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VPBROADCASTW 16(DX), X7
	VPBROADCASTW 18(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDUBSW   128(SI), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y0, Y0
	VPMADDUBSW   128(SI)(R13*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y1, Y1
	VPMADDUBSW   128(SI)(R13*2), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y12, Y12
	VPMADDUBSW   128(SI)(R10*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y13, Y13
	VPAND        Y4, Y3, Y5
	VPSLLW       $2, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VPBROADCASTW 20(DX), X7
	VPBROADCASTW 22(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDUBSW   160(SI), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y0, Y0
	VPMADDUBSW   160(SI)(R13*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y1, Y1
	VPMADDUBSW   160(SI)(R13*2), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y12, Y12
	VPMADDUBSW   160(SI)(R10*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y13, Y13
	VPSRLW       $4, Y2, Y5
	VPAND        Y4, Y5, Y5
	VMOVDQA      Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VPBROADCASTW 24(DX), X7
	VPBROADCASTW 26(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDUBSW   192(SI), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y0, Y0
	VPMADDUBSW   192(SI)(R13*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y1, Y1
	VPMADDUBSW   192(SI)(R13*2), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y12, Y12
	VPMADDUBSW   192(SI)(R10*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y13, Y13
	VPSRLW       $4, Y3, Y5
	VPAND        Y4, Y5, Y5
	VPSRLW       $2, Y10, Y6
	VPAND        Y9, Y6, Y6
	VPOR         Y6, Y5, Y5
	VPBROADCASTW 28(DX), X7
	VPBROADCASTW 30(DX), X8
	VINSERTI128  $1, X8, Y7, Y7
	VPMADDUBSW   224(SI), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y0, Y0
	VPMADDUBSW   224(SI)(R13*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y1, Y1
	VPMADDUBSW   224(SI)(R13*2), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y12, Y12
	VPMADDUBSW   224(SI)(R10*1), Y5, Y6
	VPMADDWD     Y7, Y6, Y6
	VPADDD       Y6, Y13, Y13
	MOVWLZX      208(DI), AX
	VMOVD        AX, X14
	VCVTPH2PS    X14, X14
	VEXTRACTI128 $1, Y0, X11
	VPADDD       X11, X0, X11
	VPSHUFD      $0x4E, X11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0xB1, X11, X15
	VPADDD       X15, X11, X11
	VMOVD        X11, AX
	MOVL         32(DX), BX
	SHLL         $5, BX
	SUBL         BX, AX
	VMOVD        AX, X11
	VCVTDQ2PS    X11, X11
	VMULSS       X11, X14, X11
	VMULSS       (R11), X11, X11
	VADDSS       0(R8), X11, X11
	VMOVSS       X11, 0(R8)
	VEXTRACTI128 $1, Y1, X11
	VPADDD       X11, X1, X11
	VPSHUFD      $0x4E, X11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0xB1, X11, X15
	VPADDD       X15, X11, X11
	VMOVD        X11, AX
	MOVL         36(DX), BX
	SHLL         $5, BX
	SUBL         BX, AX
	VMOVD        AX, X11
	VCVTDQ2PS    X11, X11
	VMULSS       X11, X14, X11
	MOVQ         sd+64(FP), BX
	IMULQ        $1, BX
	VMULSS       (R11)(BX*1), X11, X11
	VADDSS       4(R8), X11, X11
	VMOVSS       X11, 4(R8)
	VEXTRACTI128 $1, Y12, X11
	VPADDD       X11, X12, X11
	VPSHUFD      $0x4E, X11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0xB1, X11, X15
	VPADDD       X15, X11, X11
	VMOVD        X11, AX
	MOVL         40(DX), BX
	SHLL         $5, BX
	SUBL         BX, AX
	VMOVD        AX, X11
	VCVTDQ2PS    X11, X11
	VMULSS       X11, X14, X11
	MOVQ         sd+64(FP), BX
	IMULQ        $2, BX
	VMULSS       (R11)(BX*1), X11, X11
	VADDSS       8(R8), X11, X11
	VMOVSS       X11, 8(R8)
	VEXTRACTI128 $1, Y13, X11
	VPADDD       X11, X13, X11
	VPSHUFD      $0x4E, X11, X15
	VPADDD       X15, X11, X11
	VPSHUFD      $0xB1, X11, X15
	VPADDD       X15, X11, X11
	VMOVD        X11, AX
	MOVL         44(DX), BX
	SHLL         $5, BX
	SUBL         BX, AX
	VMOVD        AX, X11
	VCVTDQ2PS    X11, X11
	VMULSS       X11, X14, X11
	MOVQ         sd+64(FP), BX
	IMULQ        $3, BX
	VMULSS       (R11)(BX*1), X11, X11
	VADDSS       12(R8), X11, X11
	VMOVSS       X11, 12(R8)
	ADDQ         $210, DI
	ADDQ         $256, SI
	ADDQ         $4, R11
	ADDQ         $32, R12
	DECQ         CX
	JNZ          q6x4blok

q6x4son:
	VZEROUPPER
	RET
