package interp

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Kural (17.7): kodlu bir adlı argüman (öğrenme_oranı:, lr: → TR_arg_lr), bildirimi metin olan kullanıcı
// parametresiyle ya da alanıyla kodun herhangi bir yazım dilindeki adı üzerinden eşleşir. Kullanıcının
// parametre adı metin kalır, çağrıdaki dilin sözcüğü kimliktir; eşleme bir dilin adına bağlı olsaydı
// kullanıcının parametresi bir dilin adıyla (lr ya da öğrenme_oranı) yazılınca öbüründe "'S.step' 'lr'
// adında bir parametre almaz" diye düşerdi. Modülün metodu da.
func TestCodedNamedArgumentMatchesATextParameter(t *testing.T) {
	for _, c := range []struct{ name, en, tr, want string }{
		{"metot", "TR_struct S {\n    TR_func TR_step(lr) {\n        TR_return lr\n    }\n}\nTR_print(S().TR_step(TR_lr: 3))\n",
			"TR_struct S {\n    TR_func TR_step(öğrenme_oranı) {\n        TR_return öğrenme_oranı\n    }\n}\nTR_print(S().TR_step(TR_lr: 3))\n", "3\n"},
		{"kendi", "TR_struct S {\n    TR_func TR_step(lr) {\n        TR_return lr\n    }\n    TR_func run() {\n        TR_return TR_self.TR_step(TR_lr: 4)\n    }\n}\nTR_print(S().run())\n",
			"TR_struct S {\n    TR_func TR_step(öğrenme_oranı) {\n        TR_return öğrenme_oranı\n    }\n    TR_func koş() {\n        TR_return TR_self.TR_step(TR_lr: 4)\n    }\n}\nTR_print(S().koş())\n", "4\n"},
		{"fonksiyon", "TR_func f(dim) {\n    TR_return dim\n}\nTR_print(f(dim: 5))\n",
			"TR_func f(eksen) {\n    TR_return eksen\n}\nTR_print(f(eksen: 5))\n", "5\n"},
		{"alan", "TR_struct A {\n    dim = 0\n}\nTR_print(A(dim: 2).dim)\n",
			"TR_struct A {\n    eksen = 0\n}\nTR_print(A(eksen: 2).eksen)\n", "2\n"},
	} {
		if got := runCoded(t, c.en, nil); got != c.want {
			t.Errorf("%s, İngilizce ad: %q, beklenen %q", c.name, got, c.want)
		}
		if got := runCoded(t, c.tr, nil); got != c.want {
			t.Errorf("%s, Türkçe ad: %q, beklenen %q", c.name, got, c.want)
		}
	}
	// Parametresi Türkçe adla yazılmış modülün metodu kodlu adlı argümanla çağrılır.
	module := source.Header + "\nTR_struct M {\n    TR_func TR_step(öğrenme_oranı) {\n        TR_return öğrenme_oranı * 10\n    }\n}\n"
	if got := runCoded(t, "TR_import \"m\"\nTR_print(M().TR_step(TR_lr: 3))\n", map[string]string{"m.tr": module}); got != "30\n" {
		t.Errorf("Türkçe modülün metodu: %q, beklenen \"30\\n\"", got)
	}
}
