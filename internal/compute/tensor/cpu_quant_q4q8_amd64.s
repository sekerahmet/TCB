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

// Q4Q8GBLOK sekizlinin b'inci bloğu: kodları (0–15, kod − 8 değil) Y2'ye açar, ölçeğin fp16'sını X3'ün b'inci
// kelimesine koyar, Σ kod·x'in sekiz int32 kısmi toplamını r'ye yazar (X1 geçici; −8·Σx sonra eklenir).
#define Q4Q8GBLOK(b, r) \
	VMOVDQU     b*18+2(DI), X2; \
	VPSRLW      $4, X2, X1; \
	VPAND       X4, X2, X2; \
	VPAND       X4, X1, X1; \
	VINSERTI128 $1, X1, Y2, Y2; \
	MOVWLZX     b*18(DI), AX; \
	VPINSRW     $b, AX, X3, X3; \
	VMOVDQU     b*32(SI), r; \
	VPMADDUBSW  r, Y2, r; \
	VPMADDWD    Y6, r, r

// Q4Q8GEKLE X1'in alt öğesini acc'ye ekler.
#define Q4Q8GEKLE VADDSS X1, X0, X0

// func sdotq4q8(blocks int, w *byte, q *int8, d *float32, bs *int16) float32
//
// Bloklar sekizer işlenir: sekiz bloğun Σ kod·x kısmi toplamları tek VPHADDD ağacıyla sekiz toplama iner (şerit b =
// b'inci blok; eşleme sdotq4q8x8'inki), −8·Σx eklenir (Σx = bs'nin iki 16'lığı): isum = Σ (kod − 8)·x, tamsayıda
// kesin. Sekiz ölçek bir VCVTPH2PS'le açılır, t = ölçek·d8 ve t·isum şeritte; sonra acc += t·isum blok sırasıyla
// tek tek (dotQ4Q8Go'nun sırası, kaynaştırma yok). Kalan bloklar teker teker (işaretli kodlarla, aynı isum).
TEXT ·sdotq4q8(SB), NOSPLIT, $0-44
	MOVQ   blocks+0(FP), CX
	MOVQ   w+8(FP), DI
	MOVQ   q+16(FP), SI
	MOVQ   d+24(FP), R11
	MOVQ   bs+32(FP), R12
	Q4Q8SABIT
	VXORPS X0, X0, X0
	MOVQ   CX, DX
	SHRQ   $3, DX
	ANDQ   $7, CX
	TESTQ  DX, DX
	JZ     q4q8tek

q4q8grup:
	VPXOR      X3, X3, X3
	Q4Q8GBLOK(0, Y8)
	Q4Q8GBLOK(1, Y9)
	Q4Q8GBLOK(2, Y10)
	Q4Q8GBLOK(3, Y11)
	Q4Q8GBLOK(4, Y12)
	Q4Q8GBLOK(5, Y13)
	Q4Q8GBLOK(6, Y14)
	Q4Q8GBLOK(7, Y15)
	VPHADDD    Y9, Y8, Y8
	VPHADDD    Y11, Y10, Y10
	VPHADDD    Y13, Y12, Y12
	VPHADDD    Y15, Y14, Y14
	VPHADDD    Y10, Y8, Y8
	VPHADDD    Y14, Y12, Y12
	VPERM2I128 $0x20, Y12, Y8, Y13
	VPERM2I128 $0x31, Y12, Y8, Y14
	VPADDD     Y14, Y13, Y13
	VPMADDWD   (R12), Y6, Y14
	VPSLLD     $3, Y14, Y14
	VPSUBD     Y14, Y13, Y13
	VCVTDQ2PS  Y13, Y13
	VCVTPH2PS  X3, Y3
	VMULPS     (R11), Y3, Y3
	VMULPS     Y13, Y3, Y3
	VMOVAPS    X3, X1
	Q4Q8GEKLE
	VMOVSHDUP  X3, X1
	Q4Q8GEKLE
	VMOVHLPS   X3, X3, X1
	Q4Q8GEKLE
	VPSHUFD    $0xFF, X3, X1
	Q4Q8GEKLE
	VEXTRACTF128 $1, Y3, X3
	VMOVAPS    X3, X1
	Q4Q8GEKLE
	VMOVSHDUP  X3, X1
	Q4Q8GEKLE
	VMOVHLPS   X3, X3, X1
	Q4Q8GEKLE
	VPSHUFD    $0xFF, X3, X1
	Q4Q8GEKLE
	ADDQ $144, DI
	ADDQ $256, SI
	ADDQ $32, R11
	ADDQ $32, R12
	DECQ DX
	JNZ  q4q8grup

q4q8tek:
	TESTQ CX, CX
	JZ    q4q8son

q4q8blok:
	Q4Q8KOD
	Q4Q8SATIR((SI), (R11), X0)
	ADDQ $18, DI
	ADDQ $32, SI
	ADDQ $4, R11
	DECQ CX
	JNZ  q4q8blok

q4q8son:
	VMOVSS     X0, ret+40(FP)
	VZEROUPPER
	RET

// Q4PAKSATIR bir ağırlık satırının bloğunu açar: r'nin baytları öğe sırasıyla kodlar (0–15; kod − 8 değil),
// dword g = 4g..4g+3. öğeler. Y15: 0x0F baytları; X13, X14 geçici.
#define Q4PAKSATIR(adr, r) \
	VMOVDQU     adr, X14; \
	VPSRLW      $4, X14, X13; \
	VPAND       X15, X14, X14; \
	VPAND       X15, X13, X13; \
	VINSERTI128 $1, X13, Y14, r

// Q4PAKOLCEK satırın bloğunun fp16 ölçeğini X1'in r'inci kelimesine koyar.
#define Q4PAKOLCEK(adr, r) \
	MOVWLZX adr, AX; \
	VPINSRW $r, AX, X1, X1

// func packq4x8(blocks int, w *byte, rowBytes int, p *byte, ps *float32)
//
// Sekiz Q4_0 satırını (w'den rowBytes arayla) sekiz satırlı panele yazar: blok i'nin 256 baytında dword
// (g·8 + r) = r'inci satırın 4g..4g+3. öğelerinin kodları (0–15), ps[i·8 + r] = r'inci satırın ölçeği (float32).
// Paneli sdotq4x8q8x4 okur; panel yalnız yerleşimdir, sayıları değiştirmez.
TEXT ·packq4x8(SB), NOSPLIT, $0-40
	MOVQ blocks+0(FP), CX
	MOVQ w+8(FP), DI
	MOVQ rowBytes+16(FP), BX
	MOVQ p+24(FP), DX
	MOVQ ps+32(FP), R9
	LEAQ (DI)(BX*2), R10
	ADDQ BX, R10
	LEAQ (R10)(BX*2), R11
	ADDQ BX, R11
	MOVL         $0x0F0F0F0F, AX
	VMOVD        AX, X15
	VPBROADCASTD X15, Y15
	TESTQ        CX, CX
	JZ           pakson

pakblok:
	Q4PAKSATIR(2(DI), Y0)
	Q4PAKSATIR(2(DI)(BX*1), Y1)
	Q4PAKSATIR(2(DI)(BX*2), Y2)
	Q4PAKSATIR(2(R10), Y3)
	Q4PAKSATIR(2(DI)(BX*4), Y4)
	Q4PAKSATIR(2(R10)(BX*2), Y5)
	Q4PAKSATIR(2(R11), Y6)
	Q4PAKSATIR(2(R11)(BX*1), Y7)
	// 8×8 dword çevirme: satır r'nin dword g'si → çıkış g'nin dword r'si.
	VPUNPCKLDQ  Y1, Y0, Y8
	VPUNPCKHDQ  Y1, Y0, Y9
	VPUNPCKLDQ  Y3, Y2, Y0
	VPUNPCKHDQ  Y3, Y2, Y1
	VPUNPCKLDQ  Y5, Y4, Y2
	VPUNPCKHDQ  Y5, Y4, Y3
	VPUNPCKLDQ  Y7, Y6, Y4
	VPUNPCKHDQ  Y7, Y6, Y5
	VPUNPCKLQDQ Y0, Y8, Y6
	VPUNPCKHQDQ Y0, Y8, Y7
	VPUNPCKLQDQ Y1, Y9, Y10
	VPUNPCKHQDQ Y1, Y9, Y11
	VPUNPCKLQDQ Y4, Y2, Y12
	VPUNPCKHQDQ Y4, Y2, Y13
	VPUNPCKLQDQ Y5, Y3, Y14
	VPUNPCKHQDQ Y5, Y3, Y0
	VPERM2I128  $0x20, Y12, Y6, Y1
	VMOVDQU     Y1, 0(DX)
	VPERM2I128  $0x20, Y13, Y7, Y1
	VMOVDQU     Y1, 32(DX)
	VPERM2I128  $0x20, Y14, Y10, Y1
	VMOVDQU     Y1, 64(DX)
	VPERM2I128  $0x20, Y0, Y11, Y1
	VMOVDQU     Y1, 96(DX)
	VPERM2I128  $0x31, Y12, Y6, Y1
	VMOVDQU     Y1, 128(DX)
	VPERM2I128  $0x31, Y13, Y7, Y1
	VMOVDQU     Y1, 160(DX)
	VPERM2I128  $0x31, Y14, Y10, Y1
	VMOVDQU     Y1, 192(DX)
	VPERM2I128  $0x31, Y0, Y11, Y1
	VMOVDQU     Y1, 224(DX)
	VPXOR       X1, X1, X1
	Q4PAKOLCEK((DI), 0)
	Q4PAKOLCEK((DI)(BX*1), 1)
	Q4PAKOLCEK((DI)(BX*2), 2)
	Q4PAKOLCEK((R10), 3)
	Q4PAKOLCEK((DI)(BX*4), 4)
	Q4PAKOLCEK((R10)(BX*2), 5)
	Q4PAKOLCEK((R11), 6)
	Q4PAKOLCEK((R11)(BX*1), 7)
	VCVTPH2PS   X1, Y1
	VMOVUPS     Y1, (R9)
	ADDQ        $18, DI
	ADDQ        $18, R10
	ADDQ        $18, R11
	ADDQ        $256, DX
	ADDQ        $32, R9
	DECQ        CX
	JNZ         pakblok

pakson:
	VZEROUPPER
	RET

// Q4X8GRUP g'inci dört öğelik grubu dört etkinleştirme satırıyla çarpar: Y8 panelin kodları (şerit j = j'inci
// ağırlık satırı), kodların dördü x'in yayımlanmış dördüyle; ikişer int16 toplam Y4–Y7'ye eklenir (taşmaz:
// 8 grup · 2 · 15 · 127 = 30 480).
#define Q4X8GRUP(g) \
	VMOVDQU      g*32(DI), Y8; \
	VPBROADCASTD g*4(SI), Y9; \
	VPMADDUBSW   Y9, Y8, Y9; \
	VPADDW       Y9, Y4, Y4; \
	VPBROADCASTD g*4(SI)(BX*1), Y10; \
	VPMADDUBSW   Y10, Y8, Y10; \
	VPADDW       Y10, Y5, Y5; \
	VPBROADCASTD g*4(SI)(BX*2), Y9; \
	VPMADDUBSW   Y9, Y8, Y9; \
	VPADDW       Y9, Y6, Y6; \
	VPBROADCASTD g*4(SI)(DX*1), Y10; \
	VPMADDUBSW   Y10, Y8, Y10; \
	VPADDW       Y10, Y7, Y7

// Q4X8SON bir etkinleştirme satırının bloğunu bitirir: isum = Σ kod·x − 8·Σx (int32, kesin; dotQ4Q8Go'nun
// Σ (kod − 8)·x'i), float32'ye bir kez; t = ölçek·d8, acc += t·isum (şeritte, kaynaştırma yok).
#define Q4X8SON(ia, c8, dx, acc) \
	VPMADDWD     Y11, ia, ia; \
	VPBROADCASTD c8, Y9; \
	VPADDD       Y9, ia, ia; \
	VCVTDQ2PS    ia, ia; \
	VBROADCASTSS dx, Y9; \
	VMULPS       Y12, Y9, Y9; \
	VMULPS       ia, Y9, Y9; \
	VADDPS       Y9, acc, acc

// func sdotq4x8q8x4(blocks int, p *byte, ps *float32, q *int8, d *float32, c8 *int32, out *float32, sq, sd int)
//
// Sekiz satırlı panel (packq4x8) × dört Q8 satırı (kodlar sq, d8'ler ve c8'ler sd bayt arayla; c8 = −8·Σx blok
// başına): out[r·8 + j] = r'inci etkinleştirme satırının j'inci ağırlık satırıyla sonucu, dotQ4Q8Go'nunkiyle bit
// bit aynı. Blok başına yatay toplama yok: şerit j bir ağırlık satırıdır.
TEXT ·sdotq4x8q8x4(SB), NOSPLIT, $0-72
	MOVQ blocks+0(FP), CX
	MOVQ p+8(FP), DI
	MOVQ ps+16(FP), R9
	MOVQ q+24(FP), SI
	MOVQ d+32(FP), R11
	MOVQ c8+40(FP), R12
	MOVQ out+48(FP), R8
	MOVQ sq+56(FP), BX
	MOVQ sd+64(FP), R13
	LEAQ (BX)(BX*2), DX
	LEAQ (R13)(R13*2), R14
	MOVL         $0x00010001, AX
	VMOVD        AX, X11
	VPBROADCASTD X11, Y11
	VXORPS       Y0, Y0, Y0
	VXORPS       Y1, Y1, Y1
	VXORPS       Y2, Y2, Y2
	VXORPS       Y3, Y3, Y3
	TESTQ        CX, CX
	JZ           x8x4son

x8x4blok:
	VPXOR   Y4, Y4, Y4
	VPXOR   Y5, Y5, Y5
	VPXOR   Y6, Y6, Y6
	VPXOR   Y7, Y7, Y7
	Q4X8GRUP(0)
	Q4X8GRUP(1)
	Q4X8GRUP(2)
	Q4X8GRUP(3)
	Q4X8GRUP(4)
	Q4X8GRUP(5)
	Q4X8GRUP(6)
	Q4X8GRUP(7)
	VMOVUPS (R9), Y12
	Q4X8SON(Y4, (R12), (R11), Y0)
	Q4X8SON(Y5, (R12)(R13*1), (R11)(R13*1), Y1)
	Q4X8SON(Y6, (R12)(R13*2), (R11)(R13*2), Y2)
	Q4X8SON(Y7, (R12)(R14*1), (R11)(R14*1), Y3)
	ADDQ    $256, DI
	ADDQ    $32, R9
	ADDQ    $32, SI
	ADDQ    $4, R11
	ADDQ    $4, R12
	DECQ    CX
	JNZ     x8x4blok

x8x4son:
	VMOVUPS Y0, (R8)
	VMOVUPS Y1, 32(R8)
	VMOVUPS Y2, 64(R8)
	VMOVUPS Y3, 96(R8)
	VZEROUPPER
	RET
