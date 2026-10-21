# TR Yol Haritası

TR'nin hedefi: yapay zekâ modelini **eğiten** ve **kullanan** geliştirici için en iyi programlama dili. Model eğitimi dilin sınavıdır; ürün dilin kendisidir. Dilin en önemli yan faydası: biraz teknik bilgisi olan herkesin kendi yapay zekâ modelini kurup eğitebilmesi.

Bugün TR, tek GPU'da sıfırdan model eğitmekte derlenmemiş PyTorch kadar hızlı ve daha az bellek kullanıyor; ama kendi içinde kapalı: hazır bir modeli yükleyemiyor, ince ayar yapamıyor, eğittiği modeli yaygın araçlara veremiyor. Yeni ML dillerinin (Swift for TensorFlow, Julia, Mojo, Dex, Futhark) ve Python dışı çerçevelerin (Gorgonia, GoMLX, Candle, Burn, llama.cpp) ortak dersi: kullanıcıyı dil tasarımı kazanmaz. Hazır modellere erişim, doğru gradyan, kararlı sürümler ve her gün kullanan bir iç kullanıcı kazanır. Bu yüzden sıra şudur: önce ölçüm ve doğruluk kapıları, sonra tek GPU'da bellek ve baş boyu uyumu, sonra hazır ağırlık, ince ayar ve üretim yolu. Çoklu GPU ve genel derleyici ancak bunlar yerine oturunca gelir.

## İlkeler

- **Önce sağlam temel, sonra özellik.** İlk evre, yapılabildiğini görmek için hızlı bir koşuydu. Bundan sonra her özellik, kuralları belli ve testle korunan bir temelin üstüne eklenir. Her eksiği aynı anda kapatmaya çalışmak en pahalı hatadır.
- **Evreler bitiş ölçütüyle ilerler.** Bir evrenin ölçütü sağlanmadan sonrakine geçilmez. Temel işi sonu belirsiz bir temizliğe dönüşmez.
- **Her kural bir testle korunur.** Yalnızca yazılı kalan kural zamanla aşınır. Örnek: dosya ve fonksiyon boyu, katman sırası, kod listesinden üretilen adlar, ileti kodları.
- **Yanlış gradyan en pahalı hatadır.** Tek otomatik türev sistemi vardır; her türevli işlemin sayısal türev testi yayın kapısıdır.
- **Doğrusu neyse o, önce başka dillere bakılarak.** Yeni bir parça eklenirken ya da bir tasarım sorunu çıktığında önce başka dillerin ve çerçevelerin çözümü karşılaştırılır; yerleşik ve kanıtlanmış olan (best practice) TR'ye uyarlanır, hiçbiri uymuyorsa gerekçesiyle yeni bir yöntem seçilir. Gerekçe belgeye yazılır. Örnek: sınır dışı liste okuması Python, Swift, Kotlin, Rust ve Go'da hatadır, "yoksa yok" isteyen için ayrı bir yol vardır (`getOrNull`, `get`); TR'de `l[i]` hata, `l.varsa_al(i) ?? varsayılan` açık yoldur.
- **Önce ölç, sonra değiştir.** Hız işinde her değişiklik önce ve sonra ölçülür; kazandırmayan "iyileştirme" geri alınır. Genel derleyici ve zorunlu otomatik ayarlama yoktur; ölçümün gösterdiği yer küçük, kurallı ve testli adımlarla kapatılır.
- **Dış incelemeler doğrulanarak düzeltilir.** Her bulgu önce yeniden üretilir. Doğrulananın düzeltmesine, düzeltmeden önce kalan sonra geçen bir test eklenir.
- **Birlikte çalışma dosya ve protokol düzeyindedir.** Python köprüsü bilinçli olarak yoktur. Karşılığı, yaygın ve kararlı biçimleri okuyup yazmaktır: safetensors, Hugging Face `config.json` ve `tokenizer.json`, PEFT bağdaştırıcısı, GGUF, `.npy`, jsonl; sunumda OpenAI uyumlu HTTP protokolü.
- **Tutulamayacak söz verilmez.** Uyumluluk yazılı bir sürüm politikasıyla verilir; her yerleşiğin kararlılık düzeyi bellidir.
- **Hedef model sürekli TR'de eğitilir.** Dilin ilk ve her gün kullanan iç kullanıcısı odur.
- **Güçlü yanlar korunur.** Yeni özellik şunları bozmaz:
  - belirlenimcilik: sonuç işçi sayısından bağımsızdır, kesilip devam eden eğitim bit düzeyinde aynıdır;
  - tek dosya ve cgo'suz dağıtım: GPU yalnızca sürücüyle çalışır;
  - CPU ve GPU arka uçları tek tensör API'sinin arkasında ayrıdır; hiçbir dış derleyiciye kilitlenilmez;
  - güvenilmeyen girdinin sınırlarla denetlenmesi;
  - sabit ad kodları ve okunur model kodu.

## Evre 1: Kanıt (tamamlandı)

Dil, tensörler, otomatik türev, veri hattı ve GPU üst üste kuruldu; TR ile bir Transformer eğitilebiliyor. GPU'da (116 M parametreli GPT, bf16) adım süresi derlenmemiş PyTorch'la başa baş (~81 ms), bellek kullanımı ~3 GB daha az.

## Evre 2: Sağlam temel (tamamlandı)

1. **İleti kodları.** Kullanıcıya giden her ileti, adlar gibi sabit bir koda bağlanır (`H` ve dört basamak, `internal/lang/messages`). Metinler dil dosyasındadır; parametreler adlıdır (`{fn}: {path|q} bir klasör`). Testler metni değil kodu denetler.
   - Tamamlandı: altyapı ve denetim testi; ortak hata yardımcıları; standart kitaplık, `devtools`, `tr` komutu; dil çekirdeği; yapılı hata (hata konumu her yerde yapıdan okunur, metinden okunmaz); hesap çekirdeği (tensör, GPU, veri hattı, parçalayıcı). Bütün paketler geçti.
   - Bitiş testi (tamamlandı): Go metninde Türkçe harfli metin sabiti yalnızca izinli yerdedir (`TestNoTurkishLiterals`): dilbilgisi paketi, yapı etiketleri ve gerekçeli kayıtlar (veri biçimi anahtarları, Türkçe belge üreten aracın içeriği). İlk taramada 40 dosyada 244 metin bulundu; adlar ad koduna, iletiler ileti koduna taşındı, geliştirici hataları ve araç çıktıları İngilizceye geçti (`tools/` Go adları dahil).
2. **İletiler her dile hazır (tamamlandı).** Türkçenin dilbilgisi Go kodundan şablona taşındı; asıl çeviri Evre 7'de yapılır ([`internal/lang/messages/README.md`](../internal/lang/messages/README.md)):
   - Şablon dili ICU MessageFormat'ın alt kümesi: çoğul (`{n, plural, ...}`, dile göre CLDR kuralı), seçim (`select`), hâl eki (`{ad|dat}`; çekimciyi dilin paketi kaydeder), seçenek listesi (`messages.OrList`, CLDR liste kalıbı).
   - Go'da sözcük çekimlenmez (`turkish.Suffix` yalnızca kendi paketinde; testle), sayıya bakıp sözcük seçilmez, ileti metni `strings` ile düzenlenmez.
   - Her dil kodun bütün parametrelerini kullanır; aynı anlam tek koddur, parametre adı farklı olsa da (testle; bu denetim 8 çift kod buldu, birleştirildi).
   - İngilizce pilot: 38 ileti (`en.json`), `TR_DIL=en` ile gösterilir. Bir ileti bütünüyle tek dildedir: kendisinin ya da içindeki kodlu hatanın çevirisi yoksa iletinin tamamı Türkçe gösterilir; süreç dili bir kez okunur, adlar ve iletiler aynı dildedir.
   - Dış denetimden sonra (2026-10-20): çoğul kuralı CLDR işlenenlerini alır (2^53 üstünde de doğru, Rusça gibi dillere hazır); bir parametre ya dal dışında ya her dalda geçer (testle; birbirini dışlayan değerli iki ileti ikişer koda ayrıldı); parçadan kurulan iletiler (dosya hataları, yüklem ve iyelik parçaları) tam cümleye çevrildi ve bir ileti metninin başka iletiye değer olarak geçmesini test yakalar.
   - Kalan küçükler: değer olarak geçen ad öbekleri ve sona eklenen ipuçları (ör. GPU tanı zinciri H4033–H4035, `{hint}` eki, "Bunu mu demek istediniz" eki, `testrunner` sayım satırı) çeviride sorun çıkarabilir; Evre 7'de tek şablona çevrilir. Parça testi değişkene alınmış metni ve `+` birleştirmeyi henüz görmüyor.
3. **Gradyan doğruluğu kapısı (tamamlandı).** Her türevli tensör işleminin float64 sonlu fark testi ve bf16 tolerans testi var (`gradSpec`); `behavior_grad_coverage_test.go` testi olmayan işlemi yakalar; GPU'da her türevli işlemin gradyanı CPU ile karşılaştırılır (gerçek kartta doğrulandı). `stdlib/ml`'deki 14 bileşik türev de (softmax, normlar, çapraz entropi, gömme, dikkat ...) aynı kapıdadır. bf16 toleransları ölçülerek işlem başına seçildi (ölçülenin 2 katı; tablo `internal/compute/tensor/README.md`). Bulunan tek yanlış: `üstel_eksi_bir`'in türevi çok negatif girdide 0 çıkıyordu, düzeltildi. Kalan: GPU'daki bf16 sapma tablosu Colab'da ölçülecek.
4. **Anlam kuralları yazılı ve testli (tamamlandı).** Yayınlama, tür yükseltme, tensör indeksi ve dilimi, `.dilim`, liste ve metin indeksi, sayı olmayan işlenen ve NaN kuralları dil tanımının 15.41 maddesinde tek tabloda; her biri PyTorch/NumPy ile karşılaştırıldı ve `tests/tensor_test.tr`'de testli. Tek davranış değişikliği: liste ve metinde sınır dışı okuma artık hatadır, bilinçli durum için `varsa_al` (3.6).
5. **Açık işler ve bekleyen dil kararları.** Her karar başka dillerle karşılaştırılarak verildi; gerekçeleri dil tanımında.
   - Tamamlandı:
     - `csv_çöz(metin)`; `csv_oku` yalnızca dosya okur (13.8). Ortam değişkeni adları ASCII (`TR_IS_KLASORU`).
     - Liste ve metinde sınır dışı okuma hatadır, `varsa_al` açık yoldur (3.6).
     - Yeni satırın başındaki ikili işleç önceki ifadeyi sürdürmez; `-x` tuzağı kalktı, zincir işleçleri (`.`, `|>`) sürer (2.8).
     - Testin başlattığı görevdeki hata testi kaldırır (10.1); testler dosyanın üst düzeyini paylaşmaya devam eder (Go ve pytest gibi, bilinçli).
     - Hata ve uyarı çıktısında denetim karakterleri görünür yazılır (8.4).
   - Go adları Go biçimine geçti (MixedCaps): belirteç türleri `Tok` önekli (`TokIdent`, `TokEOF`), değer tipleri `Val` önekli (`ValInt`), öncelikler `prec` önekli (`precAnd`).
6. **Test düzeni (tamamlandı).** Test dosyası adlandırma kuralı (`x_test.go` yalnızca `x.go`'yu sınar; süreçleri sınayan testler `behavior_` önekli dosyalarda) ve her paket README'sinde bir "Testler" bölümü; ikisi de testle denetlenir.
7. **Kalan adlar (tamamlandı).** `cmd/tr` ve `tools` dosya ve Go adları İngilizce. `tr` komutları, seçenekleri, etkileşimli kabuk ve hata ayıklayıcının komutları `K` ad ailesinde (17.8): girdide her yazım dilinin adı kabul edilir (`tr çalıştır` = `tr run`, `--dil` = `--lang`), yardım ve iletiler adları etkin dilde yazar; programın `--yardım`'ı da koddur.
8. **Kurallar testle korunur (tamamlandı).** README kurallarına ~30 test eklendi; `docs/architecture.md`'de adı geçen Go adlarının ve README'lerde anılan her denetim testinin var olduğu testle denetlenir (`TestRuleTestsExist`; ilk koşuda iki yanlış ad buldu); C tarzı BÜYÜK_HARFLİ Go adını `TestGoNamesMixedCaps` yakalar.
9. **Belgeler yeni düzeni anlatır (tamamlandı).** Kurallar (`internal/README.md`), ekleme tarifleri (`docs/recipes.md`), mimari ve katkı rehberi yeni düzene göre yazıldı.

**Durum (2026-10-21):** bitiş ölçütünün bütün maddeleri testle korunuyor; Evre 2 tamamlandı.

**Bitiş ölçütü:** kodda Türkçe ileti metni kalmaz (testle); ileti kataloğu çoğul ve liste kalıplarını taşır, Go'da ileti sözcüğü çekimlenmez ve İngilizce pilot geçer; her türevli işlemin gradyan testi vardır; anlam kuralları yazılı ve testlidir; her kural bir testle korunur; bekleyen dil kararı kalmaz.

## Evre 3: Tek GPU'da gerçek model (şu an)

Dilin asıl sınavı, Python/PyTorch ile yazılmış, hız ve doğruluk için optimize edilmiş, yalnızca kod çözücülü (decoder-only) bir Transformer ve küçük sürümleridir (hedef model: `tools/gpu/rgn512.py`). Eksik parçalar Evre 2'nin kurallarına uyarak eklenir.

**Durum (2026-10-21):** Hedef model TR'de yazıldı (`tools/gpu/rgn512.tr`). TR, PyTorch'la aynı kalitede eğitiyor, bellekte derlenmiş PyTorch'la başa baş. Hızda H3'te: derlenmiş PyTorch'un 1,2 katı. Kapanış için kalanlar aşağıdaki "Bitiş ölçütü"ndedir.

**Kapanış kararı (2026-10-21):** Evre 3'ün hız ölçütü H3'tür (≤ 1,2 × compile).

H4 (compile ile başa baş) süren bir hedef olarak Evre 6'ya taşındı. Kalan kazançlar derin GPU işidir: norm geri yönünü kaynaştırma, adımlı dikkat, çarpım eplogları.

İki madde ilgili evrelere ertelendi, çünkü hedef model 10,8 GB'a sığıyor ve dropout kullanmıyor; ince ayar ve büyük model geldiğinde gerekir:
- aktivasyonların yeniden hesaplanması,
- rastgelelik anahtarı ve dropout.

### Sonuçlar (hedef model, RTX PRO 6000, 64 × 1024 parça/adım)

| | PyTorch eager | `torch.compile` | TR |
| :-- | --: | --: | --: |
| ms/adım, sentetik veri | 347 | 136 | **162** |
| ms/adım, gerçek veri | — | 134 | 173 |
| tepe bellek (nvidia-smi, GiB) | 17,0 | 10,8 | **10,81** |
| doğrulama kaybı, 2000. adım (gerçek veri) | — | 3,9113 | **3,8816** |

**Kod.** Aynı mimari ve eğitim tarifi, tek dosya, PyTorch dosyasıyla aynı mertebede uzunluk:
- yerel ve genel katmanlar, pencere, GQA, QK-norm,
- ½ SwiGLU (672),
- katman başına kelime gömmesi (PLE), paylaşımlı gömme,
- Muon + AdamW + satır seyrek RMS, WSD, bf16.

**Doğruluk.**
- PyTorch'un başlangıç ağırlıkları TR'ye yüklenir (`tools/gpu/trt_io.py`, `rgn_reference.py`); aynı toplu işlerle her adımın kaybı ve gradyan normu eşittir.
- float32 ve CPU'da, hedef boyutta, 4 adım: kayıp farkı ≤ 1·10⁻⁷. Küçük boyutlu sürümü `TestTargetModelMatchesPyTorch` her değişiklikte sınar.
- GPU ve bf16'da, 100 adım: kayıp farkı ≤ 0,0043, gradyan normu farkı ≤ 0,0041.

**Kalite.**
- Veri: Türkçe FineWeb-2, TR BPE 40k (250M parça); 2000 adım; doğrulama `valid.npy`'nin başından 4,2M parça.
- Doğrulama kaybı:
  - 1000. adımda PyTorch 4,2891 / TR 4,2729,
  - 2000. adımda PyTorch 3,9113 / TR 3,8816.
- Tek tohum; eğitim kaybı farkı gürültü düzeyinde.

**Hız yolu.**
- **H1 (241 ms):** ilk TR karşılığı.
- **H2 (203 ms):**
  - Muon aynı şekilli ağırlıkları tek yığında, bf16'da dikleştirir (56 dikleştirme yerine 3).
  - `Doğrusal` sapmayı çarpımın türünde ekler.
- **Normlar** 16 bitlik girdiyi açmadan okur (194).
- **Belirlenimci bırakma**, dil tanımı 15.44 (189 ms, 11,84 GB):
  - Yorumlayıcının sahiplik kuralları ve depolamanın sahip sayacı, tensörün belleğini ona artık ulaşılamayacağı kesinleşince havuza döndürür.
  - Yalnızca çöp toplayıcıyla bellek ancak toplama sıklaşınca iniyordu (128 MB eşikte 11,61 GB / 696 ms).
  - Saf skaler TR döngülerinde ek yük ~%19.
- **Kaynaştırma**, dil tanımı 15.45: `kaynaştır(f)` eleman bazlı zinciri tek geçişe indirir.
  - RoPE, SwiGLU kapısı ve PLE karışımı (179).
  - Tek çıktılı kaynaşık işlemin gradyanı girdinin gradyanına doğrudan yazılır (167).
- **H3 (162 ms):** normların ve kaynaşık zincirlerin çıktısı doğrudan çarpımın türünde yazılır.
  - Dil tanımı 15.13: `tür:`, `hesap_türü()`.
  - float32'den daraltılan çıktının gradyanı float32 tutulur; ileri ve geri yön bit düzeyinde aynıdır.
- **Profil (adım başına):**
  - çarpım 53, kopya 21 ms (dikkat başları 6, float32→bf16 5,6),
  - indirgeme 15, ikili işlemler 14, norm geri yönü 14, kaynaşık çekirdekler 13 ms,
  - çapraz entropi 11, dikkat 11 ms.

**Bellek yolu** (30,8 GB'tan 10,81 GiB'a):
- otomatik türev çizgesi yalnızca kaydedilen tensörleri tutar (PyTorch'un grad_fn / saved tensors ayrımı),
- `doğrusal_çapraz_entropi` logitleri bütün olarak tutmaz (dil tanımı 15.43),
- 1 MB üstü bloklar sürücü parçalarından en uygun boş bloğa bölünür (PyTorch'un önbellekli ayırıcısı),
- normlar 16 bitlik girdinin türünde çıktı verir,
- belirlenimci bırakma.

**Denetimler (2026-10-21).** Evre 3'ün Go değişiklikleri iki bağımsız kanıtlı denetimden geçti: Claude Opus 7 bulgu, Gemini 11 bulgu (biri öncekiyle aynı). Her bulgunun önce kalan, düzeltmeyle geçen testi depodadır. Denetimin yol açtığı tasarım kararı: program her yazım dilinde kanonik (Türkçe) biçimiyle çalışır, `TR_DIL` yalnızca gösterimi belirler (dil tanımı 17.7).

### Maddelerin durumu

1. **Önce ölçüm.** Yapıldı. 12 katman D768 düz GPT'de TR eager PyTorch'la başa baştı, `torch.compile`'dan 1,15 kat yavaştı; fark çekirdek süresindeydi (kaynaşmamış küçük işler). Hedef modelde derleme eager'a göre 2,55 kat kazandırıyordu; asıl kıyas bu modelle yapıldı (yukarıda).
2. **Bellek.**
   - Parçalı doğrusal + çapraz entropi: yapıldı.
   - Aktivasyonların yeniden hesaplanması: Evre 4'e ertelendi.
   - Dikkatin adımlı girdi okuması: H4 işi olarak Evre 6'ya taşındı.
3. **Rastgelelik anahtarı ve dropout.** Evre 4'e ertelendi; yeniden hesaplamanın önkoşulu.
4. **Baş boyu uyumu. Kalan (kapanış şartı).**
   - Tensör çekirdekli dikkatin geri yönü baş boyu 128'e: bugün en çok 64, büyüğünde SIMT yola düşer.
   - Gizli boyuttan bağımsız baş boyu (`tr/katman`'da bugün `baş_boyutu = boyut / baş`).

   Açık modellerin neredeyse hepsi 128 kullanır; Evre 4'teki hazır model yüklemenin önkoşuludur.
5. **Hız.**
   - Yapıldı: dönüşüm kopyalarının azaltılması, RoPE ve SwiGLU zincirleri, kural tabanlı birleştirici (`kaynaştır`).
   - Kalanlar H4 olarak Evre 6'da.
   - **Kalan (kapanış şartı):** gerçek veride sentetik veriye göre ~10 ms'lik farkın (173 / 162 ms) nedeni bulunur ve giderilir ya da açıklanır.
6. **Eğitim gözlemlenebilirliği.**
   - **Kalan (kapanış şartı):** NaN/Inf'te ya da kayıp hareketli ortalamasının k katını aşınca durma ya da son kontrol noktasına dönme.
   - Günlüğe kırpma öncesi ve sonrası gradyan normu, saniyede parça ve kontrol noktası bugün yazılıyor.
   - Uzun eğitimin öbür ölçüleri ve seçenekleri Evre 6'da: parametre normu, etkinleşme RMS'i, log Z, MFU, z-loss, toplu iş atlama, eşzamansız kontrol noktası.
7. **Belirlenimci veri sırası.** Sıralı ve tohumlu örnekleyiciyle sağlanıyor. Çoklu GPU'da (Evre 6) yeniden doğrulanır.
8. **T4 yolu.** bf16 tensör çekirdeği olmayan kartlar için fp16 ve dinamik kayıp ölçekleme; ölçek günlüğe yazılır. Kapanış şartı değil, ama Colab'ın ücretsiz kartı T4 olduğundan ikinci kart işiyle birlikte ele alınır.
9. **Model parçaları.**
   - Muon'un toplu Newton–Schulz adımı: yapıldı.
   - PLE tabloları için seyrek satır güncellemesi: ölçümle önemsiz çıktı (~3 ms). Indeksle yazma (Evre 4, tensör dili) gelince ele alınır.
10. **Karşılaştırma ve kartlar.**
    - Hedef model karşılaştırması: yapıldı.
    - **Kalan (kapanış şartı):** ölçümler ve GPU testleri en az bir ikinci kartta (L4 ya da A100) tekrarlanır.
    - CPU'da küçük modelle eğitim denemesi Evre 4'ün CPU maddesiyle birlikte yapılır.

**Hedef modelin gösterdiği dil eksikleri** (her biri genel çözümü bulununca ele alınır; modele özel kestirme yapılmaz):
- Liste üreteci tek değişkenlidir: `[f(i, x) döngü i, x içinde l]` yazılamaz (döngüde yazılır); dil tanımında iki biçim tutarsız.
- Satırlara indeksle yazma yoktur (PyTorch `index_add_`/`index_copy_`, JAX `x.at[i].add`); Evre 4'ün tensör dili maddesinde.
- FP8 yoktur; d > 2000 modellerde kazandırır, tür adları metin olduğundan aynı yere eklenir (sonraki evre).

**Bitiş ölçütü:**
- Hedef model TR'de PyTorch sürümüyle aynı kalitede eğitilir. **Sağlandı (2000 adım).**
- Kalite uzun koşuda da tutar: en az 0,8 milyar parça, iki tohum. Doğrulama kaybı PyTorch'la gürültü düzeyinde aynıdır.
- Hız en çok 1,2 × `torch.compile`'dır (H3). **Sağlandı (162 / 136 ms).** Gerçek verideki ~10 ms'lik fark açıklanmıştır.
- Bellek en çok 1,1 × `torch.compile`'dır. **Sağlandı (10,81 / 10,8 GiB).**
- Baş boyu 128 tensör çekirdeğiyle ileri ve geri çalışır; baş boyu gizli boyuttan bağımsızdır.
- Eğitim bir NaN'de ya da kayıp sıçramasında kendiliğinden durur ya da son kontrol noktasına döner.
- Ölçümler ve GPU testleri en az iki kart türünde geçer.

## Evre 4: Hazır modeller ve üretim yolu

Modeli kullanan geliştiricinin ilk sorusu: "Hugging Face'teki modeli yükleyip ince ayar yapabilir miyim?" Bu evre o yolu açar. Sıra bağımlılığa göredir.

1. **Tensör dili tamamlanır.**
   - Genel indeksleme: çok eksenli `t[:, 0]`, bool maskeyle seçme, topla/dağıt (gather/scatter).
   - Yığ (stack), doldur (pad), tekrarla (repeat).
   - Kullanıcıya açık tam sayı ve bool tensör türleri: token kimlikleri ve maskeler tensörle taşınır.
   - Kayıplar: BCE, KL (damıtma ve tercih eğitimi için), etiket yumuşatma (MSE var).
   - Ağırlık başlatma şemaları (Xavier, Kaiming), EMA ağırlıkları.
   - Açık, bölünebilir ve kontrol noktasına yazılan bir rastgelelik anahtarı (JAX emsali) ve üstüne dropout; blok düzeyinde, isteğe bağlı aktivasyon yeniden hesaplama (yaklaşık %33 süreye etkinleşme belleğinde 10 kattan fazla azalma). Yeniden hesaplanan blok aynı rastgeleliği yeniden üretir, bit düzeyinde aynılık korunur. Evre 3'ten ertelendi: ince ayar ve büyük modelde gerekir.

   KV önbellek, örnekleme, maske ve LoRA yükleme bunlara dayanır.
2. **safetensors yerel ağırlık biçimi olur.**
   - `.trt`'nin kullanıcıya safetensors'un vermediği bir şey vermediği için TR'nin kendi biçimi bırakılır; TR'ye özgü bilgi `__metadata__`'da durur.
   - Okuma kuralları ileti kodu, dil kuralı ve bulanıklaştırma testi olur:
     - başlık sınırı (100 MB);
     - sıralı ve çakışmasız bayt aralıkları;
     - `(son − baş) = şeklin çarpımı × eleman boyu`;
     - bilinen veri türü;
     - yinelenen anahtar yok.
   - Bölünmüş kontrol noktası dizini (`model.safetensors.index.json`) okunur.
   - **PyTorch ile alışveriş:** TR'de eğitilen ağırlıklar PyTorch'ta, PyTorch'ta eğitilenler TR'de açılır. Ana yol safetensors'tur (`safetensors.torch.load_file` / `save_file`; parametre adları bir eşleme tablosuyla). `.pt` için yazma ve yalnızca kısıtlı okuma vardır: `.pt` Python'un pickle biçimidir ve okumak dosyadaki kodu çalıştırmak demektir; TR yalnızca tensör sözlüğünü kurar (PyTorch'un `weights_only=True` kuralı), başka her nesne açık hatadır. Gidiş dönüş testi: TR → safetensors/.pt → PyTorch → TR, değerler bit düzeyinde aynı (torch kuruluysa; yoksa test atlanır). Bugünkü Python dönüştürücüsü (`tools/gpu/trt_io.py`) bu adımla dile taşınır.
3. **Hugging Face modelleri.**
   - `config.json`'dan TR modeli kurulur; ilk aile Llama (sonra Qwen ve Mistral; Qwen'deki bağımsız `head_dim` dahil).
   - Tensör adları (`model.layers.N.self_attn.q_proj.weight`) bir eşleme tablosuyla TR modelinin adlarına bağlanır.
   - `tokenizer.json` zaten okunuyor.
4. **İnce ayar.**
   - LoRA; bağdaştırıcı PEFT düzeninde kaydedilir (`adapter_config.json` + `adapter_model.safetensors`), böylece PEFT, vLLM ve llama.cpp doğrudan okur.
   - Bir ince ayar örneği.
5. **Çıkarım ve sunum.**
   - KV önbellekli `üret`; örnekleme (sıcaklık, en iyi k, en iyi p).
   - 127.0.0.1'e bağlı OpenAI uyumlu HTTP sunucusu: completions ve chat, `logprobs` ile. Bu tek adım lm-evaluation-harness ile değerlendirmeyi, arayüzleri ve ajan çerçevelerini getirir.
   - Yorumlayıcı kilidi çekirdek çağrılarında ve G/Ç'de bırakılır; hangi yerleşiklerin bıraktığı belgelenir. Böylece sunucu, parçalayıcı ve GPU eşzamanlı çalışır.
   - CUDA Graphs önce üretim için: statik şekil kipinde, isteğe bağlı yakalama. Yakalanan bölgede ana belleğe okuma dil düzeyinde hatadır. Eğitimde beklenen kazanç küçüktür (%0–5).
   - **CPU'da hazır model:** küçük bir hazır model (yüz milyonlarca parametre) GPU'suz, dizüstünde verimli çalışır. Ağırlıklar safetensors'tan bellek eşlemeyle (mmap) kopyasız yüklenir. KV önbellekli üretim tek ve çok çekirdekte koşar, AVX2/FMA çekirdekleri kullanılır; ağırlığı bf16 saklanan çarpım anında genişletilir. Ölçüt: aynı model ve aynı ayarlarla PyTorch CPU ve llama.cpp'ye karşı token/sn ile ilk token süresi. Hedef oran ilk ölçümden sonra buraya yazılır (Evre 3'teki gibi). Yalnızca ağırlık int8/int4 nicemleme (Evre 6) bu yolu hızlandırır.
6. **Sürüm politikası.**
   - 0.y.z sürüm numaralama.
   - Her yerleşiğe ad kodu üzerinden kararlı, beta ya da prototip etiketi; kararlı davranış prototip ya da beta bir alternatif uğruna kaldırılmaz.
   - İki adımlı kaldırma: önce uyarı (bir süre ya da iki sürüm), sonra hata. Ad kodları eski adı tutabildiği için bu TR'de ucuzdur.
   - Dil ve dosya biçimi dondurma kuralları; yayın ikilileri.

   Bugünkü "deneme dönemi, geriye uyum yok" durumu bu politikayla sınırlanır ve 1.0'da biter.

**Bitiş ölçütü:**
- Hugging Face'ten Llama ailesinden küçük bir model safetensors ile yüklenir, TR'de LoRA ile ince ayar yapılır.
- TR'de eğitilen hedef model safetensors ve `.pt` olarak dışa aktarılır, PyTorch'ta aynı kaybı verir; PyTorch'ta eğitilen ağırlıklar TR'ye yüklenir.
- Küçük bir hazır model CPU'da KV önbellekle üretir; token/sn PyTorch CPU ve llama.cpp ile ölçülüp yazılır ve hedef oran sağlanır.
- Bağdaştırıcı PEFT ile okunur.
- Model OpenAI uyumlu sunucudan sunulur ve lm-evaluation-harness ile değerlendirilir.
- Sürüm politikası yazılıdır; ilk 0.y.z sürümü yayımlanır.

## Evre 5: Geliştirici deneyimi

- **Not defteri:** Jupyter çekirdeği ya da VS Code defteri; TR, Colab'da ve not defterinde hücre hücre çalışır. Yapay zekâ geliştiricileri not defterinde çalışır.
- **VS Code dil sunucusu** `tr` programının içinde (ayrıştırıcı, tip denetleyicisi ve kod listesi yeniden kullanılır):
  - yazarken hata gösterimi, tek tıkla düzeltme;
  - üzerine gelince belge, adlı argüman ipuçları, tamamlama;
  - tanıma git (Ctrl+tık ya da çift tıkla başka dosyadaki tanıma ve `aktar "..."` satırından modül dosyasına), kullanımları bul, yeniden adlandır;
  - anlamsal renklendirme;
  - test gezgini ve satır üstünde çalıştır ya da hata ayıkla düğmeleri;
  - tensör şekillerinin satır içinde gösterimi, eğitimi canlı izleyen panel.
- **Hata ayıklayıcı (DAP).** Kullanıcı dili çoğunlukla düzenleyici ve hata ayıklayıcı üzerinden yaşar.
  - **Öne alınanlar (bilinen hatalar, evre beklemeden):** F5'te `girdi()` (Debug Console ya da tümleşik terminal üzerinden girdi kanalı); `kesme_noktası` deyiminin kendi satırında durması (bugün bir sonraki deyimde duruyor); `aktar` ile yüklenen modüllerde kesme noktası.
  - Koşullu kesme noktası, isabet sayılı kesme noktası, iz noktası (durmadan yazdıran); hata fırlatılınca durma.
  - Çağrı yığınında modüller arası gezinme; tensörlerde şekil, tür, cihaz, NaN/Inf sayısı ve gradyan özeti; eğitimde "kayıp NaN olunca dur".
  - Bitiş ölçütü: örneklerin hata ayıklama alıştırmaları (42, 43) VS Code'da elle ve otomatik sınamada (DAP sürüşüyle) baştan sona çalışır.
- **İzleme Masası:** Devam düğmesi; gradyan ve tensör özeti.
- **Transformer kitaplığı:** kitaplık adlarının yazım dillerinde çevirisi, hazır eğitim döngüsü, iyileştiricide sonradan değiştirilebilen öğrenme oranı.
- **Kodlu saklama:** otomatik sınamanın kodlu biçimi denetlemesi; depodaki örneklerin ve testlerin kodlu biçime çevrilmesi.
- **Standart kitaplık:** sıkıştırma (zip), özet (hash), tarih ve saat, düzenli ifadeler; her ad önce tasarlanıp onaylanır.

## Evre 6: Ölçek ve yerel çıkarım

- **Tek GPU'da `torch.compile` ile başa baş hız (H4, ≤ 136 ms/adım hedef modelde; Evre 3'ten taşındı).** Ölçümün gösterdiği sırayla, her adım önce ve sonra ölçülür:
  - norm geri yönünün kaynaştırılması (bugün adımda ~14 ms);
  - dikkatin adımlı girdi ve çıktı okuması (başlara ayırma kopyaları, ~6 ms);
  - kalan float32→bf16 dönüşümleri (~5,6 ms);
  - indirgemeler (~15 ms) ve cuBLASLt eplogları (sapma, GELU, sapma gradyanı).
- **Uzun eğitimin gözlemlenebilirliği:** parametre normu, katman başına etkinleşme RMS'i, en büyük logit ya da log Z, fp16'da kayıp ölçeği, MFU; z-loss, "N toplu işi atla", devamda öğrenme oranı ölçeği; eşzamansız kontrol noktası (Colab'da asıl arıza oturumun kesilmesidir).

- **Yerel çıkarım:**
  - GGUF dışa aktarma: önce F16/BF16 ve Q8_0, sonra Q4_K ailesi;
  - yalnızca ağırlık int8/int4 nicemleme ve NF4: matris çarpımı içinde anında geri açan tek bir çekirdek ailesi hem QLoRA eğitimine hem sunuma hizmet eder.
- **Çoklu GPU:**
  - NCCL ile veri paralelliği (DDP): kova sırası sabit (belirlenimcilik için), fp32 biriktirme, hesapla örtüşen iletişim, gradyan biriktirirken eşitlemeyi atlama;
  - her GPU'ya kendi veri parçası (Evre 3'teki belirlenimci sıra üstünde);
  - ardından ZeRO-1 (ek iletişim hacmi yok). Bölme parametre başınadır, kontrol noktasındaki adlar değişmez.
- **GPU otomatik sınaması:** gradyan kapısı ve GPU testleri zamanlanmış olarak, en az iki kart türünde (T4 ve L4).
- **Veri:** Parquet ve jsonl akışı.

**Bitiş ölçütü:**
- Aynı eğitim iki GPU'da veri paralel koşar ve tek GPU'yla aynı sonucu verir.
- Model GGUF olarak dışa aktarılır ve llama.cpp'de çalışır.
- GPU testleri otomatik koşar.

## Evre 7: Yaygınlaşma

- **Model Sihirbazı:** kod bilmeyenin bile model kurmasını sağlayan, okunur TR kodu üreten sihirbaz.
- **Az teknik kullanıcı yolu:**
  - ham metinden veri kümesi hazırlama;
  - parçalayıcı eğitme;
  - çalıştırmadan önce süre, bellek ve maliyet tahmini;
  - küçük model tarifleri;
  - eğitilen modeli tek dosya uygulama ya da web hizmeti olarak paylaşma.
- **Sunum ölçeği:** sürekli (iterasyon düzeyinde) toplu işleme ve sayfalı KV önbelleği, eşzamanlı kullanıcı olunca.
- **Model kapsamı:**
  - görüntü ve ses: evrişim (1B/2B), havuzlama, yukarı örnekleme, BatchNorm/GroupNorm, görüntü çözme (PNG/JPEG);
  - karşıtsal kayıp;
  - MoE: en iyi k yönlendirme ve gruplu çarpım.
- **Büyük ölçek:** FSDP/ZeRO-3, tensör, boru hattı ve bağlam paralelliği; bulut depolamadan (S3/GCS) veri akışı.
- **Ekosistem:**
  - TR kitaplıkları için paket yöneticisi (kitaplık ekosistemi oluşunca);
  - deney takibi aktarımı (TensorBoard, MLflow; `.jsonl` günlük taban olarak yeterli);
  - değerlendirme kitaplığı (OpenAI uyumlu sunucuyla lm-evaluation-harness bunu büyük ölçüde karşılar).
- **Yazım dilleri:** adların İngilizcesi var (`en.json`, `tr göster --dil en`). Altyapı Evre 2'de kurulduğu için burada iş çeviridir: bütün iletilerin İngilizcesi (`internal/lang/messages/en.json`), dil tanımının, kılavuzun ve kod başvurusunun İngilizcesi.
- **Platformlar:** Mac otomatik sınaması, Apple M ve ARM için vektör çekirdekleri, Metal arka ucu, AMD (ROCm).

## Sonra

- **FP8 eğitimi** (Hopper/Blackwell): 100 M–1 B ölçeğinde kazanç küçüktür. Başladığında yalnızca doğrusal katmanlarda başlar; dikkat, norm, gömme ve çıkış başı bf16'da kalır.
- **TR ile çekirdek yazma (karo dili):**
  - Triton/TileLang uzlaşısını izler:
    - ızgara üzerinde bir program;
    - derleme anında şekli belli karolar;
    - maskeli yükleme ve saklama, mma, indirgeme;
    - türlendirilmiş zamanlama düğmeleri;
    - satır içi PTX kaçış kapısı.
  - Otomatik ayarlama zorunlu değildir.
  - Bitiş: elle yazılmış norm, softmax, çapraz entropi ve AdamW çekirdekleri karo dilinde aynı hızda yeniden yazılır.
- **Genel otomatik kaynaştırma:** ancak kural tabanlı birleştirici (`kaynaştır`, dil tanımı 15.45) ölçümle yetersiz kalırsa; indirgeme ve çarpım içeren zincirler (norm geri yönü, çarpım eplogları) için H4 (Evre 6) ölçümleri karar verir.
- **Bölme bilgisi tensör tipinde:** tip denetleyicisi belirsiz bölmeyi hata sayar (JAX'ın açık kipi emsali).
- **ONNX:** büyük işlem yüzeyi, kendi yürütme motoru olan bir dil için düşük getiri.
- **Statik şekil denetimi:** tensör şekil hatalarının çalıştırmadan önce yakalanması.
- **İleri yönlü ve yüksek mertebe türev**, uzun bağlam için dizi paralelliği, hiperparametre taraması.
- **Matematik:** doğrusal sistem çözme, ters matris, ayrışımlar (QR, Cholesky, tekil değer), hızlı Fourier dönüşümü; türevli, CPU ve GPU çekirdekli.

## Doğrulanacaklar

Kararlar bu soruların yanıtıyla gözden geçirilir; ilk ikisi TR'nin kendi ölçümüyle kapanır:

- 100 M ölçekli GPT'de Blackwell üzerinde `torch.compile`'a karşı adım süresi.
- Küçük modellerde L4 ya da RTX PRO 6000'de FP8 kazancı.
- RTX PRO 6000'in (SM120) hangi tensör çekirdeği komutlarını desteklediği (SM100'ün tcgen05/TMEM'i dahil mi): PTX ISA belgesinden.
- Colab oturumlarının kesilme sıklığı (kontrol noktası aralığını belirler).

## Platform durumu

- **Windows, Linux (Intel/AMD):** tam destek, her değişiklikte otomatik sınama.
- **Linux ve Mac (ARM):** derleniyor ve sınanıyor; vektör çekirdekleri yok (düz Go yolu).
- **GPU:** NVIDIA (sürücü ve isteğe bağlı cuBLASLt çalışma anında yüklenir); ölçümler RTX PRO 6000 Blackwell üzerinde, GPU testleri elle ve Colab'da.

## Tamamlananlar

- Yorumlayıcı: kapanış derlemesi, yuva tabanlı ortamlar; Python ile başa baş hız.
- Tensör çekirdeği: AVX2/FMA matris çarpımı (çalışma anında seçilir), ters yönlü otomatik türev, kalıcı işçi havuzu.
- Veri hattı: `.npy` parçaları, önden yükleme, önbellek; genel `tokenizer.json` parçalayıcısı.
- bf16/fp16 veri türleri ve `karışık_hassasiyet` bloğu; 16 bitlik ara gradyanlar kendi türünde, float32 ağırlığın gradyanı yuvarlanmadan.
- GPU arka ucu: Go'nun ürettiği GPU kodu, bütün tensör işlemleri ve gradyanları, tensör çekirdekli dikkat (ileri ve geri, baş boyu 64'e kadar), cuBLASLt ile bf16 matris çarpımı, çok tensörlü iyileştirici adımları, kontrol noktası ve gradyan kırpma cihazda.
- Kodlar: dilin bütün adları sabit kodlara bağlı (`internal/lang/codes`); açıklamalı başvuru `docs/codes.md`; kodlu saklama (`tr kodla`, `tr çöz`, `tr göster`) ve VS Code izdüşümü; İngilizce yazım dili.
- Tek yazım: Türkçe harfsiz biçimler, eş yazımlar ve İngilizce eşler kaldırıldı; yerleşik ad gölgeleme uyarısı.
- Matematik ve model yapı taşları: genişletilmiş eleman bazlı matematik, birleşik `dikkat`, `örnekle`, `parametreler(model)`, `türev_denetle`; Transformer kitaplığı (`tr/katman`: Doğrusal, RMSNorm, RoPE, gruplu sorgu dikkati, SwiGLU, GPT).
- Standart kitaplık: dosya ve yol işlemleri, komut satırı, günlük ve ilerleme çubuğu, kontrol noktası, profil ve canlı izleme.
- Kod tabanı İngilizceye geçti (klasörler, dosyalar, Go adları); her paketin README'si standartlarını yazar.
- Evre 3'ün yolu (2026-10-21): hedef modelin TR karşılığı, PyTorch'la aynı kalite, `torch.compile` belleği ve 1,2 katı hız; belirlenimci bırakma (15.44), kaynaştırma (15.45), normların çıktı türü (15.13), parçalı doğrusal + çapraz entropi (15.43), önbellekli GPU ayırıcısı; programın her yazım dilinde kanonik biçimiyle çalışması (17.7).
- Dış incelemeler: dil çekirdeği, hesap çekirdeği, standart kitaplık, geliştirici araçları, ileti altyapısı ve VS Code eklentisi raporları doğrulanarak düzeltildi.
- Kod grafiği (`tools/codegraph`): Go çağrıları, ileti ve ad kodları, kural→test bağları ve belge başlıkları yerel, belirlenimci bir grafikte; ajanlar dosya okumadan önce sorgular.
- Güvenlik: güvenilmeyen girdiler (tensör dosyası, kontrol noktası, `.npy`, `tokenizer.json`) sınırlarla denetlenir ve bulanıklaştırma testleriyle sınanır; VS Code eklentisi yalnızca güvenilen yorumlayıcıyı çalıştırır; otomatik sınamada sabitlenmiş eylemler.
