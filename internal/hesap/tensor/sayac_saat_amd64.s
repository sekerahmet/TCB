//go:build amd64

#include "textflag.h"

// func tsc() int64
//
// Zaman damgası sayacı: EDX:EAX → 64 bit.
TEXT ·tsc(SB), NOSPLIT, $0-8
	RDTSC
	SHLQ $32, DX
	ORQ  DX, AX
	MOVQ AX, ret+0(FP)
	RET

// func cpuid(yaprak, alt uint32) (eax, ebx, ecx, edx uint32)
TEXT ·cpuid(SB), NOSPLIT, $0-24
	MOVL yaprak+0(FP), AX
	MOVL alt+4(FP), CX
	CPUID
	MOVL AX, eax+8(FP)
	MOVL BX, ebx+12(FP)
	MOVL CX, ecx+16(FP)
	MOVL DX, edx+20(FP)
	RET
