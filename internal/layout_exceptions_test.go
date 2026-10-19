package internal_test

// Kod düzeni standardının bugünkü istisnaları (kurallar: layout_test.go, docs/katki.md).
//
// Listeler YALNIZCA KÜÇÜLEBİLİR: bir dosya ya da fonksiyon sınırın altına inince kaydı
// silinir ve en fazla sayısı (max...) aynı miktarda azaltılır. Yeni kayıt eklenmez;
// tavan yükseltilmez (küçük düzeltmeler için tavanın %10'u, en az 5 satır pay vardır; bkz. fixMargin). Taşınan bir fonksiyonun anahtarı değişirse eski kayıt silinip yenisi
// aynı tavanla yazılabilir (sayı artmaz).

type exception struct {
	ceiling int    // bugünkü satır sayısı; istisna bundan uzun olamaz
	reason  string // neden şimdilik bölünmediği
}

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

const maxBigFiles = 5

// bigFiles fileLimit'i aşan el yazımı dosyalardır (üretilmiş dosyalar sayılmaz).
var bigFiles = map[string]exception{
	"internal/compute/tokenize/regex.go":          {915, "düzenli ifade ayrıştırıcısı ve sınıf tabloları; " + reasonTokenizer},
	"internal/compute/tokenize/regex_match.go":    {615, "düzenli ifade eşleştiricisi; " + reasonTokenizer},
	"internal/compute/tokenize/tokenizer_test.go": {737, "altın çıktı testleri ve kıyaslar (veri ve parçalayıcı sahibinin alanı); model türüne göre bölünebilir"},
	"internal/compute/tensor/cpu_backend.go":      {737, "CPU arka ucunun işlem tabloları ve döngüleri; " + reasonKernel},
	"internal/compute/dataload/cache.go":          {795, "önbellek durum makinesi ve kilitleri tek tipte (veri sahibinin alanı); getir/yaz/temizle olarak bölünebilir"},
}

const maxLongFuncs = 41

// longFuncs funcLimit'i aşan test dışı fonksiyonlardır ("dosya:Alici.Ad").
var longFuncs = map[string]exception{
	// Dil çekirdeği
	"internal/lang/interp/compiler_loop.go:compiler.inLoop":      {172, reasonHotPath + " (liste/aralık/sözlük/metin döngülerinin hızlı yolları)"},
	"internal/lang/interp/compiler_expr.go:compiler.expression":  {168, reasonDispatch},
	"internal/lang/interp/compiler_op.go:compiler.binary":        {132, reasonHotPath + " (işleç başına tam sayı/ondalık hızlı yolları)"},
	"internal/lang/interp/compiler_call.go:compiler.call":        {95, reasonHotPath + " (yerleşik, metot ve fonksiyon çağrısının hızlı yolları)"},
	"internal/lang/interp/operators.go:valuesEqual":              {90, reasonDispatch + " (değer tipi başına eşitlik)"},
	"internal/lang/interp/methods.go:setupListMethods":           {88, reasonTable},
	"internal/lang/lexer/lexer.go:Lexer.NextToken":               {164, reasonDispatch + " (karakter başına durum makinesi; " + reasonHotPath + ")"},
	"internal/lang/checker/checker.go:Checker.checkStatement":    {163, reasonDispatch},
	"internal/lang/checker/checker.go:Checker.inferExpression":   {133, reasonDispatch},
	"internal/lang/parser/struct.go:Parser.parseStructStatement": {97, "yapı tanımı: alanlar, varsayılanlar, metotlar ve hata kurtarma tek akışta; alan ve metot çözümlemesi ayrılabilir"},
	"internal/lang/parser/expressions.go:Parser.parseExpression": {92, reasonDispatch + " (Pratt döngüsü: önek ve ara işleç dağıtımı)"},
	"internal/lang/codes/language.go:Load":                       {160, reasonRead + " (dil dosyası)"},
	"internal/lang/codes/definition.go:CheckDefinitions":         {113, reasonRead + " (tanım dosyasının tutarlılık denetimleri)"},
	"internal/lang/codes/definition.go:ParseDefinitions":         {96, reasonRead + " (tanım dosyası)"},

	// Araçlar
	"internal/devtools/debugger/ayiklayici.go:Debugger.promptCLI": {187, reasonDispatch + " (terminal hata ayıklayıcısının komutları)"},
	"internal/devtools/formatter/bicim.go:bicimle":                {119, "biçimlendiricinin sözcük birimi döngüsü: girinti ve boşluk durumu tek döngüde"},
	"cmd/tr/bicimle.go:bicimleKomutu":                             {83, "komut satırı bayrakları ve dosya/klasör dolaşımı tek akışta"},
	"tools/gen/codesdoc/main.go:Uret":                             {151, reasonGenerator},
	"tools/gen/grammar/main.go:Uret":                              {107, reasonGenerator},
	"tools/bench/go/kiyas.go:main":                                {92, reasonGenerator + " (kıyas programları sırayla)"},

	// Motorlar
	"internal/stdlib/ml/ozel_turev.go:turevDenetle":          {181, reasonBuiltin + " (sayısal türev denetimi)"},
	"internal/stdlib/ml/yazdirma.go:Deger.Inspect":           {120, "tensör yazdırma: boyut, kısaltma ve biçim tek özyinelemede"},
	"internal/stdlib/ml/model_araclari.go:orneklemeYerlesik": {118, reasonBuiltin},
	"internal/stdlib/ml/yerlesik_indirgeme.go:indirgeme":     {85, reasonBuiltin},
	"internal/stdlib/tokenizer/parcalayici.go:Nesne.kodla":   {88, reasonBuiltin},

	// Hesap çekirdekleri
	"internal/compute/tensor/cpu_attention.go:cpuBackend.f32Attention": {146, reasonKernel},
	"internal/compute/tensor/cpu_matmul.go:gemmPart":                   {115, reasonKernel},
	"internal/compute/tensor/autograd.go:Tensor.BackwardCtx":           {89, reasonKernel},
	"internal/compute/tensor/cpu_backend.go:unaryLoop":                 {89, reasonKernel},
	"internal/compute/tensor/cpu_backend.go:cpuBackend.f32BinaryGrad":  {85, reasonKernel},
	"internal/compute/tensor/cpu_backend.go:unaryGradLoop":             {81, reasonKernel},
	"internal/compute/tokenize/regex_match.go:reMatcher.run":           {167, reasonTokenizer},
	"internal/compute/tokenize/tokenizer.go:Parse":                     {131, reasonRead + " (tokenizer.json)"},
	"internal/compute/tokenize/regex.go:reParser.group":                {103, reasonTokenizer},
	"internal/compute/tokenize/model_bpe.go:newBPE":                    {99, reasonTokenizer},
	"internal/compute/tokenize/model_other.go:unigram.encode":          {96, reasonTokenizer},
	"internal/compute/tokenize/regex.go:reParser.classInner":           {89, reasonTokenizer},
	"internal/compute/tokenize/model_bpe.go:bpeModel.symbols":          {84, reasonTokenizer},
	"internal/compute/tokenize/decoders.go:init":                       {82, reasonTable},
	"internal/compute/dataload/cache.go:cacheCore.fetch":               {112, "önbellekten getirme: kilit, bayatlık ve kopyalama durumları tek akışta (veri sahibinin alanı)"},
}
