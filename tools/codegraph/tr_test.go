package main

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

const (
	geo      = "lib/geo.tr"
	geoKare  = "tr:struct:" + geo + ":Kare"
	geoAlan  = "tr:method:" + geo + ":Kare.alan"
	geoCevre = "tr:func:" + geo + ":çevre_hesapla"
	geoYaz   = "tr:func:" + geo + ":yaz_alan"
	test1    = "tr:test:lib/geo_test.tr:3"
	test2    = "tr:test:lib/geo_test.tr:8"
	printG   = "name:TR_fn_print"
)

func TestTRNodes(t *testing.T) {
	g := fixture(t)
	want := map[string]int{
		geoKare: 5, geoAlan: 10, geoCevre: 15, "tr:func:" + geo + ":yaz_alan": 19,
		"tr:const:" + geo + ":PI2": 2, "tr:var:" + geo + ":sayaç": 3,
		"tr:field:" + geo + ":Kare.kenar": 6, "tr:method:" + geo + ":Kare.kurucu": 7,
	}
	for _, n := range g.Nodes {
		if line, ok := want[n.ID]; ok {
			if n.Line != line || n.File != geo {
				t.Errorf("%s at %s:%d, want %s:%d", n.ID, n.File, n.Line, geo, line)
			}
			delete(want, n.ID)
		}
	}
	for id := range want {
		t.Errorf("missing node %s", id)
	}
	for _, n := range g.Nodes {
		if n.ID == geoKare && (n.Kind != kindTRStruct || n.Name != "geo.Kare" || n.End < 13) {
			t.Errorf("Kare: %+v", n)
		}
		if n.ID == "tr:file:"+geo && (n.Kind != kindFile || len(n.Aliases) != 2 || n.Aliases[1] != "tr/geo") {
			t.Errorf("file node: %+v", n)
		}
	}
}

func TestTREdges(t *testing.T) {
	g := fixture(t)
	mustEdge(t, g, geoYaz, geoCevre, edgeCalls, extracted)
	mustEdge(t, g, geoYaz, printG, edgeUsesName, extracted)
	mustEdge(t, g, geoYaz, geoAlan, edgeCalls, inferred) // k.alan(): alıcının tipi bilinmez
	mustEdge(t, g, geoKare, geoAlan, edgeContains, extracted)
	mustEdge(t, g, "tr:file:lib/geo_test.tr", "tr:file:"+geo, edgeImports, extracted)
	// Aktarılan modülün adları takma adsız doğrudan görünür.
	mustEdge(t, g, test1, geoYaz, edgeCalls, extracted)
	mustEdge(t, g, test1, geoKare, edgeCalls, extracted)
	mustEdge(t, g, test1, geoYaz, edgeTests, extracted)
	// Yerel değişken yerleşiği gölgeler: ikinci testte yazdır bir yerel adtır.
	if edge(g, test2, printG, edgeUsesName) != nil {
		t.Error("a local variable shadows the built-in name")
	}
	mustEdge(t, g, test2, geoCevre, edgeTests, extracted)
	// Takma adlı aktarma: g.Kare, g.çevre_hesapla, g.PI2.
	const ana = "tr:file:examples/ana.tr"
	mustEdge(t, g, ana, "tr:file:"+geo, edgeImports, extracted)
	mustEdge(t, g, "tr:var:examples/ana.tr:k", geoKare, edgeCalls, extracted) // üst düzey değişkenin başlatıcısı
	mustEdge(t, g, ana, geoCevre, edgeCalls, extracted)
	mustEdge(t, g, ana, "tr:const:"+geo+":PI2", edgeUses, extracted)
	mustEdge(t, g, ana, printG, edgeUsesName, extracted)
}

func TestTRSyntaxErrorIsAWarning(t *testing.T) {
	g := fixture(t)
	if !hasNode(g, "tr:file:examples/broken.tr") {
		t.Error("a file with a syntax error still has a file node")
	}
	found := false
	for _, w := range g.Warnings {
		found = found || strings.HasPrefix(w, "tr syntax error, definitions skipped: examples/broken.tr")
	}
	if !found {
		t.Errorf("warnings: %v", g.Warnings)
	}
}

func TestTRQueries(t *testing.T) {
	code, out, _ := cli(t, "query", "Kare")
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	testkit.MustContain(t, out, "geo.Kare [tr_struct] lib/geo.tr:5", "<- calls", "geo_test: kare alanı yazılır [tr_test]")
	// Modül adı ve içe aktarma yolu dosyaya çözülür.
	for _, q := range []string{"geo", "tr/geo"} {
		code, out, _ = cli(t, "query", q)
		if code != exitOK || !strings.Contains(out, "lib/geo.tr [file]") {
			t.Errorf("%s: exit %d\n%s", q, code, out)
		}
	}
	// Yerleşik ad TR çağrı yerlerini de verir.
	_, out, _ = cli(t, "who-uses", "yazdır")
	testkit.MustContain(t, out, "geo.yaz_alan [tr_func] at lib/geo.tr:", "examples/ana.tr [file] at examples/ana.tr:")
	_, out, _ = cli(t, "tests-for", "geo.Kare")
	testkit.MustContain(t, out, "geo_test: kare alanı yazılır [tr_test]")
}

// Kodlu biçimli (başlıklı) dosya gösterim metniyle okunur: satırlar dosyadaki gerçek satırdır (başlık satırı sayılır).
func TestTRCodedFile(t *testing.T) {
	g := fixture(t)
	const fn = "tr:func:examples/coded.tr:kodlu_işlev"
	found := false
	for _, n := range g.Nodes {
		if n.ID == fn && n.Line == 2 && n.End == 5 && strings.Contains(n.Detail, "kodlu_işlev(x)") {
			found = true
		}
		if n.ID == "tr:file:examples/coded.tr" && n.Detail != "tr, coded" {
			t.Errorf("coded file detail: %q", n.Detail)
		}
	}
	if !found {
		t.Errorf("coded function node missing or wrong")
	}
	mustEdge(t, g, fn, printG, edgeUsesName, extracted) // G100 olarak yazılmış çağrı
}

// Parametreler grafikte düğümdür (tr_param): sahibinden `contains`, varsayılan değer metniyle; adlı
// argüman olarak geçtiği yerler `who-uses` ile bulunur.
func TestTRParametersAndNamedArguments(t *testing.T) {
	g := fixture(t)
	const param = "tr:param:lib/geo.tr:Kare.kurucu.kenar"
	found := false
	for _, n := range g.Nodes {
		if n.ID == param && n.Kind == kindTRParam && n.Detail == "kenar = 1" && n.Name == "geo.Kare.kurucu.kenar" {
			found = true
		}
	}
	if !found {
		t.Fatal("parameter node missing or wrong")
	}
	mustEdge(t, g, "tr:method:lib/geo.tr:Kare.kurucu", param, edgeContains, extracted)
	mustEdge(t, g, "tr:var:examples/ana.tr:k2", param, edgeUses, extracted) // g.Kare(kenar: 3)
	_, out, _ := cli(t, "who-uses", "geo.Kare.kurucu.kenar")
	testkit.MustContain(t, out, "examples/ana.tr")
}

// Bilerek sözdizimi hatalı yazılan hata örnekleri (examples/*/hatalar, errors) uyarı değildir.
func TestErrorExamplesAreNotWarnings(t *testing.T) {
	g := fixture(t)
	for _, w := range g.Warnings {
		if strings.Contains(w, "examples/hatalar/") {
			t.Errorf("an intentional syntax error must not warn: %s", w)
		}
	}
	if !hasNode(g, "tr:file:examples/hatalar/01_broken.tr") {
		t.Error("the file is still a node")
	}
}
