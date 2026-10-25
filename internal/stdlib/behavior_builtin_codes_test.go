package stdlib

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Tüm motorlar yüklüyken genel kapsamdaki her yerleşiğin sözlükte bir yerleşik kodu
// (G/T/V/W) olmalı ve kayıtlı ad o kodun adı olmalı; sözlükteki her yerleşik kodu da
// adıyla kayıtlı olmalı. Böylece sözlükte adı olmayan
// bir yerleşik kaydedilemez, sözlükte de kaydı olmayan bir yerleşik kodu kalamaz.
func TestBuiltinsMatchDictionary(t *testing.T) {
	m := interp.New()
	for _, name := range m.GlobalEnv.Names() {
		d, _ := m.GlobalEnv.Get(name)
		b, ok := d.(*interp.BuiltinVal)
		if !ok {
			continue
		}
		k, ok := codes.Find(codes.NamespaceGlobal, name)
		if !ok || !k.IsBuiltin() {
			t.Errorf("kayıtlı yerleşik %q sözlükte bir yerleşik kodu değil", name)
			continue
		}
		if actual, _ := codes.CanonicalNameIn(codes.NamespaceGlobal, name); b.Name != actual {
			t.Errorf("%q (%s) yerleşiğinin adı %q, sözlükte %q", name, k, b.Name, actual)
		}
	}
	for _, k := range codes.Codes() {
		if !k.IsBuiltin() {
			continue
		}
		if tn, _ := codes.DefinitionOf(k); tn.Kind == codes.KindConstant {
			// Sabitler (pi) yerleşik fonksiyon değil, değer olarak kayıtlıdır.
			for _, y := range codes.Spellings(k) {
				if d, ok := m.GlobalEnv.Get(y); !ok || d == nil {
					t.Errorf("sözlükteki sabit %s (%s) %q yazımıyla kayıtlı değil", k, codes.Name(k), y)
				}
			}
			continue
		}
		b := m.Builtin(k)
		if b == nil {
			t.Errorf("sözlükteki %s (%s) kayıtlı değil", k, codes.Name(k))
			continue
		}
		for _, y := range codes.Spellings(k) {
			if d, _ := m.GlobalEnv.Get(y); d != b {
				t.Errorf("%s yazımı %q aynı yerleşiğe bağlı değil", k, y)
			}
		}
	}
}

// runSame TR kodunu verilen VM'de çalıştırır (genel tanımlar korunur).
func runSame(t *testing.T, m *interp.VM, kd string) (interp.Value, error) {
	t.Helper()
	p := parser.New(lexer.New(kd))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), kd)
	}
	return m.Run(prog)
}

// Kod listesindeki her üye kodu (Y) bütün yazımlarıyla en az bir tipte çalışmalı: kodu
// olup da hiçbir switch'te karşılanmayan (ya da yalnızca bir yazımı karşılanan) üye
// kalmamalı. Veri kümesi ve okuyucu üyeleri (Y600-Y602) dizin gerektirdiği için
// internal/stdlib/dataset testlerinde denenir.
func TestMembersMatchDictionary(t *testing.T) {
	m := interp.New()
	m.Err = io.Discard // açılan ilerleme çubuğu test çıktısına yazmasın
	setup := strings.Join([]string{
		`değişken u_metin = "abc"`,
		`değişken u_liste = [1, 2]`,
		`değişken u_sozluk = {"a": 1}`,
		`değişken u_tensor = tensör([[1.0, 2.0]], türev_izle: doğru)`,
		`değişken u_iyilestirici = SGD([u_tensor])`,
		`değişken u_parcalayici = parçalayıcı_yükle("` + filepath.ToSlash(reporoot.Path("internal/compute/tokenize/testdata/tokenizer_byte_bpe.json")) + `")`,
		`değişken u_yol = yol("` + filepath.ToSlash(reporoot.Path("go.mod")) + `")`,
		`değişken u_ilerleme = ilerleme(3)`,
	}, "\n")
	if _, err := runSame(t, m, setup); err != nil {
		t.Fatal(err)
	}
	examples := []string{"u_metin", "u_liste", "u_sozluk", "u_tensor", "u_iyilestirici", "u_parcalayici", "u_yol", "u_ilerleme"}
	for _, k := range codes.Codes() {
		if k.Namespace() != codes.NamespaceMember || (k >= codes.MemSampler && k <= codes.MemNext) {
			continue
		}
		for _, y := range codes.Spellings(k) {
			found := false
			for _, o := range examples {
				if _, err := runSame(t, m, o+"."+y); err == nil {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("üye %s (%q) hiçbir tipte karşılanmıyor", k, y)
			}
		}
	}
	// Motor tiplerinde ASCII yazılmış üye adı hata verir ve Türkçe adı önerir; öneri
	// yalnızca o tipte geçerli bir üye için yapılır.
	for _, c := range []struct{ code, want string }{
		{"u_tensor.sekil", "Bunu mu demek istediniz: 'şekil'?"},
		{"u_iyilestirici.sifirla", "Bunu mu demek istediniz: 'sıfırla'?"},
		{"u_parcalayici.coz", "Bunu mu demek istediniz: 'çöz'?"},
	} {
		if _, err := runSame(t, m, c.code); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", c.code, c.want, err)
		}
	}
	if _, err := runSame(t, m, "u_tensor.kodla"); err == nil || strings.Contains(err.Error(), "yazmak mı istediniz") {
		t.Errorf("tensörde kodla önerilmemeli: %v", err)
	}
	// Dilde olmayan adlar sıradan adlardır.
	if v, err := runSame(t, m, "değişken web_sunucu = 3\nweb_sunucu + 1"); err != nil || v.Inspect() != "4" {
		t.Errorf("kaldırılan ad değişken adı olabilmeli: %v %v", v, err)
	}
}
