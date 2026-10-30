package layoutrules

// Kod düzeni standardının bugünkü istisnaları (kurallar: internal/layout_test.go, docs/development.md). Gerekçeleri
// internal/layout_exceptions_test.go'dadır (her kaydın gerekçesi vardır; TestLayoutFileSize ve TestLayoutFuncSize
// denetler).
//
// Listeler YALNIZCA KÜÇÜLEBİLİR: bir dosya ya da fonksiyon sınırın altına inince kaydı
// silinir ve en fazla sayısı (Max...) aynı miktarda azaltılır. Yeni kayıt eklenmez;
// tavan yükseltilmez (küçük düzeltmeler için tavanın %10'u, en az 5 satır pay vardır; bkz. FixMargin). Taşınan bir fonksiyonun anahtarı değişirse eski kayıt silinip yenisi
// aynı tavanla yazılabilir (sayı artmaz).

const MaxBigFiles = 5

// BigFiles FileLimit'i aşan el yazımı dosyaların tavanıdır (bugünkü satır sayısı; üretilmiş dosyalar sayılmaz).
var BigFiles = map[string]int{
	"internal/compute/tokenize/regex.go":          915,
	"internal/compute/tokenize/regex_match.go":    615,
	"internal/compute/tokenize/tokenizer_test.go": 737,
	"internal/compute/tensor/cpu_backend.go":      737,
	"internal/compute/dataload/cache.go":          795,
}

const MaxLongFuncs = 40

// LongFuncs FuncLimit'i aşan test dışı fonksiyonların ("dosya:Alici.Ad") tavanıdır (bugünkü satır sayısı).
var LongFuncs = map[string]int{
	// Dil çekirdeği
	"internal/lang/interp/compiler_loop.go:compiler.inLoop":     172,
	"internal/lang/interp/compiler_expr.go:compiler.expression": 168,
	"internal/lang/interp/compiler_op.go:compiler.binary":       132,
	"internal/lang/interp/compiler_call.go:compiler.call":       95,
	"internal/lang/interp/operators.go:valuesEqual":             90,
	"internal/lang/interp/methods.go:setupListMethods":          88,
	"internal/lang/checker/checker.go:Checker.checkStatement":   147,
	"internal/lang/codes/language.go:load":                      160,
	"internal/lang/codes/definition.go:CheckDefinitions":        113,
	"internal/lang/codes/definition.go:ParseDefinitions":        96,

	// Araçlar
	"internal/devtools/debugger/debugger.go:Debugger.promptCLI": 187,
	"internal/devtools/formatter/format.go:format":              119,
	"tools/gen/codesdoc/main.go:Generate":                       151,
	"tools/gen/grammar/main.go:Generate":                        107,
	"tools/bench/go/compare.go:main":                            92,

	// Motorlar
	"internal/stdlib/ml/custom_grad.go:checkGrad":     181,
	"internal/stdlib/ml/print.go:TensorValue.Text":    120,
	"internal/stdlib/ml/model_tools.go:sampleBuiltin": 118,
	"internal/stdlib/ml/builtin_reduce.go:reduction":  85,

	// Hesap çekirdekleri
	"internal/compute/tensor/cpu_attention.go:cpuBackend.f32Attention": 146,
	"internal/compute/tensor/cpu_matmul.go:gemmPart":                   115,
	"internal/compute/tensor/autograd.go:Tensor.BackwardCtx":           89,
	"internal/compute/tensor/cpu_backend.go:cpuBackend.f32BinaryGrad":  85,
	"internal/compute/tensor/cpu_backend.go:unaryGradLoop":             81,
	"internal/compute/tokenize/regex_match.go:reMatcher.run":           167,
	"internal/compute/tokenize/tokenizer.go:Parse":                     131,
	"internal/compute/tokenize/regex.go:reParser.group":                103,
	"internal/compute/tokenize/model_bpe.go:newBPE":                    99,
	"internal/compute/tokenize/model_other.go:unigram.encode":          96,
	"internal/compute/tokenize/regex.go:reParser.classInner":           89,
	"internal/compute/dataload/cache.go:cacheCore.fetch":               112,
}
