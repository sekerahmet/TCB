package codeindex

// "Bir test bu işleve bir yardımcı üzerinden ulaşıyor" ilişkisinin TEK yürüyüşü: `trlang index tested_by` (TestedBy) ve
// tools/codegraph (`tests-for`, yol sabitleri, gömülü dosyalar, ad kodu testleri) aynı fonksiyonu ve aynı sınırları
// kullanır (bağımsız denetim T3: eskiden codeindex 4 adım ve sınırsız paket, codegraph 8 adım ve 1 paket sınırıydı).
//
// Sınırlar: zincir paket (klasör) sınırını en çok TestMaxHops kez geçer ve en çok TestMaxSteps adımdır. Bir işlevi gerçekten
// sınayan test ya aynı pakette bir yardımcıdan (sınır geçmez, iç zincir uzun olabilir: yükleyici → ayrıştırıcı → kayıt →
// erişimci → test 5 adım) ya da onu doğrudan çağıran başka bir paketten gelir (bir sınır). Yalnızca adım sayısıyla
// sınırlansaydı ortak bir işleve kadar yüzlerce test "korur" görünürdü (7 adımda tr.json için 209 test).
//
// İki kademe (codegraph B1a, K1): listeler (tested_by, impact, tests-for, changed --go-tests) WalkCallers'ın sınırıyla
// kurulur. WalkCallersBeyond aynı yürüyüşü paketin içinde adım sınırı olmadan sürdürür; sınırın ötesinde bulunan testler
// listeye girmez, cevapta tek satırda toplanır ve codegraph'ın `untested`'ı bir işlevin testli olup olmadığına bununla
// karar verir. Neden: 8 adımı geçen paket içi zincirler gerçekten koşuyordu (codegraph'ın 51 "test yok"unun 49'u, tek
// `-run`'lı kapsamayla 49/49); sınırı kaldırmak ise her listeyi şişirirdi (300 işlevde ortanca 16 testten 41'e).
const (
	TestMaxHops  = 1
	TestMaxSteps = 8
)

// Reached yürüyüşte ulaşılan bir düğümdür: kaç adımda ve başlangıçtan sonraki ilk düğüm (Via). Entry yalnız
// WalkCallersBeyond'da ve Steps > TestMaxSteps iken dolar: düğümün zincirinde, ondan bu düğüme en çok TestMaxSteps adım
// kalan, başlangıca en yakın düğüm (Steps - TestMaxSteps adımda). Ondan sorulan liste bu düğümü sınır içinde bulur.
type Reached struct {
	ID    string
	Steps int
	Via   string
	Entry string
}

// WalkCallers start'tan çağıranlara doğru genişlik öncelikli yürür (callers: bir düğümün çağıranları, sıralı verilmeli ki
// sonuç belirleyici olsun; pkgOf: düğümün paketi ya da klasörü). Her ulaşılan düğüm için visit çağrılır; false dönerse o
// düğümün ötesine geçilmez (örn. bir test). Paket sınırı geçişleri TestMaxHops, adımlar TestMaxSteps ile sınırlıdır.
func WalkCallers(start string, callers func(string) []string, pkgOf func(string) string, visit func(Reached) bool) {
	walkCallers(start, callers, pkgOf, visit, false)
}

// WalkCallersBeyond WalkCallers'tır, yalnız start'ın paketinde kalan kol adım sınırı olmadan sürer; paket sınırını geçen
// kol WalkCallers'ın sınırlarındadır (toplam en çok TestMaxSteps adım, en çok TestMaxHops sınır). İlk TestMaxSteps katta
// WalkCallers'la aynı düğümleri aynı Steps ve Via ile aynı sırada verir; ötesindekiler start'ın paketindedir ve Entry taşır.
func WalkCallersBeyond(start string, callers func(string) []string, pkgOf func(string) string, visit func(Reached) bool) {
	walkCallers(start, callers, pkgOf, visit, true)
}

func walkCallers(start string, callers func(string) []string, pkgOf func(string) string, visit func(Reached) bool, beyond bool) {
	type state struct {
		Reached
		hops int
	}
	seen := map[string]bool{start: true}
	parent := map[string]string{}
	frontier := []state{{Reached{ID: start}, 0}}
	for steps := 0; len(frontier) > 0 && (beyond || steps < TestMaxSteps); steps++ {
		var next []state
		for _, st := range frontier {
			for _, c := range callers(st.ID) {
				hops := st.hops
				if pkgOf(c) != pkgOf(st.ID) {
					hops++
				}
				if hops > TestMaxHops || seen[c] || hops > 0 && steps >= TestMaxSteps {
					continue
				}
				seen[c] = true
				via := st.Via
				if steps == 0 {
					via = c
				}
				r := Reached{ID: c, Steps: steps + 1, Via: via}
				if beyond {
					parent[c] = st.ID
					if r.Steps > TestMaxSteps {
						r.Entry = c
						for range TestMaxSteps {
							r.Entry = parent[r.Entry]
						}
					}
				}
				if visit(r) {
					next = append(next, state{r, hops})
				}
			}
		}
		frontier = next
	}
}
