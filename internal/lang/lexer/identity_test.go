package lexer

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (2.3a): yapı gövdesinde bir işleçle biten satırdan sonraki satır değerin devamıdır; oradaki
// kimlik öbür yerdedir, üye değil. Bit işleçleri (4.12) de bu işleçlerdendir: TR_parameters hem
// fonksiyon hem üye olduğu için yanlış yer yanlış kodu verirdi.
func TestALineAfterABitOperatorContinuesTheValue(t *testing.T) {
	for _, op := range []string{"&", "|", "^", "<<", ">>"} {
		src := "TR_struct A {\n    x: TR_Int = 1 " + op + "\n        TR_parameters(m)\n}\n"
		found := false
		for _, tok := range allOf(NewBody(src, true)) {
			if tok.Literal == "TR_fn_parameters" {
				found = true
			}
			if tok.Literal == "TR_mem_parameters" {
				t.Errorf("%q sonrasındaki satırda TR_parameters üye okundu, fonksiyon olmalıydı:\n%s", op, src)
			}
		}
		if !found {
			t.Errorf("%q sonrasındaki satırda TR_fn_parameters bulunamadı:\n%s", op, src)
		}
	}
}

// Kural (3.5, 7.1): satır sonundaki isteğe bağlı alan tipinin '?'si işleç değildir; sonraki satırın
// adı yine yapının üyesidir (TR_mem_parameters), devam eden bir değerin adı değil.
func TestALineAfterAnOptionalFieldTypeIsAMember(t *testing.T) {
	src := "TR_struct A {\n    x: TR_Int?\n    TR_parameters: TR_Int?\n}\n"
	found := false
	for _, tok := range allOf(NewBody(src, true)) {
		if tok.Literal == "TR_mem_parameters" {
			found = true
		}
		if tok.Literal == "TR_fn_parameters" {
			t.Errorf("'?' ile biten satırdan sonra TR_parameters fonksiyon okundu, üye olmalıydı:\n%s", src)
		}
	}
	if !found {
		t.Errorf("'?' ile biten satırdan sonra TR_mem_parameters bulunamadı:\n%s", src)
	}
}

// Kural (3.12, 17.6): eski çevirme fonksiyonları tip çağrısı oldu; TR_ ile yazılmış eski ya da başka
// dilin adı, aynı yazımlı bir üye kodu ya da yazımca yakın bir kimlik yerine tipi önerir; aralık için
// ileti 0..n'yi ve TR_List(0..n)'yi söyler. Öneri kopyalanınca çalışan yazımdır.
func TestAnOldConversionNameSuggestsTheType(t *testing.T) {
	for lit, want := range map[string]string{
		"TR_str": "TR_String", "TR_int": "TR_Int", "TR_type": "TR_Type", "TR_checkpoint": "TR_Checkpoint",
		"TR_tensor": "TR_Tensor", "TR_embedding": "TR_embedding_lookup", "TR_None": "TR_none", "TR_def": "TR_func",
	} {
		err := identityError(codes.PlaceOther, lit)
		if !messages.Has(err, messages.H0026) || !strings.Contains(err.Error(), "'"+want+"'") {
			t.Errorf("%s: %v; want H0026 suggesting %s", lit, err, want)
		}
	}
	if err := identityError(codes.PlaceOther, "TR_range"); !messages.Has(err, messages.H0242) {
		t.Errorf("TR_range: %v; want H0242", err)
	}
	// Tipe giden tanıdık ad aynı yazımlı üyenin önüne geçmez: map TR_Dict'in tanıdık adıdır, TR_map üyedir.
	if err := identityError(codes.PlaceOther, "TR_map"); !messages.Has(err, messages.H0027) {
		t.Errorf("TR_map: %v; want H0027 (x.TR_map)", err)
	}
	// Yazım hatasına en yakın kimlik önerilir.
	if err := identityError(codes.PlaceOther, "TR_prnt"); !strings.Contains(err.Error(), "'TR_print'") {
		t.Errorf("TR_prnt: %v", err)
	}
}

// Kural (2.3a, 17.9): program metninde sınıflı kimlik (TR_<sınıf>_<ad>) yazılırsa H0037 yazılanı
// anar ve doğru yazımı önerir; program sınıflarında önerilen yazım çalışır ve sözcüğü değiştirmek
// yetiyorsa düzeltme applicable'dır. Gerçek kimlik olmayan sınıflı yazım bugünkü H0001 yolunda kalır.
func TestClassPrefixedIdentitiesSuggestWorkingSpelling(t *testing.T) {
	if len(codes.Classes) != 14 {
		t.Fatalf("14 sınıf bekleniyordu, %d bulundu", len(codes.Classes))
	}
	samples := map[codes.Class]struct {
		code codes.Code
		src  string
		want string
	}{
		codes.ClassKeyword:     {codes.KwIf, "TR_kw_if TR_true { TR_print(1) }", "TR_if"},
		codes.ClassContextual:  {codes.CtxAs, "TR_import \"tr/math\" TR_ctx_as m\nTR_print(1)", "TR_as"},
		codes.ClassValueName:   {codes.ValInf, "TR_print(TR_val_inf)", "TR_inf"},
		codes.ClassType:        {codes.TypeInt, "TR_var x: TR_type_Int = 1\nTR_print(x)", "TR_Int"},
		codes.ClassFunction:    {codes.FnPrint, "TR_fn_print(\"x\")", "TR_print(...)"},
		codes.ClassConstant:    {codes.ConstPi, "TR_print(TR_const_pi)", "TR_pi"},
		codes.ClassNamedArg:    {codes.ArgMax, "TR_print(TR_clamp(5, TR_min: 0, TR_arg_max: 3))", "TR_max:"},
		codes.ClassMember:      {codes.MemMap, "TR_print([1, 2].TR_mem_map((x) => x + 1))", ".TR_map"},
		codes.ClassCommand:     {codes.CmdRun, "TR_cmd_run", "tr run"},
		codes.ClassSubcommand:  {codes.SubDefinition, "TR_sub_definition", "tr index definition"},
		codes.ClassOption:      {codes.OptLang, "TR_opt_lang", "--lang"},
		codes.ClassShell:       {codes.ReplExit, "TR_repl_exit", "exit"},
		codes.ClassDebugger:    {codes.DbgStep, "TR_dbg_step", "step"},
		codes.ClassEnvironment: {codes.EnvLang, "TR_env_LANG", "TR_LANG"},
	}
	for _, c := range codes.Classes {
		s, ok := samples[c.Class]
		if !ok || s.code.Class() != c.Class {
			t.Fatalf("%s sınıfı için örnek eksik ya da yanlış: %+v", c.Class, s)
		}
		checkClassedSample(t, s.code, s.src, s.want)
	}
	if _, err := classedIdentityError(codes.OptGoProfile.String()); err == nil || !strings.Contains(err.Error(), "'--go-profile'") {
		t.Errorf("TR_opt_go_profile: %v", err)
	}
	for _, src := range []string{"TR_mem_map(x)", "TR_arg_max"} {
		l := NewBody(src, true)
		allOf(l)
		if len(l.Errs()) != 1 || !messages.Has(l.Errs()[0], messages.H0037) || l.Errs()[0].(*messages.PosError).Fixes[0].Applicable {
			t.Errorf("%q: applicable olmayan H0037 bekleniyordu: %+v", src, l.Errs())
		}
	}
	if l := NewBody(codes.IdentityPrefix+string(codes.ClassFunction)+"_yok", true); allOf(l) == nil || len(l.Errs()) != 1 || !messages.Has(l.Errs()[0], messages.H0001) {
		t.Errorf("var olmayan sınıflı kimlik H0001 vermeli: %v", l.Errors())
	}
}

func checkClassedSample(t *testing.T, k codes.Code, src, want string) {
	t.Helper()
	l := NewBody(src, true)
	allOf(l)
	if len(l.Errs()) != 1 || !messages.Has(l.Errs()[0], messages.H0037) {
		t.Fatalf("%s (%s): H0037 bekleniyordu, gelen: %v", k.Class(), k, l.Errors())
	}
	pe := l.Errs()[0].(*messages.PosError)
	trMsg, enMsg := messages.ErrorIn(pe, "tr"), messages.ErrorIn(pe, "en")
	for _, msg := range []string{trMsg, enMsg} {
		if !strings.Contains(msg, "'"+k.String()+"'") || !strings.Contains(msg, "'"+want+"'") {
			t.Errorf("%s: ileti %q ve %q içermeli: %s", k.Class(), k.String(), want, msg)
		}
	}
	if _, inProgram := codes.PlaceOf(k); !inProgram {
		if !strings.Contains(trMsg, "bu bir komut satırı adıdır") || !strings.Contains(enMsg, "this is a command-line name") || len(pe.Fixes) != 0 {
			t.Errorf("%s: komut satırı iletisi ya da düzeltmesi hatalı:\ntr: %s\nen: %s\nfixes: %+v", k.Class(), trMsg, enMsg, pe.Fixes)
		}
		return
	}
	if len(pe.Fixes) != 1 || !pe.Fixes[0].Applicable || pe.Fixes[0].Replacement != codes.Spelling(k) {
		t.Fatalf("%s: düzeltme %+v, beklenen applicable replacement=%q", k.Class(), pe.Fixes, codes.Spelling(k))
	}
	fixed := strings.Replace(src, k.String(), pe.Fixes[0].Replacement, 1)
	if l2 := NewBody(fixed, true); allOf(l2) == nil || len(l2.Errs()) != 0 {
		t.Errorf("%s: önerilen yazım (%q) hatasız okunmalı: %v", k.Class(), fixed, l2.Errors())
	}
}
