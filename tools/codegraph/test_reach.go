package main

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/devtools/codeindex"
)

// Zincir kuralının yürüyüşü ve sınırları codeindex'tedir (codeindex.WalkCallers, TestMaxHops, TestMaxSteps): `tr dizin
// sınayanlar` ile `tests-for` aynı kuralla cevap verir. Burada yalnızca grafiğe özgü kısım durur: hangi kenar zincire girer.
const (
	testMaxHops  = codeindex.TestMaxHops
	testMaxSteps = codeindex.TestMaxSteps
)

// reachEdge bir kenarın "çağıran/kullanan" zincirine girip girmediği: kesin (EXTRACTED) çağrı, tür/sabit kullanımı, değişken
// okuma ve yazma, arayüzden uygulamaya geçiş. Go'da INFERRED çağrı işlevin değer olarak geçirilmesidir ve zincire girmez;
// TR'de ise alıcı tipi bilinmeyen sıradan metot çağrısıdır (`model.ileri(x)`), bu yüzden iki ucu da TR olan INFERRED
// çağrı zincire girer (codeindex'in kendi yürüyüşü de bütün TR çağrılarını izler).
func reachEdge(kind, confidence, from, to string) bool {
	switch kind {
	case edgeCalls:
		return confidence == extracted || strings.HasPrefix(from, "tr:") && strings.HasPrefix(to, "tr:")
	case edgeUses, edgeReadsVar, edgeWritesVar, edgeDispatches:
		return true
	}
	return false
}

// reached zincirde ulaşılan düğümdür.
type reached = codeindex.Reached

// walkCallers codeindex.WalkCallers'dır.
func walkCallers(start string, callers func(string) []string, pkgOf func(string) string, visit func(reached) bool) {
	codeindex.WalkCallers(start, callers, pkgOf, visit)
}
