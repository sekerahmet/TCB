package internal_test

// Kod düzeni standardının bugünkü istisnaları (kurallar: duzen_test.go, docs/katki.md).
//
// Listeler YALNIZCA KÜÇÜLEBİLİR: bir dosya ya da fonksiyon sınırın altına inince kaydı
// silinir ve en fazla sayısı (enFazla...) aynı miktarda azaltılır. Yeni kayıt eklenmez;
// tavan yükseltilmez (küçük düzeltmeler için tavanın %10'u, en az 5 satır pay vardır; bkz. duzeltmePayi). Taşınan bir fonksiyonun anahtarı değişirse eski kayıt silinip yenisi
// aynı tavanla yazılabilir (sayı artmaz).

type istisna struct {
	tavan int    // bugünkü satır sayısı; istisna bundan uzun olamaz
	neden string // neden şimdilik bölünmediği
}

// Ortak gerekçeler.
const (
	nSicakYol  = "sıcak yol: tek döngü ya da tek kapanış kümesi; bölünmesi ölçümle (önce/sonra kıyas) yapılmalı"
	nDagitim   = "tür/komut başına dağıtım (tek switch): dallar kısa; dalları kendi fonksiyonlarına almak sıradaki düzen işi"
	nUretec    = "bakım programı (kullanıcıya gitmez): çıktıyı tek geçişte sırayla yazar"
	nOkuma     = "dosya biçimi okuma/doğrulama: adımlar sıralı ve birbirine bağlı tek geçiş"
	nYerlesik  = "yerleşik gövdesi: argüman denetimi ile iş bir arada; yardımcılara bölünebilir"
	nTablo     = "tablo kurulumu: her kayıt birkaç satırlık bir kapanış; tip ya da konu başına bölünebilir"
	nCekirdek  = "hesap çekirdeği (tensör ve GPU sahibinin alanı): hız ölçümüyle birlikte bölünmeli"
	nParcalama = "parçalayıcı/düzenli ifade çekirdeği (veri ve parçalayıcı sahibinin alanı): başvuru uygulamasıyla birebir; hız ölçümüyle bölünmeli"
)

const enFazlaBuyukDosya = 5

// buyukDosyalar dosyaSiniri'ni aşan el yazımı dosyalardır (üretilmiş dosyalar sayılmaz).
var buyukDosyalar = map[string]istisna{
	"internal/compute/tokenize/regex.go":          {915, "düzenli ifade ayrıştırıcısı ve sınıf tabloları; " + nParcalama},
	"internal/compute/tokenize/regex_match.go":    {615, "düzenli ifade eşleştiricisi; " + nParcalama},
	"internal/compute/tokenize/tokenizer_test.go": {737, "altın çıktı testleri ve kıyaslar (veri ve parçalayıcı sahibinin alanı); model türüne göre bölünebilir"},
	"internal/compute/tensor/cpu_backend.go":      {737, "CPU arka ucunun işlem tabloları ve döngüleri; " + nCekirdek},
	"internal/compute/dataload/cache.go":          {795, "önbellek durum makinesi ve kilitleri tek tipte (veri sahibinin alanı); getir/yaz/temizle olarak bölünebilir"},
}

const enFazlaUzunFonksiyon = 41

// uzunFonksiyonlar fonksiyonSiniri'ni aşan test dışı fonksiyonlardır ("dosya:Alici.Ad").
var uzunFonksiyonlar = map[string]istisna{
	// Dil çekirdeği
	"internal/lang/interp/compiler_loop.go:compiler.inLoop":      {172, nSicakYol + " (liste/aralık/sözlük/metin döngülerinin hızlı yolları)"},
	"internal/lang/interp/compiler_expr.go:compiler.expression":  {168, nDagitim},
	"internal/lang/interp/compiler_op.go:compiler.binary":        {132, nSicakYol + " (işleç başına tam sayı/ondalık hızlı yolları)"},
	"internal/lang/interp/compiler_call.go:compiler.call":        {95, nSicakYol + " (yerleşik, metot ve fonksiyon çağrısının hızlı yolları)"},
	"internal/lang/interp/operators.go:valuesEqual":              {90, nDagitim + " (değer tipi başına eşitlik)"},
	"internal/lang/interp/methods.go:setupListMethods":           {88, nTablo},
	"internal/lang/lexer/lexer.go:Lexer.NextToken":               {164, nDagitim + " (karakter başına durum makinesi; " + nSicakYol + ")"},
	"internal/lang/checker/checker.go:Checker.checkStatement":    {163, nDagitim},
	"internal/lang/checker/checker.go:Checker.inferExpression":   {133, nDagitim},
	"internal/lang/parser/struct.go:Parser.parseStructStatement": {97, "yapı tanımı: alanlar, varsayılanlar, metotlar ve hata kurtarma tek akışta; alan ve metot çözümlemesi ayrılabilir"},
	"internal/lang/parser/expressions.go:Parser.parseExpression": {92, nDagitim + " (Pratt döngüsü: önek ve ara işleç dağıtımı)"},
	"internal/lang/codes/language.go:Load":                       {160, nOkuma + " (dil dosyası)"},
	"internal/lang/codes/definition.go:CheckDefinitions":         {113, nOkuma + " (tanım dosyasının tutarlılık denetimleri)"},
	"internal/lang/codes/definition.go:ParseDefinitions":         {96, nOkuma + " (tanım dosyası)"},

	// Araçlar
	"internal/devtools/debugger/ayiklayici.go:Debugger.promptCLI": {187, nDagitim + " (terminal hata ayıklayıcısının komutları)"},
	"internal/devtools/formatter/bicim.go:bicimle":                {119, "biçimlendiricinin sözcük birimi döngüsü: girinti ve boşluk durumu tek döngüde"},
	"cmd/tr/bicimle.go:bicimleKomutu":                             {83, "komut satırı bayrakları ve dosya/klasör dolaşımı tek akışta"},
	"tools/gen/codesdoc/main.go:Uret":                             {151, nUretec},
	"tools/gen/grammar/main.go:Uret":                              {107, nUretec},
	"tools/bench/go/kiyas.go:main":                                {92, nUretec + " (kıyas programları sırayla)"},

	// Motorlar
	"internal/stdlib/ml/ozel_turev.go:turevDenetle":          {181, nYerlesik + " (sayısal türev denetimi)"},
	"internal/stdlib/ml/yazdirma.go:Deger.Inspect":           {120, "tensör yazdırma: boyut, kısaltma ve biçim tek özyinelemede"},
	"internal/stdlib/ml/model_araclari.go:orneklemeYerlesik": {118, nYerlesik},
	"internal/stdlib/ml/yerlesik_indirgeme.go:indirgeme":     {85, nYerlesik},
	"internal/stdlib/tokenizer/parcalayici.go:Nesne.kodla":   {88, nYerlesik},

	// Hesap çekirdekleri
	"internal/compute/tensor/cpu_attention.go:cpuBackend.f32Attention": {146, nCekirdek},
	"internal/compute/tensor/cpu_matmul.go:gemmParca":                  {146, nCekirdek},
	"internal/compute/tensor/autograd.go:Tensor.BackwardCtx":           {89, nCekirdek},
	"internal/compute/tensor/cpu_backend.go:unaryLoop":                 {89, nCekirdek},
	"internal/compute/tensor/cpu_backend.go:cpuBackend.f32BinaryGrad":  {85, nCekirdek},
	"internal/compute/tensor/cpu_backend.go:unaryGradLoop":             {81, nCekirdek},
	"internal/compute/tokenize/regex_match.go:reMatcher.run":           {167, nParcalama},
	"internal/compute/tokenize/tokenizer.go:Parse":                     {131, nOkuma + " (tokenizer.json)"},
	"internal/compute/tokenize/regex.go:reParser.group":                {103, nParcalama},
	"internal/compute/tokenize/model_bpe.go:newBPE":                    {99, nParcalama},
	"internal/compute/tokenize/model_other.go:unigram.encode":          {96, nParcalama},
	"internal/compute/tokenize/regex.go:reParser.classInner":           {89, nParcalama},
	"internal/compute/tokenize/model_bpe.go:bpeModel.symbols":          {84, nParcalama},
	"internal/compute/tokenize/decoders.go:init":                       {82, nTablo},
	"internal/compute/dataload/cache.go:cacheCore.fetch":               {112, "önbellekten getirme: kilit, bayatlık ve kopyalama durumları tek akışta (veri sahibinin alanı)"},
}
