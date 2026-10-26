package main

import "strings"

// "Bu bir test mi" sorusunun TEK cevabı burada (bağımsız denetim Y1, Y3, T3): test dosyası, test işlevi adı, test düğümü
// ve satır aralığı taşıyan düğüm türleri. `tests-for`, `changed`, kural satırları, ad kodu testleri, madde atıfları ve
// denetimler hepsi bunları çağırır; eskiden "test" bazı yerlerde yalnızca Go test işleviydi ve TR test blokları dolaylı
// testlerden ve `changed`'den düşüyordu.

// testPrefixes go test'in tanıdığı işlev adı önekleridir.
var testPrefixes = []string{"Test", "Benchmark", "Fuzz", "Example"}

// testNameRest adın test öneki sonrasını verir (`TestParse_Errors` → "Parse_Errors"); test adı değilse ok=false.
// go test kuralı: önekten sonra küçük harf gelmez.
func testNameRest(name string) (rest string, ok bool) {
	for _, p := range testPrefixes {
		if strings.HasPrefix(name, p) {
			rest = name[len(p):]
			return rest, rest == "" || !(rest[0] >= 'a' && rest[0] <= 'z')
		}
	}
	return "", false
}

// isTestName go test'in tanıdığı bir işlev adı mı.
func isTestName(name string) bool {
	_, ok := testNameRest(name)
	return ok
}

// isTestFile yol bir Go test dosyası mı.
func isTestFile(file string) bool { return strings.HasSuffix(file, "_test.go") }

// inTestFile düğüm bir Go test dosyasında mı tanımlı.
func inTestFile(n *Node) bool { return isTestFile(n.File) }

// isTestCode düğüm test kodu mu: bir test, ya da bir Go test dosyasında ya da TR test dosyasında (_test.tr) tanımlı
// yardımcı.
func isTestCode(n *Node) bool {
	return n != nil && (isTestNode(n) || inTestFile(n) || strings.HasSuffix(n.File, "_test.tr"))
}

// isTestNode düğüm bir test mi: Go test dosyasındaki TestX/BenchmarkX/FuzzX/ExampleX işlevi ya da TR test bloğu.
func isTestNode(n *Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == kindTRTest {
		return true
	}
	return n.Kind == kindFunc && inTestFile(n) && isTestName(n.Name[strings.LastIndex(n.Name, ".")+1:])
}

// hasLineSpan satır aralığı (Line..End) taşıyan, bir değişikliğin ya da bir atıfın bağlanabileceği düğüm türü mü: Go
// bildirimleri, TR bildirimleri ve belge başlıkları. `changed` ve madde atıfları aynı listeyi kullanır.
func hasLineSpan(kind string) bool {
	switch kind {
	case kindFunc, kindMethod, kindType, kindTRFunc, kindTRMethod, kindTRStruct, kindTRTest, kindHeading:
		return true
	}
	return false
}
