package stdlib

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Kural: tüm motorlar yüklüyken genel kapsamdaki her yerleşik bir yerleşik kodunun (fn) kimliğiyle
// kayıtlıdır ve adı o kimliktir; sözlükteki her yerleşik kodu da kimliğiyle kayıtlıdır (sabitler
// değer olarak). Böylece sözlükte kodu olmayan bir yerleşik kaydedilemez, sözlükte de kaydı olmayan
// bir yerleşik kodu kalamaz. İçeride ad kimliktir: kayıt hiçbir yazım dilinin adını taşımaz.
func TestBuiltinsMatchDictionary(t *testing.T) {
	m := interp.New()
	for _, name := range m.GlobalEnv.Names() {
		d, _ := m.GlobalEnv.Get(name)
		b, ok := d.(*interp.BuiltinVal)
		if !ok {
			continue
		}
		if k, err := codes.Parse(name); err != nil || !k.IsBuiltin() {
			t.Errorf("kayıtlı yerleşik %q sözlükte bir yerleşik kodunun kimliği değil", name)
		} else if b.Name != name {
			t.Errorf("%s yerleşiğinin adı %q, kimliği olmalı", k, b.Name)
		}
	}
	for _, k := range codes.Codes() {
		if !k.IsBuiltin() {
			continue
		}
		d, ok := m.GlobalEnv.Get(k.String())
		if tn, _ := codes.DefinitionOf(k); tn.Kind == codes.KindConstant {
			// Sabitler (pi) yerleşik fonksiyon değil, değer olarak kayıtlıdır.
			if !ok || d == nil {
				t.Errorf("sözlükteki sabit %s kimliğiyle kayıtlı değil", k)
			}
			continue
		}
		if b := m.Builtin(k); b == nil {
			t.Errorf("sözlükteki %s kayıtlı değil", k)
		} else if d != b {
			t.Errorf("%s kimliği aynı yerleşiğe bağlı değil", k)
		}
	}
}

// runCoded kodlu gövdeyi (kimliklerle yazılmış TR kodu, 17.2) verilen VM'de çalıştırır (genel
// tanımlar korunur).
func runCoded(t *testing.T, m *interp.VM, body string) (interp.Value, error) {
	t.Helper()
	p := parser.New(source.Parts{Coded: true, Body: body}.Lexer())
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), body)
	}
	return m.Run(prog)
}

// runSame Türkçe yazılmış TR kodunu verilen VM'de çalıştırır (genel tanımlar korunur).
func runSame(t *testing.T, m *interp.VM, kd string) (interp.Value, error) {
	t.Helper()
	p := parser.New(source.TextLexer(kd))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v\n%s", p.Errors(), kd)
	}
	return m.Run(prog)
}

// Kural: kod listesindeki her üye kodu (mem) en az bir tipte çalışır: kodu olup da hiçbir
// switch'te karşılanmayan üye kalmaz. Üyeye kimliğiyle erişilir (kodlu gövde). Veri kümesi ve
// okuyucu üyeleri dizin gerektirdiği için internal/stdlib/dataset testlerinde denenir. Dilde
// olmayan adlar sıradan adlardır.
func TestMembersMatchDictionary(t *testing.T) {
	m := interp.New()
	m.Err = io.Discard // açılan ilerleme çubuğu test çıktısına yazmasın
	setup := strings.Join([]string{
		`TR_var u_metin = "abc"`,
		`TR_var u_liste = [1, 2]`,
		`TR_var u_sozluk = {"a": 1}`,
		`TR_var u_tensor = TR_tensor([[1.0, 2.0]], TR_requires_grad: TR_true)`,
		`TR_var u_iyilestirici = TR_SGD([u_tensor])`,
		`TR_var u_parcalayici = TR_load_tokenizer("` + filepath.ToSlash(reporoot.Path("internal/compute/tokenize/testdata/tokenizer_byte_bpe.json")) + `")`,
		`TR_var u_yol = TR_path("` + filepath.ToSlash(reporoot.Path("go.mod")) + `")`,
		`TR_var u_ilerleme = TR_progress(3)`,
	}, "\n")
	if _, err := runSame(t, m, setup); err != nil {
		t.Fatal(err)
	}
	examples := []string{"u_metin", "u_liste", "u_sozluk", "u_tensor", "u_iyilestirici", "u_parcalayici", "u_yol", "u_ilerleme"}
	for _, k := range codes.Codes() {
		// Kitaplığın üyesi çekirdek tiplerde değil, kitaplığın yapılarında karşılanır.
		if k.Namespace() != codes.NamespaceMember || k.IsLibrary() || (k >= codes.MemSampler && k <= codes.MemNext) {
			continue
		}
		found := false
		for _, o := range examples {
			if _, err := runCoded(t, m, o+"."+codes.Spelling(k)); err == nil {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("üye %s hiçbir tipte karşılanmıyor", k)
		}
	}
	if v, err := runSame(t, m, "TR_var web_sunucu = 3\nweb_sunucu + 1"); err != nil || v.Inspect() != "4" {
		t.Errorf("dilde olmayan ad değişken adı olabilmeli: %v %v", v, err)
	}
}

// Kural: Türkçe yazılmış programda motor tiplerinin ASCII yazılmış üye adı (sekil) hata verir ve
// Türkçe adı önerir; öneri yalnızca o tipte geçerli bir üye için yapılır (tensörde kodla önerilmez).
func TestEngineMemberASCIISpellingSuggestsTurkish(t *testing.T) {
	m := interp.New()
	m.Err = io.Discard
	setup := strings.Join([]string{
		`TR_var u_tensor = TR_tensor([[1.0, 2.0]], TR_requires_grad: TR_true)`,
		`TR_var u_iyilestirici = TR_SGD([u_tensor])`,
		`TR_var u_parcalayici = TR_load_tokenizer("` + filepath.ToSlash(reporoot.Path("internal/compute/tokenize/testdata/tokenizer_byte_bpe.json")) + `")`,
	}, "\n")
	if _, err := runSame(t, m, setup); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		code string
		want codes.Code
	}{{"u_tensor.sekil", codes.MemShape}, {"u_iyilestirici.sifirla", codes.MemZeroGrad}, {"u_parcalayici.coz", codes.MemDecode}} {
		want := "'" + codes.NameRef(c.want).MessageText("tr") + "'?"
		if _, err := runSame(t, m, c.code); err == nil || !strings.Contains(messages.ErrorIn(err, "tr"), want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", c.code, want, err)
		}
	}
	if _, err := runSame(t, m, "u_tensor.TR_encode"); err == nil || strings.Contains(messages.ErrorIn(err, "tr"), "?") {
		t.Errorf("tensörde kodla önerilmemeli: %v", err)
	}
}
