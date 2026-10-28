# 11. Hızı izlemek: `tr izle` ve `tr profil`

Eğitim yavaşsa önce zamanın nereye gittiğini görün: hangi tensör işlemi, hangi satırınız, GPU ne kadar meşgul, bellek ne durumda. TR bunu programınıza tek satır eklemeden yapar:

```bash
tr izle egit.tr --adım 2000      # çalışırken saniyede bir yenilenen tablo
tr profil egit.tr --adım 2000    # program bitince hız raporu
tr profil --json runs/deneme1/ egit.tr   # rapor ayrıca koşu klasörüne: runs/deneme1/speed.json
```

Program dosyasından sonraki argümanlar her zamanki gibi programa gider (`TR_fn_command_line`). İki komutun seçenekleri dosyadan **önce** yazılır.

## Ne gösterilir

**Adım hızı.** Eğitim döngünüzde `TR_type_Progress(...)` ve `.TR_mem_update(...)` kullanıyorsanız her `TR_mem_update` bir adım sınırıdır: ms/adım, adım/sn ve kalan süre buradan ölçülür. Kullanmıyorsanız iyileştiricinin her `TR_mem_step()` çağrısı bir adım sınırı sayılır (birden çok iyileştiricide ilk adım atan); hiçbiri yoksa tablo bunu söyler, işlem ve satır ölçüleri yine çalışır. `TR_cmd_profile` raporu adımı üç evreye böler: ileri, geri (geri yayılım) ve sonrası (iyileştirici, gradyan kırpma, günlük). Evreler konakta ölçülür; GPU'da çekirdekler eşzamansız çalıştığından GPU'nun beklendiği süre beklenen evreye düşer.

**Tensör işlemleri.** Her tensör işlemi türüne (matris çarpımı, `TR_fn_softmax`, `TR_fn_attention` ...), yönüne (ileri ya da geri yayılım: `geri · matris çarpımı (@)`) ve cihazına göre sayılır: çağrı sayısı ve ayırdığı bellek kesindir, süresi ölçülür (çok kısa işlemlerde örneklenerek, tahmin yansızdır). Tablo en pahalı işlemleri pay, adım başına süre (ms/adım) ve adım başına çağrı sayısıyla gösterir. İşlemlerin adları dildeki adlarıdır (`TR_fn_cross_entropy`, `TR_fn_AdamW.TR_mem_step`); işleçler simgesiyle yazılır.

**Örtük kopyalar.** Bir işlem bazen içinde kopya ya da tür dönüşümü yapar: bitişik olmayan bir girdiyi (`.TR_mem_transpose()`, dilim) bitişikleştirir, bf16 girdiyi float32'ye çevirir, bir tensöre ikinci kez gelen gradyanı öncekinin üstüne ekler. `TR_cmd_profile` raporu bunları ayrı bir tabloda, yapan işlemle birlikte gösterir (`adımlı kopya · matris çarpımı (@)`, `ekleyerek yazma (+=) · geri · görünüm`): çağrı ve eleman sayısı kesindir, süre CPU'daki büyük kopyalarda ölçülür. Kaynaştırılmış fonksiyonlar (`TR_fn_fuse`) da kendi adlarıyla sayılır. İndeksle atama (`t[i] = x`) `yerinde yazma` satırındadır.

**Geri yayılımın satırı.** Satır tablosunda geri yayılımın süresi `TR_fn_backward(kayıp)` satırında toplanmaz: her işlemin geri hesabı, o işlemi yapan ileri satıra yazılır. `geri %` sütunu satırın süresinin ne kadarının geri hesaptan geldiğini gösterir; örneğin `geri %97` olan bir satırda maliyet neredeyse tamamen o satırın türevindedir.

**Sıcak satırlar.** `TR_cmd_watch` ve `TR_cmd_profile` programınızı milisaniyede bir örnekler: o an hangi satır çalışıyor ve bir tensör işleminin içinde mi. "Öz süre" o satırın kendisinde geçen zamandır (çağırdığı fonksiyonların satırları ayrıca sayılır). `tensörde` sütunu satırın zamanının ne kadarının tensör işlemlerinde geçtiğidir: yüksekse satır hesap yapıyordur, düşükse zaman TR kodunda, dosya okumada ya da beklemede geçiyordur. Aktarılan bir modüldeki satır `TR_kw_func():satır` diye yazılır (`dikleştir():34`).

**GPU.** GPU kullanan programda iki komut GPU'nun kernel ölçümünü de açar: kernellerin cihazda geçen süresi (`kerneller %63 meşgul`), adım başına GPU süresi ve en pahalı kernel aileleri (`carpim`, `TR_fn_attention`, `cok_adamw` ...). GPU işleri eşzamansızdır: tensör tablosunda GPU işlemlerinin süresi konakta geçen başlatma süresidir, programın GPU'yu beklediği zaman `konağa okuma` satırında görünür (`.TR_mem_tolist`, yazdırma, `ç.TR_mem_update({TR_mem_loss: kayıp})`). Çekirdek ölçümü her başlatmaya iki zamanlama olayı ekler; GPU'yu tam hızda ölçmek için ölçümsüz bir koşuyla karşılaştırın. `TR_cmd_profile` raporu GPU süresini ayrıca işleme ve satıra göre verir (`GPU süresi satıra göre`): GPU'da satır tablosundaki süre konakta geçen başlatma ve beklemedir, GPU'yu asıl meşgul eden satır bu tablodadır; geri yöndeki çekirdekler ileri satırında, `geri %` ile.

**Bellek.** Go yığını (CPU tensörleri dahil), CPU tensör işlemlerinin adım başına ayırdığı bellek, GPU'da tensörlerin kullandığı, sürücüden ayrılmış ve en yüksek (tepe) bellek; çöp toplayıcının kaç kez çalıştığı ve programı ne kadar durdurduğu. `TR_cmd_profile` raporu ayrıca GPU belleğinin tepesinde canlı olan tensörleri onları ayıran satıra göre sıralar (PyTorch'un bellek anlık görüntüsünün karşılığı): hangi satırın tepe belleğin ne kadarını tuttuğu görünür. Geri yayılımda ayrılan gradyanlar `TR_fn_backward(...)` satırında sayılır; aktarılan modüldeki satır `dosya:satır` diye yazılır.

## `tr izle`

```text
⏱ izleniyor: examples/tr/41_gpt.tr · 12 sn · durdurmak için Ctrl+C
Adım       715/1 500 · 17,1 ms/adım · 58,3 adım/sn · kalan ~13,5 sn
Zaman      tensör işlemleri %96 · geri kalanı (TR kodu, bekleme) %4,2   (son 5 sn)

Tensör işlemleri (en pahalı)             pay   ms/adım  çağrı/adım  cihaz
  matris çarpımı (@)                     %43      7,10         225    cpu
  geri · matris çarpımı (@)              %25      4,08        15,1    cpu
  ...
Sıcak satırlar (öz süre)             pay tensörde
  28_gpt.tr:49                       %39      %99  geri_yayılım(kayıp)
  dikleştir():34                     %21      %96  (aktarılan modülde)
  ...
Bellek     Go yığını 7,5 MB · CPU tensör ayırması 10,2 MB/adım
Çöp top.   423 kez, toplam duraklama 113 ms (son 5 sn: 165 kez, 54,7 ms)
```

Tablonun hız satırları **son 5 saniyenin** ölçüleridir; program bitince bütün koşunun raporu yazılır. Terminalde tablo ekranın üstünde yeniden çizilir; programınızın kendi çıktısı (yazdır, ilerleme çubuğu, hatalar) tabloyla karışmasın diye bir dosyaya yazılır, son satırları tablonun altında görünür ve program bitince yeniden gösterilir. Dosyanın yerini `--çıktı yol.txt` ile seçebilirsiniz (verilmezse geçici klasördedir; tablo yolunu yazar). Çıktı bir terminal değilse (bir dosyaya yönlendirildiyse ya da not defteri hücresindeyse) programın çıktısı olduğu gibi akar ve 10 saniyede bir tek satırlık özet eklenir:

```text
⏱ 10 sn · adım 607/1 500 · 15,8 ms/adım · tensör %96 (matris çarpımı (@) %44, geri · matris çarpımı (@) %24, dikkat %3,7) · satır 28_gpt.tr:49 %37 · Go yığını 9,1 MB · çöp toplama 361 (84,3 ms)
```

Ctrl+C her zamanki gibi programı nazikçe durdurur (kontrol noktası varsa kaydedilir); rapor yine yazılır.

## `tr profil`

`TR_cmd_profile` programı aynı ölçümlerle çalıştırır ve bitince (hata ya da Ctrl+C ile bitse de) metin raporunu standart çıktıya yazar: süre ve çıkış kodu, adım hızı, tensör işlemleri tablosu (süre, pay, çağrı, ms/adım, ayrılan bellek, cihaz), en çok zaman alan satırlar ve fonksiyonlar, GPU kernelleri, bellek ve çöp toplama. Programın kendi çıktısı olduğu gibi kalır.

| Seçenek | Ne yapar |
| :-- | :-- |
| `--json yol.json` | Raporu ayrıca JSON olarak yazar (biçim: `docs/architecture.md`, "Canlı profil"). Yol bir klasörse (ya da `/` ile bitiyorsa) içine `speed.json` yazılır: koşu klasörünüzün `config.json`, `metrics.jsonl` dosyalarının yanına |
| `--go-profili cpu.prof` | Yorumlayıcının kendi Go işlemci profilini yazar (TR'nin kendisini hızlandıranlar için; `go tool pprof` ile okunur) |

Rapor dosyası ağ ya da bulut sürücüsündeyse ve yazılamazsa program durmaz: uyarı yazılır, rapor standart çıktıda yine vardır.

## Arka plan işlerinde

Program bir arka plan işi olarak çalışıyorsa (iş klasörü `.tr/işler/<iş>/`), canlı profili saniyede bir o klasördeki `profile.json`'a yazılır; işi izleyen araçlar buradan okur. Bu kipte satır örneklemesi ve GPU kernel ölçümü kapalıdır: yalnızca her zaman açık sayaçlar (adım hızı, tensör işlemleri, bellek) yazılır, ek yük saniyede bir dosya yazmasıdır. Dosya önce geçici bir dosyaya yazılıp yerine konur (okuyan yarım dosya görmez); yerine koymayı desteklemeyen sürücülerde bir kez uyarılır ve doğrudan yazılır, yazma hiç olmuyorsa sonraki saniyede yeniden denenir; eğitim hiçbir durumda durmaz.

## Maliyeti

- **Tensör sayaçları her zaman açıktır** (`tr` ile çalışan her programda): bir işlem başına birkaç bellek yazması (~4 ns). Mikrosaniyeler süren gerçek eğitim işlemlerinde ölçülemeyecek kadar küçüktür; birkaç yüz nanosaniyelik çok küçük tensör işlemlerinde %1–3 civarındadır.
- **Satır örneklemesi** yalnızca `TR_cmd_watch` ve `TR_cmd_profile` altında açıktır; kapalıyken maliyeti sıfırdır. Açıkken yorumlayıcı her deyimde konumu yazar ve fonksiyon çağrılarında hızlı yolu kullanamaz: çağrı yoğun TR kodu yaklaşık iki kat yavaşlar (özyinelemeli `fib(27)`: 57 → 110 ms); zamanı tensör işlemlerinde geçen eğitimde etkisi küçüktür.
- **GPU kernel ölçümü** yalnızca `TR_cmd_watch` ve `TR_cmd_profile` altında (ya da `TR_GPU_PROFIL=1` ile) açıktır.

Ölçülen hız ölçümün kendisiyle birlikte ölçülür; iki sürümü karşılaştırırken ikisini de aynı komutla çalıştırın. Profilleme eğitimin sonucunu değiştirmez: aynı tohumla `tr` ve `TR_cmd_watch` aynı kaybı verir.
