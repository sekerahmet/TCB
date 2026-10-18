//go:build amd64

#include "textflag.h"

// func islemciBekleAsm()
//
// Dönerek beklemede 8 PAUSE komutu: çekirdeği (ve aynı fiziksel çekirdeği
// paylaşan diğer iş parçacığını) yormadan kısa bir süre bekler.
TEXT ·islemciBekleAsm(SB), NOSPLIT, $0-0
	PAUSE
	PAUSE
	PAUSE
	PAUSE
	PAUSE
	PAUSE
	PAUSE
	PAUSE
	RET
