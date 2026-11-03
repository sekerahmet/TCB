package checker

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yerleşik fonksiyonun argümanı (13.9, 13.11): tanımın A|B|C parametresi birden çok imzadır (13.10
// "Birden çok imza"); argüman en az bir seçeneğe uymalıdır. Denetim çalışma anının kodunu kullanır
// (messages/README.md, 13.11'in istisnası): motor seçenek dışı tipe fonksiyona ve verilen tipe göre
// H2423 (TR_abs("s")) ya da alanının kodunu (H6103: TR_sin("s")) verir; FunctionArgCodes bunu ölçülmüş
// olarak tutar. Yalnız tablodaki fonksiyonlar denetlenir: tanımın seçenekleri motorla anlamca uyuşan
// ve kodunun parametreleri ({fn}, {given} ya da H2423'ünkiler) denetimde doldurulabilenler. Motorun
// önerili kodla reddettiği tipler (liste: H6104 tensöre çevirmeyi, Tam: H6020 şekil listesini önerir)
// denetimde karar vermez. Tam sayı ondalık seçeneğe uyar (3.13); isteğe bağlı değer (T?) ancak bir
// seçenek TR_NoneType ise açılmadan geçer, değilse yokluğun koduyla (TR_abs(l.TR_first) H2423); ad
// 4b'ye kalır. Tipi bilinmeyen, JSON, fonksiyon ve tip değeri argümanı ile örneklenmemiş tipler
// (aralık gibi) denetlenmez. Dönüşün imzadan seçilmesi 4a5'tedir.

// argCodes bir fonksiyonun seçenek dışı argümana çalışma anındaki kodu ve denetimin karar vermediği
// (çalışma anının önerili kodu kalan) verilen tiplerdir.
type argCodes struct {
	code messages.Code
	open []codes.Code
}

// FunctionArgCodes are the builtin functions whose positional arguments the checker checks, with the
// run-time code for an argument outside the options (stdlib.TestFunctionParamOptionsMatchEngine).
var FunctionArgCodes = map[codes.Code]argCodes{
	codes.FnAbs:         {messages.H2423, []codes.Code{codes.TypeList}},
	codes.FnAcos:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnArgsort:     {messages.H6203, []codes.Code{codes.TypeList}},
	codes.FnAsin:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnAtan:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnAtan2:       {messages.H6032, []codes.Code{codes.TypeList}},
	codes.FnCeil:        {messages.H2423, []codes.Code{codes.TypeList}},
	codes.FnCommandLine: {messages.H2423, []codes.Code{}},
	codes.FnCos:         {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnCosh:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnCumsum:      {messages.H6203, []codes.Code{codes.TypeList}},
	codes.FnEnv:         {messages.H2423, []codes.Code{}},
	codes.FnExp:         {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnExpm1:       {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnFloor:       {messages.H2423, []codes.Code{codes.TypeList}},
	codes.FnGelu:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnIsFinite:    {messages.H2423, []codes.Code{codes.TypeList}},
	codes.FnLayerNorm:   {messages.H6032, []codes.Code{codes.TypeFloat, codes.TypeInt}},
	codes.FnLog:         {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnLog10:       {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnLog1p:       {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnLog2:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnNoGrad:      {messages.H6404, []codes.Code{}},
	codes.FnOnes:        {messages.H6021, []codes.Code{codes.TypeInt}},
	codes.FnParseCsv:    {messages.H2423, []codes.Code{}},
	codes.FnRandn:       {messages.H6021, []codes.Code{codes.TypeInt}},
	codes.FnRelu:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnRmsNorm:     {messages.H6032, []codes.Code{codes.TypeFloat, codes.TypeInt}},
	codes.FnRound:       {messages.H2423, []codes.Code{}},
	codes.FnSigmoid:     {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnSign:        {messages.H2423, []codes.Code{codes.TypeList}},
	codes.FnSilu:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnSin:         {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnSinh:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnSqrt:        {messages.H2423, []codes.Code{}},
	codes.FnTan:         {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnTanh:        {messages.H6103, []codes.Code{codes.TypeList}},
	codes.FnTopk:        {messages.H6203, []codes.Code{codes.TypeBool, codes.TypeDict, codes.TypeFloat, codes.TypeList, codes.TypeNoneType, codes.TypeString, codes.TypeTensor}},
	codes.FnZeros:       {messages.H6021, []codes.Code{codes.TypeInt}},
}

// ProbedTypes are the given types the engine was probed with; the checker decides only for them.
var ProbedTypes = []codes.Code{codes.TypeInt, codes.TypeFloat, codes.TypeString, codes.TypeBool, codes.TypeList, codes.TypeDict, codes.TypeTensor, codes.TypeNoneType}

// ArgTypeCode is the code the check gives when k gets an argument of type typ outside its options;
// ok is false when k is not checked or typ is left to run time.
func ArgTypeCode(k, typ codes.Code) (messages.Code, bool) {
	a, ok := FunctionArgCodes[k]
	if !ok || slices.Contains(a.open, typ) || !slices.Contains(ProbedTypes, typ) {
		return 0, false
	}
	return a.code, true
}

// ParamOptions is the list of types a positional parameter of a builtin's signature accepts when the
// checker checks it: every alternative a core type written as itself (no TR_Any, type parameter or
// element type).
func ParamOptions(p codes.TypeExpr) ([]codes.Code, bool) {
	if len(p.Types) == 0 {
		return nil, false
	}
	for i, k := range p.Types {
		if k == codes.TypeAny || p.Text[i] != codes.Spelling(k) || k != codes.TypeNoneType && !coreTypes[codes.Name(k)] {
			return nil, false
		}
	}
	return p.Types, true
}

// OptionAccepts says whether a value of the core type typ fits one of the options: its own type, or
// TR_Int where TR_Float is accepted (3.13).
func OptionAccepts(opts []codes.Code, typ codes.Code) bool {
	return slices.Contains(opts, typ) || typ == codes.TypeInt && slices.Contains(opts, codes.TypeFloat)
}

// checkBuiltinArgs yerleşik fonksiyon çağrısının konumlu argümanlarını seçeneklere göre denetler.
func (c *Checker) checkBuiltinArgs(e *ast.CallExpression, argTypes []string) {
	k, recv := c.calledBuiltin(e)
	if _, checked := FunctionArgCodes[k]; recv != nil || !checked {
		return
	}
	def, _ := codes.DefinitionOf(k)
	for i, arg := range e.Arguments {
		p, ok := paramAt(def.Signature, i)
		opts, checked := ParamOptions(p)
		if !ok || !checked {
			continue
		}
		if typ, bad := c.outsideOptions(opts, argTypes[i], arg); bad {
			if code, decided := ArgTypeCode(k, typ); decided {
				c.addErrorAt(arg, c.argTypeError(code, k, i+1, opts, argTypes[i], arg))
			}
		}
	}
}

// outsideOptions argümanın (tipi given) seçeneklerin dışında olduğunu ve çalışma anında hangi tipte
// geleceğini söyler: açılmamış isteğe bağlı değer yokluk olarak gelir. Karar verilemeyen argüman
// dışarıda sayılmaz.
func (c *Checker) outsideOptions(opts []codes.Code, given string, arg ast.Expression) (codes.Code, bool) {
	none := slices.Contains(opts, codes.TypeNoneType)
	if given == typeNull {
		return codes.TypeNoneType, !none
	}
	if !c.knownType(given) || requiredType(given) == typeJSON || functionType(given) || given == typeType {
		return codes.None, false
	}
	if _, name := arg.(*ast.Identifier); shapeOf(given).optional && !none && !name {
		return codes.TypeNoneType, true // açılmamış isteğe bağlı değer (3.5)
	}
	typ, found := codes.Find(codes.NamespaceType, normalizeTypeName(baseType(requiredType(given))))
	if !found || shapeOf(given).elems != nil {
		return codes.None, false
	}
	for _, o := range opts {
		if o != codes.TypeNoneType && OptionAccepts([]codes.Code{o}, typ) {
			return codes.None, false
		}
	}
	return typ, true
}

// argTypeError k'nın order. argümanının seçenek dışı tipte olduğu hatadır, çalışma anının koduyla:
// H2423 seçenekleri sayar, öbür kodlar ({fn}, {given}) fonksiyonun adını ve verilen tipi söyler.
func (c *Checker) argTypeError(code messages.Code, k codes.Code, order int, opts []codes.Code, given string, arg ast.Expression) error {
	if code != messages.H2423 {
		return messages.New(code, codes.NameRef(k), c.typeArg(given))
	}
	var names typeChoices
	for _, o := range opts {
		if o != codes.TypeNoneType {
			names = append(names, c.typeArg(codes.Name(o)))
		}
	}
	return messages.New(messages.H2423, codes.CallNameRef(k), order, names, c.typeArg(given), c.openHint(requiredType(given), given, arg))
}
