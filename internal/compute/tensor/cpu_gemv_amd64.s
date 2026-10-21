#include "textflag.h"

// func sgemv64(k int, x *float32, w *float32, ldw int, y *float32)
//
// y[0:64] += Σ_i x[i] · w[i*ldw + 0:64]   (i < k; ldw eleman; gemm gibi ekler)
//
// Y0..Y7: 64 sütunun biriktiricileri; Y8: yayınlanan x[i]. Her adımda W'nin bir satır
// parçası (256 bayt) okunur, 8 kaynaştırılmış çarpma-toplama yapılır. Toplama sırası i'ye
// göre sabittir (belirlenimci).
TEXT ·sgemv64(SB), NOSPLIT, $0-40
	MOVQ k+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ w+16(FP), DI
	MOVQ ldw+24(FP), R8
	MOVQ y+32(FP), DX
	SHLQ $2, R8

	VXORPS Y0, Y0, Y0
	VXORPS Y1, Y1, Y1
	VXORPS Y2, Y2, Y2
	VXORPS Y3, Y3, Y3
	VXORPS Y4, Y4, Y4
	VXORPS Y5, Y5, Y5
	VXORPS Y6, Y6, Y6
	VXORPS Y7, Y7, Y7

	TESTQ CX, CX
	JZ    yaz64

dongu64:
	VBROADCASTSS (SI), Y8
	VFMADD231PS  (DI), Y8, Y0
	VFMADD231PS  32(DI), Y8, Y1
	VFMADD231PS  64(DI), Y8, Y2
	VFMADD231PS  96(DI), Y8, Y3
	VFMADD231PS  128(DI), Y8, Y4
	VFMADD231PS  160(DI), Y8, Y5
	VFMADD231PS  192(DI), Y8, Y6
	VFMADD231PS  224(DI), Y8, Y7
	ADDQ         $4, SI
	ADDQ         R8, DI
	DECQ         CX
	JNZ          dongu64

yaz64:
	VADDPS  (DX), Y0, Y0
	VADDPS  32(DX), Y1, Y1
	VADDPS  64(DX), Y2, Y2
	VADDPS  96(DX), Y3, Y3
	VADDPS  128(DX), Y4, Y4
	VADDPS  160(DX), Y5, Y5
	VADDPS  192(DX), Y6, Y6
	VADDPS  224(DX), Y7, Y7
	VMOVUPS Y0, (DX)
	VMOVUPS Y1, 32(DX)
	VMOVUPS Y2, 64(DX)
	VMOVUPS Y3, 96(DX)
	VMOVUPS Y4, 128(DX)
	VMOVUPS Y5, 160(DX)
	VMOVUPS Y6, 192(DX)
	VMOVUPS Y7, 224(DX)
	VZEROUPPER
	RET

// func sdot32(n int, x *float32, y *float32) float32
//
// Σ_i x[i]·y[i], i < n (n 32'nin katı). Dört biriktirici (Y0..Y3) 8'er kulvar; sonda
// (Y0+Y1)+(Y2+Y3), sonra kulvarlar sabit sırayla toplanır.
TEXT ·sdot32(SB), NOSPLIT, $0-28
	MOVQ n+0(FP), CX
	MOVQ x+8(FP), SI
	MOVQ y+16(FP), DI
	VXORPS Y0, Y0, Y0
	VXORPS Y1, Y1, Y1
	VXORPS Y2, Y2, Y2
	VXORPS Y3, Y3, Y3
	SHRQ   $5, CX
	JZ     topla

dongu32:
	VMOVUPS     (SI), Y4
	VMOVUPS     32(SI), Y5
	VMOVUPS     64(SI), Y6
	VMOVUPS     96(SI), Y7
	VFMADD231PS (DI), Y4, Y0
	VFMADD231PS 32(DI), Y5, Y1
	VFMADD231PS 64(DI), Y6, Y2
	VFMADD231PS 96(DI), Y7, Y3
	ADDQ        $128, SI
	ADDQ        $128, DI
	DECQ        CX
	JNZ         dongu32

topla:
	VADDPS       Y1, Y0, Y0
	VADDPS       Y3, Y2, Y2
	VADDPS       Y2, Y0, Y0
	VEXTRACTF128 $1, Y0, X1
	VADDPS       X1, X0, X0
	VMOVHLPS     X0, X1, X1
	VADDPS       X1, X0, X0
	VMOVSHDUP    X0, X1
	VADDSS       X1, X0, X0
	VMOVSS       X0, ret+24(FP)
	VZEROUPPER
	RET
