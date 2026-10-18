package internal_test

// Kod düzeni standardının bugünkü istisnaları (kurallar: duzen_test.go, docs/katki.md).
//
// Listeler YALNIZCA KÜÇÜLEBİLİR: bir dosya ya da fonksiyon sınırın altına inince kaydı
// silinir ve en fazla sayısı (enFazla...) aynı miktarda azaltılır. Yeni kayıt eklenmez;
// tavan yükseltilmez. Taşınan bir fonksiyonun anahtarı değişirse eski kayıt silinip yenisi
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

// izinliAdlar İngilizce sözcük denetiminden muaf dosyalardır (yol → gerekçe). Bugün boş:
// protokol ve biçim adları (dap, npy, json, bpe) zaten yasak listede değildir.
var izinliAdlar = map[string]string{}

const enFazlaBuyukDosya = 6

// buyukDosyalar dosyaSiniri'ni aşan el yazımı dosyalardır (üretilmiş dosyalar sayılmaz).
var buyukDosyalar = map[string]istisna{
	"internal/hesap/parca/duzifade.go":         {915, "düzenli ifade ayrıştırıcısı ve sınıf tabloları; " + nParcalama},
	"internal/hesap/parca/duzifade_esle.go":    {615, "düzenli ifade eşleştiricisi; " + nParcalama},
	"internal/hesap/parca/parcalayici_test.go": {737, "altın çıktı testleri ve kıyaslar (veri ve parçalayıcı sahibinin alanı); model türüne göre bölünebilir"},
	"internal/hesap/tensor/cpu_arkauc.go":      {737, "CPU arka ucunun işlem tabloları ve döngüleri; " + nCekirdek},
	"internal/hesap/tensor/cpu_tur.go":         {733, "veri türü (bf16/f16) dönüşüm çekirdekleri; " + nCekirdek},
	"internal/hesap/veriyolu/onbellek.go":      {795, "önbellek durum makinesi ve kilitleri tek tipte (veri sahibinin alanı); getir/yaz/temizle olarak bölünebilir"},
}

const enFazlaUzunFonksiyon = 43

// uzunFonksiyonlar fonksiyonSiniri'ni aşan test dışı fonksiyonlardır ("dosya:Alici.Ad").
var uzunFonksiyonlar = map[string]istisna{
	// Dil çekirdeği
	"internal/dil/yurut/derleyici_dongu.go:derleyici.donguIcinde": {172, nSicakYol + " (liste/aralık/sözlük/metin döngülerinin hızlı yolları)"},
	"internal/dil/yurut/derleyici_ifade.go:derleyici.ifade":       {168, nDagitim},
	"internal/dil/yurut/derleyici_islec.go:derleyici.ikili":       {132, nSicakYol + " (işleç başına tam sayı/ondalık hızlı yolları)"},
	"internal/dil/yurut/derleyici_cagri.go:derleyici.cagri":       {95, nSicakYol + " (yerleşik, metot ve fonksiyon çağrısının hızlı yolları)"},
	"internal/dil/yurut/islecler.go:degerlerEsit":                 {90, nDagitim + " (değer tipi başına eşitlik)"},
	"internal/dil/yurut/metotlar.go:listeMetotlariniKur":          {88, nTablo},
	"internal/dil/sozcuk/sozcuk.go:Lexer.NextToken":               {164, nDagitim + " (karakter başına durum makinesi; " + nSicakYol + ")"},
	"internal/dil/denetci/denetci.go:Checker.checkStatement":      {163, nDagitim},
	"internal/dil/denetci/denetci.go:Checker.inferExpression":     {133, nDagitim},
	"internal/dil/ayristir/yapi.go:Parser.parseStructStatement":   {97, "yapı tanımı: alanlar, varsayılanlar, metotlar ve hata kurtarma tek akışta; alan ve metot çözümlemesi ayrılabilir"},
	"internal/dil/ayristir/ifadeler.go:Parser.parseExpression":    {92, nDagitim + " (Pratt döngüsü: önek ve ara işleç dağıtımı)"},
	"internal/dil/kod/dil.go:Yukle":                               {160, nOkuma + " (dil dosyası)"},
	"internal/dil/kod/tanim.go:TanimDenetle":                      {113, nOkuma + " (tanım dosyasının tutarlılık denetimleri)"},
	"internal/dil/kod/tanim.go:TanimCoz":                          {96, nOkuma + " (tanım dosyası)"},

	// Araçlar
	"internal/arac/ayikla/ayiklayici.go:Debugger.promptCLI": {187, nDagitim + " (terminal hata ayıklayıcısının komutları)"},
	"internal/arac/bicim/bicim.go:bicimle":                  {119, "biçimlendiricinin sözcük birimi döngüsü: girinti ve boşluk durumu tek döngüde"},
	"internal/arac/testkosucu/kosucu.go:DosyaCalistir":      {91, "test dosyasını ayrıştır, denetle, her testi yalıtılmış çalıştır ve raporla: sıralı tek akış"},
	"cmd/tr/bicimle.go:bicimleKomutu":                       {83, "komut satırı bayrakları ve dosya/klasör dolaşımı tek akışta"},
	"bakim/uret/kod_belgesi/main.go:Uret":                   {151, nUretec},
	"bakim/uret/eklenti_listeleri/main.go:Uret":             {107, nUretec},
	"bakim/kiyas/go/kiyas.go:main":                          {92, nUretec + " (kıyas programları sırayla)"},

	// Motorlar
	"internal/motor/ogrenme/ozel_turev.go:turevDenetle":             {181, nYerlesik + " (sayısal türev denetimi)"},
	"internal/motor/ogrenme/kayit.go:dosyadanOku":                   {142, nOkuma + " (tensör kayıt dosyası)"},
	"internal/motor/ogrenme/yazdirma.go:Deger.Inspect":              {120, "tensör yazdırma: boyut, kısaltma ve biçim tek özyinelemede"},
	"internal/motor/ogrenme/model_araclari.go:orneklemeYerlesik":    {118, nYerlesik},
	"internal/motor/ogrenme/yerlesik_indirgeme.go:indirgeme":        {85, nYerlesik},
	"internal/motor/ogrenme/model_araclari.go:parametrelerYerlesik": {82, nYerlesik},
	"internal/motor/parcalayici/parcalayici.go:Nesne.kodla":         {88, nYerlesik},

	// Hesap çekirdekleri
	"internal/hesap/tensor/cpu_dikkat.go:cpuBackend.f32Attention":  {146, nCekirdek},
	"internal/hesap/tensor/cpu_carpim.go:gemmParca":                {146, nCekirdek},
	"internal/hesap/tensor/turev.go:Tensor.BackwardCtx":            {89, nCekirdek},
	"internal/hesap/tensor/cpu_arkauc.go:unaryDongu":               {89, nCekirdek},
	"internal/hesap/tensor/cpu_arkauc.go:cpuBackend.f32BinaryGrad": {85, nCekirdek},
	"internal/hesap/tensor/cpu_arkauc.go:unaryGradDongu":           {81, nCekirdek},
	"internal/hesap/parca/duzifade_esle.go:dfEslestirici.calistir": {167, nParcalama},
	"internal/hesap/parca/yukle.go:Ayristir":                       {131, nOkuma + " (tokenizer.json)"},
	"internal/hesap/parca/duzifade.go:dfAyristirici.grup":          {103, nParcalama},
	"internal/hesap/parca/model_bpe.go:bpeKur":                     {99, nParcalama},
	"internal/hesap/parca/model_diger.go:unigram.kodla":            {96, nParcalama},
	"internal/hesap/parca/duzifade.go:dfAyristirici.sinifIc":       {89, nParcalama},
	"internal/hesap/parca/model_bpe.go:bpeModel.semboller":         {84, nParcalama},
	"internal/hesap/parca/cozuculer.go:init":                       {82, nTablo},
	"internal/hesap/veriyolu/onbellek.go:obCekirdek.getir":         {112, "önbellekten getirme: kilit, bayatlık ve kopyalama durumları tek akışta (veri sahibinin alanı)"},
}
