package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Kural (17.7): kodlu bir adlı argüman (öğrenme_oranı:, lr: → TR_arg_lr), bildirimi metin olan kullanıcı
// parametresiyle ya da alanıyla kodun herhangi bir yazım dilindeki adı üzerinden eşleşir. Kullanıcının
// parametre adı metin kalır, çağrıdaki dilin sözcüğü kodlanır; eşleme bir dilin adına bağlı olsaydı öbür
// dilde "'S.step' 'lr' adında bir parametre almaz" diye düşerdi. Aynı program iki dilde aynı sonucu verir;
// başka dilde yazılmış modülün metodu da.
func TestCodedNamedArgumentMatchesATextParameter(t *testing.T) {
	for _, c := range []struct{ name, en, tr, want string }{
		{"metot", "struct S {\n    func step(lr) {\n        return lr\n    }\n}\nprint(S().step(lr: 3))\n",
			"TR_struct S {\n    TR_func TR_step(öğrenme_oranı) {\n        TR_return öğrenme_oranı\n    }\n}\nTR_print(S().TR_step(TR_lr: 3))\n", "3\n"},
		{"kendi", "struct S {\n    func step(lr) {\n        return lr\n    }\n    func run() {\n        return self.step(lr: 4)\n    }\n}\nprint(S().run())\n",
			"TR_struct S {\n    TR_func TR_step(öğrenme_oranı) {\n        TR_return öğrenme_oranı\n    }\n    TR_func koş() {\n        TR_return TR_self.TR_step(TR_lr: 4)\n    }\n}\nTR_print(S().koş())\n", "4\n"},
		{"fonksiyon", "func f(dim) {\n    return dim\n}\nprint(f(dim: 5))\n",
			"TR_func f(eksen) {\n    TR_return eksen\n}\nTR_print(f(eksen: 5))\n", "5\n"},
		{"alan", "struct A {\n    dim = 0\n}\nprint(A(dim: 2).dim)\n",
			"TR_struct A {\n    eksen = 0\n}\nTR_print(A(eksen: 2).eksen)\n", "2\n"},
	} {
		if got := runCoded(t, c.en, "en", nil); got != c.want {
			t.Errorf("%s, İngilizce: %q, beklenen %q", c.name, got, c.want)
		}
		if got := runCoded(t, c.tr, "tr", nil); got != c.want {
			t.Errorf("%s, Türkçe: %q, beklenen %q", c.name, got, c.want)
		}
	}
	// Türkçe yazılmış modülün metodu İngilizce programdan kodlu adlı argümanla çağrılır.
	module, err := source.EncodeIn("TR_struct M {\n    TR_func TR_step(öğrenme_oranı) {\n        TR_return öğrenme_oranı * 10\n    }\n}\n", codes.Source())
	if err != nil {
		t.Fatal(err)
	}
	if got := runCoded(t, "import \"m\"\nprint(M().step(lr: 3))\n", "en", map[string]string{"m.tr": module}); got != "30\n" {
		t.Errorf("Türkçe modülün metodu: %q, beklenen \"30\\n\"", got)
	}
}
