package main

// "Bir test bu işleve (dosyaya, ada) bir yardımcı üzerinden ulaşıyor" ilişkisinin TEK kuralı burada: hangi kenarlar
// zincire girer (reachEdge), zincir ne kadar uzayabilir (testMaxHops, testMaxSteps) ve nasıl yürünür (walkCallers).
// Dosya yolu sabitleri, gömülen dosyalar, TR metni taşıyan Go testleri, ad kodlarının testleri ve `tests-for`un
// dolaylı testleri aynı fonksiyonu kullanır; bir sınırı değiştirmek hepsini birlikte değiştirir.
//
// Sınır seçimi: zincir paket sınırını en çok testMaxHops (=1) kez geçer ve en çok testMaxSteps (=8) adımdır. Veriyi ya da
// işlevi gerçekten sınayan test ya aynı pakette bir yardımcıdan (sınır geçmez, adım serbest: yükleyici → ayrıştırıcı →
// kayıt gibi uzun iç zincirler) ya da onu doğrudan çağıran başka bir paketten gelir (bir sınır). İkinci sınırı geçen test
// ("çağıranın çağıranı") konunun dışına çıkar: yalnızca adım sayısıyla sınırlandığında (7 adımda tr.json için 209 test)
// bir yükleyiciden her yerde kullanılan bir işleve kadar yüzlerce test "korur" görünürdü. Eski iki kural (≤3 adım
// yol sabitlerinde, ≤4 adım dolaylı testlerde) bu tek kurala indirildi; 4 ve 3 adım iç zincirleri kesiyordu
// (yükleyici → active → Codes → test 5 adım).
const (
	testMaxHops  = 1
	testMaxSteps = 8
)

// reachEdge bir kenarın "çağıran/kullanan" zincirine girip girmediği: kesin (EXTRACTED) çağrı, tür/sabit kullanımı, değişken
// okuma ve yazma. İşlev değeri olarak geçirilen (INFERRED) çağrı zincire girmez.
func reachEdge(kind, confidence string) bool {
	switch kind {
	case edgeCalls:
		return confidence == extracted
	case edgeUses, edgeReadsVar, edgeWritesVar:
		return true
	}
	return false
}

// reached zincirde ulaşılan bir düğümdür: kaç adımda, ve start'tan ilk adım hangi düğümdü.
type reached struct {
	id    string
	steps int
	via   string
}

// walkCallers start'tan çağıranlara doğru genişlik öncelikli yürür (callers: bir düğümün çağıranları; sıra
// belirleyicidir). Her ulaşılan düğüm için visit çağrılır; false dönerse o düğümün ötesine geçilmez (örn. bir test).
// pkgOf düğümün paket klasörüdür; paket sınırı geçişleri testMaxHops ile sınırlıdır.
func walkCallers(start string, callers func(string) []string, pkgOf func(string) string, visit func(reached) bool) {
	type state struct {
		reached
		hops int
	}
	seen := map[string]bool{start: true}
	frontier := []state{{reached{start, 0, ""}, 0}}
	for steps := 0; steps < testMaxSteps && len(frontier) > 0; steps++ {
		var next []state
		for _, st := range frontier {
			for _, c := range callers(st.id) {
				hops := st.hops
				if pkgOf(c) != pkgOf(st.id) {
					hops++
				}
				if hops > testMaxHops || seen[c] {
					continue
				}
				seen[c] = true
				via := st.via
				if steps == 0 {
					via = c
				}
				r := reached{c, steps + 1, via}
				if visit(r) {
					next = append(next, state{r, hops})
				}
			}
		}
		frontier = next
	}
}
