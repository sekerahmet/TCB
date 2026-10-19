#include "textflag.h"

// func xgetbv0() (eax uint32)
//
// İşletim sisteminin kaydedip geri yüklediği yazmaç durumları (XCR0'ın alt yarısı).
TEXT ·xgetbv0(SB), NOSPLIT, $0-4
	MOVL   $0, CX
	XGETBV
	MOVL   AX, eax+0(FP)
	RET
