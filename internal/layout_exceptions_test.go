package internal_test

// Kod düzeni istisnalarının gerekçeleri (tavanları: internal/layoutrules/exceptions.go). Her kaydın bir gerekçesi
// vardır, gerekçesi olan her kayıt listededir (checkExceptions). Gerekçe geliştiricinin notudur, metin sabiti olarak
// burada (test dosyası) durur: kural paketi yalnızca sayıları taşır.

// Ortak gerekçeler.
const (
	reasonHotPath   = "sıcak yol: tek döngü ya da tek kapanış kümesi; bölünmesi ölçümle (önce/sonra kıyas) yapılmalı"
	reasonDispatch  = "tür/komut başına dağıtım (tek switch): dallar kısa; dalları kendi fonksiyonlarına almak sıradaki düzen işi"
	reasonGenerator = "bakım programı (kullanıcıya gitmez): çıktıyı tek geçişte sırayla yazar"
	reasonRead      = "dosya biçimi okuma/doğrulama: adımlar sıralı ve birbirine bağlı tek geçiş"
	reasonBuiltin   = "yerleşik gövdesi: argüman denetimi ile iş bir arada; yardımcılara bölünebilir"
	reasonTable     = "tablo kurulumu: her kayıt birkaç satırlık bir kapanış; tip ya da konu başına bölünebilir"
	reasonKernel    = "hesap çekirdeği (tensör ve GPU sahibinin alanı): hız ölçümüyle birlikte bölünmeli"
	reasonTokenizer = "parçalayıcı/düzenli ifade çekirdeği (veri ve parçalayıcı sahibinin alanı): başvuru uygulamasıyla birebir; hız ölçümüyle bölünmeli"
)

// bigFileReasons layoutrules.BigFiles'ın gerekçeleridir.
var bigFileReasons = map[string]string{
	"internal/compute/tokenize/regex.go":          "düzenli ifade ayrıştırıcısı ve sınıf tabloları; " + reasonTokenizer,
	"internal/compute/tokenize/regex_match.go":    "düzenli ifade eşleştiricisi; " + reasonTokenizer,
	"internal/compute/tokenize/tokenizer_test.go": "altın çıktı testleri ve kıyaslar (veri ve parçalayıcı sahibinin alanı); model türüne göre bölünebilir",
	"internal/compute/tensor/cpu_backend.go":      "CPU arka ucunun işlem tabloları ve döngüleri; " + reasonKernel,
	"internal/compute/dataload/cache.go":          "önbellek durum makinesi ve kilitleri tek tipte (veri sahibinin alanı); getir/yaz/temizle olarak bölünebilir",
}

// longFuncReasons layoutrules.LongFuncs'ın gerekçeleridir.
var longFuncReasons = map[string]string{
	// Dil çekirdeği
	"internal/lang/interp/compiler_loop.go:compiler.inLoop":     reasonHotPath + " (liste/aralık/sözlük/metin döngülerinin hızlı yolları)",
	"internal/lang/interp/compiler_expr.go:compiler.expression": reasonDispatch,
	"internal/lang/interp/compiler_op.go:compiler.binary":       reasonHotPath + " (işleç başına tam sayı/ondalık hızlı yolları)",
	"internal/lang/interp/compiler_call.go:compiler.call":       reasonHotPath + " (yerleşik, metot ve fonksiyon çağrısının hızlı yolları)",
	"internal/lang/interp/operators.go:valuesEqual":             reasonDispatch + " (değer tipi başına eşitlik)",
	"internal/lang/interp/methods.go:setupListMethods":          reasonTable,
	"internal/lang/checker/checker.go:Checker.checkStatement":   reasonDispatch,
	"internal/lang/checker/checker.go:Checker.inferExpression":  reasonDispatch,
	"internal/lang/codes/language.go:load":                      reasonRead + " (dil dosyası)",
	"internal/lang/codes/definition.go:CheckDefinitions":        reasonRead + " (tanım dosyasının tutarlılık denetimleri)",
	"internal/lang/codes/definition.go:ParseDefinitions":        reasonRead + " (tanım dosyası)",

	// Araçlar
	"internal/devtools/debugger/debugger.go:Debugger.promptCLI": reasonDispatch + " (terminal hata ayıklayıcısının komutları)",
	"internal/devtools/formatter/format.go:format":              "biçimlendiricinin sözcük birimi döngüsü: girinti ve boşluk durumu tek döngüde",
	"tools/gen/codesdoc/main.go:Generate":                       reasonGenerator,
	"tools/gen/grammar/main.go:Generate":                        reasonGenerator,
	"tools/bench/go/compare.go:main":                            reasonGenerator + " (kıyas programları sırayla)",

	// Motorlar
	"internal/stdlib/ml/custom_grad.go:checkGrad":     reasonBuiltin + " (sayısal türev denetimi)",
	"internal/stdlib/ml/print.go:TensorValue.Text":    "tensör yazdırma: boyut, kısaltma ve biçim tek özyinelemede",
	"internal/stdlib/ml/model_tools.go:sampleBuiltin": reasonBuiltin,
	"internal/stdlib/ml/builtin_reduce.go:reduction":  reasonBuiltin,

	// Hesap çekirdekleri
	"internal/compute/tensor/cpu_attention.go:cpuBackend.f32Attention": reasonKernel,
	"internal/compute/tensor/cpu_matmul.go:gemmPart":                   reasonKernel,
	"internal/compute/tensor/autograd.go:Tensor.BackwardCtx":           reasonKernel,
	"internal/compute/tensor/cpu_backend.go:cpuBackend.f32BinaryGrad":  reasonKernel,
	"internal/compute/tensor/cpu_backend.go:unaryGradLoop":             reasonKernel,
	"internal/compute/tokenize/regex_match.go:reMatcher.run":           reasonTokenizer,
	"internal/compute/tokenize/tokenizer.go:Parse":                     reasonRead + " (tokenizer.json)",
	"internal/compute/tokenize/regex.go:reParser.group":                reasonTokenizer,
	"internal/compute/tokenize/model_bpe.go:newBPE":                    reasonTokenizer,
	"internal/compute/tokenize/model_other.go:unigram.encode":          reasonTokenizer,
	"internal/compute/tokenize/regex.go:reParser.classInner":           reasonTokenizer,
	"internal/compute/dataload/cache.go:cacheCore.fetch":               "önbellekten getirme: kilit, bayatlık ve kopyalama durumları tek akışta (veri sahibinin alanı)",
}
