package main

import "fmt"

// Bu dosya depoyu ya da bir paketi tip bilgisiyle yükleyip (`goload.Load`, `(*types.Config).Check`) ad ya da arayüz
// sorgulayan Go testlerini gerekçeli kapsam listesiyle bağlar. Bağ değişiklikten sonraki grafikten kurulduğunda metodun
// adı değişince, uygulayıcı dosya silinince, kullanım kalkınca ya da başka pakete aynı çıplak ad eklenince kanca koptuğu
// için test seçilmiyordu (DENETİM 101 Ş2); kapsam içindeki her Go değişikliği `scans` yolu gibi bu testleri seçer.

// typeQueryRule tip bilgisiyle yükleme yapıp ad ya da arayüz sorgulayan bir test grubunu ve yükleme kapsamını tanımlar.
type typeQueryRule struct {
	pkg    string    // depo köküne göre paket klasörü ("." kök)
	tests  []string  // paketteki Test*/Fuzz* işlev adları
	scope  scanScope // testlerin tip bilgisiyle yüklediği dosya kapsamı
	reason string    // kapsamın ve seçimin kısa gerekçesi
}

// repoTypeQueryRules bu depoda tip bilgisiyle yükleyip ad ya da arayüz sorgulayan testlerin listesidir; yeni bir
// yükleyici testi eklendiğinde TestRepositoryTypeLoadedTestsAreSelected bu listeyi zorlar.
var repoTypeQueryRules = []typeQueryRule{
	{
		pkg:    "tools/impact",
		tests:  []string{"TestInterfaceImplementers", "TestMembersAndPackages", "TestNameUses", "TestUnknownNames"},
		scope:  scanScope{bases: []string{"."}, inc: []string{".go"}},
		reason: "loads the repository (including _test.go) via goload.Load and queries interface implementers, symbol uses, and package dependencies",
	},
	{
		pkg:    "internal/compute/tensor",
		tests:  []string{"TestBackendDataCallsGoThroughTheGate", "TestHostBlockReadersKeepTheirTensors", "TestHostBlockSlicesStayWithTheirBlock"},
		scope:  scanScope{bases: []string{"internal/compute/tensor"}, inc: []string{".go"}, exc: []string{"_test.go"}},
		reason: "type-checks non-test .go files of internal/compute/tensor via (*types.Config).Check to enforce backend data gate calls and host block lifetimes",
	},
}

// linkTypeQueries verilen tip sorgusu kurallarını `type_query` düğümlerine çevirir ve listedeki test işlevlerinden
// `type_queries` kenarları kurar.
func linkTypeQueries(b *builder, rules []typeQueryRule) {
	for i, r := range rules {
		pkgPath := b.module
		if r.pkg != "" && r.pkg != "." {
			pkgPath += "/" + r.pkg
		}
		var testNodes []*Node
		for _, name := range r.tests {
			tn := b.nodes[memberID(pkgPath, "", name)]
			if tn == nil {
				tn = b.nodes[memberID(pkgPath+"_test", "", name)]
			}
			if tn != nil {
				testNodes = append(testNodes, tn)
			}
		}
		if len(testNodes) == 0 {
			continue
		}
		first := testNodes[0]
		id := fmt.Sprintf("type_query:%s:%d", first.File, i+1)
		b.addNode(Node{
			ID:     id,
			Kind:   kindTypeQuery,
			Name:   r.scope.String(),
			File:   first.File,
			Line:   first.Line,
			Detail: r.scope.encode(),
		})
		for _, tn := range testNodes {
			b.addEdge(tn.ID, id, edgeTypeQueries, inferred, tn.File, tn.Line)
		}
	}
}
