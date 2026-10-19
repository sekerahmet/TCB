# internal/compute

TR'nin hesap çekirdeği: tensörler ve otomatik türev, CPU ve GPU arka uçları, veri yükleme ve
parçalayıcı. Dil bu paketleri motorlar üzerinden kullanır; buradaki kod dili bilmez.

## Standartlar

**Dili bilmez.** Hiçbir `internal/lang` paketini içe aktarmaz; tek başına derlenir, sınanır ve
ölçülür.

**Tek dosya, cgo'suz dağıtım.** Dış bağımlılık yalnızca GPU arka ucunun sürücüyü cgo'suz
yüklemek için kullandığı kitaplıktır. GPU'suz derleme statik kalır.

**Her yapılandırma derlenir.** Kod işletim sistemine, mimariye ve derleme etiketlerine göre
ayrılmış dosyalar içerir; her biri derlenir ve sınanır. Assembly ile yazılan çekirdeklerin Go'da
bir başvuru karşılığı vardır ve aynı sonucu verir.

**İşlemci yetenekleri çalışma anında seçilir.** Tek program dosyası her işlemcide çalışır: AVX2
gibi bir yetenek varsa onun çekirdeği, yoksa Go karşılığı kullanılır. Hız derleme ayarına
bağlanmaz; Go karşılıkları da sınanır (yeteneği kapatan ortam değişkeniyle).

**Dosya biçimleri kararlıdır.** Diske yazılan ya da diskten okunan biçimler (veri parçaları,
meta dosyaları, parçalayıcı tanımı, kontrol noktaları) Go adlarına bağlı değildir; bir ad
değişikliği kullanıcının dosyalarını okunamaz kılmaz.

**Hız ölçülerek değişir.** Sıcak yoldaki değişiklikler önce/sonra ölçümüyle yapılır.

**Hata mesajları kullanıcıya ulaşabilir.** Kullanıcının dilinde, neyin yanlış olduğunu ve ne
yapılacağını söyler.
