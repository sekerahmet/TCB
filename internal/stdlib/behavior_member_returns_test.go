package stdlib

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// memberProbe örneğin başına eklenir: member_recv(x) alıcıyı, member_ret(x) üyenin değerini kaydeder
// ve x'i olduğu gibi döndürür. Kayıt değerin çalışma anındaki tipi ve kapta öğelerinin tipleridir
// (çalışma anı öğe tipini taşımaz, öğeler taşır): "List[Int|String]", "Dict[String;Int]", "Int".
// Alıcı argümanlardan önce hesaplandığı için kayıtlar bir yığınla eşlenir (memberPairs). Parametrenin
// yazılan tipi (TR_String) her değeri alır: çalışma anı parametre tipini zorlamaz (returnProbe'daki gibi;
// 6.10'un tip parametreli fonksiyonu bugün sözdizimi hatası).
const memberProbe = `TR_var member_log: TR_List[TR_String] = []
TR_func member_types(v: TR_String) -> TR_String {
    TR_var es: TR_List[TR_String] = []
    TR_for e TR_in v {
        TR_var s = TR_String(TR_Type(e))
        TR_if TR_not es.TR_contains(s) { es.TR_append(s) }
    }
    TR_return es.TR_sort().TR_join("|")
}
TR_func member_desc(v: TR_String) -> TR_String {
    TR_var t = TR_String(TR_Type(v))
    TR_if t == "List" { TR_return t + "[" + member_types(v) + "]" }
    TR_if t == "Dict" { TR_return t + "[" + member_types(v.TR_keys()) + ";" + member_types(v.TR_values()) + "]" }
    TR_return t
}
TR_func member_recv(x: TR_String) -> TR_String {
    member_log.TR_append("R " + member_desc(x))
    TR_return x
}
TR_func member_ret(x: TR_String) -> TR_String {
    member_log.TR_append("V " + member_desc(x))
    TR_return x
}`

const memberMarker = "MEMBER_LOG"

// coreReceivers üye dönüşü denetleyicide kullanılan alıcı tipleridir (checker.countedReceivers ile aynı).
var coreReceivers = []codes.Code{codes.TypeString, codes.TypeList, codes.TypeDict, codes.TypeRange}

// Kural (1.2, 13.9; 4a2'nin ön şartı, (b) örnekle): denetleyici çekirdek alıcıda üyenin dönüşünü
// alıcıdaki imzadan, tip parametrelerini alıcının öğe tipinden kurar (checker.MemberReturn). Her
// üyenin tanım örneği, üyenin her kullanımında alıcı ve değer kaydedilerek koşulur; değerin tipi ve
// öğelerinin tipi denetleyicinin o alıcı için kurduğu tipe uymalıdır. Sınanamayan (örnekte o alıcıda
// kullanımı ya da öğe tipi belli alıcısı olmayan) ve uyuşmayan alıcılar
// checker.MemberReturnsNotVerified'dadır; doğrulanan alıcı listeden çıkar.
func TestMemberReturnsMatchEngine(t *testing.T) {
	chdirTemp(t)
	verified := 0
	var names []string
	for _, k := range codes.Codes() {
		def, _ := codes.DefinitionOf(k)
		if k.Namespace() != codes.NamespaceMember || k.IsLibrary() || def.Kind != codes.KindMethod && def.Kind != codes.KindProperty {
			continue
		}
		if _, skip := checker.ReturnsNotVerified[k]; skip {
			continue
		}
		results := probeMemberReturns(t, k, def)
		for _, recv := range coreReceivers {
			sig, ok := memberSig(def, recv)
			if !ok || len(sig.Return.Text) != 1 || neverTyped(sig.Return.Text[0], recv) {
				continue
			}
			got, tried := results[recv]
			listed := slices.Contains(checker.MemberReturnsNotVerified[k], recv)
			switch {
			case tried && got == "" && listed:
				t.Errorf("%s (%s): dönüşü (%s) doğrulandı; checker.MemberReturnsNotVerified'dan çıkarın", k, recv, sig.Return.Text[0])
			case tried && got == "":
				verified++
				names = append(names, k.String()+"/"+recv.String())
			case !listed && !tried:
				t.Errorf("%s (%s): dönüşü (%s) örnekte sınanamadı; checker.MemberReturnsNotVerified'a yazın", k, recv, sig.Return.Text[0])
			case !listed:
				t.Errorf("%s (%s): tanımın dönüşü %s, örnekte %s", k, recv, sig.Return.Text[0], got)
			}
		}
	}
	t.Logf("%d üye-alıcı dönüşü doğrulandı: %s", verified, strings.Join(names, " "))
	if verified < 20 {
		t.Errorf("yalnız %d üye-alıcı dönüşü doğrulandı", verified)
	}
}

// neverTyped dönüş yazımından denetleyici hiçbir alıcıda tip kurmuyor mu (TR_Any, birleşim, kitaplık
// tipi): öyleyse denetleyici onu kullanmaz, sınanması gerekmez.
func neverTyped(text string, recv codes.Code) bool {
	for _, r := range []string{recv.String(), "TR_type_List[TR_type_Int]", "TR_type_Dict[TR_type_String, TR_type_Int]"} {
		if checker.MemberReturn(text, r) != "" {
			return false
		}
	}
	return true
}

func memberSig(def codes.Definition, recv codes.Code) (codes.Signature, bool) {
	for _, m := range def.Members {
		if m.Type == recv {
			return m.Signature, true
		}
	}
	return codes.Signature{}, false
}

// probeMemberReturns k'nın örneğini, k'nın her kullanımında alıcısı ve değeri kaydedilerek koşar.
// Sonuç alıcı tipi → "" (her kullanım uydu) ya da uyuşmazlığın tarifi; denetleyicinin tip kuramadığı
// (öğe tipi belli olmayan alıcı) kullanım sayılmaz, kullanımı sayılmayan alıcı sonuçta yoktur.
func probeMemberReturns(t *testing.T, k codes.Code, def codes.Definition) map[codes.Code]string {
	prog, _ := exampleProgram(t, k)
	if prog == nil {
		return nil
	}
	wrapMemberUses(t, prog, k)
	probe := parseCoded(t, memberProbe)
	tail := parseCoded(t, `TR_print("`+memberMarker+`", member_log.TR_join(" ; "))`)
	prog.Statements = append(append(probe.Statements, prog.Statements...), tail.Statements...)
	out := runExampleOutput(t, prog)
	i := strings.LastIndex(out, memberMarker+" ")
	if i < 0 {
		return nil
	}
	results := map[codes.Code]string{}
	for _, p := range memberPairs(strings.SplitN(out[i+len(memberMarker)+1:], "\n", 2)[0]) {
		recv, recvType, ok := runtimeType(p[0])
		sig, found := memberSig(def, recv)
		if !ok || !found || len(sig.Return.Text) != 1 {
			continue
		}
		want := checker.MemberReturn(sig.Return.Text[0], recvType)
		if want == "" {
			continue
		}
		if _, seen := results[recv]; !seen {
			results[recv] = ""
		}
		if !valueFits(want, p[1]) && results[recv] == "" {
			results[recv] = p[0] + " -> " + p[1] + " (" + want + ")"
		}
	}
	return results
}

// memberPairs kayıt dizisini (R alıcı, V değer) yığınla alıcı-değer çiftlerine eşler.
func memberPairs(log string) [][2]string {
	var stack []string
	var out [][2]string
	for _, ev := range strings.Split(strings.TrimSpace(log), " ; ") {
		switch {
		case strings.HasPrefix(ev, "R "):
			stack = append(stack, ev[2:])
		case strings.HasPrefix(ev, "V ") && len(stack) > 0:
			out = append(out, [2]string{stack[len(stack)-1], ev[2:]})
			stack = stack[:len(stack)-1]
		}
	}
	return out
}

// runtimeType çalışma anı kaydının (member_desc) tip kodu ve denetleyicideki tip metnidir: tek öğe
// tipli kap öğe tipiyle (TR_List[TR_Int]), öbürleri öğesiz.
func runtimeType(desc string) (codes.Code, string, bool) {
	base, inner, _ := strings.Cut(strings.TrimSuffix(desc, "]"), "[")
	k, err := codes.Parse("TR_type_" + base)
	if err != nil {
		return codes.None, "", false
	}
	text := k.String()
	parts := strings.Split(inner, ";")
	single := func(s string) (string, bool) {
		e, err := codes.Parse("TR_type_" + s)
		return e.String(), err == nil && s != "" && !strings.Contains(s, "|")
	}
	switch {
	case k == codes.TypeList:
		if e, ok := single(parts[0]); ok {
			text += "[" + e + "]"
		}
	case k == codes.TypeDict && len(parts) == 2:
		kk, ok1 := single(parts[0])
		vv, ok2 := single(parts[1])
		if ok1 && ok2 {
			text += "[" + kk + ", " + vv + "]"
		}
	}
	return k, text, true
}

// valueFits çalışma anı kaydı desc (member_desc) denetleyicinin tipi want'a uyuyor mu: isteğe bağlı
// tipte yokluk da uyar; kabın her öğesinin tipi öğe tipinin asıl adıdır (boş kap uyar).
func valueFits(want, desc string) bool {
	if inner, ok := strings.CutSuffix(want, "?"); ok {
		return desc == "NoneType" || valueFits(inner, desc)
	}
	base, elems, hasElems := strings.Cut(want, "[")
	gotBase, gotInner, _ := strings.Cut(strings.TrimSuffix(desc, "]"), "[")
	if base != "TR_type_"+gotBase {
		return false
	}
	if !hasElems || base != codes.TypeList.String() || gotInner == "" {
		return true
	}
	elem, _, _ := strings.Cut(strings.TrimSuffix(elems, "]"), "[")
	for _, g := range strings.Split(gotInner, "|") {
		if "TR_type_"+g != strings.TrimSuffix(elem, "?") && !(strings.HasSuffix(elem, "?") && g == "NoneType") {
			return false
		}
	}
	return true
}

// wrapMemberUses k üyesinin her kullanımını sarar: alıcı member_recv(...) ile, değer (metotta çağrı,
// özellikte üye ifadesi) member_ret(...) ile. Çağrının fonksiyonu olan üye ifadesi ayrıca sarılmaz.
func wrapMemberUses(t *testing.T, prog *ast.Program, k codes.Code) {
	targets := map[ast.Expression]bool{}
	callees := map[*ast.MemberExpression]bool{}
	ast.Walk(prog, func(n any) bool {
		switch e := n.(type) {
		case *ast.CallExpression:
			if m, ok := e.Function.(*ast.MemberExpression); ok && m.NameToken.Code == k {
				targets[e], callees[m] = true, true
			}
		case *ast.MemberExpression:
			if e.NameToken.Code == k && !callees[e] {
				targets[e] = true
			}
		}
		return true
	})
	for e := range targets {
		m, _ := e.(*ast.MemberExpression)
		if c, isCall := e.(*ast.CallExpression); isCall {
			m = c.Function.(*ast.MemberExpression)
		}
		m.Object = wrapCall(t, "member_recv", m.Object)
	}
	replaceTargets(t, reflect.ValueOf(prog), targets)
}

// replaceTargets ağaçta targets'taki her ifadeyi member_ret(ifade) ile değiştirir (önce çocuklar).
func replaceTargets(t *testing.T, v reflect.Value, targets map[ast.Expression]bool) {
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		replaceTargets(t, v.Elem(), targets)
		if e, ok := v.Interface().(ast.Expression); ok && targets[e] && v.CanSet() {
			delete(targets, e)
			v.Set(reflect.ValueOf(wrapCall(t, "member_ret", e)))
		}
	case reflect.Pointer:
		if !v.IsNil() {
			replaceTargets(t, v.Elem(), targets)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() && v.Type().Field(i).Type.Name() != "Token" {
				replaceTargets(t, v.Field(i), targets)
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			replaceTargets(t, v.Index(i), targets)
		}
	}
}

// wrapCall fn(arg) çağrısıdır.
func wrapCall(t *testing.T, fn string, arg ast.Expression) ast.Expression {
	c := parseCoded(t, fn+"(0)").Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
	c.Arguments[0] = arg
	return c
}
