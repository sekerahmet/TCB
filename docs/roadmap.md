# TR Yol Haritası

TR'nin hedefi: yapay zekâ modelini **eğiten** ve **kullanan** geliştirici için en iyi programlama dili. Model eğitimi dilin sınavıdır; ürün dilin kendisidir. Dilin en önemli yan faydası: biraz teknik bilgisi olan herkesin kendi yapay zekâ modelini kurup eğitebilmesi.

Bugün TR, tek GPU'da sıfırdan model eğitmekte derlenmiş PyTorch'un (`torch.compile`) %7–15 gerisinde, belleği eşit kullanıyor ve aynı tarifle daha düşük kayba iniyor; hazır modelleri (Llama, Qwen3) safetensors'tan yükleyip GPU'da derlenmiş transformers'tan hızlı üretiyor; ama ince ayar yapamıyor, eğittiği modeli yaygın araçlara veremiyor. Yeni ML dillerinin (Swift for TensorFlow, Julia, Mojo, Dex, Futhark) ve Python dışı çerçevelerin (Gorgonia, GoMLX, Candle, Burn, llama.cpp) ortak dersi: kullanıcıyı dil tasarımı kazanmaz. Hazır modellere erişim, doğru gradyan, kararlı sürümler ve her gün kullanan bir iç kullanıcı kazanır. Bu yüzden sıra şudur: önce ölçüm ve doğruluk kapıları, sonra tek GPU'da bellek ve bakış boyu uyumu, sonra hazır ağırlık, ince ayar ve üretim yolu. Çoklu GPU ve genel derleyici ancak bunlar yerine oturunca gelir.

## Bugün nerede duruyoruz (2026-10-23)

Tek GPU'da Transformer eğitmek ve hazır bir modeli çalıştırmak için kullanılabilir bir dil; henüz bir ekosistem dili değil: modeli dışarı vermek, ince ayar, sunum ve ölçek eksik. Python + PyTorch'la kıyasla çekirdek dil ve eğitim hızında yakın, eğitim kalitesi, belirlenimcilik ve bellek bırakmada önde; kullanıcıyı kazandıran alışveriş tarafında Evre 5'in yarısında.

| Alan | Durum | Kanıt / eksik |
| :-- | :-- | :-- |
| Dil çekirdeği | güçlü | dil tanımı (her kural testli), tip denetimi, sahiplik ve belirlenimci bellek bırakma, iki yazım dili, kodlu saklama |
| Tensör ve türev | güçlü | otomatik türev, bf16/fp16, karışık hassasiyet, `kaynaştır`, `tekrar_oynat`, birleşik dikkat; eksik: genel indeksleme (`t[:, 0]`), tam sayı ve bool tensör türleri, yığ/doldur |
| GPU eğitim hızı | iyi | hedef modelde RTX PRO 6000'de 145 ms/adım, `torch.compile` 136 (1,07 kat); A100'de 246 / 214 (1,15); L4'te TR önde (892 / 974); SimpleStories d512 (tam dikkat) 144,5 / 128,8 (1,12); bellek eşit; H4 süren hedef (Evre 7) |
| Eğitim kalitesi | iyi | 0,8 milyar parça, iki tohum: doğrulama kaybı TR 3,5305, PyTorch 3,5536 (−0,023; tohum gürültüsü ≤ 0,005); ölçücüler birebir aynı (Evre 3, "Uzun koşu") |
| GPU üretim hızı | iyi | transformers'ın en hızlı yolu geçildi; llama.cpp f32'nin %63–83'ü (Evre 5, "Üretim hızı") |
| CPU | zayıf | eğitimde PyTorch CPU ile başa baş (karakter modeli, TR/PyTorch 0,87–1,08; `tools/bench/README.md`); üretimde llama.cpp'nin ~1/5'i; CPU'da 16 bitlik ağırlık kerneli ve nicemleme yok |
| Eğitim araçları | iyi | Muon/AdamW, kontrol noktası ve sürdürme, NaN/sıçrama koruması, veri hattı ve önbellek, parçalayıcı, profil, canlı izleme |
| Hazır modellerle alışveriş | yarım | Hugging Face'ten okuma var (Llama, Qwen3; transformers'la aynı sonuç); yazma (safetensors, `.pt`, GGUF), LoRA ve ince ayar yok |
| Sunum ve değerlendirme | yok | OpenAI uyumlu sunucu ve lm-evaluation-harness bağlantısı yok |
| Ölçek | yok | tek GPU; çoklu GPU, nicemleme, AMD ve Apple arka ucu yok |
| Araçlar | iyi | VS Code eklentisi, hata ayıklayıcı, `tr dizin`, kod grafiği, profil, Colab'da GPU sınaması; ağır koşular (tam denetim, zorlama, ölçüm) Colab'ın CPU çalışma zamanında ve her koşu kendiliğinden kayıtlı (`tools/record` → Drive, `TR_dili/olcumler/<iş>/<zaman>.json`) |
| Belgeler ve örnekler | iyi | dil tanımı, rehber, iki dilde örnekler; kitaplığın kullanıcıya görünen adları örneklerde geçer (kitaplık kapsamı testi; istisna listesi gerekçeli ve küçülüyor); iletilerin hepsi İngilizce; dil tanımı ve rehber yalnızca Türkçe |
| Kullanıcı | yok | yalnızca geliştiriciler; sürüm ve kararlılık sözü yok (Evre 5, sürüm politikası) |

Öncelik (2026-10-23): Evre 4, dilin ve kod tabanının sağlamlaştırılması (tekrarlar, güvenlik, eksik kodlar, codegraph, yeni yazım dilleri, sayısal doğruluk); maddeler kolaydan zora. Ardından dili kullanılır yapan Evre 5 (safetensors yazma, LoRA ile ince ayar, OpenAI uyumlu sunucu: "Hugging Face modelini al, ince ayar yap, kaydet, sun" yolu). H4'te önce ölçümün gösterdiği ucuz kalemler alınır.

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
2. **İletiler her dile hazır (tamamlandı).** Türkçenin dilbilgisi Go kodundan şablona taşındı; asıl çeviri Evre 8'de yapılır ([`internal/lang/messages/README.md`](../internal/lang/messages/README.md)):
   - Şablon dili ICU MessageFormat'ın alt kümesi: çoğul (`{n, plural, ...}`, dile göre CLDR kuralı), seçim (`select`), hâl eki (`{ad|dat}`; çekimciyi dilin paketi kaydeder), seçenek listesi (`messages.OrList`, CLDR liste kalıbı).
   - Go'da sözcük çekimlenmez (`turkish.Suffix` yalnızca kendi paketinde; testle), sayıya bakıp sözcük seçilmez, ileti metni `strings` ile düzenlenmez.
   - Her dil kodun bütün parametrelerini kullanır; aynı anlam tek koddur, parametre adı farklı olsa da (testle; bu denetim 8 çift kod buldu, birleştirildi).
   - İngilizce: bütün iletiler (1648 kod, `en.json`; eksik çeviri test hatasıdır, 2026-10-22), `TR_DIL=en` ile gösterilir. Bir ileti bütünüyle tek dildedir: kendisinin ya da içindeki kodlu hatanın çevirisi yoksa iletinin tamamı Türkçe gösterilir; süreç dili bir kez okunur, adlar ve iletiler aynı dildedir.
   - Dış denetimden sonra (2026-10-20): çoğul kuralı CLDR işlenenlerini alır (2^53 üstünde de doğru, Rusça gibi dillere hazır); bir parametre ya dal dışında ya her dalda geçer (testle; birbirini dışlayan değerli iki ileti ikişer koda ayrıldı); parçadan kurulan iletiler (dosya hataları, yüklem ve iyelik parçaları) tam cümleye çevrildi ve bir ileti metninin başka iletiye değer olarak geçmesini test yakalar.
   - Kalan küçükler: değer olarak geçen ad öbekleri ve sona eklenen ipuçları (ör. GPU tanı zinciri H4033–H4035, `{hint}` eki, "Bunu mu demek istediniz" eki, `testrunner` sayım satırı) çeviride sorun çıkarabilir; Evre 8'de tek şablona çevrilir. Parça testi değişkene alınmış metni ve `+` birleştirmeyi henüz görmüyor.
3. **Gradyan doğruluğu kapısı (tamamlandı).** Her türevli tensör işleminin float64 sonlu fark testi ve bf16 tolerans testi var (`gradSpec`); `behavior_grad_coverage_test.go` testi olmayan işlemi yakalar; GPU'da her türevli işlemin gradyanı CPU ile karşılaştırılır (gerçek kartta doğrulandı). `stdlib/ml`'deki 14 bileşik türev de (softmax, normlar, çapraz entropi, gömme, dikkat ...) aynı kapıdadır. bf16 toleransları ölçülerek işlem başına seçildi (ölçülenin 2 katı; tablo `internal/compute/tensor/README.md`). Bulunan tek yanlış: `üstel_eksi_bir`'in türevi çok negatif girdide 0 çıkıyordu, düzeltildi. Kalan: GPU'daki bf16 sapma tablosu Colab'da ölçülecek.
4. **Anlam kuralları yazılı ve testli (tamamlandı).** Yayınlama, tür yükseltme, tensör indeksi ve dilimi, `.dilim`, liste ve metin indeksi, sayı olmayan işlenen ve NaN kuralları dil tanımının 15.49 maddesinde tek tabloda; her biri PyTorch/NumPy ile karşılaştırıldı ve `tests/tensor_test.tr`'de testli. Tek davranış değişikliği: liste ve metinde sınır dışı okuma artık hatadır, bilinçli durum için `varsa_al` (3.6).
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

## Evre 3: Tek GPU'da gerçek model (tamamlandı)

Dilin asıl sınavı, Python/PyTorch ile yazılmış, hız ve doğruluk için optimize edilmiş, yalnızca kod çözücülü (decoder-only) bir Transformer ve küçük sürümleridir (hedef model: `tools/gpu/rgn512.py`). Eksik parçalar Evre 2'nin kurallarına uyarak eklenir.

**Durum (2026-10-23): tamamlandı.** Bitiş ölçütünün bütün maddeleri sağlandı. Hedef model TR'de yazıldı (`tools/gpu/rgn512.tr`); 0,8 milyar parçalık iki tohumlu koşuda PyTorch'tan daha düşük doğrulama kaybına iniyor (aşağıda "Uzun koşu"), bellekte derlenmiş PyTorch'la başa baş, hızda derlenmiş PyTorch'un 1,07 katı (H3 ölçütü 1,2). Üç kart türünde (RTX PRO 6000, A100, L4) bütün GPU testleri geçiyor.

**Kapanış kararı (2026-10-21):** Evre 3'ün hız ölçütü H3'tür (≤ 1,2 × compile).

H4 (compile ile başa baş) süren bir hedef olarak Evre 7'ye taşındı. Kalan kazançlar derin GPU işidir: norm geri yönünü kaynaştırma, adımlı dikkat, çarpım eplogları.

İki madde ilgili evrelere ertelendi, çünkü hedef model 10,8 GB'a sığıyor ve dropout kullanmıyor; ince ayar ve büyük model geldiğinde gerekir:
- aktivasyonların yeniden hesaplanması,
- rastgelelik anahtarı ve dropout.

### Sonuçlar (hedef model, RTX PRO 6000, 64 × 1024 parça/adım)

| | PyTorch eager | `torch.compile` | TR |
| :-- | --: | --: | --: |
| ms/adım, sentetik veri | 347 | 136 | **145** (kapanışta 162) |
| ms/adım, gerçek veri (uzun koşu) | — | 133 | **147** |
| tepe bellek (nvidia-smi, GiB) | 17,0 | 10,8 | **10,81** |
| doğrulama kaybı, 2000. adım (gerçek veri) | — | 3,9113 | **3,8816** |
| doğrulama kaybı, 0,8 milyar parça (iki tohum ortalaması) | — | 3,5536 | **3,5305** |

### Uzun koşu (2026-10-22, RTX PRO 6000, Türkçe FineWeb-2, 0,8 milyar parça = 12 207 adım, iki tohum)

Veri: dört parça dosyasından tam 800M parça (veri bir kez görülür). Doğrulama: `valid.npy`'nin başından 4,2M parça, 1000 adımda bir.

| adım | PyTorch t0 | PyTorch t1 | TR t0 | TR t1 | TR − PyTorch (ortalama) |
| --: | --: | --: | --: | --: | --: |
| 1000 | 4,3547 | 4,3499 | 4,3510 | 4,3485 | −0,003 |
| 2000 | 4,1112 | 4,1157 | 4,1000 | 4,1001 | −0,013 |
| 4000 | 3,9392 | 3,9362 | 3,9206 | 3,9209 | −0,017 |
| 6000 | 3,8618 | 3,8626 | 3,8404 | 3,8415 | −0,021 |
| 8000 | 3,8191 | 3,8171 | 3,7931 | 3,7929 | −0,025 |
| 10000 | 3,7695 | 3,7721 | 3,7473 | 3,7465 | −0,024 |
| 12000 | 3,5628 | 3,5645 | 3,5395 | 3,5402 | −0,024 |
| **12207 (son)** | **3,5530** | **3,5542** | **3,5303** | **3,5306** | **−0,023** |

- Tohum farkı iki tarafta da ≤ 0,005; TR'nin farkı bunun ~20 katı, iki tohumda aynı yönde. Perplexity'de ~%2,3 daha düşük; aynı kayba PyTorch ~%9–10 daha fazla adımda ulaşıyor.
- Ölçücü denetimi: PyTorch'un iki modeli TR'ye aktarılıp TR'nin değerlendirmesiyle ölçüldü (`tools/gpu/rgn_to_tr.py`, `rgn512.tr --adım 0`): 3,5530 ve 3,5542, PyTorch'un kendi ölçümüyle dört basamakta aynı. Fark ölçümden değil eğitimden.
- Açık soru, araştırma maddesi (Evre 4): aynı tarifle TR neden daha iyi eğitiyor? Adaylar (kanıtsız): sayısal hassasiyet (16 bite daraltılan çıktının float32 tutulan gradyanı, telafili toplamlar), iyileştiricinin bir ayrıntısı, örnekleme. Sebep bulunup PyTorch'a uygulanınca PyTorch da iyileşmeli; "TR daha iyi eğitir" ancak o zaman yazılır.
- Koşudan çıkan ders: TR tohum 1'in ağırlıkları sondaki bir hatayla kayboldu (tek adım sayısında hız dosyası satırı); düzeltildi, testli (`TestTargetModelWritesResultsAfterOddSteps`); ağırlıklar artık önce, kayıp kaydı her değerlendirmede yazılır. Metin kıyası ve "PyTorch'un ölçücüsü TR modelini ölçerse" köşesi araştırma maddesinin koşularıyla tamamlanır.

### Kartlar (2026-10-22/23, Colab; hedef model, sentetik veri)

| kart | GPU testleri ve dil tanımı testleri | TR ms/adım | `torch.compile` | eager | TR / compile | tepe bellek TR / compile / eager (GiB) | ölçüm |
| :-- | :-- | --: | --: | --: | --: | :-- | :-- |
| RTX PRO 6000 (Blackwell, sm_120) | geçti | 145 | 136 | 347 | 1,07 | 10,81 / 10,8 / 17,0 | 120 adımın ortancası, A/B 3 tur |
| A100 40 GB (Ampere, sm_80) | geçti | 246 | 214 | 549 | 1,15 | 10,76 (TR) | 300 adım × 2 tur, 100. adımdan sonraki 50'lik pencereler |
| L4 (Ada, sm_89) | geçti | 892 | 974 | 2348 | **0,92** | 10,20 / 10,43 / 16,54 | 120 adım (300 adımlık doğrulama sürüyordu) |

- TR her kartı ayar gerektirmeden tanır (sürücü, mimari, cuBLASLt) ve çekirdeklerini o karta göre üretir. İlk adımın ek yükü ~0,7 sn; `torch.compile`'ın hazırlığı L4'te ~45–50 sn.
- A100 profili (RTX'le aynı kod, adım başına GPU süresi oranı): cuBLASLt çarpımı ×1,45 (kart farkı), TR'nin kendi çekirdeklerinden `dikkat_d` ×3,4, kaynaşık zincirler ×2,15, dikkat ×2, norm ileri ×2. A100'deki farkın kaynağı bu çekirdekler (H4 adayları, Evre 7).
- H100 Colab'da verilemedi; T4'te bf16 tensör çekirdeği yoktur (fp16 yolu, Evre 7).

**Kod.** Aynı mimari ve eğitim tarifi, tek dosya, PyTorch dosyasıyla aynı mertebede uzunluk:
- yerel ve genel katmanlar, pencere, GQA, QK-norm,
- ½ SwiGLU (672),
- katman başına kelime gömmesi (PLE), paylaşımlı gömme,
- Muon + AdamW + satır seyrek RMS, WSD, bf16.

**Doğruluk.**
- PyTorch'un başlangıç ağırlıkları TR'ye yüklenir (`tools/gpu/trt_io.py`, `rgn_reference.py`); aynı toplu işlerle her adımın kaybı ve gradyan normu eşittir.
- float32 ve CPU'da, hedef boyutta, 4 adım: kayıp farkı ≤ 1·10⁻⁷. Küçük boyutlu sürümü `TestTargetModelMatchesPyTorch` her değişiklikte sınar.
- GPU ve bf16'da, 100 adım: kayıp farkı ≤ 0,0045, gradyan normu farkı ≤ 0,0041.

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
- **Belirlenimci bırakma**, dil tanımı 15.43 (189 ms, 11,84 GB):
  - Yorumlayıcının sahiplik kuralları ve depolamanın sahip sayacı, tensörün belleğini ona artık ulaşılamayacağı kesinleşince havuza döndürür.
  - Yalnızca çöp toplayıcıyla bellek ancak toplama sıklaşınca iniyordu (128 MB eşikte 11,61 GB / 696 ms).
  - Saf skaler TR döngülerinde ek yük ~%19.
- **Kaynaştırma**, dil tanımı 15.44: `kaynaştır(f)` eleman bazlı zinciri tek geçişe indirir.
  - RoPE, SwiGLU kapısı ve PLE karışımı (179).
  - Tek çıktılı kaynaşık işlemin gradyanı girdinin gradyanına doğrudan yazılır (167).
- **H3 (162 ms):** normların ve kaynaşık zincirlerin çıktısı doğrudan çarpımın türünde yazılır.
  - Dil tanımı 15.4: `tür:`, `hesap_türü()`.
  - float32'den daraltılan çıktının gradyanı float32 tutulur; ileri ve geri yön bit düzeyinde aynıdır.
- **Gerçek veri (173 → 162 ms):** gömmenin geri yönü sık geçen parçaların satırını tek blokta topluyordu.
  - Gerçek metinde parça sıklığı Zipf dağılımlıdır; aynı satıra binlerce konum düşer. Tekdüze sentetik veride bu görünmez: Zipf verisinde adım 196, tekdüzede 163 ms ölçüldü.
  - Çözüm: 32'den çok konumu olan satır parçalara bölünür, parçalar paralel toplanır, sonra sırayla eklenir (PyTorch `EmbeddingBackwardKernel.cu`, belirlenimci yol). CPU aynı sırayla toplar; CPU ve GPU bit düzeyinde eşittir.
  - Sonra: Zipf 163, gerçek veri 162 ms.
- **Profil (adım başına):**
  - çarpım 53, kopya 21 ms (dikkat bakışları 6, float32→bf16 5,6),
  - indirgeme 15, ikili işlemler 14, norm geri yönü 14, kaynaşık kerneller 13 ms,
  - çapraz entropi 11, dikkat 11 ms.

**Bellek yolu** (30,8 GB'tan 10,81 GiB'a):
- otomatik türev çizgesi yalnızca kaydedilen tensörleri tutar (PyTorch'un grad_fn / saved tensors ayrımı),
- `doğrusal_çapraz_entropi` logitleri bütün olarak tutmaz (dil tanımı 15.25),
- 1 MB üstü bloklar sürücü parçalarından en uygun boş bloğa bölünür (PyTorch'un önbellekli ayırıcısı),
- normlar 16 bitlik girdinin türünde çıktı verir,
- belirlenimci bırakma.

**Denetimler (2026-10-21).** Evre 3'ün Go değişiklikleri iki bağımsız kanıtlı denetimden geçti: Claude Opus 7 bulgu, Gemini 11 bulgu (biri öncekiyle aynı). Her bulgunun önce kalan, düzeltmeyle geçen testi depodadır. Denetimin yol açtığı tasarım kararı: program her yazım dilinde kanonik (Türkçe) biçimiyle çalışır, `TR_DIL` yalnızca gösterimi belirler (dil tanımı 17.7).

### Maddelerin durumu

1. **Önce ölçüm.** Yapıldı. 12 katman D768 düz GPT'de TR eager PyTorch'la başa baştı, `torch.compile`'dan 1,15 kat yavaştı; fark kernel süresindeydi (kaynaşmamış küçük işler). Hedef modelde derleme eager'a göre 2,55 kat kazandırıyordu; asıl kıyas bu modelle yapıldı (yukarıda).
2. **Bellek.**
   - Parçalı doğrusal + çapraz entropi: yapıldı.
   - Aktivasyonların yeniden hesaplanması: Evre 5'e ertelendi.
   - Dikkatin adımlı girdi okuması: H4 işi olarak Evre 7'ye taşındı.
3. **Rastgelelik anahtarı ve dropout.** Evre 5'e ertelendi; yeniden hesaplamanın önkoşulu.
4. **Bakış boyu uyumu. Yapıldı.** (Bakış: dikkatin *head*'i; 2026-10-21'de "baş" yerine, literatürün birebir çevirisi eş sesliydi.)
   - Tensör çekirdekli dikkatin geri yönü bakış boyu 128'e: **yapıldı (2026-10-22).** Bakış boyu 64'ü aşınca iki geri çekirdeğin döşemesi 32'dir (FlashAttention-2 `run_mha_bwd_hdim128` ve Triton fused-attention emsali; paylaşımlı bellek 48 KB'nin altında). RTX PRO 6000'de bf16 nedensel ileri + geri: B8 H8 T1024 3,81 → 0,73 ms, B4 H16 GQA T2048 14,04 → 2,41 ms; bakış boyu 64 değişmedi.
   - Gizli boyuttan bağımsız bakış boyu: yapıldı (2026-10-21). `NedenselDikkat`, `TransformerBloğu` ve `GPT` `bakış_boyutu:` alır (Hugging Face `head_dim` kuralı: verilmezse `boyut / bakış_sayısı`; sorgu `[boyut, bakış_sayısı · bakış_boyutu]`, çıkış `[bakış_sayısı · bakış_boyutu, boyut]`).

   Açık modellerin neredeyse hepsi 128 kullanır; Evre 5'teki hazır model yüklemenin önkoşuludur.
5. **Hız.**
   - Yapıldı: dönüşüm kopyalarının azaltılması, RoPE ve SwiGLU zincirleri, kural tabanlı birleştirici (`kaynaştır`).
   - Kalanlar H4 olarak Evre 7'de.
   - Gerçek verideki ~10 ms'lik fark: giderildi (gömme geri yönü, yukarıda "Hız yolu").
6. **Eğitim gözlemlenebilirliği.**
   - NaN/Inf ve kayıp sıçramasında durma ya da son kontrol noktasına dönme: yapıldı (2026-10-21). Dil parçaları verir, politikayı döngü seçer: `sonlu_mu`, `gradyan_kırp(..., sonlu_değilse_hata:)` (dil tanımı 15.36), `kn.geri_dön()` ve `ö.atla(n)` (20.14, 20.15), kitaplıkta `tr/egitim`'in sıçrama bekçisi (OLMo-core ölçütü: son 128 sağlam adıma göre 6σ). `rgn512.tr --koşu` NaN ya da sıçramada son noktaya döner ve 200 toplu iş atlar (PaLM §5.1); kontrol noktası yoksa NaN'de açık hatayla durur. Bekçi adımın sonunda zaten okunan değerleri kullanır, adıma bekleme eklemez. Örnek 44.
   - Günlüğe kırpma öncesi ve sonrası gradyan normu, saniyede parça ve kontrol noktası bugün yazılıyor.
   - Uzun eğitimin öbür ölçüleri ve seçenekleri Evre 7'de: parametre normu, etkinleşme RMS'i, log Z, MFU, z-loss, toplu iş atlama, eşzamansız kontrol noktası.
7. **Belirlenimci veri sırası.** Sıralı ve tohumlu örnekleyiciyle sağlanıyor. Çoklu GPU'da (Evre 7) yeniden doğrulanır.
8. **T4 yolu.** bf16 tensör çekirdeği olmayan kartlar için fp16 ve dinamik kayıp ölçekleme; ölçek günlüğe yazılır. Kapanış şartı değildi; Colab'ın ücretsiz kartı T4 olduğu için Evre 7'ye taşındı.
9. **Model parçaları.**
   - Muon'un toplu Newton–Schulz adımı: yapıldı.
   - PLE tabloları için seyrek satır güncellemesi: ölçümle önemsiz çıktı (~3 ms). Indeksle yazma (Evre 5, tensör dili) gelince ele alınır.
10. **Karşılaştırma ve kartlar.**
    - Hedef model karşılaştırması: yapıldı.
    - İkinci kart: **yapıldı (2026-10-23)**, üç kart türü (yukarıda "Kartlar").
    - CPU'da küçük modelle eğitim denemesi Evre 5'in CPU maddesiyle birlikte yapılır.

**Hedef modelin gösterdiği dil eksikleri** (her biri genel çözümü bulununca ele alınır; modele özel kestirme yapılmaz):
- Liste üreteci tek değişkenlidir: `[f(i, x) döngü i, x içinde l]` yazılamaz (döngüde yazılır); dil tanımında iki biçim tutarsız.
- Satırlara indeksle yazma yoktur (PyTorch `index_add_`/`index_copy_`, JAX `x.at[i].add`); Evre 5'in tensör dili maddesinde.
- FP8 yoktur; d > 2000 modellerde kazandırır, tür adları metin olduğundan aynı yere eklenir (sonraki evre).

**Bitiş ölçütü:**
- Hedef model TR'de PyTorch sürümüyle aynı kalitede eğitilir. **Sağlandı (2000 adım).**
- Kalite uzun koşuda da tutar: en az 0,8 milyar parça, iki tohum. Doğrulama kaybı PyTorch'la gürültü düzeyinde aynıdır. **Sağlandı, TR daha iyi (2026-10-22):** 3,5305 / 3,5536; ölçücüler birebir aynı ("Uzun koşu").
- Hız en çok 1,2 × `torch.compile`'dır (H3). **Sağlandı:** kapanışta 162 / 136 ms, bugün 145 / 136 (1,07); gerçek veride uzun koşuda 147 / 133 (1,10).
- Bellek en çok 1,1 × `torch.compile`'dır. **Sağlandı (10,81 / 10,8 GiB).**
- Bakış boyu 128 tensör çekirdeğiyle ileri ve geri çalışır; bakış boyu gizli boyuttan bağımsızdır. **Sağlandı (2026-10-22).**
- Eğitim bir NaN'de ya da kayıp sıçramasında kendiliğinden durur ya da son kontrol noktasına döner. **Sağlandı** (`TestTargetModelRollsBackOnNaN`).
- Ölçümler ve GPU testleri en az iki kart türünde geçer. **Sağlandı (2026-10-23):** RTX PRO 6000, A100, L4 ("Kartlar").

## Evre 4: Dilin ve kod tabanının sağlamlaştırılması (şu an)

Bundan sonra yazılacak her şey (hazır modeller, ince ayar, sunum, ölçek) dilin, tensör katmanının ve araçların üstüne kuruluyor. Evre 3'ün kapanışında bile telafisiz GPU toplamları, CPU ile GPU'da −0 farkı, çöp toplayıcıya bağlı kararsız testler ve uzun bir koşunun sonunda düşüp sonucunu kaybettiren bir satır bulundu. Temeldeki her çatlak üstüne kurulan her şeye taşınır; bu evre yeni özellik eklemeden önce temeli sağlamlaştırır ve dili yeni yazım dillerine açar. Bitince sıradaki evre (hazır modeller ve üretim yolu) başlar.

Maddeler kolaydan zora sıralıdır (2026-10-23 kararı); iş bu sırayla alınır.

1. **Özel değerlerde CPU/GPU birliği (tamamlandı).** ±0, ±∞ ve NaN için her işlemin davranışı dil tanımı 15.50'nin tablosunda yazılı (PyTorch 2.14 ve NumPy 2.1 ile karşılaştırmalı, C99 Ek F kaynaklı); ileri ve türev, CPU ve GPU testli (`TestSpecialValuesSameOnBothDevices`, `TestSpecialValuesTable`). Bulunup düzeltilenler: GPU'da `tanh`, `tanjant`, `expm1`, `sinüs` (CPU'da da) −0'ı +0 yapıyordu; GPU üssü C99'dan ayrılıyordu; CPU `tanh`(∞) 1 değildi; −0 toplamı GPU'da −0 idi (karar: +0, PyTorch ve NumPy gibi); `arktanjant2` türevi iki cihazda farklıydı; `gelu`(−∞) NaN'dı. Büyük sonlu sayılarda ara taşma ve büyük açılı trigonometri madde 8'e kaldı.
2. **Kararlı test düzeni (tamamlandı).** Çöp toplayıcıya, tekrar sayısına, sıraya ya da süreç geneli duruma bağlı test kalmadı; zorlama koşusu tam denetimde bir kiptir (`--kip=zorlama`: bütün Go testleri `-count=2 -shuffle=on`, sonra `GOGC=1`) ve haftada bir otomatik sınamada koşar (`zorlama.yml`). İlk koşuda iki test kaldı: yayın uyarılarının durumu ve çıktısı süreç genelindeydi (artık programa bağlı, `VM.ProgramValue`; uyarılar programın hata akışına yazılır, `tr test`'in dosyaları birbirinin uyarısını bastırmaz) ve codeindex'in mutlak süre tavanı yüklü makinede aşılıyordu. Süre ölçen testler artık büyümeye bakar (`testkit.ScalesLinearly`: n ve 4n aynı yükte); duvar saati tavanı kalan iki test (kesme, önden yükleme) ayırt ettikleri süreye göre genişletildi. `GOGC=1` koşusunda kalan test yok.
3. **Kartlar arası sınama.** Her GPU değişikliği en az iki mimaride sınanır; bunun için tek komutluk kart sınama betiği (kurulum, GPU testleri, dil tanımı testleri, hız; Colab, RunPod ya da başka bir GPU makinesinde aynı). Betik koşularını `tools/record` ile sarar: her kartın sonucu commit, kart ve sürücü bilgisiyle Drive'daki kayıtlara girer, kartlar ve zaman arasında karşılaştırılır.
4. **Eksik kodlar.** Kullanıcıya görünüp henüz kodlu olmayan metinler (Türkçe sabitler, adı içine gömülü iletiler, `TR_GPU_*` ortam adları), kalan CLI adları, değer olarak geçen ad öbekleri ve sona eklenen ipuçları (Evre 2'nin "kalan küçükler"i).
5. **Eğitim koşularının güvenliği, kitaplık düzeyinde.** `tr/egitim`'de bir koşu yapısı: ağırlıklar önce yazılır, kayıplar ve değerlendirmeler ara ara kaydedilir, kontrol noktası, prova kipi (aynı komut küçük boyda), sonuçların dışarı yazılması. Evre 3'ün uzun koşusunda elle eklenen güvenceler dilin kendisine geçer.
6. **Tekrar eden kodun temizlenmesi.** `codegraph dups`'ın adayları ve Gemini tekrar denetiminin kalan partisi (paketler arası) birleştirilir; kalibre olunca "yeni tekrar" tam denetimde önce uyarı, sonra kural olur.
7. **codegraph'ın geliştirilmesi.** `.tr` kodunda tekrar bulma; sorgu ve etki analizinin TR kitaplığını da kapsaması; CODEGRAPH oturumunda biriken fikirler. 2026-10-23 işlerinde elle yapılıp codegraph'ta olması gerekenler:
   - **Arayüz denetiminin sakladığı ölü yol.** Yalnızca derleme anı arayüz denetimiyle (`var _ I = (*T)(nil)`) canlı görünen, çağıranı olmayan metot; deadcode bunu görmez (`TensorValue.Operator`, yorumlayıcı önce `TaskOperatorHook`'u denediği için hiç çağrılmıyordu; `who-uses` "no uses" diyerek buldu).
   - **Süreç geneli değişebilir durum.** `init` dışında yazılan paket değişkenleri (harita, bayrak, yazıcı): programa ya da göreve ait olması gereken durumun adayları (yayın uyarıları süreç genelindeydi; kural: `VM.ProgramValue`/`VM.TaskValue`).
   - **Duvar saati tavanı olan testler.** `_test.go`'da `time.Since(...)` sabitle karşılaştırılıyorsa uyarı (kural: süre testi büyümeye bakar, `testkit.ScalesLinearly`); bugün 14 yer elle tarandı.
   - **Dil tanımı atıfları.** `**15.N**` ve "dil tanımı 15.N" anmalarının hedef maddenin başlığıyla listelenmesi ve olmayan maddeye atfın hata olması; numaradan sonraki Türkçe ekin ses uyumu (bölüm 15 yeniden numaralanırken 6 ek ve 1 yanlış atıf betikle bulundu).
   - **Madde yeniden numaralama.** Eski → yeni eşlemeyle bütün depodaki atıfları, test adlarını ve aralıkları güncelleyen komut (bugün tek seferlik bir betikle yapıldı).
8. **Sayısal doğruluk kapısı.** Her tensör işleminin CPU ve GPU sonucu float64 referansa karşı sınanır (normlar ve toplamlar için 2026-10-22'de başladı: `TestNormsMatchFloat64Reference`, `TestSumsMatchFloat64Reference`); hata sınırları işlem başına yazılır; Evre 2'den kalan GPU bf16 sapma tablosu burada kapanır. Madde 1'den kalanlar: büyük sonlu sayılarda ara taşma (normlarda |x| ≳ 1,8e19, toplamı 3,4e38'i aşan ortalama: CPU float64'te sonlu, GPU sonsuz), GPU trigonometrisi |x| > 1e5'te yanlış, `log10` ve `arktanjant2` türevinde 3e38 civarında alt normal yerine 0, çapraz entropi çok büyük logitte `lse − x` ile hassasiyet kaybediyor ([3e38, 3e38, 0]'da log 2 yerine 0; `(m − x) + log Σ` sırası düzeltir).
9. **Güvenlik.** Saldırı yüzeyinin baştan sona denetimi (dosya biçimleri, kum havuzu, komut satırı, hata ayıklayıcı, VS Code eklentisi), bulanıklaştırma kapsamının genişletilmesi; govulncheck, staticcheck ve ölü kod kuralları sürer (2026-10-22'de kuruldu). Her bulgu önce yeniden üretilir, testle kapanır.
10. **Yeni yazım dilleri: İspanyolca, Almanca, Fransızca** (Evre 8'den öne çekildi; altyapı Evre 2'de kuruldu, İngilizcede iletilerin hepsi var). Her dil için adların ve iletilerin çevirisi (anahtar sözcük ve ad çakışmaları testle, dile göre çoğul ve liste kalıpları), `tr göster --dil` ile gidiş-dönüş testi, Gemini çapraz okuması ve mümkünse ana dili konuşan birine örnek okutma.
11. **"TR neden daha iyi eğitiyor?"** (Evre 3, "Uzun koşu"). Sebep bulunur, PyTorch'a uygulanıp PyTorch'un da iyileştiği gösterilir; kendi sayısal yolumuzu anlamak bu evrenin parçasıdır.
12. **GPU belleği TR programından okunur.** Sayaçlar zaten var: GPU arka ucunun havuzu canlı, ayrılmış, havuzda bekleyen bellekle canlı ve ayrılmış belleğin tepesini tutuyor (`internal/compute/gpu` `MemorySample`, `memory.go` `measurement`); profil aracı bunları (H4047) ve tepe anında belleği ayıran satırları gösteriyor (`PeakLiveBySite`). Eksik olan yalnızca bunların dile açılması: bir TR programı (`tools/gpu/gpt_training.tr` ve hedef model betiği) günlüğüne tepe belleği yazamıyor, dışarıdan `nvidia-smi` yoklaması kullanılıyor (sürecin bütün ayırıcı önbelleği; tensör belleği değil). Kıyas koşularının `metrics.jsonl`'i her kayıtta `mem_gb` (PyTorch `torch.cuda.max_memory_allocated`) yazıyor. Yapılacak: var olan sayaçları okuyan bir yerleşik ve tepeyi sıfırlama, yeni sayaç yazılmaz. Tasarımdan önce kaynaklı karşılaştırma (PyTorch `memory_allocated` / `max_memory_allocated` / `memory_reserved` / `reset_peak_memory_stats`, JAX `Device.memory_stats()`), sonra ad kodu ve dil tanımı maddesi.

Evre içinde maddelerin dışında yapılanlar (2026-10-23):

- **SimpleStories'te PyTorch'la adil kıyas.** Araştırma deposunun `ss_d512_transformer` koşusu (düz Transformer, d512, 8 katman, tam dikkat, 1 tur = 9241 adım) aynı tarifle TR'de eğitildi (`tools/gpu/gpt_training.tr` artık tarifin bütün ayarlarını açar; `tools/gpu/ss_exam.tr` araştırma deposunun ölçüsüyle sınar: bütün doğrulama verisi, bpb, doğruluk, konum bantları, açgözlü sondalar). Sonuç (RTX PRO 6000, kayıt `TR_dili/kosular/ss_d512_tf_tr`): kayıp 1,5134 / 1,5196, bpb 0,5053 / 0,5073, doğruluk 0,5837 / 0,5827; bantların ve ara değerlendirmelerin hepsinde TR önde; açgözlü üretimde döngü yok, tutarlı hikâyeler. Hız 144,5 / 128,8 ms/adım (1,12 kat; profil H4 listesinde, Evre 7). Kalan tek bilinçli fark örnekleme: TR her turda veriyi karıştırıp her pencereyi bir kez gösterir, araştırma deposu yerine koyarak seçer.
- **Dil tanımı bölüm 15 konulara göre düzenlendi ve kodla karşılaştırıldı.** 50 madde 10 ara başlık altında bir kez yeniden numaralandı (Array API standardı, PyTorch ve NumPy'ın düzenine göre); yeni madde konusunun sonuna harfle eklenir (`15.17a`), numaralar kaymaz. Her madde yaklaşık 70 TR yoklamasıyla kodla karşılaştırıldı, 5 yer düzeltildi. Ardından Gemini bağımsız denetledi; DENETİM doğruladı: 17 bulgudan 6'sı onay, 10'u uydurma alıntı yüzünden ret, ama davranışları gerçek olan 10'u (Y1–Y10) yeniden yazıldı. Düzeltmeler MEKANİK'te, beş partide; en önemlisi dikkatte NaN'lı satırın bakmadığı anahtarlara NaN sızdırması (CPU ve GPU, gerçek kart sınaması bekliyor).
- **Ölçüm kaydı ve Colab'da ağır koşular.** `tools/record` her koşuyu (commit, makine, süre, ölçümler) JSON olarak Drive'a yazar; tam denetim, zorlama ve hız ölçümleri geliştiricinin meşgul makinesi yerine Colab'ın CPU çalışma zamanında koşar. İlk CPU temel ölçümü (`cpu-temel`, 2026-10-23: dil kıyası ve 39 tensör kıyası, 5'er tekrar; CV ortancası %4,4) Evre 3 kapanışıyla (`ed98a547`) aynı makinede karşılaştırıldı: gerileme yok; `TrainStep` 11,4 → 11,1 ms, `exp` ×0,81, `BiasGrad` ×0,72. Colab CPU %10'dan büyük farkı güvenle gösterir. Ölçüm artık karakter modelini de TR ve PyTorch'ta (PyPI'daki son sürüm, 2.14.1) 1–8 iş parçacığıyla karşılaştırır (`tools/bench/cpu_baseline.sh`): TR/PyTorch 0,87–1,08, yani başa baş; düz dil programlarında ise TR Python'dan ~1,5 kat yavaş (metin ve sözlük 2 kat).

**Bitiş ölçütü:**
- Özel değer tablosu yazılı ve testlidir; zorlama koşusunda kararsız test yoktur; GPU testleri her değişiklikte en az iki mimaride geçer.
- Kullanıcıya görünen her metin kodludur (testle).
- Uzun bir eğitim koşusu sondaki bir hatayla sonucunu kaybedemez (testle).
- Tam denetimde "yeni tekrar" kuralı açıktır; `.tr` kodu için tekrar bulma çalışır.
- Her türevli tensör işleminin CPU ve GPU'da float64 referans testi vardır.
- Güvenlik denetiminin bütün bulguları testle kapanmıştır.
- Dört yazım dilinde (İngilizce, İspanyolca, Almanca, Fransızca) bütün adlar ve iletiler vardır; eksik çeviri test hatasıdır.
- "TR neden daha iyi eğitiyor" sorusu kanıtla cevaplanmıştır.

## Evre 5: Hazır modeller ve üretim yolu

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
   - **Okuma yapıldı (2026-10-21, öne çekildi):** `tensör_yükle` safetensors'u okur (dil tanımı 15.47); `tr/hazir` Llama ve Qwen3 biçimindeki modelleri `GPT`'ye yükler, küçük modellerde transformers'la logit farkı ~3·10⁻⁷. Gerçek modellerle (SmolLM2-135M, Qwen3-0.6B) transformers'la en büyük logit farkı 1,8·10⁻⁴ / 2,5·10⁻⁵, açgözlü üretim aynı (2026-10-22). Kalan: yazma, bölünmüş dizin.
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
5. **İyileştirici ve öğrenme oranı (araştırma maddesi; gerekirse öne çekilir).** TR yeni bir optimizasyon matematiği icat etmez; bilinen en iyi yöntemleri varsayılan, hızlı ve ölçülebilir yapar. TR her ağırlığın şeklini ve rolünü (gömme, gizli matris, norm, sapma) bildiği için PyTorch'ta elle yapılan işaretlemeler kendiliğinden yapılabilir. Adaylar (tasarımdan önce kaynaklı karşılaştırma, sonra hedef modelde ölçüm: aynı kaliteye daha az adım ya da daha az ayar):
   - Öğrenme oranının küçük modelden büyüğe aktarımı (μP): oran küçük modelde ayarlanır, genişlik büyüyünce hesapla aktarılır.
   - Planlamasız iyileştirici (Schedule-Free): ısınma ve azalma planı ile toplam adım sayısını önceden bilmek gerekmez.
   - Düşük bitli iyileştirici durumu (bf16 ya da 8 bit momentler): durum bugün modelin 2–3 katı yer tutar.
   - Ağırlık gruplamanın kural tabanlı olması: hangi ağırlığa Muon, hangisine AdamW, hangisine ağırlık çürümesi (bugün `MuonVeAdamW` yarı yarıya yapıyor).
   - Katman başına güncelleme/ağırlık oranı gibi göstergeler (Evre 7'nin gözlemlenebilirlik maddesiyle birlikte).

   Kaynaklar henüz okunmadı (kanıtsız): μP (Yang ve ark. 2022), Schedule-Free (Defazio ve ark. 2024), 8 bit Adam (Dettmers ve ark. 2021), Muon (Jordan 2024).
6. **Çıkarım ve sunum.**
   - KV önbellekli `üret`: **yapıldı (2026-10-21)**; `KVÖnbellek` sabit ve büyüyen (`çeşit:`), `kırp`, yalnızca son logit. Kalan: kayan pencereli katmanlarda halka tampon (pencere boyunda önbellek), 8 bitlik önbellek, önek saklama (ortak başlangıcı yeniden işlememe), tahminli çözme (taslak model ya da MTP; `kırp` hazır). Örnekleme: en iyi p kalan.
   - **Üretim hızı (2026-10-22, aşağıdaki "Üretim hızı" tablosu).** Yapılanlar sırasıyla: tek satırlık çarpım (M = 1) kernelleri CPU'da (AVX2/FMA) ve GPU'da; kendi akış; CUDA Graph (`tekrar_oynat`, dil tanımı 15.45; `yerine_yaz` 15.8; `GPT.tek_adım`: konum ve kimlik cihazda, transformers sabit önbelleği gibi); katman başına kernel azaltma (maske adımda bir kez, maske kopyasız, K|V ve kapı|yukarı birleşik yükleme); 16 bitlik ağırlık (`hazır_model(ağırlık_türü: "bf16")`: ağırlık açılmadan okunur, hesap float32). Kalan, ölçülmüş sırayla: açılış süresi (Qwen3 ~5 sn, çoğu boşa giden rastgele başlatma); katman başına kalan 14 kernelin kaynaştırılması (kalıntı + norm, q ve k RoPE tek kernel, k ve v yazımı tek kernel); CPU'da 16 bitlik ağırlık kerneli (bf16 varsayılan olabilsin diye); CPU hızının yeni kernellerle yeniden ölçülmesi.
   - 127.0.0.1'e bağlı OpenAI uyumlu HTTP sunucusu: completions ve chat, `logprobs` ile. Bu tek adım lm-evaluation-harness ile değerlendirmeyi, arayüzleri ve ajan çerçevelerini getirir.
   - Yorumlayıcı kilidi kernel çağrılarında ve G/Ç'de bırakılır; hangi yerleşiklerin bıraktığı belgelenir. Böylece sunucu, parçalayıcı ve GPU eşzamanlı çalışır.
   - CUDA Graphs önce üretim için: **yapıldı (2026-10-22)**, `tekrar_oynat(f)` (dil tanımı 15.45): ikinci çağrı kaydeder, sonrakiler oynatır; kayıtta cihazdan okuma, konaktan yükleme ve eşzamanlama kodlu hatadır; kayda özel bellek havuzu; ulaşılamaz kaydın belleği bırakılır. Gemini kayıt ve sayısal denetiminden sonra (2026-10-22, 16 bulgu, hepsi testli): kayıt dokunduğu her depolamayı tutar (bırakılsa da kayıt yaşadıkça başka işe verilmez), iç içe `tekrar_oynat` doğrudan çağrılır, kayıt sırasında yorumlayıcı kilidi başka göreve geçmez, yerinde değişen argüman çağırana geri yazılır, sayı işlenenleri cihazda doldurulur. Eğitimde kazanç konakta boşa giden süreye bağlıdır; H4 profili ölçecek.
   - **CPU'da hazır model:** küçük bir hazır model (yüz milyonlarca parametre) GPU'suz, dizüstünde verimli çalışır. Ağırlıklar safetensors'tan bellek eşlemeyle (mmap) kopyasız yüklenir. KV önbellekli üretim tek ve çok çekirdekte koşar, AVX2/FMA kernelleri kullanılır (tek satırlık çarpım kerneli yapıldı, 2026-10-21: dizüstünde SmolLM2 ~%40); ağırlığı bf16 saklanan çarpım anında genişletilir (bugün CPU'da ağırlık her çağrıda açılıyor; kernel yok). Ölçüt: aynı model ve aynı ayarlarla PyTorch CPU ve llama.cpp'ye karşı token/sn ile ilk token süresi. Hedef oran ilk ölçümden sonra buraya yazılır (Evre 3'teki gibi). Yalnızca ağırlık int8/int4 nicemleme (Evre 7) bu yolu hızlandırır. CPU eğitim kernellerinden `relu`, `sigmoid` ve `silu` vektörsüzdür (hiç olmadı; gerileme değil): Colab CPU'da `relu` ileri + geri 6,9 ms, vektörlü `gelu` 2,2 ms. `relu`'nun dalı (NaN korunur) rastgele işaretli veride yanlış tahmin edilir ve maliyeti 1 sayıldığı için az işçi alır; AVX2 `VMAXPS` ile dalsız yazılır (NaN'ı ikinci işlenen verir), `sigmoid` ve `silu` vektörel üstelle.
7. **Sürüm politikası.**
   - 0.y.z sürüm numaralama.
   - Her yerleşiğe ad kodu üzerinden kararlı, beta ya da prototip etiketi; kararlı davranış prototip ya da beta bir alternatif uğruna kaldırılmaz.
   - İki adımlı kaldırma: önce uyarı (bir süre ya da iki sürüm), sonra hata. Ad kodları eski adı tutabildiği için bu TR'de ucuzdur.
   - **Adlandırma standardı (2026-10-21, Ahmet'in önerisi). Kısmen yapıldı.** Kaynaklı karşılaştırma (PEP 8, Rust varsayılan uyarıları, Dart kitaplığa özel `_`, Kotlin, Swift API Design Guidelines) ile üç kural: ad yazılışı uyarısı (2.3b: sabit `BÜYÜK_YILAN`, yapı `BüyükDeve`, öbürleri `küçük_yılan`, matematik adları muaf), eş adlar (17: aynı ad yalnızca aynı anlamda ya da ayrı alanda; tensörde `birleştir` → `bitiştir`, türev denetiminde `adım` → `fark_adımı`; `docs/terimler.md` tablosu ve `TestSharedNamesListed`), başı `_` olan üst düzey adlar modüle özel (11.7, Dart gibi dil uygular). Kalan: `_` kuralının yapı alanları ve metotlarına uygulanması (`nesne._alan`; tanımlayan modülü çalışma anında bilmek gerekir), `tr dizin` ve eklentinin tamamlamasında özel adların gizlenmesi, kitaplığın iç yardımcılarının (`SıçramaBekçisi.aşıyor` ...) `_` ile işaretlenmesi.
   - **Kitaplık adları için eski ad bildirimi (araştırma maddesi; gerekirse öne çekilir).** Yerleşik adlar ad kodlarıyla korunur: ad değişince eski ad (`eski`) uyarıyla çalışmaya devam eder. Kitaplıktaki (`tr/...`) yapı, fonksiyon ve parametre adlarında bu yoktur; her yeniden adlandırma kullanıcı kodunu kırar. Türkçe terimler literatürün ilk tercümesinden geldiği için değişecektir (ilk örnek 2026-10-21: dikkat "baş"ı → "bakış"). Adaylar: Swift `@available(renamed:)`, Kotlin `@Deprecated(ReplaceWith)`, Python `warnings.warn(DeprecationWarning)` ile eski parametre adını kabul etmek. Tasarımdan önce kaynaklı karşılaştırma; aracı `tr dizin yeniden_adlandır` (etki listesi) ile birlikte düşünülür.
   - Dil ve dosya biçimi dondurma kuralları; yayın ikilileri.

   Bugünkü "deneme dönemi, geriye uyum yok" durumu bu politikayla sınırlanır ve 1.0'da biter.

### Üretim hızı (2026-10-22, Colab RTX PRO 6000 Blackwell + EPYC 9B45 48 iş parçacığı; parça/sn, 100 parça açgözlü)

| Model / cihaz | TR | transformers eager | transformers static + `torch.compile` | llama.cpp f32 | llama.cpp f16 |
| :-- | --: | --: | --: | --: | --: |
| SmolLM2-135M GPU | **608** (bf16 ağırlık; float32 548) | 107 | 472 | 965 | 1091 |
| Qwen3-0.6B GPU | **366** (bf16 ağırlık; float32 290) | 94 | 281 | 442 | 665 |
| SmolLM2-135M CPU | 36 (tek satırlık CPU kernelinden önce; yeniden ölçülecek) | 37 | — | 213 | 300 |
| Qwen3-0.6B CPU | 12,1 (aynı) | 11,5 | — | 47,5 | 91 |

GPU yolu: 213 → 224 (tek satırlık çarpım kernelleri) → 230 (kendi akış) → 496 (CUDA Graph) → 548 (kernel azaltma) → 608 (bf16 ağırlık); Qwen3 177 → 187 → 192 → 277 → 292 → 366. Her adımda çıktılar aynı parçalar (kayıtlı ve kayıtsız, float32 ve bf16 ağırlık). Yeniden üretmek: `tools/gpu/generation_race.sh` (`tools/gpu/COLAB.md` §12; rakip sürümleri sonuç dosyasının başına yazılır). GPU'da transformers'ın en hızlı yolu geçildi; llama.cpp f32'nin SmolLM2'de %63'ü, Qwen3'te %83'ü. CPU'da fark büyük (llama.cpp ~5 kat): CPU yolu Evre 5'in bitiş ölçütündedir.

**Bitiş ölçütü:**
- Hugging Face'ten Llama ailesinden küçük bir model safetensors ile yüklenir, TR'de LoRA ile ince ayar yapılır.
- TR'de eğitilen hedef model safetensors ve `.pt` olarak dışa aktarılır, PyTorch'ta aynı kaybı verir; PyTorch'ta eğitilen ağırlıklar TR'ye yüklenir.
- Küçük bir hazır model CPU'da KV önbellekle üretir; token/sn PyTorch CPU ve llama.cpp ile ölçülüp yazılır ve hedef oran sağlanır.
- Bağdaştırıcı PEFT ile okunur.
- Model OpenAI uyumlu sunucudan sunulur ve lm-evaluation-harness ile değerlendirilir.
- Sürüm politikası yazılıdır; ilk 0.y.z sürümü yayımlanır.

## Evre 6: Geliştirici deneyimi

- **Not defteri:** Jupyter çekirdeği ya da VS Code defteri; TR, Colab'da ve not defterinde hücre hücre çalışır. Yapay zekâ geliştiricileri not defterinde çalışır.
- **VS Code dil sunucusu** `tr` programının içinde (ayrıştırıcı, tip denetleyicisi ve kod listesi yeniden kullanılır; tanıma git, kullanımları bul, yeniden adlandır ve anlamsal renklendirme [`internal/devtools/codeindex`](../internal/devtools/codeindex/README.md) dizininin üstüne kurulur, istek eşlemesi o README'nin "Dil sunucusu sözleşmesi" bölümündedir):
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

## Evre 7: Ölçek ve yerel çıkarım

- **Tek GPU'da `torch.compile` ile başa baş hız (H4, ≤ 136 ms/adım hedef modelde; Evre 3'ten taşındı; profilde 162 ms, kaynaşık norm geri yönünden sonra 155, dikkat düzeninden sonra 151, normların warp çekirdeklerinden sonra 145 ms, 10,81 GiB, 2026-10-22; aşağıdaki kernel payları 151 ms'deki profilden).** Profil (2026-10-22, RTX PRO 6000): GPU boşluğu duvar süresinin %4,5'i, yani adım GPU'yla sınırlı; eğitim adımında CUDA Graph en çok ~%5 kazandırır, sona kalır. Kernel süresinin payları: çarpım %30, kopya %12 (bakışlara ayırma kopyası bf16 %3,5, float32→bf16 dönüşümü %3,2), indirge %8,4, ikili %8,2 (float32 + bf16 kalıntı toplaması %2,4), norm geri yönü %8,2, kaynaşık %7,5, çapraz entropi %6,2, dikkat %6,4, norm ileri %4. Ölçümün gösterdiği sırayla, her adım önce ve sonra ölçülür:
  - norm geri yönünün ağırlık gradyanı indirgemesiyle birlikte kaynaştırılması: **yapıldı (2026-10-22)**, tek geçiş, ağırlık gradyanı blok başına kısmi toplamlarla; 163 → 155 ms;
  - dikkatin bakışlara ayırma kopyaları: **yapıldı (2026-10-22)**, 155 → 151 ms, PyTorch'la eşleşme 0,0039 (önce 0,0037), üretim hızı değişmedi. `dikkat` `[..., T, H, dh]` alır (dil tanımı 15.24; JAX ve FlashAttention düzeni); birleşik çekirdekler bakışları adımlı okuyup yazar, kitaplık eksen sıralamayı bıraktı; katman başına 6 kopya kalkar;
  - normların satır başına warp çekirdekleri (ileri D ≤ 256, geri D ≤ 512; adımlı ve iki düzeyli görünümler yerinde okunur) ve ilk x gradyanının okunmadan üzerine yazılması (`NormGradSetter`): **yapıldı (2026-10-22)**, 151 → 145 ms. bf16 mikro ölçüm (µs, `torch.compile` parantezde): q normu ileri 150 → 31,5 (84), geri 96 → 31,8 (121); blok normu geri 132 → 98,6 (98);
  - GPU toplamlarının telafili (Neumaier) yapılması: doğruluk işi (gerçek kartta `TestNorms`'u düşüren sütun toplamı hatası), 145,0 → 145,4 ms (2026-10-22; `internal/compute/gpu/README.md`);
  - sütun toplamının paralelliği (yeni aday, 2026-10-22 ölçümü): tek aşamalı sütun toplamı 2000×512'de 0,1 ms, 41 GB/s (çıktı başına bir iş parçacığı; satırlar 2048'den azsa iki aşama yok). Norm geri yönünün ağırlık gradyanının son toplamı bu yoldadır. PyTorch ve XLA satırları bloğun iş parçacıklarına böler (32×32 karo, paylaşımlı bellekte ağaç);
  - float32→bf16 dönüşümleri (2026-10-22 ölçümü): adım başına ~247 dönüşüm, hepsi mikro toplu iş başına (~122). Ağırlık dönüşümlerini parametre değişene kadar saklamak (PyTorch autocast'in yaprak önbelleği gibi, mikro toplu işler arasında) denendi: 57 dönüşüm/adım kalktı ama yalnızca ~0,5 ms/adım kazandırdı, adım süresinde görünmedi; geri alındı. Asıl pay etkinleşmelerin dönüşümlerinde (kitaplıktaki açık `.türe`: döner konum, norm çıktıları; ara sonuçların `CastShared`'i): sıradaki aday bu dönüşümleri üreten çekirdeklerin sonucu doğrudan hedef türde yazması;
  - A100 profilinin gösterdikleri (2026-10-23, "Kartlar"): `dikkat_d` çekirdeği (A100'de ×3,4; paralelliği zayıf, ucuz aday, RTX'te de kazandırabilir), kaynaşık zincirlerin, dikkatin ve norm ileri yönün karta göre ayarı (A100'de ×2);
  - TR'de adım sayısıyla hafif artan adım süresi (A100'de 1000 adımda 244 → 251 ms, RTX'te 146 → 147; `torch.compile` düz): sebebi araştırılacak;
  - doğrusal + çapraz entropi (LCE) en büyük tek kalem (RTX'te ~%21); ama payı model büyüdükçe düşer (çıkış katmanı d·V, gövde d²·L): karar model boyuna göre verilir;
  - kalıntı toplamasının normla kaynaştırılması (float32 + bf16 toplama);
  - cuBLASLt eplogları (sapma, GELU, sapma gradyanı);
  - SimpleStories d512 modelinin profili (2026-10-23; 8 katmanın hepsi tam dikkat, MLP sapmalı; TR 144,5, `torch.compile` 128,8 ms/adım; kayıt `TR_dili/olcumler/ss-profil/`): doğrusal + çapraz entropide TR 23 ms hızlı (48,9 / 71,7 ms, dağarcık 50304'e yuvarlanınca); açık dikkatte +13 ms (geri yönde 32 milyar elemanlık tür dönüşümü; kaynağı doğrulanacak), normlarda +11, sapma eklemede +9,6 (ayrı toplama ve tür dönüşümü; yukarıdaki sapma eplogu), SwiGLU'da +4. Hedef modelde (6/8 katman yerel pencereli) dikkatin payı küçük olduğu için görünmüyordu;
  - en son, eğitim adımında CUDA Graph (`tekrar_oynat`'ın altyapısı hazır).
- **Uzun eğitimin gözlemlenebilirliği:** parametre normu, katman başına etkinleşme RMS'i, en büyük logit ya da log Z, fp16'da kayıp ölçeği, MFU; z-loss, "N toplu işi atla", devamda öğrenme oranı ölçeği; eşzamansız kontrol noktası (Colab'da asıl arıza oturumun kesilmesidir).

- **Yerel çıkarım:**
  - GGUF dışa aktarma: önce F16/BF16 ve Q8_0, sonra Q4_K ailesi;
  - yalnızca ağırlık int8/int4 nicemleme ve NF4: matris çarpımı içinde anında geri açan tek bir kernel ailesi hem QLoRA eğitimine hem sunuma hizmet eder.
- **Çoklu GPU:**
  - NCCL ile veri paralelliği (DDP): kova sırası sabit (belirlenimcilik için), fp32 biriktirme, hesapla örtüşen iletişim, gradyan biriktirirken eşitlemeyi atlama;
  - her GPU'ya kendi veri parçası (Evre 3'teki belirlenimci sıra üstünde);
  - ardından ZeRO-1 (ek iletişim hacmi yok). Bölme parametre başınadır, kontrol noktasındaki adlar değişmez.
- **T4 yolu (Evre 3'ten):** bf16 tensör çekirdeği olmayan kartlar (Colab'ın ücretsiz kartı T4, sm_75) için fp16 ve dinamik kayıp ölçekleme; önce T4'te uyumluluk testi (GPU testleri ve dil tanımı testleri).
- **GPU otomatik sınaması:** gradyan kapısı ve GPU testleri zamanlanmış olarak, en az iki kart türünde (T4 ve L4).
- **Veri:** Parquet ve jsonl akışı.

**Bitiş ölçütü:**
- Aynı eğitim iki GPU'da veri paralel koşar ve tek GPU'yla aynı sonucu verir.
- Model GGUF olarak dışa aktarılır ve llama.cpp'de çalışır.
- GPU testleri otomatik koşar.

## Evre 8: Yaygınlaşma

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
- **Yazım dilleri:** adların İngilizcesi var (`en.json`, `tr göster --dil en`). Altyapı Evre 2'de kurulduğu için burada iş çeviridir: iletilerin İngilizcesi **yapıldı (2026-10-22, 1648/1648)**; İspanyolca, Almanca ve Fransızca Evre 4'e öne çekildi; kalan dil tanımının, kılavuzun ve kod başvurusunun çevirisi.
- **Platformlar:** Mac otomatik sınaması, Apple M ve ARM için vektör kernelleri, Metal arka ucu, AMD (ROCm).

## Sonra

- **FP8 eğitimi** (Hopper/Blackwell): 100 M–1 B ölçeğinde kazanç küçüktür. Başladığında yalnızca doğrusal katmanlarda başlar; dikkat, norm, gömme ve çıkış başı bf16'da kalır.
- **TR ile kernel yazma (karo dili):**
  - Triton/TileLang uzlaşısını izler:
    - ızgara üzerinde bir program;
    - derleme anında şekli belli karolar;
    - maskeli yükleme ve saklama, mma, indirgeme;
    - türlendirilmiş zamanlama düğmeleri;
    - satır içi PTX kaçış kapısı.
  - Otomatik ayarlama zorunlu değildir.
  - Bitiş: elle yazılmış norm, softmax, çapraz entropi ve AdamW kernelleri karo dilinde aynı hızda yeniden yazılır.
- **Genel otomatik kaynaştırma:** ancak kural tabanlı birleştirici (`kaynaştır`, dil tanımı 15.44) ölçümle yetersiz kalırsa; indirgeme ve çarpım içeren zincirler (norm geri yönü, çarpım eplogları) için H4 (Evre 7) ölçümleri karar verir.
- **Bölme bilgisi tensör tipinde:** tip denetleyicisi belirsiz bölmeyi hata sayar (JAX'ın açık kipi emsali).
- **ONNX:** büyük işlem yüzeyi, kendi yürütme motoru olan bir dil için düşük getiri.
- **Statik şekil denetimi:** tensör şekil hatalarının çalıştırmadan önce yakalanması.
- **İleri yönlü ve yüksek mertebe türev**, uzun bağlam için dizi paralelliği, hiperparametre taraması.
- **Matematik:** doğrusal sistem çözme, ters matris, ayrışımlar (QR, Cholesky, tekil değer), hızlı Fourier dönüşümü; türevli, CPU ve GPU kernelli.

## Doğrulanacaklar

Kararlar bu soruların yanıtıyla gözden geçirilir; ilki TR'nin kendi ölçümüyle kapanır:

- Küçük modellerde L4 ya da RTX PRO 6000'de FP8 kazancı.
- RTX PRO 6000'in (SM120) hangi tensör çekirdeği komutlarını desteklediği (SM100'ün tcgen05/TMEM'i dahil mi): PTX ISA belgesinden.
- Colab oturumlarının kesilme sıklığı (kontrol noktası aralığını belirler).

## Platform durumu

- **Windows, Linux (Intel/AMD):** tam destek, her değişiklikte otomatik sınama.
- **Linux ve Mac (ARM):** derleniyor ve sınanıyor; vektör kernelleri yok (düz Go yolu).
- **GPU:** NVIDIA (sürücü ve isteğe bağlı cuBLASLt çalışma anında yüklenir); ölçümler ve GPU testleri RTX PRO 6000 (Blackwell), A100 (Ampere) ve L4 (Ada) üzerinde, elle ve Colab'da; H100 ve T4 denenmedi.

## Tamamlananlar

- Yorumlayıcı: kapanış derlemesi, yuva tabanlı ortamlar; Python ile başa baş hız.
- Tensör çekirdeği: AVX2/FMA matris çarpımı (çalışma anında seçilir), ters yönlü otomatik türev, kalıcı işçi havuzu.
- Veri hattı: `.npy` parçaları, önden yükleme, önbellek; genel `tokenizer.json` parçalayıcısı.
- bf16/fp16 veri türleri ve `karışık_hassasiyet` bloğu; 16 bitlik ara gradyanlar kendi türünde, float32 ağırlığın gradyanı yuvarlanmadan.
- GPU arka ucu: Go'nun ürettiği GPU kodu, bütün tensör işlemleri ve gradyanları, tensör çekirdekli dikkat (ileri ve geri, bakış boyu 128'e kadar), cuBLASLt ile bf16 matris çarpımı, çok tensörlü iyileştirici adımları, kontrol noktası ve gradyan kırpma cihazda.
- Kodlar: dilin bütün adları sabit kodlara bağlı (`internal/lang/codes`); açıklamalı başvuru `docs/codes.md`; kodlu saklama (`tr kodla`, `tr çöz`, `tr göster`) ve VS Code izdüşümü; İngilizce yazım dili.
- Tek yazım: Türkçe harfsiz biçimler, eş yazımlar ve İngilizce eşler kaldırıldı; yerleşik ad gölgeleme uyarısı.
- Matematik ve model yapı taşları: genişletilmiş eleman bazlı matematik, birleşik `dikkat`, `örnekle`, `parametreler(model)`, `türev_denetle`; Transformer kitaplığı (`tr/katman`: Doğrusal, RMSNorm, RoPE, gruplu sorgu dikkati, SwiGLU, GPT).
- Standart kitaplık: dosya ve yol işlemleri, komut satırı, günlük ve ilerleme çubuğu, kontrol noktası, profil ve canlı izleme.
- Kod tabanı İngilizceye geçti (klasörler, dosyalar, Go adları); her paketin README'si standartlarını yazar.
- Evre 3'ün yolu (2026-10-21): hedef modelin TR karşılığı, PyTorch'la aynı kalite, `torch.compile` belleği ve 1,2 katı hız; belirlenimci bırakma (15.43), kaynaştırma (15.44), normların çıktı türü (15.4), parçalı doğrusal + çapraz entropi (15.25), önbellekli GPU ayırıcısı; programın her yazım dilinde kanonik biçimiyle çalışması (17.7).
- Dış incelemeler: dil çekirdeği, hesap çekirdeği, standart kitaplık, geliştirici araçları, ileti altyapısı ve VS Code eklentisi raporları doğrulanarak düzeltildi.
- Kod grafiği (`tools/codegraph`): Go çağrıları, ileti ve ad kodları, kural→test bağları ve belge başlıkları yerel, belirlenimci bir grafikte; ajanlar dosya okumadan önce sorgular.
- Kod disiplini (2026-10-22): staticcheck ve ölü kod denetimi (deadcode) tam denetimde; govulncheck her gönderimde ve haftada bir üç işletim sistemi için; araç zinciri Go 1.26.10; uçtan uca testler 220 → 124 sn; GPU toplamları telafili ve float64 referansa karşı sınanır.
- Güvenlik: güvenilmeyen girdiler (tensör dosyası, kontrol noktası, `.npy`, `tokenizer.json`) sınırlarla denetlenir ve bulanıklaştırma testleriyle sınanır; VS Code eklentisi yalnızca güvenilen yorumlayıcıyı çalıştırır; otomatik sınamada sabitlenmiş eylemler.
