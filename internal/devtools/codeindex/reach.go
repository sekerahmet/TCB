package codeindex

// "Bir test bu işleve bir yardımcı üzerinden ulaşıyor" ilişkisinin TEK yürüyüşü: `tr index tested_by` (TestedBy) ve
// tools/codegraph (`tests-for`, yol sabitleri, gömülü dosyalar, ad kodu testleri) aynı fonksiyonu ve aynı sınırları
// kullanır (bağımsız denetim T3: eskiden codeindex 4 adım ve sınırsız paket, codegraph 8 adım ve 1 paket sınırıydı).
//
// Sınırlar: zincir paket (klasör) sınırını en çok TestMaxHops kez geçer ve en çok TestMaxSteps adımdır. Bir işlevi gerçekten
// sınayan test ya aynı pakette bir yardımcıdan (sınır geçmez, iç zincir uzun olabilir: yükleyici → ayrıştırıcı → kayıt →
// erişimci → test 5 adım) ya da onu doğrudan çağıran başka bir paketten gelir (bir sınır). Yalnızca adım sayısıyla
// sınırlansaydı ortak bir işleve kadar yüzlerce test "korur" görünürdü (7 adımda tr.json için 209 test).
const (
	TestMaxHops  = 1
	TestMaxSteps = 8
)

// Reached yürüyüşte ulaşılan bir düğümdür: kaç adımda ve başlangıçtan sonraki ilk düğüm (Via).
type Reached struct {
	ID    string
	Steps int
	Via   string
}

// WalkCallers start'tan çağıranlara doğru genişlik öncelikli yürür (callers: bir düğümün çağıranları, sıralı verilmeli ki
// sonuç belirleyici olsun; pkgOf: düğümün paketi ya da klasörü). Her ulaşılan düğüm için visit çağrılır; false dönerse o
// düğümün ötesine geçilmez (örn. bir test). Paket sınırı geçişleri TestMaxHops, adımlar TestMaxSteps ile sınırlıdır.
func WalkCallers(start string, callers func(string) []string, pkgOf func(string) string, visit func(Reached) bool) {
	type state struct {
		Reached
		hops int
	}
	seen := map[string]bool{start: true}
	frontier := []state{{Reached{start, 0, ""}, 0}}
	for steps := 0; steps < TestMaxSteps && len(frontier) > 0; steps++ {
		var next []state
		for _, st := range frontier {
			for _, c := range callers(st.ID) {
				hops := st.hops
				if pkgOf(c) != pkgOf(st.ID) {
					hops++
				}
				if hops > TestMaxHops || seen[c] {
					continue
				}
				seen[c] = true
				via := st.Via
				if steps == 0 {
					via = c
				}
				r := Reached{c, steps + 1, via}
				if visit(r) {
					next = append(next, state{r, hops})
				}
			}
		}
		frontier = next
	}
}
