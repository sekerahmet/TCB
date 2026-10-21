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

**Durum (2026-10-21):** bitiş ölçütünün bütün maddeleri testle korunuyor; Evre 2 tamamlandı. Sıradaki Evre 3'ün ölçümü (`tools/gpu/COLAB.md` §10).

**Bitiş ölçütü:** kodda Türkçe ileti metni kalmaz (testle); ileti kataloğu çoğul ve liste kalıplarını taşır, Go'da ileti sözcüğü çekimlenmez ve İngilizce pilot geçer; her türevli işlemin gradyan testi vardır; anlam kuralları yazılı ve testlidir; her kural bir testle korunur; bekleyen dil kararı kalmaz.

## Evre 3: Tek GPU'da gerçek model (şu an)

Dilin asıl sınavı, Python/PyTorch ile yazılmış, hız ve doğruluk için optimize edilmiş, yalnızca kod çözücülü (decoder-only) bir Transformer ve küçük sürümleridir (hedef model). Tek GPU'da sınır hız değil, bellek ve baş boyu uyumudur; sıra buna göredir. Eksik parçalar Evre 2'nin kurallarına uyarak eklenir.

1. **Önce ölçüm (yapıldı, 2026-10-21).** RTX PRO 6000 (Blackwell, 95 GB), torch 2.11, 12 katman D768, bağlam 512, toplu 32, bf16, AdamW (`tools/gpu/COLAB.md` §10):

   | | ms/adım (medyan) | tepe bellek | ilk adım |
   | :-- | --: | --: | --: |
   | TR | 82,0 | 13,4 GB | 0,7 sn |
   | PyTorch eager | 81,1 | 14,4 GB | 1,4 sn |
   | `torch.compile` | 71,1 | 12,8 GB | 9,7 sn |
   | `max-autotune` | 71,6 | 12,7 GB | 56 sn |

   TR eager PyTorch'la başa baş, `torch.compile`'dan 1,15 kat yavaştır. Nsight: fark başlatma boşluğunda değil (TR'de GPU %4,3 boşta, compile'da %5,2), **çekirdek süresindedir**: adım başına TR ~78,5 ms, compile ~68 ms çekirdek çalıştırır (TR 920 çekirdek/adım, compile ~675). Farkın çoğu kaynaşmamış küçük işlerdedir: float32→bf16 dönüşüm kopyaları (adımda ~1850 çağrı, ~4,7 ms), bf16 bitişik kopyalar (~3,8 ms), eleman bazlı ikili işlemler (~5,2 ms) ve norm geri yönü (~4 ms); compile bunları Triton çekirdeklerinde kaynaştırır. Matris çarpımları iki tarafta da aynı cuBLASLt/CUTLASS çekirdekleridir. Kayıp eğrileri aynı eğilimdedir (TR'nin başlangıcı farklı üreteçten).

   Hedef model (`tools/gpu/rgn512.py`: yerel ve genel katmanlar, GQA, katman başına gömme, Muon; 205M parametre, 64×1024 token/adım, sentetik veri) aynı kartta: `torch.compile` 136 ms/adım (481 bin token/sn), eager 347 ms/adım. Bu modelde derleme eager'a göre 2,55 kat kazandırır (düz GPT'de 1,14): kayan pencereli dikkat, katman başına gömme ve küçük işlerin kaynaştırılması burada belirleyicidir. TR karşılığı yazılınca (madde 10) asıl kıyas bu modelle yapılır ve hız sırası onunla yeniden doğrulanır.
2. **Bellek:**
   - parçalı ya da birleşik doğrusal katman + çapraz entropi: logitler bütünüyle bellekte tutulmaz (büyük sözlükte en büyük tek tampon logitlerdir);
   - blok düzeyinde, isteğe bağlı aktivasyon yeniden hesaplama (yaklaşık %33 süreye etkinleşme belleğinde 10 kattan fazla azalma);
   - dikkatin adımlı girdi okuması (başlara ayırma kopyasız).
3. **Rastgelelik ve dropout.** Açık, bölünebilir ve kontrol noktasına yazılan bir rastgelelik anahtarı (JAX emsali); üstüne dropout. Yeniden hesaplanan blok aynı rastgeleliği yeniden üretir, böylece bit düzeyinde aynılık korunur. Yeniden hesaplamanın önkoşuludur.
4. **Baş boyu uyumu.**
   - tensör çekirdekli dikkatin geri yönü baş boyu 128'e (bugün en çok 64; büyüğünde SIMT yola düşer), sonra 256'ya;
   - gizli boyuttan bağımsız baş boyu (`tr/katman`'da bugün `baş_boyutu = boyut / baş`).

   Bu bir hız değil uyumluluk konusudur: açık modellerin neredeyse hepsi 128 kullanır ve hazır ağırlık yüklemenin önkoşuludur.
5. **Hız (ölçümün gösterdiği sırayla; her adım önce ve sonra ölçülür):**
   - karışık hassasiyette ağırlıkların float32→bf16 dönüşümü adımda bir kez yapılsın ve dönüşüm kopyaları azaltılsın (en büyük tek kalem);
   - görünümden doğan bitişik kopyalar (devrik, başlara ayırma) adımlı okumayla kalksın (madde 2'deki dikkat girdisiyle ortak);
   - birleşik RMSNorm (ileri ve geri), RoPE ve SwiGLU çekirdekleri;
   - eleman bazlı zincirleri tek GPU çekirdeğine kaynaştıran küçük, kural tabanlı birleştirici; her kaynaşmanın kaynaşmamış yola karşı sayısal testi vardır. Genel derleyici değildir;
   - cuBLASLt epilogları (bias, GELU, bias gradyanı).
6. **Eğitim gözlemlenebilirliği.**
   - Günlüğe yazılanlar: kırpma öncesi ve sonrası gradyan normu, parametre normu, katman başına etkinleşme RMS, en büyük logit ya da log Z, fp16'da kayıp ölçeği, saniyede token, MFU.
   - NaN/Inf'te ya da kayıp hareketli ortalamasının k katını aşınca durma ya da son kontrol noktasına dönme.
   - Seçenekler: z-loss, "N toplu işi atla", devamda öğrenme oranı ölçeği.
   - Eşzamansız kontrol noktası: Colab'da asıl arıza oturumun kesilmesidir, sık ve ucuz kontrol noktası ile sağlam otomatik devam bu yüzden gerekir.
7. **Belirlenimci veri sırası.** Örnek sırası (tohum, dönem, küresel örnek indeksi) üçlüsünün saf bir fonksiyonudur; GPU ve işçi sayısından bağımsızdır. Çoklu GPU geldiğinde belirlenimcilik bozulmaz; geri dönüp toplu iş atlama bunun üstüne kurulur.
8. **T4 yolu.** bf16 tensör çekirdeği olmayan kartlar için fp16 ve dinamik kayıp ölçekleme; ölçek değeri günlüğe yazılır.
9. **Model parçaları:** katman başına gömme tabloları (PLE) için seyrek satır güncellemesi; Muon'da aynı şekilli matrislerin toplu Newton–Schulz adımı.
10. **Karşılaştırma ve kartlar.** Hedef model TR'de yazılır; PyTorch sürümüyle hız, bellek ve kayıp eğrisi karşılaştırılır. Ölçümler ve GPU testleri birden çok kartta tekrarlanır (T4, L4, A100). CPU'da küçük bir modelle eğitim denemesi ve işlem başına sabit maliyetin profili.

**Hedef: `tools/gpu/rgn512.py`'nin TR karşılığı, kod ve hız olarak** (2026-10-21). Bileşen eşleştirmesi: ileri hesabın tamamı bugünkü yerleşiklerle yazılabilir; gerçek eksikler satır seyrek iyileştirici, logitleri tutmayan parçalı çapraz entropi ve kaynaştırmadır.

- **Kod:** aynı mimari ve eğitim tarifi, tek dosya (`tools/gpu/rgn512.tr`), PyTorch dosyasıyla aynı mertebede uzunluk. Yerel/genel katmanlar ve pencere, GQA, QK-norm, ½ SwiGLU (672), katman başına kelime gömmesi (PLE), paylaşımlı gömme, Muon + AdamW + satır seyrek RMS, WSD, bf16.
- **Doğruluk:** PyTorch'un başlangıç ağırlıkları TR'ye yüklenir (`tools/gpu/trt_io.py`, `rgn_reference.py`); aynı toplu işlerle her adımın kaybı ve gradyan normu eşittir. **Sağlandı (2026-10-21), float32 ve CPU'da:** hedef boyutta 4 adımda kayıp farkı ≤ 1·10⁻⁷, gradyan normu farkı 3·10⁻⁶ (göreli); küçük boyutlu sürümü `TestTargetModelMatchesPyTorch` her değişiklikte sınar. **GPU ve bf16'da da sağlandı (2026-10-21, RTX PRO 6000):** hedef boyut, 100 adım × 2 × 32 × 1024, aynı başlangıç ve sıralı toplu işlerle kayıp farkı her adımda ≤ 0,0033, gradyan normu farkı ≤ 0,002; daha önce görülen 10,70 / 10,58 farkı TR'den değil, iki koşunun farklı veri sırasından gelir. **Gerçek veride kalite sağlandı (2026-10-21):** Türkçe FineWeb-2, TR BPE 40k (`fw2_28_01.npy`, 250M parça), 2000 adım × 65.536 parça, WSD, iki taraf ayrı ayrı aynı kartta: doğrulama kaybı (valid.npy'nin başından 4,2M parça, hedefin yöntemi) 1000. adımda PyTorch 4,2878 / TR 4,2721, 2000. adımda PyTorch 3,9105 / TR 3,8821; 500 adımlık dilimlerde ortalama eğitim kaybı farkı −0,02 … +0,11 (tek tohum, farklı rastgele toplu işler: fark gürültü düzeyinde). Hız bu koşuda PyTorch compile 134, TR 201 ms/adım (sentetik veride 194; ~7 ms gerçek dosyadan rastgele okuma, incelenecek). Kalan: uzun koşu (≥ 0,8B parça) ve birden çok tohum.
- **H1 sağlandı (2026-10-21):** TR 241 ms/adım (eager 347, compile 136); süreyi iyileştirici adımı (Muon'un tek tek dikleştirmesi, yoğun satır RMS) belirliyor.
- **H2 sağlandı (2026-10-21):** TR 203 ms/adım (ortanca, 120 adım; GPU bf16 doğruluğu korunur, 100 adımda kayıp farkı ≤ 0,0013). Kazanç iki TR kitaplığı değişikliğinden: Muon aynı şekilli ağırlıkları tek yığında, bf16'da dikleştirir (241 → 221; 56 dikleştirme yerine 3); `Doğrusal` sapmayı çarpımın türünde ekler, karışık hassasiyette SwiGLU ve izdüşümler bf16 kalır (221 → 203; PyTorch autocast gibi). Sapma değişikliğinden önceki profil (iyileştiricisiz ileri+geri 213 ms/adım): çarpım ~51, eleman bazlı ~35, indirgeme ~26, norm ~25, dikkat ~16, fp32→bf16 dönüşüm ~16, düzen kopyası ~13, çapraz entropi ~11 ms. Satır seyrek RMS'in bütün maliyeti ~3 ms çıktı (öncelik düştü); iyileştiricinin tamamı ~10 ms. H3 için yön: kaynaştırma (norm + dönüşüm, eleman bazlı zincirler), düzen kopyalarının kaldırılması.
- **Kaynaştırma (2026-10-21):** `kaynaştır(f)` (dil tanımı 15.45) eleman bazlı zinciri tek geçişe indirir: GPU'da çıktı başına tek çekirdek, ileri yön kaynaşmamışla bit düzeyinde aynı, geri yön float32'de ve ara değerleri yeniden hesaplayarak. Kitaplıkta RoPE, SwiGLU kapısı ve PLE karışımı kaynaşık: 189 → **179 ms/adım**, 11,80 GB (tepe canlı 10,0 GB, ayrılmış 10,9 GB); eşleşme 100 adımda kayıp farkı ≤ 0,0047. Profil (adım başına): ikili işlemler 32 → 14 ms, indirgeme 22 → 15 ms, kaynaşık çekirdekler 13 ms. Tek çıktılı kaynaşık işlemin geri yönü gradyanı girdinin gradyanına doğrudan, onun türünde yazınca (float32 ara tampon ve ayrı ekleme kalktı): **167 ms/adım, 10,87 GB** (tepe canlı 9,8 GB, ayrılmış 10,3 GB), kayıp birebir aynı; kopya ailesi 33 → 25 ms/adım. Sıradaki (H3 ≤ 163): float32→bf16 dönüşümleri (~9,5 ms/adım; çoğu normun float32 çıktısının çarpım için dönüşümü), dikkat başlarının bitişik kopyaları (~6 ms), artık toplamaları.
- **H3 sağlandı (2026-10-21): 162 ms/adım, 10,81 GiB**, kayıp birebir aynı (120 adımda 10,698), PyTorch eşleşmesi 100 adımda ≤ 0,0043. Normların çıktı türü (`tür:`, `hesap_türü()`, dil tanımı 15.13): blok normları ve son norm çıktısını doğrudan çarpımın türünde yazar (167 → 163); döner konum ve PLE karışımı sonucu kaynaşık geçişin sonunda girdinin türüne çevirir (163 → 162). float32'den daraltılan çıktının gradyanı float32 tutulur (ayrı dönüşümdeki gibi); böylece ileri ve geri yön bit düzeyinde aynı kalır. Gerçek veride (2000 adım) kalite: doğrulama kaybı 1000. adımda PyTorch 4,2891 / TR 4,2729, 2000. adımda 3,9113 / 3,8816; hız gerçek veride PyTorch 134, TR 173 ms (aynı ikili sentetik veride 162-163: 10 ms'lik fark incelenecek). Profil (adım başına): çarpım 53, kopya 21 (dikkat başları 6, float32→bf16 5,6), indirge 15, ikili 14, norm geri 14, kaynaşık 13, çapraz entropi 11, dikkat 11 ms.
- **Go denetimi (2026-10-21):** Evre 3'ün Go değişiklikleri (1c92485'ten bu yana) kanıtlı denetlendi, 7 bulgu çıktı. Düzeltilenler: doğrusal+CE'de tamamen yok sayılan dilimin x gradyanı, `kaynaştır`'da dış sayının kayıtta sabitlenmesi (belgelendi, JAX gibi), kaynaşık karşılaştırmalar, profiler'ın yarış derlemesi, kod grafiğinin TR_DIL'e bağlılığı. Açık (yüksek): TR_DIL=en iken kodsuz dosya İngilizce görünümüyle çalışıyor; kullanıcı adları (`it`, `init`, `print`) ve sözlük anahtarları (`s.adım` → `step`) dile göre anlam değiştiriyor. Karar: program her yazım dilinde kanonik (Türkçe) biçimiyle çalışır, TR_DIL yalnızca gösterimi belirler (iletiler, görünüm); ad ve anahtar gibi veri olan şeyler dilden bağımsızdır.
- **Hız (aynı kart, 64×1024 token/adım):** H1 eager düzeyi ≤ 347 ms/adım; H2 ≤ 1,5 × compile (≤ 204 ms); H3 ≤ 1,2 × compile (≤ 163 ms); H4 compile ile başa baş (≤ 136 ms). Her adım 300 adımlık kısa koşularla doğrulanır; büyük boyutlar (d1024–d1536) yalnızca kilometre taşlarında ölçülür.
- **Bellek:** tepe bellek en çok compile'ın 1,1 katı.
  - Ölçüm (2026-10-21, nvidia-smi tepesi, 120 adım): PyTorch compile 10,8 GB, eager 17,0 GB; TR 30,8 GB (tepe canlı 25,2 GB). `tr profil`'in yeni tepe bellek dökümü (satır, tür, tensör sayısı) kök nedeni gösterdi: otomatik türev çizgesi düğümleri tensörlerin kendisiyle bağlıyordu, kaydedilmeyen her ara sonucun verisi geri yayılıma kadar canlı kalıyordu. Çizge artık düğümlerden oluşur, veri yalnızca kaydedilen tensörler için tutulur (PyTorch'un grad_fn / saved tensors ayrımı): TR tepe canlı 15,3 GB, nvidia-smi 21,2 GB; hız ve kayıp eğrisi değişmedi.
  - Logitler: `doğrusal_çapraz_entropi` (dil tanımı 15.43; `GPT.kayıp` kendiliğinden kullanır) logitleri 8192'lik dilimlerle hesaplar, gradyanı dilimde çıkarıp bırakır: tepe canlı 15,3 → 12,6 GB, nvidia-smi 21,8 → 17,9 GB; hız ve GPU eşleşmesi değişmedi.
  - Havuz: 1 MB üstü bloklar sürücü parçalarından en uygun boş bloğa bölünerek verilir, serbest kalınca birleşir (PyTorch CUDA caching allocator yöntemi; boyut sınıflı listede bir sınıfın boş blokları başka boyuta verilemiyordu): ayrılmış 17,3 → 13,5 GB, nvidia-smi 18,0 → 14,1 GB; canlı 12,2 GB, hız ve eşleşme değişmedi. Önce denenen ayarlar (çöp toplama eşiği, ince sınıflar) en çok 0,5 GB kazandırıyordu.
  - Normlar 16 bitlik girdiyi açmadan okur, `float32`'de hesaplar, sonucu girdinin türünde verir (dil tanımı 15.13; hedef modelin RMSNorm'u gibi): nvidia-smi 14,1 → 12,5 GB, tepe canlı 10,7 GB, hız 203 → 194 ms/adım (dönüşüm ve kopyalar kalktı); eşleşme korundu.
  - Belirlenimci bırakma (dil tanımı 15.44): tensörün belleği ona artık ulaşılamayacağı kesinleşince havuza döner (yorumlayıcının sahiplik kuralları, depolamanın sahip sayacı; kaçan değerler çöp toplayıcıya kalır). Önce yalnızca çöp toplayıcıyla denendi: bellek ancak toplama sıklaşınca iniyordu (eşik 1536 MB'de 12,38 GB / 195 ms/adım, 512 MB'de 11,85 / 277, 128 MB'de 11,61 / 696); toplama pahalıdır ve sayısı modelle büyür. Aynı ikiliyle bırakma kapalı (`TR_BIRAKMA=0`) 12,51 GB / 194 ms, açık **11,84 GB / 189 ms**; 20 adımda açık çöp toplama 1122 → 201, tepe canlı 10,7 → 10,2 GB. PyTorch eşleşmesinde (100 adım, bf16) bırakma açık ve kapalı TR kayıpları her adımda birebir aynıdır. Saf skaler TR döngülerinde, tensör kullanan programda ek yük ~%19 (tensörsüz programda yok).
  - Durum: TR 10,87 GB (nvidia-smi; GB = 1024³ bayt, bütün ölçümlerde aynı; bırakma 11,84, kaynaştırma ve doğrudan gradyanla 10,87), hedef ≤ 11,9 GB **sağlandı** (compile 10,8). Kalan fark: havuzun ayrılmış−canlı farkı ve kaçan değerler (ör. üst düzey değişkenler, `karışık_hassasiyet` bloğunun yakaladıkları).
- **Hedef modelin gösterdiği eksikler** (her biri genel çözümü bulununca ele alınır; modele özel kestirme yapılmaz):
  - Liste üreteci tek değişkenlidir: `[f(i, x) döngü i, x içinde l]` yazılamaz (döngüde yazılır); dil tanımında iki biçim tutarsız.
  - Satırlara indeksle yazma yoktur (PyTorch `index_add_`/`index_copy_`, JAX `x.at[i].add`): satır seyrek iyileştirici bu yüzden yoğun maskelidir. Hedef modelde bütün maliyeti ~3 ms; büyük sözlükte genel ihtiyaç.
  - Kaynaştırma yoktur: norm, tür dönüşümü ve eleman bazlı zincirler ayrı çekirdeklerdir; derlenmiş PyTorch'un asıl üstünlüğü budur (H3, H4).
  - FP8 yoktur; d > 2000 modellerde kazandırır, tür adları metin olduğundan aynı yere eklenir (sonraki evre).

İş sırası:
1. Kitaplık yapı taşları (`lib/katman.tr`, `lib/iyilestirici.tr`): dikkat katmanına pencere, QK-norm ve birleşik KV matrisi; SwiGLU'ya birleşik [G|U] matrisi; PLE bloğu; ada göre iyileştirici grupları, iki ağırlık sönümlü AdamW, Muon ölçek seçeneği; üretimde eos'ta durma; satır seyrek RMS'nin yoğun maskeli ilk sürümü.
2. Tam (erf) GELU seçeneği (yerleşik, CPU ve GPU).
3. Başlangıç ağırlıklarının PyTorch'tan aktarımı ve doğruluk testi; TR karşılığı yazılır, H1 ölçülür. **Yapıldı.**
4. Logitleri tutmayan doğrusal + çapraz entropi (yerleşik, GPU çekirdeği). **Yapıldı** (`doğrusal_çapraz_entropi`, var olan çekirdeklerle).
5. Seyrek gömme gradyanı ve yerleşik satır seyrek iyileştirici. *Ölçümle önceliği düştü (~3 ms); indeksle yazma eksiği gelince.*
6. Muon: aynı şekilli matrislerin toplu ve bf16 Newton–Schulz adımı. **Yapıldı** (TR kitaplığında; `Doğrusal`'ın sapma türüyle birlikte H2).
7. Kaynaştırma (madde 5'in sırasıyla: dönüşüm kopyaları, RoPE, SwiGLU ve PLE kapısı zincirleri, iyileştirici zincirleri).

**Bitiş ölçütü:**
- Hedef model TR'de PyTorch sürümüyle aynı kalitede eğitilir.
- Hız ölçütü derlenmiş (`torch.compile`) PyTorch'tur; ilk ölçümden sonra hedef oran buraya yazılır ve sağlanır.
- Baş boyu 128 tensör çekirdeğiyle ileri ve geri çalışır.
- Eğitim bir NaN'de ya da kayıp sıçramasında kendiliğinden durur ya da geri döner.

## Evre 4: Hazır modeller ve üretim yolu

Modeli kullanan geliştiricinin ilk sorusu: "Hugging Face'teki modeli yükleyip ince ayar yapabilir miyim?" Bu evre o yolu açar. Sıra bağımlılığa göredir.

1. **Tensör dili tamamlanır.**
   - Genel indeksleme: çok eksenli `t[:, 0]`, bool maskeyle seçme, topla/dağıt (gather/scatter).
   - Yığ (stack), doldur (pad), tekrarla (repeat).
   - Kullanıcıya açık tam sayı ve bool tensör türleri: token kimlikleri ve maskeler tensörle taşınır.
   - Kayıplar: BCE, KL (damıtma ve tercih eğitimi için), etiket yumuşatma (MSE var).
   - Ağırlık başlatma şemaları (Xavier, Kaiming), EMA ağırlıkları.

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
6. **Sürüm politikası.**
   - 0.y.z sürüm numaralama.
   - Her yerleşiğe ad kodu üzerinden kararlı, beta ya da prototip etiketi; kararlı davranış prototip ya da beta bir alternatif uğruna kaldırılmaz.
   - İki adımlı kaldırma: önce uyarı (bir süre ya da iki sürüm), sonra hata. Ad kodları eski adı tutabildiği için bu TR'de ucuzdur.
   - Dil ve dosya biçimi dondurma kuralları; yayın ikilileri.

   Bugünkü "deneme dönemi, geriye uyum yok" durumu bu politikayla sınırlanır ve 1.0'da biter.

**Bitiş ölçütü:**
- Hugging Face'ten Llama ailesinden küçük bir model safetensors ile yüklenir, TR'de LoRA ile ince ayar yapılır.
- Bağdaştırıcı PEFT ile okunur.
- Model OpenAI uyumlu sunucudan sunulur ve lm-evaluation-harness ile değerlendirilir.
- Sürüm politikası yazılıdır; ilk 0.y.z sürümü yayımlanır.

## Evre 5: Geliştirici deneyimi

- **Not defteri:** Jupyter çekirdeği ya da VS Code defteri; TR, Colab'da ve not defterinde hücre hücre çalışır. Yapay zekâ geliştiricileri not defterinde çalışır.
- **VS Code dil sunucusu** `tr` programının içinde (ayrıştırıcı, tip denetleyicisi ve kod listesi yeniden kullanılır):
  - yazarken hata gösterimi, tek tıkla düzeltme;
  - üzerine gelince belge, adlı argüman ipuçları, tamamlama;
  - tanıma git, kullanımları bul, yeniden adlandır;
  - anlamsal renklendirme;
  - test gezgini ve satır üstünde çalıştır ya da hata ayıkla düğmeleri;
  - tensör şekillerinin satır içinde gösterimi, eğitimi canlı izleyen panel.
- **İzleme Masası:** Devam düğmesi; gradyan ve tensör özeti.
- **Transformer kitaplığı:** kitaplık adlarının yazım dillerinde çevirisi, hazır eğitim döngüsü, iyileştiricide sonradan değiştirilebilen öğrenme oranı.
- **Kodlu saklama:** otomatik sınamanın kodlu biçimi denetlemesi; depodaki örneklerin ve testlerin kodlu biçime çevrilmesi.
- **Standart kitaplık:** sıkıştırma (zip), özet (hash), tarih ve saat, düzenli ifadeler; her ad önce tasarlanıp onaylanır.

## Evre 6: Ölçek ve yerel çıkarım

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
- **Genel otomatik kaynaştırma:** ancak küçük birleştirici ölçümle yetersiz kalırsa.
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
- Dış incelemeler: dil çekirdeği, hesap çekirdeği, standart kitaplık, geliştirici araçları, ileti altyapısı ve VS Code eklentisi raporları doğrulanarak düzeltildi.
- Kod grafiği (`tools/codegraph`): Go çağrıları, ileti ve ad kodları, kural→test bağları ve belge başlıkları yerel, belirlenimci bir grafikte; ajanlar dosya okumadan önce sorgular.
- Güvenlik: güvenilmeyen girdiler (tensör dosyası, kontrol noktası, `.npy`, `tokenizer.json`) sınırlarla denetlenir ve bulanıklaştırma testleriyle sınanır; VS Code eklentisi yalnızca güvenilen yorumlayıcıyı çalıştırır; otomatik sınamada sabitlenmiş eylemler.
