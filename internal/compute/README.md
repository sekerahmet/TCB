# internal/compute

TR'nin hesap çekirdeği: tensörler ve otomatik türev, CPU ve GPU arka uçları, veri yükleme ve
parçalayıcı. Dil bu paketleri motorlar üzerinden kullanır; buradaki kod dili bilmez.

## Standartlar

**Dili bilmez.** `internal/lang` paketlerinden yalnızca ileti kataloğunu (`internal/lang/messages`, yalnızca Go standart kitaplığına dayanan en alt paket) içe aktarır; tek başına derlenir, sınanır ve
ölçülür. Denetim: `TestLayerRule` (internal/layers_test.go).

**Tek dosya, cgo'suz dağıtım.** Dış bağımlılık yalnızca GPU arka ucunun sürücüyü cgo'suz
yüklemek için kullandığı kitaplıktır. GPU'suz derleme statik kalır. Denetim: `TestExternalDependencies` (internal/layers_test.go), `TestFileSelectionIsConsistentInEveryConfiguration`.

**Her yapılandırma derlenir.** Kod işletim sistemine, mimariye ve derleme etiketlerine göre
ayrılmış dosyalar içerir; her biri derlenir ve sınanır. Assembly ile yazılan çekirdeklerin Go'da
bir başvuru karşılığı vardır ve aynı sonucu verir. Denetim: `TestFileSelectionIsConsistentInEveryConfiguration`, `TestCompileEveryConfiguration` (uzun kip), tensor `TestAssemblyKernelsMatchGo`.

**İşlemci yetenekleri çalışma anında seçilir.** Tek program dosyası her işlemcide çalışır: AVX2
gibi bir yetenek varsa onun çekirdeği, yoksa Go karşılığı kullanılır. Hız derleme ayarına
bağlanmaz; Go karşılıkları da sınanır (yeteneği kapatan ortam değişkeniyle). Denetim: `TestBuildTagsNeverSelectCPUFeatures`, tensor `TestNormSameAcrossBuilds`.

**Dosya biçimleri kararlıdır.** Diske yazılan ya da diskten okunan biçimler (veri parçaları,
meta dosyaları, parçalayıcı tanımı, kontrol noktaları) Go adlarına bağlı değildir; bir ad
değişikliği kullanıcının dosyalarını okunamaz kılmaz. Denetim: `TestJSONFieldsHaveExplicitNames`, profiler `TestSnapshotKeysFrozenPerVersion`.

**Hız ölçülerek değişir.** Sıcak yoldaki değişiklikler önce/sonra ölçümüyle yapılır.

**Hata mesajları kullanıcıya ulaşabilir.** Kullanıcının dilinde, neyin yanlış olduğunu ve ne
yapılacağını söyler.

## Testler

Bu klasör yalnızca testleri taşır (paket yok); testler hesap çekirdeğinin tüm paketlerini kesen davranışlardır:

- `behavior_build_configs_test.go`: her işletim sistemi/mimari/etiket kümesinde dosya seçiminin tutarlılığı, işlemci yeteneği etiketi yokluğu ve (uzun kipte) gerçek derleme + vet
- `behavior_file_format_names_test.go`: JSON şeması olan yapıların bütün dışa açık alanlarının açık json adı taşıması
