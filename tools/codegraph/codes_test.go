package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// commonNameFiles bir kodun adı yalnız definitions.json'daki ortak addır (madde 15 adım 2): dil dosyalarında satırı var
// (açıklama, örnek) ama adı yok; ortam değişkeninin ise dil dosyalarında satırı da yok.
var commonNameFiles = map[string]string{
	"p/p.go": "package p\n",
	"internal/lang/codes/definitions.json": `{
  "TR_fn_sin": { "tür": "fonksiyon", "ortak_ad": "sin" },
  "TR_fn_print": { "tür": "fonksiyon" },
  "TR_env_CPU_AVX2": { "tür": "ortam", "ortak_ad": "TR_CPU_AVX2" }
}
`,
	"internal/lang/codes/tr.json": `{
  "TR_fn_sin": { "aile": "matematik" },
  "TR_fn_print": { "ad": "yazdır", "aile": "çıktı" }
}
`,
	"internal/lang/codes/en.json": `{
  "TR_fn_sin": { "aile": "math" },
  "TR_fn_print": { "ad": "print", "aile": "output" }
}
`,
}

// Kural: dil dosyasında adı yazılmayan kodun adı definitions.json'daki ortak addır (tek kaynak): ad kodu düğümünün adı
// ve açıklaması ortak ad, `query <ortak ad>` kimliği bulur, ortak adlı ortam değişkeni koduna bağlanır.
func TestACommonNameComesFromTheDefinitions(t *testing.T) {
	root := auditModule(t, commonNameFiles)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	ix := newIndex(g)
	sin := ix.byID["name:TR_fn_sin"]
	if sin == nil || !slices.Equal(sin.Aliases, []string{"sin"}) || !strings.HasPrefix(sin.Detail, "sin (") {
		t.Fatalf("TR_fn_sin: %+v", sin)
	}
	if found := ix.lookup("sin"); len(found) != 1 || found[0] != sin {
		t.Errorf("query sin: %v", found)
	}
	if p := ix.byID["name:TR_fn_print"]; p == nil || !slices.Equal(p.Aliases, []string{"yazdır", "print"}) {
		t.Errorf("a name in the language files stays: %+v", p)
	}
	if got := envCodes(root)["TR_CPU_AVX2"]; got != "TR_env_CPU_AVX2" {
		t.Errorf("the common name of an environment variable: %q", got)
	}
}

// Kural: codegraph'ın her ad kodu düğümünün adı codes paketinin adıdır (Türkçe ve İngilizce), ortam değişkeni kodlarının
// her dildeki adı koduna bağlanır. Örnekle değil, bütün kodlarla: adı başka bir kaynaktan (ortak ad) gelen bir kod
// okuyucu tarafından kaçırılırsa düşer.
func TestEveryNameCodeNodeHasTheNameOfTheCodesPackage(t *testing.T) {
	root, _, g := repoGraph(t)
	ix := newIndex(g)
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	env := envCodes(root)
	for _, k := range codes.Codes() {
		id := k.String()
		n := ix.byID["name:"+id]
		if n == nil {
			t.Errorf("%s has no node", id)
			continue
		}
		want := []string{codes.Source().Name(k)}
		if e, ok := en.Entry(k); ok && e.Name != want[0] {
			want = append(want, e.Name)
		}
		if !slices.Equal(n.Aliases, want) {
			t.Errorf("%s: node names %v, the codes package %v", id, n.Aliases, want)
		}
		if k.Namespace() == codes.NamespaceEnvironment {
			for _, name := range want {
				if env[name] != id {
					t.Errorf("environment variable %s is not linked to %s (%q)", name, id, env[name])
				}
			}
		}
	}
}
