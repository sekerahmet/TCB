#include "textflag.h"

// 16 bitlik ağırlığın gemv çekirdekleri: sgemv64 ve sdot32'nin (cpu_gemv_amd64.s) eşleri. Ağırlık
// yazmaçta açılır; açma kayıpsızdır, toplama sırası ve kaynaştırılmış çarpma-toplamanın işlenenleri
// aynıdır: sonuç "aç + float32 çekirdek" ile bit düzeyinde aynıdır. Şerit çekirdekleri 8 satır
// sonrasını öncü yüklemeyle ister: her satırdan yalnız 128 bayt okunur, donanımın öncü yüklemesi
// bu adımı izlemez (bf16 şerit Xeon'da 12,5 → 23,5 GB/sn; float32 şeridi aynı öncü yüklemeyle
// hızlanmadı, sgemv64'te yoktur). Derinlik 8 ile 16 arası ölçüde aynıdır.

#define ZERO8 \
	VXORPS Y0, Y0, Y0; \
	VXORPS Y1, Y1, Y1; \
	VXORPS Y2, Y2, Y2; \
	VXORPS Y3, Y3, Y3; \
	VXORPS Y4, Y4, Y4; \
	VXORPS Y5, Y5, Y5; \
	VXORPS Y6, Y6, Y6; \
	VXORPS Y7, Y7, Y7

// y[0:64] += Y0..Y7 (DX: y)
#define STORE64 \
	VADDPS  (DX), Y0, Y0; \
	VADDPS  32(DX), Y1, Y1; \
	VADDPS  64(DX), Y2, Y2; \
	VADDPS  96(DX), Y3, Y3; \
	VADDPS  128(DX), Y4, Y4; \
	VADDPS  160(DX), Y5, Y5; \
	VADDPS  192(DX), Y6, Y6; \
	VADDPS  224(DX), Y7, Y7; \
	VMOVUPS Y0, (DX); \
	VMOVUPS Y1, 32(DX); \
	VMOVUPS Y2, 64(DX); \
	VMOVUPS Y3, 96(DX); \
	VMOVUPS Y4, 128(DX); \
	VMOVUPS Y5, 160(DX); \
	VMOVUPS Y6, 192(DX); \
	VMOVUPS Y7, 224(DX)

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

// r = float32(bfloat16 sekizlisi off(DI)): 16 bit üst yarım sözcüğe kayar.
#define BF8(off, r) \
	VPMOVZXWD off(DI), r; \
	VPSLLD    $16, r, r

// func sgemv64bf(k int, x *float32, w *uint16, ldw int, y *float32)
//
// y[0:64] += Σ_i x[i] · float32(w[i*ldw + 0:64])  (i < k; ldw eleman)
TEXT ·sgemv64bf(SB), NOSPLIT, $0-40
	MOVQ k+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ w+16(FP), DI
	MOVQ ldw+24(FP), R8
	MOVQ y+32(FP), DX
	SHLQ $1, R8
	MOVQ R8, R9
	SHLQ $3, R9
	ZERO8
	TESTQ CX, CX
	JZ    yazbf

dongubf:
	VBROADCASTSS (SI), Y8
	PREFETCHT0   (DI)(R9*1)
	PREFETCHT0   64(DI)(R9*1)
	BF8(0, Y9)
	BF8(16, Y10)
	BF8(32, Y11)
	BF8(48, Y12)
	VFMADD231PS  Y9, Y8, Y0
	VFMADD231PS  Y10, Y8, Y1
	VFMADD231PS  Y11, Y8, Y2
	VFMADD231PS  Y12, Y8, Y3
	BF8(64, Y9)
	BF8(80, Y10)
	BF8(96, Y11)
	BF8(112, Y12)
	VFMADD231PS  Y9, Y8, Y4
	VFMADD231PS  Y10, Y8, Y5
	VFMADD231PS  Y11, Y8, Y6
	VFMADD231PS  Y12, Y8, Y7
	ADDQ         $4, SI
	ADDQ         R8, DI
	DECQ         CX
	JNZ          dongubf

yazbf:
	STORE64
	VZEROUPPER
	RET

// func sgemv64h(k int, x *float32, w *uint16, ldw int, y *float32)
//
// y[0:64] += Σ_i x[i] · float32(w[i*ldw + 0:64])  (i < k; ldw eleman; w float16)
TEXT ·sgemv64h(SB), NOSPLIT, $0-40
	MOVQ k+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ w+16(FP), DI
	MOVQ ldw+24(FP), R8
	MOVQ y+32(FP), DX
	SHLQ $1, R8
	MOVQ R8, R9
	SHLQ $3, R9
	ZERO8
	TESTQ CX, CX
	JZ    yazh

dongugh:
	VBROADCASTSS (SI), Y8
	PREFETCHT0   (DI)(R9*1)
	PREFETCHT0   64(DI)(R9*1)
	VCVTPH2PS    (DI), Y9
	VCVTPH2PS    16(DI), Y10
	VCVTPH2PS    32(DI), Y11
	VCVTPH2PS    48(DI), Y12
	VFMADD231PS  Y9, Y8, Y0
	VFMADD231PS  Y10, Y8, Y1
	VFMADD231PS  Y11, Y8, Y2
	VFMADD231PS  Y12, Y8, Y3
	VCVTPH2PS    64(DI), Y9
	VCVTPH2PS    80(DI), Y10
	VCVTPH2PS    96(DI), Y11
	VCVTPH2PS    112(DI), Y12
	VFMADD231PS  Y9, Y8, Y4
	VFMADD231PS  Y10, Y8, Y5
	VFMADD231PS  Y11, Y8, Y6
	VFMADD231PS  Y12, Y8, Y7
	ADDQ         $4, SI
	ADDQ         R8, DI
	DECQ         CX
	JNZ          dongugh

yazh:
	STORE64
	VZEROUPPER
	RET

// func sdot32bf(n int, x *float32, y *uint16) float32
//
// Σ_i x[i]·float32(y[i]), i < n (n 32'nin katı).
TEXT ·sdot32bf(SB), NOSPLIT, $0-28
	MOVQ n+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ y+16(FP), DI
	VXORPS Y0, Y0, Y0
	VXORPS Y1, Y1, Y1
	VXORPS Y2, Y2, Y2
	VXORPS Y3, Y3, Y3
	SHRQ   $5, CX
	JZ     toplabf

dot32bf:
	VMOVUPS     (SI), Y4
	VMOVUPS     32(SI), Y5
	VMOVUPS     64(SI), Y6
	VMOVUPS     96(SI), Y7
	BF8(0, Y8)
	BF8(16, Y9)
	BF8(32, Y10)
	BF8(48, Y11)
	VFMADD231PS Y8, Y4, Y0
	VFMADD231PS Y9, Y5, Y1
	VFMADD231PS Y10, Y6, Y2
	VFMADD231PS Y11, Y7, Y3
	ADDQ        $128, SI
	ADDQ        $64, DI
	DECQ        CX
	JNZ         dot32bf

toplabf:
	SUM32
	VMOVSS X0, ret+24(FP)
	VZEROUPPER
	RET

// func sdot32h(n int, x *float32, y *uint16) float32
//
// Σ_i x[i]·float32(y[i]), i < n (n 32'nin katı; y float16).
TEXT ·sdot32h(SB), NOSPLIT, $0-28
	MOVQ n+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ y+16(FP), DI
	VXORPS Y0, Y0, Y0
	VXORPS Y1, Y1, Y1
	VXORPS Y2, Y2, Y2
	VXORPS Y3, Y3, Y3
	SHRQ   $5, CX
	JZ     toplah

dot32h:
	VMOVUPS     (SI), Y4
	VMOVUPS     32(SI), Y5
	VMOVUPS     64(SI), Y6
	VMOVUPS     96(SI), Y7
	VCVTPH2PS   (DI), Y8
	VCVTPH2PS   16(DI), Y9
	VCVTPH2PS   32(DI), Y10
	VCVTPH2PS   48(DI), Y11
	VFMADD231PS Y8, Y4, Y0
	VFMADD231PS Y9, Y5, Y1
	VFMADD231PS Y10, Y6, Y2
	VFMADD231PS Y11, Y7, Y3
	ADDQ        $128, SI
	ADDQ        $64, DI
	DECQ        CX
	JNZ         dot32h

toplah:
	SUM32
	VMOVSS X0, ret+24(FP)
	VZEROUPPER
	RET
