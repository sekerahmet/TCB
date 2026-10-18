//go:build amd64.v3

#include "textflag.h"

// func sgemm6x16(kb int, a *float32, rs, ks int, bp, c *float32, ldc int)
//
// C[0:6, 0:16] += Σ_k A(0:6, k) · B[k, 0:16]
// A(r, k) = a[r*rs + k*ks] (paketli: rs=1, ks=6; satır öncelikli: rs=lda, ks=1;
// devrik: rs=1, ks=M). B paketli (kb×16), C satır adımı ldc (hepsi eleman).
//
// Y4..Y15: biriktiriciler (satır r için Y(4+2r) sütun 0-7, Y(5+2r) sütun 8-15)
// Y0, Y1: B satırı; Y2, Y3: yayınlanan A elemanları.
// SI: A satır 0, R11: A satır 3 (= SI + 3·rs); satır 1, 2, 4, 5 R10 = rs ile
// adreslenir. Her k adımında SI ve R11, R12 = ks bayt ilerler.
TEXT ·sgemm6x16(SB), NOSPLIT, $0-56
	MOVQ kb+0(FP), CX
	MOVQ a+8(FP), SI
	MOVQ rs+16(FP), R10
	MOVQ ks+24(FP), R12
	MOVQ bp+32(FP), DI
	MOVQ c+40(FP), DX
	MOVQ ldc+48(FP), R8
	SHLQ $2, R10
	SHLQ $2, R12
	SHLQ $2, R8
	LEAQ (R10)(R10*2), R11
	ADDQ SI, R11

	VXORPS Y4, Y4, Y4
	VXORPS Y5, Y5, Y5
	VXORPS Y6, Y6, Y6
	VXORPS Y7, Y7, Y7
	VXORPS Y8, Y8, Y8
	VXORPS Y9, Y9, Y9
	VXORPS Y10, Y10, Y10
	VXORPS Y11, Y11, Y11
	VXORPS Y12, Y12, Y12
	VXORPS Y13, Y13, Y13
	VXORPS Y14, Y14, Y14
	VXORPS Y15, Y15, Y15

	TESTQ CX, CX
	JZ    yaz

dongu:
	VMOVUPS      (DI), Y0
	VMOVUPS      32(DI), Y1
	VBROADCASTSS (SI), Y2
	VBROADCASTSS (SI)(R10*1), Y3
	VFMADD231PS  Y0, Y2, Y4
	VFMADD231PS  Y1, Y2, Y5
	VFMADD231PS  Y0, Y3, Y6
	VFMADD231PS  Y1, Y3, Y7
	VBROADCASTSS (SI)(R10*2), Y2
	VBROADCASTSS (R11), Y3
	VFMADD231PS  Y0, Y2, Y8
	VFMADD231PS  Y1, Y2, Y9
	VFMADD231PS  Y0, Y3, Y10
	VFMADD231PS  Y1, Y3, Y11
	VBROADCASTSS (R11)(R10*1), Y2
	VBROADCASTSS (R11)(R10*2), Y3
	VFMADD231PS  Y0, Y2, Y12
	VFMADD231PS  Y1, Y2, Y13
	VFMADD231PS  Y0, Y3, Y14
	VFMADD231PS  Y1, Y3, Y15
	ADDQ         R12, SI
	ADDQ         R12, R11
	ADDQ         $64, DI
	DECQ         CX
	JNZ          dongu

yaz:
	VADDPS  (DX), Y4, Y4
	VADDPS  32(DX), Y5, Y5
	VMOVUPS Y4, (DX)
	VMOVUPS Y5, 32(DX)
	ADDQ    R8, DX
	VADDPS  (DX), Y6, Y6
	VADDPS  32(DX), Y7, Y7
	VMOVUPS Y6, (DX)
	VMOVUPS Y7, 32(DX)
	ADDQ    R8, DX
	VADDPS  (DX), Y8, Y8
	VADDPS  32(DX), Y9, Y9
	VMOVUPS Y8, (DX)
	VMOVUPS Y9, 32(DX)
	ADDQ    R8, DX
	VADDPS  (DX), Y10, Y10
	VADDPS  32(DX), Y11, Y11
	VMOVUPS Y10, (DX)
	VMOVUPS Y11, 32(DX)
	ADDQ    R8, DX
	VADDPS  (DX), Y12, Y12
	VADDPS  32(DX), Y13, Y13
	VMOVUPS Y12, (DX)
	VMOVUPS Y13, 32(DX)
	ADDQ    R8, DX
	VADDPS  (DX), Y14, Y14
	VADDPS  32(DX), Y15, Y15
	VMOVUPS Y14, (DX)
	VMOVUPS Y15, 32(DX)
	VZEROUPPER
	RET
