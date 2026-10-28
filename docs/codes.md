# TR kod listesi

<!-- Bu dosya internal/lang/codes/definitions.json ve internal/lang/codes/tr.json'dan üretilir; elle düzenlemeyin: go run ./tools/gen/codesdoc -->

Dilin kullanıcıya görünen her adı sabit bir **koda** bağlıdır (bkz. `docs/spec/17_kodlar.md` §17). Bu belge her kodun ne işe yaradığını, imzasını ve örneğini verir; "hangi kod ne işe yarar" sorusunun tek başvurusudur. Yeni bir ad gerekmeden önce burada aynı işi yapan bir kod olup olmadığına bakın.

İmzalarda parametrelerin yalnızca tipleri yazılır: `[Tip]` isteğe bağlı, `[Tip...]` sıfır ya da çok, `Tip...` bir ya da çok argümandır; `ad: Tip = varsayılan` adlı argümandır, `→` dönüş tipidir. Metot ve özellikler `Tip.ad` biçiminde, geçerli oldukları her tip için ayrı satırda yazılır.

**Değer dönüştüren ya da değere soru soran iş metottur** (`liste.sırala()`, `metin.uzunluk`); **matematik ve tensör işi fonksiyondur** (`toplam(liste)`, `relu(x)`).

Toplam 575 kod: 26 anahtar kelime, 6 bağlamsal sözcük, 2 değer adı, 42 tip, 139 fonksiyon, 11 sabit, 61 metot, 97 özellik, 136 adlı argüman.

## İçindekiler

- Dilin çekirdeği: Anahtar kelimeler · Bağlamsal sözcükler · Değer adları · Tip adları
- Yerleşikler: Yerleşik fonksiyonlar · Yerleşik sabitler · Adlı argümanlar
- Metot ve özellikler (x.ad): Metot ve özellikler
- Komutlar (tr, kabuk, hata ayıklayıcı): tr komutları · tr dizin alt komutları · Komut satırı seçenekleri · Kabuk komutları · Hata ayıklayıcı komutları
- Boru hattıyla kullanılabilen fonksiyonlar

## Dilin çekirdeği

### Anahtar kelimeler

#### TR_kw_and `ve`

Mantıksal ve (&& ile aynı); kısa devre yapar ve sonucu her zaman Mantık olarak verir.

```tr
yazdır(1 < 2 ve 2 < 3, 1 && "a")
```

İlişkili: TR_kw_or `veya`, TR_kw_not `değil`

#### TR_kw_async `eşzamanlı`

Bloğu ya da çağrıyı ayrı bir görev olarak başlatıp hemen devam eder; aynı anda yalnızca bir görev TR kodu çalıştırır.

```tr
eşzamanlı { yazdır("görevde") }; bekle_hepsi(); yazdır("bitti")
```

İlişkili: TR_fn_wait_all `bekle_hepsi`

#### TR_kw_break `çık`

En içteki döngüden çıkar; döngü dışında yazılması derleme hatasıdır (fonksiyondan çıkmak için sonuç yazılır).

```tr
döngü x içinde 1..10 { eğer x == 3 { çık }; yazdır(x) }
```

İlişkili: TR_kw_continue `devam`

#### TR_kw_breakpoint `kesme_noktası`

Programı bu satırda durdurup hata ayıklayıcıyı açar; kesme_noktası eğer koşul biçiminde yalnızca koşul doğruyken durur.

```tr
değişken x = 2; kesme_noktası eğer x > 5; yazdır(x)
```

#### TR_kw_catch `yakala`

dene bloğunda hata olursa çalışır ve hata mesajını verilen ada (ad verilmezse hata adına) Metin olarak bağlar.

```tr
dene { değişken z = 1 / 0 } yakala h { yazdır(h) }
```

İlişkili: TR_kw_try `dene`, TR_ctx_error `hata`

#### TR_kw_const `sabit`

Yeniden atanamayan bir ad tanımlar; ihlal çalıştırmadan önce yakalanır, ama değer bir listeyse içeriği değişebilir.

```tr
sabit PI = 3.14; yazdır(PI * 2)
```

İlişkili: TR_kw_var `değişken`

#### TR_kw_continue `devam`

En içteki döngünün sonraki turuna geçer; yalnızca bir döngü gövdesinde geçerlidir.

```tr
döngü x içinde 1..=5 { eğer x % 2 == 0 { devam }; yazdır(x) }
```

İlişkili: TR_kw_break `çık`

#### TR_kw_defer `ertele`

Çağrıyı ya da bloğu içinde bulunduğu fonksiyon bitince çalıştırır; birden çok ertele ters sırayla çalışır, erken sonuç ve hata durumunda da çalışır.

```tr
fonksiyon iş() { ertele yazdır("temizlik"); yazdır("iş") }; iş()
```

#### TR_kw_else `değilse`

eğer koşulu tutmadığında çalışacak dalı başlatır; değilse eğer ile yeni bir koşul zincirlenir.

```tr
değişken n = -3; değişken tür = eğer n < 0 { "negatif" } değilse eğer n == 0 { "sıfır" } değilse { "pozitif" }; yazdır(tür)
```

İlişkili: TR_kw_if `eğer`

#### TR_kw_false `yanlış`

Mantık tipinin yanlış değeridir; koşullarda yok, 0, boş metin, boş liste ve boş sözlük de yanlış sayılır.

```tr
yazdır(yanlış veya doğru, değil yanlış)
```

İlişkili: TR_kw_true `doğru`

#### TR_kw_for `döngü`

Bir kaynağı gezer (döngü x içinde ...), koşul doğru oldukça tekrarlar (döngü koşul { }) ya da sonsuz döner (döngü { }); her tur kendi kapsamında çalışır.

```tr
döngü i, x içinde ["a", "b"] { yazdır(i, x) }
```

İlişkili: TR_kw_in `içinde`

#### TR_kw_func `fonksiyon`

Fonksiyon tanımlar; parametrelere varsayılan değer verilebilir, tanım öne alınmaz (tanımdan önce çağrılamaz).

```tr
fonksiyon topla(a, b = 1) { sonuç a + b }; yazdır(topla(2, 3), topla(5))
```

İlişkili: TR_kw_return `sonuç`

#### TR_kw_if `eğer`

Koşul doğruysa bloğu çalıştırır; eğer bir ifadedir, değeri çalışan bloğun son deyimidir (hiçbir dal çalışmazsa yok).

```tr
değişken n = 7; eğer n % 2 == 0 { yazdır("çift") } değilse { yazdır("tek") }
```

İlişkili: TR_kw_else `değilse`

#### TR_kw_import `aktar`

Başka bir .tr dosyasını program boyunca bir kez çalıştırır ve üst düzey adlarını getirir; yol metin sabiti olmalıdır, .tr uzantısı isteğe bağlıdır.

```tr
aktar "tests/yardimci_modul"; yazdır(kare(PI))
```

İlişkili: TR_ctx_as `olarak`

#### TR_kw_in `içinde`

döngüde ve liste üretecinde gezilecek kaynağı belirtir; iki değişkenli biçimde indeks ile eleman (sözlükte anahtar ile değer) verir.

```tr
yazdır([x * x döngü x içinde 1..=5 eğer x % 2 == 1])
```

İlişkili: TR_kw_for `döngü`

#### TR_kw_match `eşle`

Konuyu kollarla sırayla karşılaştırır ve ilk eşleşen kolun sonucunu verir; Aralık deseni içerme sınar, eşleşme yoksa yok döner.

```tr
değişken puan = 95; yazdır(eşle puan { 100 => "tam puan"; 90..100 => "pekiyi"; _ => "diğer" })
```

#### TR_kw_none `yok`

Değerin yokluğudur; değer döndürmeyen fonksiyonlar, sözlükte olmayan anahtar ve varsa_al ile istenen olmayan öğe yok verir.

```tr
değişken l = [1, 2]; yazdır(l.varsa_al(5) == yok, l.varsa_al(5) ?? "boş", {"a": 1}["b"])
```

İlişkili: TR_type_None `Yok`

#### TR_kw_not `değil`

Doğruluğun tersini verir; karşılaştırmadan gevşek bağlanır: değil 1 == 2, değil (1 == 2) demektir.

```tr
yazdır(değil 1 == 2)
```

İlişkili: TR_kw_and `ve`, TR_kw_or `veya`

#### TR_kw_or `veya`

Mantıksal veya (|| ile aynı); kısa devre yapar ve sonucu her zaman Mantık olarak verir.

```tr
yazdır(yanlış veya 0, 1 > 2 || 3 > 2)
```

İlişkili: TR_kw_and `ve`, TR_kw_not `değil`

#### TR_kw_return `sonuç`

Fonksiyondan değer döndürür; yalnız sonuç yok döndürür, üst düzeyde programı bitirir, modülün üst düzeyinde ve test gövdesinde derleme hatasıdır.

```tr
fonksiyon işaret(n) { eğer n < 0 { sonuç "negatif" }; sonuç "pozitif" }; yazdır(işaret(-2))
```

İlişkili: TR_kw_func `fonksiyon`

#### TR_kw_self `kendi`

Yapı metodunun içinde metodun çağrıldığı nesneyi gösterir; alanlar kendi ile okunur ve değiştirilir.

```tr
yapı Sayaç { n = 0; fonksiyon artır() { kendi.n += 1 } }; değişken s = Sayaç(); s.artır(); s.artır(); yazdır(s.n)
```

İlişkili: TR_kw_struct `yapı`

#### TR_kw_struct `yapı`

Alanları ve metotları olan bir yapı tanımlar; nesne yapının adıyla, alan sırasıyla ya da adlı argümanlarla oluşturulur. Kalıtım yoktur.

```tr
yapı Nokta { x = 0, y = 0 }; yazdır(Nokta(3, 4), Nokta(y: 1).x)
```

İlişkili: TR_kw_self `kendi`, TR_ctx_init `kurucu`

#### TR_kw_throw `fırlat`

Bir hata başlatır; verilen değer metne çevrilir, yakalayan değişken her zaman Metin'dir.

```tr
dene { fırlat {"kod": 5} } yakala h { yazdır(tip(h), h) }
```

İlişkili: TR_kw_try `dene`

#### TR_kw_true `doğru`

Mantık tipinin doğru değeridir.

```tr
değişken hazır = doğru; eğer hazır { yazdır("hazır") }
```

İlişkili: TR_kw_false `yanlış`

#### TR_kw_try `dene`

Bloktaki hatayı yakala bloğuna yönlendirir; çalışma zamanı hataları da yakalanır.

```tr
dene { fırlat "patladı" } yakala h { yazdır("yakalandı:", h) }
```

İlişkili: TR_kw_catch `yakala`, TR_kw_throw `fırlat`

#### TR_kw_var `değişken`

Bulunulan kapsamda yeni bir ad tanımlar; değer verilmezse yok olur, virgülle birden çok ad tanımlanabilir.

```tr
değişken a, b = 1, 2; a, b = b, a; yazdır(a, b)
```

İlişkili: TR_kw_const `sabit`

### Bağlamsal sözcükler

#### TR_ctx_acc `birikim`

liste.indirge(...) { ... } sondaki bloğunda birikim o ana kadarki sonuçtur, o sıradaki elemandır. Başka her yerde sıradan bir addır.

```tr
yazdır([1, 2, 3, 4].indirge(0) { birikim + o }, [1, 2, 3].indirge(1) { birikim * o })
```

İlişkili: TR_mem_reduce `indirge`, TR_ctx_it `o`

#### TR_ctx_as `olarak`

aktar ile alınan modülün adlarını tek bir Sözlük olarak verilen ada bağlar.

```tr
aktar "tests/yardimci_modul" olarak ym; yazdır(ym.kare(2), ym["PI"])
```

İlişkili: TR_kw_import `aktar`

#### TR_ctx_error `hata`

yakala bloğunda ad verilmediğinde hata mesajının bağlandığı değişkendir.

```tr
dene { fırlat "patladı" } yakala { yazdır(hata) }
```

İlişkili: TR_kw_catch `yakala`

#### TR_ctx_init `kurucu`

Bu adlı metot nesne oluşturulurken aynı argümanlarla çağrılır; argümanlar kurucunun imzasına göre denetlenir.

```tr
yapı Hesap { bakiye; fonksiyon kurucu(b) { kendi.bakiye = b * 2 } }; yazdır(Hesap(5).bakiye)
```

İlişkili: TR_kw_struct `yapı`

#### TR_ctx_it `o`

Sondaki blok { ... } tek parametreli bir fonksiyondur ve parametresinin adı o'dur; iç içe bloklarda en içteki elemanı gösterir.

```tr
yazdır([1, 2, 3, 4].filtrele { o > 2 }.haritala { o * 10 })
```

İlişkili: TR_ctx_acc `birikim`

#### TR_ctx_test `test`

test "ad" { ... } bir test tanımlar; testler yalnızca tr test ile çalışır, normal çalıştırmada atlanır. Deyim başı dışında sıradan bir addır.

```tr
test "toplama" { eşit_mi(4, 2 + 2) }
```

İlişkili: TR_fn_assert `doğrula`, TR_fn_assert_equal `eşit_mi`

### Değer adları

#### TR_val_inf `sonsuz`

Ondalık sonsuz değerin metindeki adıdır; eksi sonsuz başına - alır. Ayrılmış sözcük değildir: kodda ondalık("sonsuz") ile yazılır, sıradan bir ad olarak da kullanılabilir.

```tr
yazdır(ondalık("inf"), -ondalık("inf"))
```

İlişkili: TR_val_nan `NaN`, TR_type_Float `Ondalık`, TR_fn_float `ondalık`

#### TR_val_nan `NaN`

Ondalık tanımsız değerin (sayı olmayan, NaN) metindeki adıdır. Ayrılmış sözcük değildir: kodda ondalık("NaN") ile yazılır.

```tr
yazdır(ondalık("NaN"))
```

İlişkili: TR_val_inf `sonsuz`, TR_type_Float `Ondalık`, TR_fn_float `ondalık`

### Tip adları

#### TR_type_Any `Herhangi`

Tip denetleyicisinin tipi bilinmeyen değerler için kullandığı tiptir; Herhangi açıklamalı bir ada her değer verilebilir.

```tr
fonksiyon kimlik(x: Herhangi) { sonuç x }; yazdır(kimlik(5), kimlik("beş"))
```

#### TR_type_Bool `Mantık`

doğru ve yanlış değerlerinin tipidir.

```tr
değişken hazır: Mantık = doğru; yazdır(tip(hazır))
```

İlişkili: TR_kw_true `doğru`, TR_kw_false `yanlış`

#### TR_type_BuiltinFunction `YerleşikFonksiyon`

Dilin yerleşik fonksiyonlarının tipidir.

```tr
değişken f: YerleşikFonksiyon = yazdır; f(tip(f))
```

İlişkili: TR_type_Function `Fonksiyon`

#### TR_type_CausalAttention `NedenselDikkat`



#### TR_type_Checkpoint `KontrolNoktası`

kontrol_noktası(...) ile kurulan, eğitimin bütün durumunu (parametreler, iyileştirici, okuyucular, günlük, düz değerler) atomik kaydeden ve yeniden başlatılan programı kaldığı adımdan sürdüren nesnenin tipidir; .güncelle(adım) adım sınırında aralıkla kaydeder, .kaydet() hemen kaydeder.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken kn = kontrol_noktası("koşu_a258", {w: w}); yazdır(tip(kn) == KontrolNoktası)
```

Üyeleri: TR_mem_resume_step `kaldığı_adım`, TR_mem_rollback `geri_dön`, TR_mem_save `kaydet`, TR_mem_update `güncelle` · İlişkili: TR_fn_checkpoint `kontrol_noktası`, TR_mem_update `güncelle`, TR_mem_resume_step `kaldığı_adım`, TR_mem_save `kaydet`, TR_mem_rollback `geri_dön`

#### TR_type_Dataset `VeriKümesi`

veri_kümesi(...) ile açılan, .npy veri dosyalarından oluşan veri kümesinin tipidir.

```tr
değişken küme = veri_kümesi("tests/data/numbers"); yazdır(tip(küme) == VeriKümesi)
```

Üyeleri: TR_mem_sampler `örnekleyici`, TR_mem_sequential `sıralı` · İlişkili: TR_fn_dataset `veri_kümesi`

#### TR_type_Dict `Sözlük`

Eklenme sırasını koruyan anahtar–değer tablosu tipidir; olmayan anahtar yok verir.

```tr
değişken s: Sözlük = {"ad": "Ali"}; yazdır(tip(s), s.ad)
```

Üyeleri: TR_mem_contains `içerir`, TR_mem_keys `anahtarlar`, TR_mem_length `uzunluk`, TR_mem_values `değerler`

#### TR_type_Embedding `GömmeKatmanı`



#### TR_type_Float `Ondalık`

64 bit kayan noktalı sayı tipidir; Tam bir değer Ondalık açıklamalı bir ada da atanabilir.

```tr
değişken oran: Ondalık = 0.75; yazdır(tip(oran))
```

İlişkili: TR_type_Int `Tam`, TR_fn_float `ondalık`

#### TR_type_Function `Fonksiyon`

Kullanıcı tanımlı fonksiyonların ve ok fonksiyonlarının tipidir; fonksiyonlar birinci sınıf değerdir.

```tr
değişken f: Fonksiyon = x => x * 2; yazdır(tip(f), f(3))
```

İlişkili: TR_kw_func `fonksiyon`, TR_type_BuiltinFunction `YerleşikFonksiyon`

#### TR_type_GPT `GPT`



#### TR_type_Int `Tam`

64 bit işaretli tam sayı tipidir; sınırı aşan işlem sessizce sarmak yerine taşma hatası verir.

```tr
değişken x: Tam = 5; yazdır(tip(x), x * 2)
```

İlişkili: TR_type_Float `Ondalık`, TR_fn_int `tam`

#### TR_type_KVCache `KVÖnbellek`



#### TR_type_LineReader `SatırOkuyucu`

dosya_satırları(...) ile açılan, dosyayı akışla satır satır okuyan okuyucunun tipidir; döngü ile gezilir ya da .sonraki() ile sıradaki satır alınır, dosya bitince yok verir; .kapat() dosyayı sonuna kadar okumadan bırakır.

```tr
dosya_yaz("okuyucu.txt", "a\nb"); değişken o = dosya_satırları("okuyucu.txt"); yazdır(tip(o) == SatırOkuyucu, o.sonraki(), o.sonraki(), o.sonraki())
```

Üyeleri: TR_mem_close `kapat`, TR_mem_next `sonraki` · İlişkili: TR_fn_read_lines `dosya_satırları`, TR_mem_next `sonraki`, TR_mem_close `kapat`

#### TR_type_Linear `Doğrusal`



#### TR_type_List `Liste`

Sıralı, karışık tipli, değiştirilebilir dizi tipidir; listeler başvuruyla paylaşılır.

```tr
değişken l: Liste = [1, "iki", 3.0]; yazdır(tip(l), l[-1])
```

Üyeleri: TR_mem_append `ekle`, TR_mem_contains `içerir`, TR_mem_filter `filtrele`, TR_mem_first `ilk`, TR_mem_get `varsa_al`, TR_mem_join `birleştir`, TR_mem_last `son`, TR_mem_length `uzunluk`, TR_mem_map `haritala`, TR_mem_reduce `indirge`, TR_mem_reverse `ters_çevir`, TR_mem_sort `sırala`, TR_mem_unique `benzersiz`

#### TR_type_Logger `Günlük`

günlük(...) ile açılan, her satırı bir JSON kaydı olan eğitim günlüğü dosyasının tipidir; .ekle(...) bir kayıt yazar.

```tr
değişken g = günlük("kayit/a.jsonl"); yazdır(tip(g) == Günlük)
```

Üyeleri: TR_mem_append `ekle` · İlişkili: TR_fn_logger `günlük`, TR_mem_append `ekle`

#### TR_type_Message `İleti`

ileti(H6903, ...) ile kurulan katalog iletisinin tipidir: kod ve değerlerinden oluşur, metni gösterilirken iletinin dilinde kurulur. fırlat ve doğrula ona verilince hata o kodla fırlar.

```tr
değişken i = ileti("H6903", -1); yazdır(tip(i))
```

İlişkili: TR_fn_message `ileti`, TR_fn_error_code `hata_kodu`

#### TR_type_Muon `Muon`



#### TR_type_MuonAndAdamW `MuonVeAdamW`



#### TR_type_None `Yok`

yok değerinin tipidir; yok, her tip açıklamalı ada atanabilir.

```tr
değişken d: Yok = yok; yazdır(tip(d))
```

İlişkili: TR_kw_none `yok`

#### TR_type_NorMuon `NorMuon`



#### TR_type_Object `Nesne`

Yapıdan oluşturulan nesnelerin genel adıdır; tip() bir nesne için Nesne değil yapının kendisini verir (tip(p) == Nokta).

```tr
yapı Nokta { x, y }; değişken p = Nokta(1, 2); yazdır(tip(p) == Nokta, p)
```

İlişkili: TR_type_StructType `YapıTanımı`

#### TR_type_Optimizer `İyileştirici`

AdamW(...) ve SGD(...) ile oluşturulan iyileştiricilerin tipidir; .adım() parametreleri günceller, .sıfırla() gradyanları sıfırlar.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken op = AdamW([w], öğrenme_oranı: 0.1); yazdır(tip(op), tip(SGD([w])))
```

Üyeleri: TR_mem_load_state `durumu_yükle`, TR_mem_lr `öğrenme_oranı`, TR_mem_state `durum`, TR_mem_step `adım`, TR_mem_zero_grad `sıfırla` · İlişkili: TR_fn_AdamW `AdamW`, TR_fn_SGD `SGD`

#### TR_type_PLEBlock `PLEBloğu`



#### TR_type_PLEContext `PLEBağlamı`



#### TR_type_Path `Yol`

yol(...) ile kurulan dosya ya da klasör yolunun tipidir; değiştirilemez, ayırıcıyı / ile saklar ve gösterir. Özellikleri yola soru sorar (dosya_adı, uzantı, var_mı, bayt ...); yol alan her yerleşik Metin yerine Yol da kabul eder.

```tr
değişken y = yol("veri", "eğitim.txt"); yazdır(tip(y) == Yol, y)
```

Üyeleri: TR_mem_absolute `mutlak`, TR_mem_exists `var_mı`, TR_mem_extension `uzantı`, TR_mem_is_dir `klasör_mü`, TR_mem_is_file `dosya_mı`, TR_mem_name `dosya_adı`, TR_mem_parent `üst_klasör`, TR_mem_size `bayt`, TR_mem_stem `uzantısız_ad` · İlişkili: TR_fn_path `yol`

#### TR_type_Progress `İlerleme`

ilerleme(...) ile açılan ilerleme çubuğunun tipidir; .güncelle(...) bir adım ilerletir, .durdur() çubuğu bitirir.

```tr
değişken ç = ilerleme(2); yazdır(tip(ç) == İlerleme)
```

Üyeleri: TR_mem_stop `durdur`, TR_mem_update `güncelle` · İlişkili: TR_fn_progress `ilerleme`, TR_mem_update `güncelle`, TR_mem_stop `durdur`

#### TR_type_RMSNorm `RMSNormKatmanı`



#### TR_type_Range `Aralık`

a..b (b hariç) ve a..=b (b dahil) aralıklarının tipidir; uçlar Tam olmalıdır. aralık(...) yerleşiği ise Liste döndürür.

```tr
değişken a: Aralık = 1..5; yazdır(tip(a), tip(aralık(5)))
```

İlişkili: TR_fn_range `aralık`

#### TR_type_RowRMS `SatırRMS`



#### TR_type_Run `Koşu`



#### TR_type_Sampler `Örnekleyici`

Veri kümesinden tohuma göre rastgele [girdi, hedef] tensör çiftleri üreten okuyucunun tipidir; dönemler boyunca sonsuz üretir.

```tr
değişken ö = veri_kümesi("tests/data/numbers").örnekleyici(toplu: 2, bağlam: 4, tohum: 1); yazdır(tip(ö) == Örnekleyici)
```

Üyeleri: TR_mem_load_state `durumu_yükle`, TR_mem_next `sonraki`, TR_mem_skip `atla`, TR_mem_state `durum` · İlişkili: TR_mem_sampler `örnekleyici`, TR_type_SequentialReader `SıralıOkuyucu`

#### TR_type_SequentialReader `SıralıOkuyucu`

Veri kümesini pencereler hâlinde sırayla okuyan okuyucunun tipidir; veri bitince .sonraki() yok verir.

```tr
değişken s = veri_kümesi("tests/data/numbers").sıralı(toplu: 2, bağlam: 4); yazdır(tip(s) == SıralıOkuyucu)
```

Üyeleri: TR_mem_load_state `durumu_yükle`, TR_mem_next `sonraki`, TR_mem_skip `atla`, TR_mem_state `durum` · İlişkili: TR_mem_sequential `sıralı`, TR_type_Sampler `Örnekleyici`

#### TR_type_SpikeGuard `SıçramaBekçisi`



#### TR_type_String `Metin`

Değiştirilemez Unicode metin tipidir; uzunluk ve indeks karakter bazındadır.

```tr
değişken ad: Metin = "şeker"; yazdır(tip(ad), ad[-1])
```

Üyeleri: TR_mem_contains `içerir`, TR_mem_ends_with `biter`, TR_mem_get `varsa_al`, TR_mem_length `uzunluk`, TR_mem_lower `küçük_harf`, TR_mem_replace `değiştir`, TR_mem_reverse `ters_çevir`, TR_mem_split `böl`, TR_mem_starts_with `başlar`, TR_mem_strip `kırp`, TR_mem_upper `büyük_harf` · İlişkili: TR_fn_str `metin`

#### TR_type_StructType `YapıTanımı`

yapı ile tanımlanan yapının kendisinin tipidir; tip açıklamasında kullanılamaz.

```tr
yapı Nokta { x, y }; yazdır(tip(Nokta) == YapıTanımı)
```

İlişkili: TR_kw_struct `yapı`, TR_type_Object `Nesne`

#### TR_type_SwiGLU `SwiGLU`



#### TR_type_Tensor `Tensör`

N boyutlu sayı dizisi tipidir (matris 2 boyutlu tensördür); elemanlar varsayılan olarak float32'dir.

```tr
değişken t: Tensör = tensör([[1, 2], [3, 4]]); yazdır(tip(t), t.şekil)
```

Üyeleri: TR_mem_device `cihaz`, TR_mem_dtype `tür`, TR_mem_grad `gradyan`, TR_mem_permute `eksenleri_sırala`, TR_mem_reshape `şekillendir`, TR_mem_shape `şekil`, TR_mem_slice `dilim`, TR_mem_split `böl`, TR_mem_to_device `cihaza`, TR_mem_to_dtype `türe`, TR_mem_tolist `değer`, TR_mem_transpose `devrik` · İlişkili: TR_fn_tensor `tensör`

#### TR_type_Tokenizer `Parçalayıcı`

parçalayıcı_yükle(...) ile yüklenen, metni parçalara ayırıp parça kimliklerine çeviren nesnenin tipidir.

```tr
değişken p = parçalayıcı_yükle("tests/data/tokenizer.json"); yazdır(tip(p) == Parçalayıcı)
```

Üyeleri: TR_mem_decode `çöz`, TR_mem_encode `kodla`, TR_mem_token_id `parça_kimliği`, TR_mem_vocab_size `dağarcık_boyutu` · İlişkili: TR_fn_load_tokenizer `parçalayıcı_yükle`

#### TR_type_TransformerBlock `TransformerBloğu`



#### TR_type_Type `Tip`

tip() sonucu olan tip değerlerinin tipidir; tip adları (Tam, Liste, Tensör ...) ifadede tip değeridir. Karşılaştırma değerle yapılır: tip(x) == Tam; tip değeri metinle karşılaştırılamaz.

```tr
değişken t: Tip = tip(42); yazdır(t == Tam, t, tip(t))
```

İlişkili: TR_fn_type `tip`

## Yerleşikler

### Yerleşik fonksiyonlar

#### TR_fn_AdamW `AdamW`

    AdamW(Liste | Tensör, öğrenme_oranı: Tam | Ondalık = 0.001, ağırlık_çürümesi: Tam | Ondalık = 0.01, beta1: Tam | Ondalık = 0.9, beta2: Tam | Ondalık = 0.999, epsilon: Tam | Ondalık = 1e-08, ana_kopya: Mantık = doğru) → İyileştirici

Türev izleyen yaprak tensörler için ağırlık çürümesi ayrık uygulanan Adam iyileştiricisini kurar; .adım() parametreleri günceller, .sıfırla() gradyanları sıfırlar. Varsayılanlar: öğrenme_oranı 0.001, ağırlık_çürümesi 0.01, beta1 0.9, beta2 0.999, epsilon 1e-8.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken opt = AdamW([w], öğrenme_oranı: 0.1); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

İlişkili: TR_fn_SGD `SGD`, TR_fn_clip_grad `gradyan_kırp`, TR_mem_step `adım`, TR_mem_zero_grad `sıfırla`, TR_type_Optimizer `İyileştirici`

#### TR_fn_SGD `SGD`

    SGD(Liste | Tensör, öğrenme_oranı: Tam | Ondalık = 0.01, momentum: Tam | Ondalık = 0, ağırlık_çürümesi: Tam | Ondalık = 0, ana_kopya: Mantık = doğru) → İyileştirici

Türev izleyen yaprak tensörler için olasılıksal gradyan inişi iyileştiricisini kurar; .adım() ve .sıfırla() metotları vardır. Varsayılanlar: öğrenme_oranı 0.01, momentum 0, ağırlık_çürümesi 0.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken opt = SGD([w], öğrenme_oranı: 0.1, momentum: 0.9); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

İlişkili: TR_fn_AdamW `AdamW`, TR_mem_step `adım`, TR_mem_zero_grad `sıfırla`, TR_type_Optimizer `İyileştirici`

#### TR_fn_abs `mutlak`

    mutlak(Tam | Ondalık | Tensör) → Tam | Ondalık | Tensör

Sayının mutlak değerini verir; Tam Tam, Ondalık Ondalık kalır, en küçük Tam değerde taşma hatası verir. Tensörde eleman bazlı çalışır (−0 sıfır olur, NaN korunur, 16 bitlik tensör kendi türünde döner); türevi işaret(x)'tir, 0'da 0. Listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(mutlak(-5), mutlak(-2.5), mutlak(tensör([-2.0, 0.0, 3.0])).değer)
```

İlişkili: TR_fn_round `yuvarla`, TR_fn_sign `işaret`

#### TR_fn_acos `acos`

    acos(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın arkkosinüsünü radyan olarak [0, π] aralığında verir; [-1, 1] dışında NaN döner. Türevi -1 / karekök(1 - x²)'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(acos(0.0), acos(tensör([1.0, -1.0])).değer)
```

İlişkili: TR_fn_asin `asin`, TR_fn_cos `cos`

#### TR_fn_any_pattern_matches `bir_desen_uyar_mı`

    bir_desen_uyar_mı()



#### TR_fn_apply_rotary `döner_konum_uygula`

    döner_konum_uygula()



#### TR_fn_apply_rotary_fused `döner_konum_uygula_kaynaşık`

    döner_konum_uygula_kaynaşık()



#### TR_fn_argmax `argmax`

    argmax(Tensör, eksen: Tam = -1) → Tensör

Verilen eksende (varsayılan son eksen) en büyük elemanın indeksini verir; eşitlikte ilki seçilir. Sonuç tam sayılı ve türev izlemeyen bir tensördür; eksen yalnızca adlı verilir.

```tr
yazdır(argmax(tensör([[0.1, 2.5, 0.3], [1.7, 0.2, 0.9]]), eksen: -1).değer)
```

İlişkili: TR_fn_sample `örnekle`

#### TR_fn_argsort `sıralama_indeksi`

    sıralama_indeksi(Tensör, eksen: Tam = -1) → Tensör

Verilen eksende (varsayılan son eksen) elemanları küçükten büyüğe sıralayan indeksleri verir; sıralama kararlıdır (eşitlerde küçük indeks önce), NaN'lar sondadır. Sonuç tam sayılı ve türevsiz bir tensördür.

```tr
yazdır(sıralama_indeksi(tensör([3.0, 1.0, 2.0])).değer)
```

İlişkili: TR_fn_topk `en_büyük_k`, TR_mem_sort `sırala`

#### TR_fn_asin `asin`

    asin(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın arksinüsünü radyan olarak [-π/2, π/2] aralığında verir; [-1, 1] dışında NaN döner. Türevi 1 / karekök(1 - x²)'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(asin(1.0), asin(tensör([0.0, 0.5])).değer)
```

İlişkili: TR_fn_acos `acos`, TR_fn_sin `sin`

#### TR_fn_assert `doğrula`

    doğrula(Herhangi, [Herhangi]) → Mantık

Koşul yanlışsa "doğrulama başarısız" hatası fırlatır, doğruysa doğru döndürür; isteğe bağlı mesaj hataya eklenir. Test dışında da çalışır.

```tr
doğrula(2 > 1, "iki birden büyük olmalı"); yazdır("doğrulandı")
```

İlişkili: TR_fn_assert_equal `eşit_mi`, TR_ctx_test `test`

#### TR_fn_assert_equal `eşit_mi`

    eşit_mi(Herhangi, Herhangi, [Herhangi]) → Mantık

İki değer == anlamında eşit değilse ikisini de tipiyle gösteren hata fırlatır, eşitse doğru döndürür. Test dışında da çalışır.

```tr
eşit_mi(4, 2 + 2); yazdır("eşit")
```

İlişkili: TR_fn_assert `doğrula`, TR_ctx_test `test`

#### TR_fn_atan `atan`

    atan(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın arktanjantını radyan olarak (-π/2, π/2) aralığında verir; türevi 1 / (1 + x²)'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(atan(1.0), atan(tensör([0.0, 1.0])).değer)
```

İlişkili: TR_fn_atan2 `atan2`, TR_fn_tan `tan`

#### TR_fn_atan2 `atan2`

    atan2(Tensör | Tam | Ondalık, Tensör | Tam | Ondalık) → Tensör | Ondalık

atan2(y, x), (x, y) noktasının açısını (-π, π] aralığında radyan olarak verir; dört bölgeyi ayırır, (0, 0)'da 0 döner. Sayıda Ondalık, tensörde yayınlamalı ve türev izleyen tensör verir; (0, 0)'da türev 0 alınır. Listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(atan2(1.0, -1.0), atan2(tensör([1.0, -1.0]), tensör([0.0])).değer)
```

İlişkili: TR_fn_atan `atan`

#### TR_fn_attention `dikkat`

    dikkat(Tensör, Tensör, Tensör, nedensel: Mantık = doğru, pencere: Tam = 0, maske: Tensör | Liste | Yok = yok) → Tensör

Birleşik ölçekli nokta çarpımı dikkatini (softmax(q @ kᵀ / √dh + maske) @ v) bellek dostu, tek türev düğümüyle hesaplar. Varsayılan olarak nedenseldir; k/v bakış sayısı q'nunkinin böleni olabilir (gruplu sorgu dikkati; bakış, İngilizce head).

```tr
değişken q = rastgele_normal([2, 4, 8], tohum: 1); yazdır(dikkat(q, q, q, nedensel: doğru).şekil)
```

İlişkili: TR_fn_softmax `softmax`

#### TR_fn_backward `geri_yayılım`

    geri_yayılım(Tensör) → Mantık

Tek elemanlı (skaler) bir kayıptan geri yayılım yaparak yaprak tensörlerin .gradyan değerlerini hesaplar. Gradyanlar birikir; adımlar arasında iyileştiricinin .sıfırla() metoduyla sıfırlanmalıdır.

```tr
değişken x = tensör(3.0, türev_izle: doğru); geri_yayılım(x * x); yazdır(x.gradyan.değer)
```

İlişkili: TR_fn_no_grad `türevsiz`, TR_mem_grad `gradyan`

#### TR_fn_build_pretrained_model `hazır_model_kur`

    hazır_model_kur()



#### TR_fn_ceil `tavan`

    tavan(Tam | Ondalık | Tensör) → Tam | Ondalık | Tensör

Sayıyı yukarı (artı sonsuza doğru) en yakın tam sayıya yuvarlar; Tam girdide Tam, Ondalık girdide Ondalık döner. Tensörde eleman bazlı çalışır (tam sayılı tensör tam sayılı kalır), türevi 0'dır; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(tavan(3.2), tavan(7), tavan(tensör([-1.5, 2.5])).değer)
```

İlişkili: TR_fn_floor `taban`, TR_fn_round `yuvarla`

#### TR_fn_checkpoint `kontrol_noktası`

    kontrol_noktası(Metin | Yol, Sözlük, her_dakika: Tam | Ondalık = 15, tut: Tam = 2) → KontrolNoktası

Koşu klasöründe (noktalar <koşu>/checkpoints/ altına) eğitimin kontrol noktasını kurar; geçerli bir nokta varsa kurulurken yükler ve kaldığı adımı yazar, yoksa boş başlar. Sözlüğün değerleri modeller (türev izleyen tensörler), iyileştiriciler, okuyucular, günlükler ve düz değerli sözlük ya da listelerdir; anahtarlar veridir.

```tr
değişken w = tensör([1.0, 2.0], türev_izle: doğru); değişken op = SGD([w], öğrenme_oranı: 0.1); değişken kn = kontrol_noktası("koşu_t702", {model: [w], op: op}, her_dakika: 15, tut: 2); yazdır(kn.kaldığı_adım)
```

İlişkili: TR_mem_resume_step `kaldığı_adım`, TR_mem_save `kaydet`, TR_mem_update `güncelle`, TR_fn_load_parameters `parametreleri_yükle`, TR_mem_state `durum`

#### TR_fn_clamp `sınırla`

    sınırla(Tam | Ondalık | Tensör, en_az: Tam | Ondalık = yok, en_fazla: Tam | Ondalık = yok) → Tam | Ondalık | Tensör

Değeri en_az ile en_fazla arasına sınırlar; ikisinden en az biri verilmeli, en_az en_fazla'dan büyük olamaz. Sayıda tip korunur (sınırlardan biri Ondalık ise Ondalık), NaN korunur. Tensörde eleman bazlıdır; türev sınırların içinde (sınırlar dahil) 1, dışında 0'dır; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(sınırla(15, en_fazla: 10), sınırla(tensör([-2.0, 0.5, 3.0]), en_az: 0, en_fazla: 1).değer)
```

İlişkili: TR_fn_min `min`, TR_fn_max `max`

#### TR_fn_clip_grad `gradyan_kırp`

    gradyan_kırp(Liste | Tensör, en_fazla: Tam | Ondalık = 1, sonlu_değilse_hata: Mantık = yanlış) → Tensör

Parametre gradyanlarının toplam L2 normu en_fazla değerini (varsayılan 1.0) aşıyorsa hepsini aynı oranda küçültür. Kırpma öncesi normu 0 boyutlu tensör olarak döndürür (kayıp gibi): GPU'da norm ve kırpma cihazda yapılır, program norm okununca (.değer, yazdırma) bekler.

```tr
değişken w = tensör([3.0, 4.0], türev_izle: doğru); geri_yayılım(toplam(w * w)); yazdır(gradyan_kırp([w], en_fazla: 1.0))
```

İlişkili: TR_fn_AdamW `AdamW`, TR_fn_SGD `SGD`

#### TR_fn_command_line `komut_satırı`

    komut_satırı(Sözlük, açıklama: Metin = "", açıklamalar: Sözlük) → Sözlük

Betiğin seçeneklerini varsayılanlarıyla tanımlar ve komut satırında verilen --ad değer argümanlarıyla doldurulmuş yeni bir Sözlük döndürür; seçeneğin tipi varsayılanından çıkar. --yardım seçenekleri listeler; hatalı argüman programı 2 koduyla bitirir. Programda bir kez çağrılır; argüman yoksa varsayılanlar döner.

```tr
değişken ayar = komut_satırı({adım: 100, oran: 0.01, hızlı: yanlış}, açıklama: "Küçük modeli eğitir"); yazdır(ayar.adım, ayar.oran, ayar.hızlı)
```

İlişkili: TR_fn_env `ortam_değişkeni`

#### TR_fn_compute_dtype `hesap_türü`

    hesap_türü() → Metin

Etkin karışık_hassasiyet bloğunun hesap türünün adını verir ("bf16", "fp16"); blok dışında "float32". Çıktısını yalnızca çarpımların okuyacağı bir işlem sonucunu doğrudan o türde yazmak için kullanılır (ör. rms_normu(x, w, tür: hesap_türü())).

```tr
yazdır(hesap_türü(), karışık_hassasiyet("bf16") { hesap_türü() })
```

İlişkili: TR_fn_mixed_precision `karışık_hassasiyet`, TR_fn_rms_norm `rms_normu`, TR_arg_dtype `tür`

#### TR_fn_concat `bitiştir`

    bitiştir(Liste, [Tam], eksen: Tam = -1) → Tensör

Bir tensör listesini verilen eksen boyunca (varsayılan son eksen) uç uca ekleyip tek tensör yapar. Birleşen eksen dışındaki eksenler eşit olmalıdır; sonuç türev izler.

```tr
yazdır(bitiştir([birler([2, 1]), sıfırlar([2, 2])], eksen: -1).şekil)
```

İlişkili: TR_mem_split `böl`

#### TR_fn_config_value `ayar_değeri`

    ayar_değeri()



#### TR_fn_copy `kopyala`

    kopyala(Metin | Yol, Metin | Yol, üzerine_yaz: Mantık = yanlış) → Yol

Dosyayı ya da klasörü (içindekilerle) kopyalar ve hedefin yolunu döndürür. Hedef var olan bir klasörse kaynak onun içine aynı adla gider; hedef zaten varsa üzerine_yaz: doğru gerekir. Değiştirilme zamanı korunur, bağlantılar kopyalanmaz (atlanır, uyarı yazılır); kopya önce geçici bir ada yazılıp yerine konur.

```tr
dosya_yaz("ağırlık.txt", "w"); klasör_oluştur("yedek"); yazdır(kopyala("ağırlık.txt", "yedek"))
```

İlişkili: TR_fn_move `taşı`

#### TR_fn_cos `cos`

    cos(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın kosinüsünü (radyan) hesaplar; türevi -sin(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(cos(tensör([0.0, 1.0])).değer)
```

İlişkili: TR_fn_sin `sin`

#### TR_fn_cosh `cosh`

    cosh(Tensör | Tam | Ondalık) → Tensör | Ondalık

Eleman bazlı hiperbolik kosinüstür: (e^x + e^(-x)) / 2; türevi sinh(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(cosh(0.0), cosh(tensör([0.0, 1.0])).değer)
```

İlişkili: TR_fn_sinh `sinh`, TR_fn_tanh `tanh`

#### TR_fn_cross_entropy `çapraz_entropi`

    çapraz_entropi(Tensör | Liste, Tam | Liste | Tensör) → Tensör

Son ekseni sınıf olan çıktıyla satır başına bir sınıf indeksi arasındaki ortalama çapraz entropi kaybını skaler tensör olarak verir. Negatif hedefler yok sayılır; hepsi yok sayılırsa hata verir.

```tr
yazdır(çapraz_entropi(tensör([[2.0, 0.5, 0.1], [0.2, 1.5, 0.3]]), [0, 1]).değer)
```

İlişkili: TR_fn_log_softmax `log_softmax`

#### TR_fn_cumsum `birikimli_toplam`

    birikimli_toplam(Tensör, eksen: Tam = -1) → Tensör

Verilen eksende (varsayılan son eksen) her konuma kadar olan elemanların toplamını verir; şekil korunur, türev izler (gradyan sondan birikimli toplamdır), toplama sırası sabittir. Yalnızca tensörde çalışır, listede çalışmaz (tensör(...) ile çevrilir); eksen yalnızca adlı verilir.

```tr
yazdır(birikimli_toplam(tensör([1, 2, 3, 4])).değer, birikimli_toplam(tensör([[1, 2], [3, 4]]), eksen: 0).değer)
```

İlişkili: TR_fn_sum `toplam`

#### TR_fn_custom_grad `özel_türev`

    özel_türev(ileri: Fonksiyon | YerleşikFonksiyon, geri: Fonksiyon | YerleşikFonksiyon) → YerleşikFonksiyon

İleri ve geri hesabı kullanıcının yazdığı yeni bir türevlenebilir işlem kurar ve çağrılabilir bir fonksiyon döndürür. Yalnızca ileri: ve geri: adlı argümanlarını alır; ileri [çıktı, saklananlar], geri her konumlu girdi için bir gradyan (ya da yok) döndürür.

```tr
değişken kare = özel_türev(ileri: fonksiyon(x) { sonuç [x * x, [x]] }, geri: fonksiyon(s, g) { sonuç [g * 2 * s[0]] }); değişken x = tensör([3.0], türev_izle: doğru); geri_yayılım(toplam(kare(x))); yazdır(x.gradyan.değer)
```

İlişkili: TR_fn_grad_check `türev_denetle`, TR_fn_backward `geri_yayılım`

#### TR_fn_dataset `veri_kümesi`

    veri_kümesi(Metin, önbellek: Metin, kota_gb: Tam | Ondalık) → VeriKümesi

Bir klasördeki .npy veri dosyalarını (ad sırasıyla) ya da tek bir .npy veri dosyasını VeriKümesi olarak açar; okuyucular .örnekleyici ve .sıralı ile alınır. önbellek: verilirse kota_gb: de verilmelidir.

```tr
değişken küme = veri_kümesi("tests/data/numbers"); yazdır(küme)
```

İlişkili: TR_mem_sampler `örnekleyici`, TR_mem_sequential `sıralı`

#### TR_fn_default_device `varsayılan_cihaz`

    varsayılan_cihaz([Metin]) → Metin

Argümanla çağrılınca ("cpu", "gpu", "gpu:1") bundan sonra oluşturulan bütün tensörlerin cihazını seçer: tensör(...), sıfırlar, birler, rastgele_normal ve veri kümesi örnekleyicisinin toplu işleri o cihazda kurulur. Argümansız çağrı geçerli cihazın adını verir. Model GPU'da eğitilecekse bu çağrı model kurulmadan önce yapılır; olmayan cihaz hatadır.

```tr
yazdır(varsayılan_cihaz()); varsayılan_cihaz("cpu"); yazdır(tensör([1.0]).cihaz)
```

İlişkili: TR_fn_devices `cihazlar`, TR_mem_to_device `cihaza`, TR_mem_device `cihaz`

#### TR_fn_devices `cihazlar`

    cihazlar() → Liste

Bu makinede kullanılabilir cihazların adlarını liste olarak verir; ilk eleman her zaman "cpu"dur, NVIDIA GPU'su ve sürücüsü varsa "gpu" (birden çoksa "gpu:1", ...) gelir.

```tr
yazdır(cihazlar()[0], cihazlar().içerir("gpu"))
```

İlişkili: TR_fn_default_device `varsayılan_cihaz`, TR_mem_to_device `cihaza`

#### TR_fn_embedding `gömme`

    gömme(Tensör, Tam | Liste | Tensör) → Tensör

[V, D] şeklindeki tablodan verilen indekslerin satırlarını seçer; sonuç şekli indeks şekli + [D]'dir. Tek tam sayı indeks [D] şeklinde tek satır verir; gradyan tabloya akar.

```tr
değişken tablo = tensör([[0.1, 0.2], [0.3, 0.4], [0.5, 0.6]]); yazdır(gömme(tablo, [2, 0]).değer)
```

İlişkili: TR_fn_cross_entropy `çapraz_entropi`

#### TR_fn_env `ortam_değişkeni`

    ortam_değişkeni(Metin) → Metin | Yok

Ortam değişkeninin değerini Metin olarak verir; tanımsızsa yok döner (varsayılan için ?? yazılır). Adının bir sözcüğü TOKEN, KEY, SECRET, PASSWORD, ŞİFRE, PAROLA, ANAHTAR gibi bir gizli sözcükse (HF_TOKEN, api_anahtarı; TOKENIZER_PATH değil) sayı ve mantık olmayan değeri gizli sayılır ve insanın göreceği çıktılarda *** olarak görünür.

```tr
değişken klasör = ortam_değişkeni("VERİ_KLASÖRÜ") ?? "veri"; yazdır(klasör)
```

İlişkili: TR_fn_command_line `komut_satırı`

#### TR_fn_error_code `hata_kodu`

    hata_kodu(Fonksiyon | YerleşikFonksiyon) → Metin | Yok

Verilen fonksiyonu çalıştırır; fonksiyon bir ileti (ileti) fırlatırsa onun kodunu Metin olarak ("H6903") döndürür, hata fırlatmazsa ya da hata kodlu değilse yok döndürür. İletinin metnini değil kodunu sınamak içindir.

```tr
yazdır(hata_kodu(() => { fırlat ileti("H6903", -1) }))
```

İlişkili: TR_fn_message `ileti`, TR_fn_assert `doğrula`

#### TR_fn_exit `sonlandır`

    sonlandır(Tam) → Yok

Programı verilen çıkış koduyla nazikçe bitirir: ertele blokları çalışır, dene/yakala bunu yakalamaz; eşzamanlı bir görevden çağrılsa da bütün program biter. Kod 0–125 arasında bir Tam olmalıdır, dışındaki değer çalışma hatasıdır; programın kendi kodları 4'ten başlar (1 hata, 2 kullanım hatası).

```tr
sonlandır(4)
```

İlişkili: TR_fn_handle_stop `durdurmayı_üstlen`

#### TR_fn_exp `exp`

    exp(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın e tabanında üstelini (e^x) hesaplar. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(exp(tensör([0.0, 1.0])).değer)
```

İlişkili: TR_fn_log `log`

#### TR_fn_expm1 `expm1`

    expm1(Tensör | Tam | Ondalık) → Tensör | Ondalık

e^x - 1 hesaplar; küçük x'te çıkarmadaki basamak kaybı olmaz (expm1(1e-10) = 1e-10). Türevi exp(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(expm1(0.000001), expm1(tensör([0.0, 1.0])).değer)
```

İlişkili: TR_fn_exp `exp`, TR_fn_log1p `log1p`

#### TR_fn_eye `birim_matris`

    birim_matris(Tam) → Tensör

n×n birim matrisi (köşegeni 1, geri kalanı 0 olan 2 boyutlu tensör) oluşturur. n pozitif bir Tam olmalıdır.

```tr
yazdır(birim_matris(3).değer)
```

İlişkili: TR_fn_tensor `tensör`

#### TR_fn_float `ondalık`

    ondalık(Ondalık | Tam | Metin) → Ondalık

Tam ya da Metin'i Ondalık'a çevirir; metinde ondalık ayırıcı nokta olmalıdır, Mantık kabul edilmez.

```tr
yazdır(ondalık("3.14"), ondalık(2))
```

İlişkili: TR_fn_int `tam`, TR_fn_str `metin`

#### TR_fn_floor `taban`

    taban(Tam | Ondalık | Tensör) → Tam | Ondalık | Tensör

Sayıyı aşağı (eksi sonsuza doğru) en yakın tam sayıya yuvarlar; Tam girdide Tam, Ondalık girdide Ondalık döner. Tensörde eleman bazlı çalışır (tam sayılı tensör tam sayılı kalır), türevi 0'dır; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(taban(3.7), taban(-2), taban(tensör([-1.5, 2.5])).değer)
```

İlişkili: TR_fn_ceil `tavan`, TR_fn_round `yuvarla`

#### TR_fn_fuse `kaynaştır`

    kaynaştır(Fonksiyon | YerleşikFonksiyon) → YerleşikFonksiyon

Eleman bazlı tensör işlemlerinden oluşan bir fonksiyonu kaynaştırır ve aynı şeyi hesaplayan bir fonksiyon döndürür: her çağrıda zincirin tamamı tek geçişte (GPU'da tek çekirdekte) hesaplanır, ara sonuçlar belleğe yazılmaz; geri yayılım ara değerleri saklamadan girdilerden yeniden hesaplar. Sonuç kaynaşmamış hesapla aynıdır. Fonksiyon eleman bazlı işlemleri, dilimi, .böl'ü, bitiştir'i, boyu 1 eksen ekleyip çıkarmayı ve tür dönüşümünü kullanabilir; argümanları tensör ya da sayıdır (sayı programa sabit olarak girer).

```tr
değişken f = kaynaştır((a, b) => a * b + a); yazdır(f(tensör([1.0, 2.0]), tensör([3.0, 4.0])).değer)
```

İlişkili: TR_fn_custom_grad `özel_türev`

#### TR_fn_gelu `gelu`

    gelu(Tensör | Tam | Ondalık, yaklaşık: Mantık = doğru) → Tensör | Ondalık

Eleman bazlı GELU etkinleştirmesidir; varsayılan tanh yaklaşımıyla hesaplanır, yaklaşık: yanlış tam (erf tabanlı) GELU verir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(gelu(tensör([-1.0, 0.0, 1.0])).değer, gelu(1.0, yaklaşık: yanlış))
```

İlişkili: TR_fn_relu `relu`

#### TR_fn_grad_check `türev_denetle`

    türev_denetle(Fonksiyon | YerleşikFonksiyon, Liste, fark_adımı: Tam | Ondalık = 0.001, tolerans: Tam | Ondalık = 0.01) → Mantık

Bir fonksiyonun geri yayılımla bulunan gradyanını merkezi farkla bulunan sayısal gradyanla karşılaştırır; uyuşursa doğru döndürür, uyuşmazsa en büyük farkı gösteren bir hata verir. Yalnızca türev izleyen tensör girdileri, kopyaları üzerinde denetlenir.

```tr
yazdır(türev_denetle(fonksiyon(x) { sonuç x * x }, [tensör([0.5, -1.0], türev_izle: doğru)], fark_adımı: 0.001, tolerans: 0.01))
```

İlişkili: TR_fn_custom_grad `özel_türev`

#### TR_fn_handle_stop `durdurmayı_üstlen`

    durdurmayı_üstlen() → Yok

Ctrl+C ve sonlandırma isteklerinin karşılanmasını programa bırakır: ondan sonra gelen ilk istek programı durdurmaz, kaydedilir (durdurma_istendi() doğru döner). Program kendi sınırında biter; ertele blokları çalışır.

```tr
durdurmayı_üstlen()
```

İlişkili: TR_fn_stop_requested `durdurma_istendi`, TR_fn_exit `sonlandır`

#### TR_fn_has_rope_scaling `rope_ölçekleme_var_mı`

    rope_ölçekleme_var_mı()



#### TR_fn_init_weight `ağırlık_başlat`

    ağırlık_başlat()



#### TR_fn_input `girdi`

    girdi([Herhangi]) → Metin

Standart girişten bir satır okur, varsa önce istemi yazar; satır sonu atılır, giriş bittiyse boş metin döner.

```tr
değişken ad = girdi("Adınız: "); yazdır("Merhaba " + ad)
```

İlişkili: TR_fn_print `yazdır`

#### TR_fn_int `tam`

    tam(Tam | Ondalık | Metin | Mantık) → Tam

Değeri Tam'a çevirir: Ondalık'ın kesirli kısmını atar, metni ayrıştırır, doğru 1 ve yanlış 0 olur; çevrilemeyen metin hatadır.

```tr
yazdır(tam("42") + 1, tam(3.9), tam(doğru))
```

İlişkili: TR_fn_float `ondalık`, TR_fn_str `metin`

#### TR_fn_is_finite `sonlu_mu`

    sonlu_mu(Tam | Ondalık | Tensör) → Mantık | Tensör

Sayının sonlu olup olmadığını verir: NaN, sonsuz ve eksi sonsuzda yanlış, öbür her sayıda (her Tam dahil) doğru. Tensörde eleman bazlı çalışır: sonlu elemanda 1, öbürlerinde 0 olan float32 bir tensör verir (karşılaştırmalar gibi), türev izlemez; listede çalışmaz (tensör(...) ile çevrilir). Eğitimde gradyan normunu denetlemek için kullanılır: eğer değil sonlu_mu(norm.değer) { ... }.

```tr
yazdır(sonlu_mu(1.5), sonlu_mu(ondalık("nan")), sonlu_mu(tensör([1.0, ondalık("inf")])).değer)
```

İlişkili: TR_fn_sign `işaret`, TR_fn_clip_grad `gradyan_kırp`

#### TR_fn_json_parse `json_çöz`

    json_çöz(Metin) → Herhangi

JSON metnini TR değerine çevirir (nesne Sözlük, dizi Liste olur); anahtar sırası korunur, tam sayı değerli ondalıklar Tam olur.

```tr
değişken v = json_çöz(`{"ad": "Ali", "yaş": 30}`); yazdır(v["ad"], v["yaş"] + 1)
```

İlişkili: TR_fn_json_string `json_metin`

#### TR_fn_json_string `json_metin`

    json_metin(Herhangi) → Metin

Değeri boşluksuz JSON metnine çevirir; anahtarlar sözlükteki sırayla yazılır, NaN, sonsuz ya da kendini içeren değer hatadır.

```tr
yazdır(json_metin({"ad": "Ali", "puan": [90, 85]}))
```

İlişkili: TR_fn_json_parse `json_çöz`

#### TR_fn_layer_norm `katman_normu`

    katman_normu(Tensör | Liste, [Tensör | Liste | Yok], [Tensör | Liste | Yok], epsilon: Tam | Ondalık = 1e-05, ağırlık: Tensör | Liste | Yok = yok, sapma: Tensör | Liste | Yok = yok, tür: Metin = yok) → Tensör

Girdiyi son eksende ortalaması 0, varyansı 1 olacak biçimde normalleştirir; isteğe bağlı ağırlık ve sapma [D] şeklinde olmalıdır. Konumlu ya da adlı verilebilirler; epsilon varsayılanı 0.00001'dir.

```tr
yazdır(katman_normu(tensör([[1.0, 2.0, 3.0]]), ağırlık: birler([3]), sapma: sıfırlar([3])).değer)
```

İlişkili: TR_fn_rms_norm `rms_normu`

#### TR_fn_layer_windows `katman_pencereleri`

    katman_pencereleri()



#### TR_fn_linear_cross_entropy `doğrusal_çapraz_entropi`

    doğrusal_çapraz_entropi(Tensör, Tensör, Tam | Liste | Tensör) → Tensör

çapraz_entropi(girdi @ ağırlık, hedef) ile aynı kaybı ve gradyanları verir, ama bütün logitleri bellekte tutmaz: satırları dilim dilim işler (büyük sözlüklü dil modellerinin çıkış katmanı için). Ağırlık [girdinin son ekseni, sınıf sayısı] şeklindedir; paylaşımlı gömmede tablo.devrik() verilir.

```tr
yazdır(doğrusal_çapraz_entropi(tensör([[1.0, 2.0]]), tensör([[0.5, -1.0, 0.2], [0.1, 0.3, -0.4]]), [2]).değer)
```

İlişkili: TR_fn_cross_entropy `çapraz_entropi`

#### TR_fn_list_dir `klasörü_listele`

    klasörü_listele([Metin | Yol], [Metin]) → Liste

Klasörde kalıba uyan dosya ve klasörleri sıralı bir Yol listesi olarak verir (varsayılan: "." ve "*"). Kalıpta * ve ? bir bileşenin içinde, [a-z] bir karakter kümesi, ** tam bileşen olarak sıfır ya da çok klasör eşler; nokta ile başlayan adlar, kalıp da nokta ile başlamadıkça listelenmez, klasör bağlantılarının içine girilmez.

```tr
klasör_oluştur("liste/alt"); dosya_yaz("liste/a.npy", "1"); dosya_yaz("liste/alt/b.npy", "2"); yazdır(klasörü_listele("liste", "**/*.npy"))
```

İlişkili: TR_fn_path `yol`, TR_fn_make_dir `klasör_oluştur`

#### TR_fn_load_parameters `parametreleri_yükle`

    parametreleri_yükle(Nesne | Liste | Sözlük | Tensör, Sözlük) → Tam

Ad → tensör sözlüğünü (tensör_yükle sonucu) modelin var olan parametrelerine, parametreler(model, adlarıyla: doğru) ile aynı adlarla yerinde yazar ve yazılan parametre sayısını verir; tensörler parametrenin cihazına taşınır, türü farklıysa dönüştürülür. Eksik, fazla ya da şekli farklı ad hatadır; iyileştiricinin tuttuğu tensörler aynı kalır.

```tr
değişken m = {w: tensör([1.0, 2.0], türev_izle: doğru)}; tensör_kaydet("m.trt", {w: tensör([5.0, 6.0])}); yazdır(parametreleri_yükle(m, tensör_yükle("m.trt")), m.w.değer)
```

İlişkili: TR_fn_parameters `parametreler`, TR_fn_load_tensors `tensör_yükle`, TR_fn_checkpoint `kontrol_noktası`

#### TR_fn_load_tensors `tensör_yükle`

    tensör_yükle(Metin, türev_izle: Mantık = yanlış) → Sözlük

tensör_kaydet ile yazılmış dosyayı ya da safetensors dosyasını (Hugging Face, PyTorch; biçim içerikten anlaşılır) ad → tensör sözlüğü olarak, dosyadaki sırayla okur; tensörler kaydedildikleri türdedir (F32, BF16, F16). Bozuk, eksik ya da başka türden dosya açık bir hatayla reddedilir.

```tr
tensör_kaydet("a.trt", {"w": tensör([1.0])}); yazdır(tensör_yükle("a.trt", türev_izle: doğru)["w"].gradyan)
```

İlişkili: TR_fn_save_tensors `tensör_kaydet`

#### TR_fn_load_tokenizer `parçalayıcı_yükle`

    parçalayıcı_yükle(Metin) → Parçalayıcı

tokenizer.json biçimindeki bir parçalayıcı dosyasını yükler ve Parçalayıcı nesnesi döndürür. Adlı argüman almaz; klasör yolu verilirse hata verir.

```tr
değişken p = parçalayıcı_yükle("tests/data/tokenizer.json"); yazdır(p)
```

İlişkili: TR_mem_encode `kodla`, TR_mem_decode `çöz`, TR_mem_vocab_size `dağarcık_boyutu`, TR_mem_token_id `parça_kimliği`

#### TR_fn_log `log`

    log(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın doğal logaritmasını hesaplar. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(log(tensör([1.0, 10.0])).değer)
```

İlişkili: TR_fn_exp `exp`

#### TR_fn_log10 `log10`

    log10(Tensör | Tam | Ondalık) → Tensör | Ondalık

10 tabanında logaritmadır; 0'da -sonsuz, negatifte NaN. Türevi 1 / (x · ln 10)'dur. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(log10(1000), log10(tensör([1.0, 100.0])).değer)
```

İlişkili: TR_fn_log `log`, TR_fn_log2 `log2`

#### TR_fn_log1p `log1p`

    log1p(Tensör | Tam | Ondalık) → Tensör | Ondalık

log(1 + x) hesaplar; küçük x'te 1 + x yuvarlamasından etkilenmez (log1p(1e-10) = 1e-10). x = -1'de -sonsuz, x < -1'de NaN; türevi 1 / (1 + x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(log1p(0.000001), log1p(tensör([0.0, 1.0])).değer)
```

İlişkili: TR_fn_log `log`, TR_fn_expm1 `expm1`

#### TR_fn_log2 `log2`

    log2(Tensör | Tam | Ondalık) → Tensör | Ondalık

2 tabanında logaritmadır; ikinin kuvvetlerinde tam sonuç verir, 0'da -sonsuz, negatifte NaN. Türevi 1 / (x · ln 2)'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(log2(8), log2(tensör([1.0, 1024.0])).değer)
```

İlişkili: TR_fn_log `log`, TR_fn_log10 `log10`

#### TR_fn_log_softmax `log_softmax`

    log_softmax(Tensör, [Tam], eksen: Tam = -1) → Tensör

softmax'ın logaritmasını verilen eksende (varsayılan son eksen) sayısal olarak kararlı biçimde hesaplar. Yalnızca tensörde çalışır, listede çalışmaz (tensör(...) ile çevrilir); tamamen -sonsuz bir satırın sonucu -sonsuz, gradyanı 0'dır.

```tr
yazdır(log_softmax(tensör([1.0, 2.0, 3.0])).değer)
```

İlişkili: TR_fn_softmax `softmax`, TR_fn_cross_entropy `çapraz_entropi`

#### TR_fn_logger `günlük`

    günlük(Metin | Yol) → Günlük

Eğitim günlüğünü .jsonl dosyası (her satır bir JSON kaydı) olarak açar; dosya varsa sonuna eklenir, klasörü yoksa oluşturulur. İlk kayıttan önce program adı ve komut satırı ayarlarıyla bir başlangıç kaydı yazılır.

```tr
değişken g = günlük("kayit/egitim.jsonl"); g.ekle({kayıp: 2.31, doğruluk: 0.5}); yazdır(g)
```

İlişkili: TR_mem_append `ekle`, TR_fn_progress `ilerleme`

#### TR_fn_lr_schedule `öğrenme_oranı_planı`

    öğrenme_oranı_planı()



#### TR_fn_make_dir `klasör_oluştur`

    klasör_oluştur(Metin | Yol) → Yol

Klasörü üst klasörleriyle birlikte oluşturur ve Yol olarak döndürür; klasör zaten varsa sorun değildir, aynı adda bir dosya varsa hatadır.

```tr
değişken k = klasör_oluştur("çıktı/kontrol_noktaları"); yazdır(k, k.klasör_mü)
```

İlişkili: TR_fn_list_dir `klasörü_listele`, TR_fn_remove `sil`

#### TR_fn_map_hugging_face `hugging_face_eşle`

    hugging_face_eşle()



#### TR_fn_max `max`

    max(Liste | Tam | Ondalık | Tensör, [Tam | Ondalık...], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Tam | Ondalık | Yok | Tensör

Tek bir listenin ya da birden çok sayının en büyüğünü tipini koruyarak verir; boş listede yok döner. Tensörde eksen yalnızca eksen: ile adlı verilir.

```tr
yazdır(max([2.0, 1.5]), max(3, 7, 5), max(tensör([[1, 5], [3, 2]]), eksen: 0).değer)
```

İlişkili: TR_fn_min `min`, TR_fn_argmax `argmax`

#### TR_fn_mean `ortalama`

    ortalama(Liste | Tam | Ondalık | Tensör, [Tam | Ondalık...], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Ondalık | Tensör

Sayıların aritmetik ortalamasını Ondalık olarak verir; tek bir liste ya da birden çok sayı alır, boş girdi ve sayı olmayan eleman hatadır. Tensörde eksen: alır, boş tensörün ortalaması hatadır.

```tr
yazdır(ortalama([1, 2, 3, 4]), ortalama(tensör([[1, 2], [3, 4]]), eksen: 1).değer)
```

İlişkili: TR_fn_sum `toplam`, TR_fn_median `ortanca`

#### TR_fn_median `ortanca`

    ortanca(Liste | Tam | Ondalık, [Tam | Ondalık...]) → Ondalık

Sayıların ortancasını Ondalık olarak verir; eleman sayısı çiftse ortadaki ikisinin ortalamasıdır; ortalama gibi boş girdi ve sayı olmayan eleman hatadır.

```tr
yazdır(ortanca([3, 1, 4, 2]))
```

İlişkili: TR_fn_mean `ortalama`

#### TR_fn_message `ileti`

    ileti(Metin, [Herhangi...]) → İleti

İleti kataloğundaki bir iletiyi kodu (H6903) ve değerleriyle (katalogdaki parametre sırasıyla) kurar. Dönen metindir ve kodunu taşır: fırlat ile doğrula ona verilince hata o kodla fırlar, metni her dilde kataloğun metnidir. Kitaplık hata metnini böyle yazar.

```tr
değişken i = ileti("H6903", -1); yazdır(i)
```

İlişkili: TR_fn_error_code `hata_kodu`, TR_fn_assert `doğrula`, TR_kw_throw `fırlat`

#### TR_fn_min `min`

    min(Liste | Tam | Ondalık | Tensör, [Tam | Ondalık...], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Tam | Ondalık | Yok | Tensör

Tek bir listenin ya da birden çok sayının en küçüğünü tipini koruyarak verir; boş listede yok döner. Tensörde eksen yalnızca eksen: ile adlı verilir.

```tr
yazdır(min([3, 1, 2]), min(4, 2.5), min([]))
```

İlişkili: TR_fn_max `max`

#### TR_fn_mixed_precision `karışık_hassasiyet`

    karışık_hassasiyet(Metin, Fonksiyon | YerleşikFonksiyon) → Herhangi

Bloğu verilen hesap türüyle ("bf16", "fp16" ya da kapatmak için "float32") çalıştırır ve bloğun son ifadesinin değerini döndürür. Blok içinde @ ve dikkat girdileri bu türe çevrilir; biriktirme float32'dir, sonuç bu türdedir (dil tanımı 15.4); ağırlıkların türü değişmez.

```tr
değişken y = karışık_hassasiyet("bf16") { tensör([[1.0, 2.0]]) @ tensör([[3.0], [4.0]]) }; yazdır(y.değer, y.tür)
```

İlişkili: TR_arg_dtype `tür`, TR_fn_compute_dtype `hesap_türü`

#### TR_fn_move `taşı`

    taşı(Metin | Yol, Metin | Yol, üzerine_yaz: Mantık = yanlış) → Yol

Dosyayı ya da klasörü taşır ve hedefin yolunu döndürür; hedef kuralları kopyala ile aynıdır. Aynı diskte yeniden adlandırır; başka bir diske kopyalar, boyutu doğrular, sonra kaynağı siler (kaynağa sil'in korumaları uygulanır).

```tr
dosya_yaz("taşınacak.txt", "x"); yazdır(taşı("taşınacak.txt", "taşındı.txt"))
```

İlişkili: TR_fn_copy `kopyala`, TR_fn_remove `sil`

#### TR_fn_name_matches `ad_uyar_mı`

    ad_uyar_mı()



#### TR_fn_no_grad `türevsiz`

    türevsiz(Tensör | Fonksiyon | YerleşikFonksiyon) → Herhangi

türevsiz { ... } bloğu türev izlemeden çalışır ve bloğun değerini döndürür; türevsiz(x) ise tensörü hesap çizgesinden ayırır. Türevsiz kip görev başınadır, başka eşzamanlı görevlere sızmaz.

```tr
değişken x = tensör([1.0], türev_izle: doğru); değişken y = türevsiz { x * 2 }; yazdır(y.değer)
```

İlişkili: TR_fn_backward `geri_yayılım`

#### TR_fn_ones `birler`

    birler(Liste, türev_izle: Mantık = yanlış, tür: Metin = "float32") → Tensör

Verilen şekilde, bütün elemanları 1 olan bir tensör oluşturur. Şekil yalnızca liste olarak verilir ([2, 3]).

```tr
yazdır(birler([2, 2], tür: "bf16").değer)
```

İlişkili: TR_fn_zeros `sıfırlar`, TR_fn_tensor `tensör`

#### TR_fn_only_2d `yalnızca_2b`

    yalnızca_2b()



#### TR_fn_orthogonalize `dikleştir`

    dikleştir()



#### TR_fn_orthogonalize_stack `yığın_dikleştir`

    yığın_dikleştir()



#### TR_fn_parameter_groups `parametre_grupları`

    parametre_grupları()



#### TR_fn_parameters `parametreler`

    parametreler(Nesne | Liste | Sözlük | Tensör, adlarıyla: Mantık = yanlış) → Liste | Sözlük

Bir yapı nesnesini (iç içe yapılar, listeler ve sözlüklerle) gezip türev izleyen yaprak tensörleri sabit sırayla ve her birini bir kez toplar. adlarıyla: doğru ile yol adı → tensör sözlüğü döndürür; bu sözlük tensör_kaydet ile kaydedilebilir.

```tr
değişken model = {"w": tensör([1.0], türev_izle: doğru), "b": tensör([0.0], türev_izle: doğru)}; yazdır(parametreler(model, adlarıyla: doğru))
```

İlişkili: TR_fn_AdamW `AdamW`, TR_fn_save_tensors `tensör_kaydet`

#### TR_fn_parse_csv `csv_çöz`

    csv_çöz(Metin) → Liste

CSV metnini satır listelerine çevirir (json_çöz gibi; dosya okumaz, dosya için csv_oku). Baştaki bayt sırası işareti atılır; hücreler yalnızca katı sayı biçimindeyse sayı olur ("007" Metin kalır).

```tr
değişken tablo = csv_çöz("ad,yaş\nAli,30"); yazdır(tablo[1][0], tablo[1][1] + 1)
```

İlişkili: TR_fn_read_csv `csv_oku`, TR_fn_json_parse `json_çöz`

#### TR_fn_path `yol`

    yol(Metin | Yol, [Metin | Yol...]) → Yol

Parçaları / ile birleştirip temizlenmiş bir Yol kurar (a/./b/../c → a/c). İlk parçadan sonra mutlak bir parça, boş parça ya da sekme/satır sonu içeren parça hatadır; göreli yol, kullanıldığı anda dosya_oku ile aynı kuralla çözülür.

```tr
yazdır(yol("veri", "./eğitim/../derlem.txt"))
```

İlişkili: TR_type_Path `Yol`, TR_fn_list_dir `klasörü_listele`

#### TR_fn_ple_gelu `ple_gelu`

    ple_gelu()



#### TR_fn_ple_mix `ple_karışımı`

    ple_karışımı()



#### TR_fn_ple_mix_fused `ple_karışımı_kaynaşık`

    ple_karışımı_kaynaşık()



#### TR_fn_pretrained_model `hazır_model`

    hazır_model()



#### TR_fn_print `yazdır`

    yazdır([Herhangi...]) → Metin

Değerleri aralarına boşluk koyarak tek satırda yazar ve yazdığı metni döndürür; argümansız çağrı boş satır yazar.

```tr
yazdır("Toplam:", 2 + 3)
```

İlişkili: TR_fn_input `girdi`

#### TR_fn_progress `ilerleme`

    ilerleme([Tam | Yok], günlük: Günlük = yok) → İlerleme

Toplam adımla bir ilerleme çubuğu açar (toplam verilmezse kalan süresiz sayaç). Çubuk standart hataya yazılır: terminalde tek satırda yenilenir, dosyaya ya da boruya giderken 10 saniyede bir düz satır yazılır. günlük: verilirse her güncelleme o günlüğe de yazılır.

```tr
değişken ç = ilerleme(3); döngü i içinde 0..3 { ç.güncelle({kayıp: 1.0 / (i + 1)}) }; yazdır(ç)
```

İlişkili: TR_mem_update `güncelle`, TR_mem_stop `durdur`, TR_fn_logger `günlük`

#### TR_fn_randn `rastgele_normal`

    rastgele_normal(Liste, türev_izle: Mantık = yanlış, tür: Metin = "float32", tohum: Tam) → Tensör

Ortalaması 0, standart sapması 1 olan normal dağılımdan rastgele sayılarla dolu bir tensör oluşturur. Aynı tam sayı tohum her çalıştırmada aynı değerleri verir; şekil liste olarak verilir.

```tr
yazdır(rastgele_normal([2, 3], tohum: 42).şekil)
```

İlişkili: TR_fn_zeros `sıfırlar`

#### TR_fn_range `aralık`

    aralık(Tam, [Tam]) → Liste

aralık(bitiş) 0'dan, aralık(başlangıç, bitiş) başlangıçtan bitişe kadar (bitiş hariç) Tam listesi üretir; Aralık değil Liste döner ve en fazla 50 milyon eleman üretilir.

```tr
yazdır(aralık(5), aralık(2, 5))
```

İlişkili: TR_type_Range `Aralık`

#### TR_fn_read_csv `csv_oku`

    csv_oku(Metin | Yol) → Liste

Bir CSV dosyasını (Metin ya da Yol olarak verilen yol) satır listelerine çevirir; .gz uzantılı dosya açılarak okunur. CSV metninin kendisi csv_çöz ile ayrıştırılır. Hücreler yalnızca katı sayı biçimindeyse sayı olur ("007" Metin kalır).

```tr
dosya_yaz("tablo.csv", "ad,yaş\nAli,30"); değişken tablo = csv_oku("tablo.csv"); yazdır(tablo[1][0], tablo[1][1] + 1)
```

İlişkili: TR_fn_read_file `dosya_oku`, TR_fn_parse_csv `csv_çöz`

#### TR_fn_read_file `dosya_oku`

    dosya_oku(Metin | Yol) → Metin

Dosyanın tüm içeriğini Metin olarak okur; yol Metin ya da Yol olabilir. Göreli yol önce çalışma klasörüne, sonra modülün ve programın klasörüne göre çözülür, dosya yoksa hata bakılan yerleri listeler. .gz uzantılı dosya açılarak okunur (açılmış içerik en çok 1 GB; daha büyüğü için dosya_satırları).

```tr
dosya_yaz("notlar.txt", "merhaba"); yazdır(dosya_oku("notlar.txt"))
```

İlişkili: TR_fn_write_file `dosya_yaz`, TR_fn_read_lines `dosya_satırları`, TR_fn_read_csv `csv_oku`

#### TR_fn_read_lines `dosya_satırları`

    dosya_satırları(Metin | Yol) → SatırOkuyucu

Dosyayı akışla satır satır okuyan bir SatırOkuyucu açar; bellek dosyanın boyutundan bağımsızdır. Satır sonu (\n, \r\n) ve baştaki BOM atılır, .gz uzantılı dosya açılarak okunur; tek satır en çok 64 MB olabilir.

```tr
dosya_yaz("satırlar.txt", "bir\niki\n"); döngü satır içinde dosya_satırları("satırlar.txt") { yazdır(satır) }
```

İlişkili: TR_fn_read_file `dosya_oku`, TR_mem_next `sonraki`

#### TR_fn_relu `relu`

    relu(Tensör | Tam | Ondalık) → Tensör | Ondalık

Negatif elemanları 0 yapan, pozitifleri koruyan eleman bazlı etkinleştirme fonksiyonudur. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir); NaN değerini korur.

```tr
yazdır(relu(tensör([-1.0, 0.5, 2.0])).değer)
```

İlişkili: TR_fn_gelu `gelu`, TR_fn_sigmoid `sigmoid`

#### TR_fn_remove `sil`

    sil(Metin | Yol, içindekilerle: Mantık = yanlış) → Tam

Dosyayı, bağlantıyı ya da boş klasörü siler ve silinen dosya sayısını verir; dolu klasör için içindekilerle: doğru gerekir. Olmayan yol, salt okunur dosya, kök, ev ve sistem klasörleri, çalışma ve program klasörü ile onların üstleri hatadır; bağlantıların gösterdiği yere dokunulmaz, joker karakter genişletilmez.

```tr
dosya_yaz("silinecek.txt", "x"); yazdır(sil("silinecek.txt"))
```

İlişkili: TR_fn_make_dir `klasör_oluştur`, TR_fn_move `taşı`

#### TR_fn_replay `tekrar_oynat`

    tekrar_oynat(Fonksiyon | YerleşikFonksiyon) → YerleşikFonksiyon

Tensör alıp tensör veren bir fonksiyonu sarar: GPU'da ikinci çağrı f'in işini kaydeder (CUDA Graph), sonraki çağrılar girdileri kopyalayıp kaydı tek seferde oynatır, yorumlayıcı f'i yeniden çalıştırmaz. Yalnızca türevsiz içinde; şekil ya da sayı argümanları değişince yeniden kaydeder, CPU'da f doğrudan çağrılır.

```tr
değişken f = tekrar_oynat((x) => x * 2); türevsiz { yazdır(f(tensör([1.0, 2.0])).değer) }
```

İlişkili: TR_fn_fuse `kaynaştır`

#### TR_fn_rms_norm `rms_normu`

    rms_normu(Tensör | Liste, [Tensör | Liste | Yok], epsilon: Tam | Ondalık = 1e-05, ağırlık: Tensör | Liste | Yok = yok, tür: Metin = yok) → Tensör

Girdiyi son eksende karelerinin ortalamasının köküne bölerek normalleştirir (ortalama çıkarılmaz). İsteğe bağlı ağırlık [D] şeklinde olmalıdır; epsilon varsayılanı 0.00001'dir.

```tr
yazdır(rms_normu(tensör([[3.0, 4.0]]), ağırlık: birler([2])).değer)
```

İlişkili: TR_fn_layer_norm `katman_normu`, TR_fn_compute_dtype `hesap_türü`

#### TR_fn_rope_theta `rope_tabanı`

    rope_tabanı()



#### TR_fn_rotary_table `döner_konum_tablosu`

    döner_konum_tablosu()



#### TR_fn_round `yuvarla`

    yuvarla(Tam | Ondalık, [Tam]) → Tam | Ondalık

Sayıyı en yakına yuvarlar (yarımlar sıfırdan uzağa); basamak pozitifse Ondalık, 0 ya da negatifse Tam verir.

```tr
yazdır(yuvarla(3.14159, 2), yuvarla(2.5), yuvarla(1234, -2))
```

İlişkili: TR_fn_int `tam`

#### TR_fn_sample `örnekle`

    örnekle(Tensör, sıcaklık: Tam | Ondalık = 1, en_iyi_k: Tam = 0, tohum: Tam | Yok = yok) → Tensör

Son eksendeki softmax(logit / sıcaklık) dağılımından bir indeks örnekler; sonuç tam sayılı ve türev izlemeyen bir tensördür. sıcaklık 0 en olasıyı verir, en_iyi_k > 0 seçimi en olası k sınıfla sınırlar, aynı tohum aynı sonucu verir.

```tr
yazdır(örnekle(tensör([1.0, 3.0, 0.5]), sıcaklık: 0.8, en_iyi_k: 2, tohum: 7).değer)
```

İlişkili: TR_fn_argmax `argmax`, TR_fn_softmax `softmax`

#### TR_fn_save_tensors `tensör_kaydet`

    tensör_kaydet(Metin, Sözlük) → Yok

Anahtarları metin, değerleri tensör olan bir sözlüğü sürüm başlıklı ikili TR tensör dosyasına yazar. Yazma atomiktir; dosya varsa üzerine yazılır ve tensörler kendi türlerinde saklanır.

```tr
tensör_kaydet("agirliklar.trt", {"w": tensör([[1.0, 2.0]]), "b": tensör([0.5])}); yazdır(tensör_yükle("agirliklar.trt"))
```

İlişkili: TR_fn_load_tensors `tensör_yükle`, TR_fn_parameters `parametreler`

#### TR_fn_shape_groups `şekil_grupları`

    şekil_grupları()



#### TR_fn_sigmoid `sigmoid`

    sigmoid(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemana 1 / (1 + e^(-x)) uygulayarak değerleri 0 ile 1 arasına sıkıştırır. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(sigmoid(tensör([-2.0, 0.0, 2.0])).değer)
```

İlişkili: TR_fn_tanh `tanh`, TR_fn_relu `relu`

#### TR_fn_sign `işaret`

    işaret(Tam | Ondalık | Tensör) → Tam | Ondalık | Tensör

Pozitif sayıda 1, negatifte -1, sıfırda 0 verir; Tam girdide Tam, Ondalık girdide Ondalık döner, NaN korunur. Tensörde eleman bazlı çalışır, türevi her yerde (0'da da) 0'dır; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(işaret(-4), işaret(0.0), işaret(tensör([-2.0, 0.0, 3.0])).değer)
```

İlişkili: TR_fn_abs `mutlak`

#### TR_fn_silu `silu`

    silu(Tensör | Tam | Ondalık) → Tensör | Ondalık

Eleman bazlı SiLU etkinleştirmesidir: x · sigmoid(x); tek geçişte hesaplanır, kapılı ileri beslemeli katmanların (SwiGLU) temelidir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(silu(tensör([-1.0, 0.0, 2.0])).değer)
```

İlişkili: TR_fn_sigmoid `sigmoid`, TR_fn_gelu `gelu`

#### TR_fn_sin `sin`

    sin(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın sinüsünü (radyan) hesaplar; türevi cos(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(sin(tensör([0.0, 1.0])).değer)
```

İlişkili: TR_fn_cos `cos`

#### TR_fn_sinh `sinh`

    sinh(Tensör | Tam | Ondalık) → Tensör | Ondalık

Eleman bazlı hiperbolik sinüstür: (e^x - e^(-x)) / 2; türevi cosh(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(sinh(1.0), sinh(tensör([0.0, 1.0])).değer)
```

İlişkili: TR_fn_cosh `cosh`, TR_fn_tanh `tanh`

#### TR_fn_sleep `bekle`

    bekle(Tam | Ondalık) → Mantık

Verilen saniye kadar bekler ve doğru döndürür; beklerken yorumlayıcı kilidini bırakır, öbür görevler ilerler.

```tr
bekle(0.1); yazdır("0.1 saniye geçti")
```

İlişkili: TR_fn_wait_all `bekle_hepsi`, TR_fn_time_ms `zaman_ms`

#### TR_fn_softmax `softmax`

    softmax(Tensör, [Tam], eksen: Tam = -1) → Tensör

Verilen eksende (varsayılan son eksen) değerleri toplamı 1 olan olasılıklara çevirir. Yalnızca tensörde çalışır, listede çalışmaz (tensör(...) ile çevrilir); tamamen -sonsuz bir satırın sonucu 0'dır.

```tr
yazdır(softmax(tensör([[1.0, 2.0, 3.0]]), eksen: -1).değer)
```

İlişkili: TR_fn_log_softmax `log_softmax`, TR_fn_sample `örnekle`

#### TR_fn_sqrt `karekök`

    karekök(Tam | Ondalık | Tensör) → Ondalık | Tensör

Sayının karekökünü Ondalık olarak verir (negatif sayıda NaN); tensörde eleman bazlı ve türev izleyerek çalışır, liste kabul etmez.

```tr
yazdır(karekök(16), karekök(2.25))
```

İlişkili: TR_fn_abs `mutlak`

#### TR_fn_state_tensor `durum_tensörü`

    durum_tensörü()



#### TR_fn_std `standart_sapma`

    standart_sapma(Liste | Tam | Ondalık, [Tam | Ondalık...]) → Ondalık

Sayıların popülasyon standart sapmasını (n'ye bölerek) Ondalık olarak verir; ortalama gibi tek bir liste ya da birden çok sayı alır, boş girdi ve sayı olmayan eleman hatadır.

```tr
yazdır(standart_sapma([2, 4, 4, 4, 5, 5, 7, 9]))
```

İlişkili: TR_fn_mean `ortalama`

#### TR_fn_stop_requested `durdurma_istendi`

    durdurma_istendi() → Mantık

Bir durdurma isteği geldiyse doğru döner ve hiçbir şeyi değiştirmez; isteği durdurmayı_üstlen() ya da kontrol noktası ertelemişse sorgulanır.

```tr
durdurma_istendi()
```

İlişkili: TR_fn_handle_stop `durdurmayı_üstlen`

#### TR_fn_str `metin`

    metin(Herhangi) → Metin

Her değeri yazdırıldığı biçimiyle Metin'e çevirir; hiçbir değerde hata vermez.

```tr
yazdır(metin(12) + "a", metin([1, 2]))
```

İlişkili: TR_fn_int `tam`, TR_fn_float `ondalık`

#### TR_fn_sum `toplam`

    toplam(Liste | Tensör, [Tam], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Tam | Ondalık | Tensör

Sayı listesini toplar (hepsi Tam ise Tam, bir Ondalık varsa Ondalık); sayı olmayan eleman ve iç içe liste hatadır. Tensörde bütün elemanları ya da eksen: ile verilen ekseni indirger.

```tr
yazdır(toplam([1, 2, 3]), toplam([1, 2.5]), toplam(tensör([[1, 2], [3, 4]]), eksen: 0).değer)
```

İlişkili: TR_fn_mean `ortalama`

#### TR_fn_swiglu_gate `swiglu_kapısı`

    swiglu_kapısı()



#### TR_fn_swiglu_gate_fused `swiglu_kapısı_kaynaşık`

    swiglu_kapısı_kaynaşık()



#### TR_fn_swiglu_intermediate_dim `swiglu_ara_boyutu`

    swiglu_ara_boyutu()



#### TR_fn_swiglu_merged_gate `swiglu_birleşik_kapısı`

    swiglu_birleşik_kapısı()



#### TR_fn_swiglu_merged_gate_fused `swiglu_birleşik_kapısı_kaynaşık`

    swiglu_birleşik_kapısı_kaynaşık()



#### TR_fn_tan `tan`

    tan(Tensör | Tam | Ondalık) → Tensör | Ondalık

Her elemanın tanjantını (radyan) hesaplar; türevi 1 + tan(x)²'dir. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(tan(0.5), tan(tensör([0.0, 1.0])).değer)
```

İlişkili: TR_fn_sin `sin`, TR_fn_cos `cos`, TR_fn_atan `atan`

#### TR_fn_tanh `tanh`

    tanh(Tensör | Tam | Ondalık) → Tensör | Ondalık

Eleman bazlı hiperbolik tanjanttır; değerleri -1 ile 1 arasına sıkıştırır. Tensörde türev izleyen tensör, sayıda Ondalık döndürür; listede çalışmaz (tensör(...) ile çevrilir).

```tr
yazdır(tanh(0.5), tanh(tensör([-1.0, 1.0])).değer)
```

İlişkili: TR_fn_sigmoid `sigmoid`

#### TR_fn_temp_dir `geçici_klasör`

    geçici_klasör() → Yol

İşletim sisteminin geçici klasörünün altında bu program için yeni, boş ve yalnızca kullanıcıya açık bir klasör oluşturur ve Yol olarak döndürür; her çağrı ayrı bir klasör verir. Program bitince klasör içindekilerle birlikte silinir (sil'in korumaları uygulanır, bağlantıların gösterdiği yere dokunulmaz).

```tr
değişken k = geçici_klasör(); dosya_yaz(yol(k, "not.txt"), "x"); yazdır(yol(k, "not.txt").var_mı, klasörü_listele(k).uzunluk)
```

İlişkili: TR_fn_make_dir `klasör_oluştur`, TR_fn_remove `sil`

#### TR_fn_tensor `tensör`

    tensör(Tam | Ondalık | Liste | Tensör, [Mantık], türev_izle: Mantık = yanlış, tür: Metin) → Tensör

İç içe sayı listesinden, tek sayıdan ya da başka bir tensörden yeni bir yaprak tensör oluşturur. türev_izle varsayılan olarak yanlıştır; tür verilmezse listeden float32 olur, tensörden kaynağın türü korunur (tensörün türünü değiştirmek için tür: değil .türe kullanılır).

```tr
değişken t = tensör([[1, 2], [3, 4]], türev_izle: doğru); yazdır(t.şekil, t.tür)
```

İlişkili: TR_fn_zeros `sıfırlar`, TR_fn_ones `birler`, TR_fn_randn `rastgele_normal`

#### TR_fn_time_ms `zaman_ms`

    zaman_ms() → Tam

Şimdiki zamanı 1970'ten bu yana geçen milisaniye olarak Tam verir; süre ölçmek için kullanılır.

```tr
değişken baş = zaman_ms(); bekle(0.05); yazdır(zaman_ms() - baş >= 50)
```

İlişkili: TR_fn_sleep `bekle`

#### TR_fn_topk `en_büyük_k`

    en_büyük_k(Tensör, Tam, eksen: Tam = -1) → Liste

Verilen eksende (varsayılan son eksen) en büyük k elemanı büyükten küçüğe [değerler, indeksler] listesi olarak verir: değerler türev izler, indeksler tam sayılı ve türevsizdir. Eşit değerlerde küçük indeks önce gelir, NaN her sayıdan büyüktür; k 0 ile eksen uzunluğu arasında olmalıdır.

```tr
değişken s = en_büyük_k(tensör([3.0, 1.0, 4.0, 1.5]), 2); yazdır(s[0].değer, s[1].değer)
```

İlişkili: TR_fn_argsort `sıralama_indeksi`, TR_fn_argmax `argmax`

#### TR_fn_type `tip`

    tip(Herhangi) → Tip

Değerin tipini bir tip değeri (Tip) olarak verir; tip adlarıyla karşılaştırılır (tip(x) == Tam), yazdırılınca adı görünür. Bir yapı nesnesi için yapının kendisini verir (tip(p) == Nokta).

```tr
yazdır(tip(3.5), tip([1, 2]) == Liste, tip(yok))
```

İlişkili: TR_fn_int `tam`, TR_fn_float `ondalık`, TR_fn_str `metin`

#### TR_fn_wait_all `bekle_hepsi`

    bekle_hepsi() → Mantık

Başlatılmış bütün eşzamanlı görevlerin bitmesini bekler ve doğru döndürür; eşzamanlı bloğunun içinden çağrılması hatadır.

```tr
değişken s = 0; döngü i içinde 1..=3 { eşzamanlı { s += i } }; bekle_hepsi(); yazdır(s)
```

İlişkili: TR_kw_async `eşzamanlı`, TR_fn_sleep `bekle`

#### TR_fn_where `koşullu`

    koşullu(Tensör | Mantık | Tam | Ondalık, Tensör | Tam | Ondalık, Tensör | Tam | Ondalık) → Tensör | Tam | Ondalık

koşullu(koşul, a, b), koşulun doğru (sıfırdan farklı) olduğu elemanlarda a'yı, diğerlerinde b'yi seçer; üçü de yayınlamalıdır, koşul 0/1 maskesi (ör. x > 0), sayı ya da Mantık olabilir. Gradyan a'ya koşulun doğru, b'ye yanlış olduğu yerlerden akar, koşul türev almaz; hiçbiri tensör değilse a ya da b olduğu gibi döner.

```tr
değişken x = tensör([-1.0, 2.0, -3.0]); yazdır(koşullu(x > 0, x, 0).değer)
```

İlişkili: TR_kw_if `eğer`

#### TR_fn_write_file `dosya_yaz`

    dosya_yaz(Metin | Yol, Herhangi) → Mantık

Değeri metne çevirip dosyaya yazar ve doğru döndürür; var olan dosyanın yerine konur. Yazma atomiktir: içerik önce aynı klasörde geçici bir dosyaya yazılıp diske işlenir, sonra eski dosyanın yerine konur (yarıda kalan yazma eski dosyayı bozmaz). .gz uzantılı yola sıkıştırarak yazar.

```tr
dosya_yaz("not.txt", "merhaba"); yazdır(dosya_oku("not.txt"))
```

İlişkili: TR_fn_read_file `dosya_oku`

#### TR_fn_write_slice `yerine_yaz`

    yerine_yaz(Tensör, Tensör, eksen: Tam = -1, başlangıç: Tam | Tensör = 0) → Yok

Kaynağı hedefin verilen eksenindeki dilime, başlangıç konumundan itibaren yerinde yazar (hedefin öbür eksenleri kaynakla aynı olmalı). Başlangıç tam sayı ya da 0 boyutlu tam sayı tensörüdür; tensör verilirse tekrar_oynat kaydında konum her oynatmada cihazdan okunur (KV önbelleği). Türev izlenen hedefe yalnızca türevsiz içinde yazılır; sonuç yok.

```tr
değişken t = sıfırlar([2, 4]); yerine_yaz(t, birler([2, 2]), eksen: 1, başlangıç: 1); yazdır(t.değer)
```

İlişkili: TR_fn_concat `bitiştir`

#### TR_fn_zero_grads `gradyanları_sıfırla`

    gradyanları_sıfırla()



#### TR_fn_zeros `sıfırlar`

    sıfırlar(Liste, türev_izle: Mantık = yanlış, tür: Metin = "float32") → Tensör

Verilen şekilde, bütün elemanları 0 olan bir tensör oluşturur. Şekil yalnızca liste olarak verilir; sıfırlar(2, 3) yazımı hatadır ve liste biçimini önerir.

```tr
yazdır(sıfırlar([2, 3]).şekil)
```

İlişkili: TR_fn_ones `birler`, TR_fn_tensor `tensör`

### Yerleşik sabitler

#### TR_const_BLOCK_NAMES `BLOK_ADLARI`

    BLOK_ADLARI



#### TR_const_DRY_RUN_STEPS `PROVA_ADIMI`

    PROVA_ADIMI



#### TR_const_MIN_RELATIVE_STD `EN_AZ_GÖRELİ_SAPMA`

    EN_AZ_GÖRELİ_SAPMA



#### TR_const_NONFINITE `SONLU_DEĞİL`

    SONLU_DEĞİL



#### TR_const_NS_A `NS_A`

    NS_A



#### TR_const_NS_B `NS_B`

    NS_B



#### TR_const_NS_C `NS_C`

    NS_C



#### TR_const_OK `TAMAM`

    TAMAM



#### TR_const_SPIKE `SIÇRAMA`

    SIÇRAMA



#### TR_const_SUPPORTED_ARCHITECTURES `DESTEKLENEN_MİMARİLER`

    DESTEKLENEN_MİMARİLER



#### TR_const_pi `pi`

    pi → Ondalık

Bir çemberin çevresinin çapına oranı olan π sayısıdır (Ondalık, 3.141592653589793); çağrılmaz, değer olarak yazılır. sin, cos ve tan radyanla çalışır: 180 derece pi radyandır.

```tr
yazdır(pi, sin(pi / 2), cos(pi))
```

İlişkili: TR_fn_sin `sin`, TR_fn_tan `tan`

### Adlı argümanlar

#### TR_arg_G_muon `G`



#### TR_arg_T `T`



#### TR_arg_adamw_lr `adamw_öğrenme_oranı`



#### TR_arg_adamw_weight_decay `adamw_çürümesi`



#### TR_arg_approximate `yaklaşık`

gelu'nun hangi biçimde hesaplanacağını seçer: doğru (varsayılan) tanh yaklaşımı, yanlış tam (erf tabanlı) GELU. Mantık olmalıdır.

```tr
yazdır(gelu(tensör([-1.0, 0.0, 1.0]), yaklaşık: yanlış).değer)
```

Kullananlar: TR_fn_gelu `gelu` · İlişkili: TR_fn_gelu `gelu`

#### TR_arg_backward `geri`

özel_türev'in geri hesabıdır: fonksiyon(saklananlar, g) biçiminde iki parametre alır ve her konumlu girdi için girdiyle aynı şekilde bir gradyan ya da yok döndürür.

```tr
değişken f = özel_türev(ileri: fonksiyon(x) { sonuç [x * 3, []] }, geri: fonksiyon(s, g) { sonuç [g * 3] }); değişken x = tensör([2.0], türev_izle: doğru); geri_yayılım(toplam(f(x))); yazdır(x.gradyan.değer)
```

Kullananlar: TR_fn_custom_grad `özel_türev` · İlişkili: TR_arg_forward `ileri`

#### TR_arg_batch_size `toplu`

Okuyucunun her .sonraki() çağrısında verdiği satır (toplu iş) sayısıdır. Pozitif bir tam sayı olmalıdır ve verilmesi zorunludur.

```tr
değişken ö = veri_kümesi("tests/data/numbers").örnekleyici(toplu: 8, bağlam: 16)
```

Kullananlar: TR_mem_sampler `örnekleyici`, TR_mem_sequential `sıralı` · İlişkili: TR_arg_context `bağlam`

#### TR_arg_beta1 `beta1`

AdamW'nin gradyan ortalaması için kullandığı üstel azalma katsayısıdır. [0, 1) aralığında olmalıdır; varsayılan 0.9'dur.

```tr
değişken w = tensör([1.0], türev_izle: doğru); yazdır(AdamW([w], beta1: 0.8))
```

Kullananlar: TR_fn_AdamW `AdamW` · İlişkili: TR_arg_beta2 `beta2`

#### TR_arg_beta2 `beta2`

AdamW'nin gradyan karelerinin ortalaması için kullandığı üstel azalma katsayısıdır. [0, 1) aralığında olmalıdır; varsayılan 0.999'dur.

```tr
değişken w = tensör([1.0], türev_izle: doğru); yazdır(AdamW([w], beta2: 0.99))
```

Kullananlar: TR_fn_AdamW `AdamW` · İlişkili: TR_arg_beta1 `beta1`

#### TR_arg_bias `sapma`

katman_normu sonucuna son eksende eklenen [D] şeklindeki kayma tensörüdür; yok verilirse eklenmez. Konumlu da verilebilir.

```tr
yazdır(katman_normu(tensör([[1.0, 3.0]]), sapma: tensör([0.5, 0.5])).değer)
```

Kullananlar: TR_fn_layer_norm `katman_normu` · İlişkili: TR_arg_weight `ağırlık`

#### TR_arg_buffers `tamponlar`



#### TR_arg_cache `önbellek`

veri_kümesi'nin veri dosyalarını kullanmadan önce kopyalayacağı yerel önbellek klasörüdür; kota dolunca en uzun süredir kullanılmayan kopyalar silinir. Tek başına verilemez: kota_gb: zorunludur.

```tr
değişken küme = veri_kümesi("tests/data/numbers", önbellek: "onbellek/veri", kota_gb: 1)
```

Kullananlar: TR_fn_dataset `veri_kümesi` · İlişkili: TR_arg_quota_gb `kota_gb`

#### TR_arg_cache_kind `önbellek_çeşidi`



#### TR_arg_capacity `kapasite`



#### TR_arg_causal `nedensel`

doğru iken (varsayılan) her sorgu yalnızca kendisine kadarki anahtarlara bakar; Tq < Tk iken hizalama sondandır. yanlış ile dikkat iki yönlüdür.

```tr
değişken q = rastgele_normal([4, 8], tohum: 1); yazdır(dikkat(q, q, q, nedensel: yanlış).şekil)
```

Kullananlar: TR_fn_attention `dikkat` · İlişkili: TR_arg_window `pencere`, TR_arg_mask `maske`

#### TR_arg_config `ayar`



#### TR_arg_context `bağlam`

Okuyucunun her satırdaki pencere uzunluğudur; girdi ve hedef [toplu, bağlam] şeklinde olur. Zorunludur ve en uzun veri dosyasından kısa olmalıdır.

```tr
değişken s = veri_kümesi("tests/data/numbers").sıralı(toplu: 4, bağlam: 16)
```

Kullananlar: TR_mem_sampler `örnekleyici`, TR_mem_sequential `sıralı` · İlişkili: TR_arg_batch_size `toplu`

#### TR_arg_cos_table `cos_tablosu`



#### TR_arg_cuda_graph `cuda_çizgesi`



#### TR_arg_decay_fraction `azalma`



#### TR_arg_description `açıklama`

komut_satırı'nın --yardım çıktısının başında gösterilen, programın ne yaptığını anlatan metindir; varsayılan boştur.

```tr
değişken ayar = komut_satırı({adım: 10}, açıklama: "Modeli eğitir"); yazdır(ayar.adım)
```

Kullananlar: TR_fn_command_line `komut_satırı` · İlişkili: TR_arg_descriptions `açıklamalar`

#### TR_arg_descriptions `açıklamalar`

komut_satırı seçeneklerinin --yardım çıktısındaki açıklamalarıdır: anahtarlar seçenek adları, değerler Metin olmalıdır; seçeneklerde olmayan bir ad hatadır.

```tr
değişken ayar = komut_satırı({adım: 10}, açıklamalar: {adım: "eğitim adımı sayısı"}); yazdır(ayar.adım)
```

Kullananlar: TR_fn_command_line `komut_satırı` · İlişkili: TR_arg_description `açıklama`

#### TR_arg_device `cihaz`



#### TR_arg_dim `eksen`

İşlemin uygulanacağı ekseni seçer; negatif değer sondan sayar (-1 son eksen). İndirgemelerde verilmezse bütün elemanlar, softmax, bitiştir ve argmax'te son eksen kullanılır.

```tr
yazdır(toplam(tensör([[1, 2], [3, 4]]), eksen: 0).değer)
```

Kullananlar: TR_fn_argmax `argmax`, TR_fn_argsort `sıralama_indeksi`, TR_fn_concat `bitiştir`, TR_fn_cumsum `birikimli_toplam`, TR_fn_log_softmax `log_softmax`, TR_fn_max `max`, TR_fn_mean `ortalama`, TR_fn_min `min`, TR_fn_softmax `softmax`, TR_fn_sum `toplam`, TR_fn_topk `en_büyük_k`, TR_fn_write_slice `yerine_yaz`, TR_mem_split `böl` · İlişkili: TR_arg_keepdim `boyutu_koru`

#### TR_arg_dry_run `prova`



#### TR_arg_dtype `tür`

Oluşturulan tensörün veri türünü seçer: "float32" (varsayılan), "bf16" ya da "fp16". Normlarda (katman_normu, rms_normu) çıktının türüdür; hesap yine float32'dir, verilmezse çıktı girdinin türündedir. Başka bir ad en yakın geçerli adı öneren bir hatadır.

```tr
yazdır(sıfırlar([2, 2], tür: "fp16").tür)
```

Kullananlar: TR_fn_layer_norm `katman_normu`, TR_fn_ones `birler`, TR_fn_randn `rastgele_normal`, TR_fn_rms_norm `rms_normu`, TR_fn_tensor `tensör`, TR_fn_zeros `sıfırlar`

#### TR_arg_embed_dim `boyut`



#### TR_arg_end `bitiş`



#### TR_arg_eos `eos`



#### TR_arg_eps `epsilon`

Sıfıra bölmeyi önlemek için paydaya eklenen küçük pozitif sayıdır. Normlarda varsayılan 0.00001, AdamW'de 1e-8'dir.

```tr
yazdır(katman_normu(tensör([[1.0, 2.0, 3.0]]), epsilon: 0.001).değer)
```

Kullananlar: TR_fn_AdamW `AdamW`, TR_fn_layer_norm `katman_normu`, TR_fn_rms_norm `rms_normu`

#### TR_arg_error_if_nonfinite `sonlu_değilse_hata`

gradyan_kırp'ın seçeneğidir (Mantık, varsayılan yanlış): doğruysa gradyanların toplam normu NaN ya da sonsuzsa hata verir (PyTorch clip_grad_norm_(error_if_nonfinite=True)). Normu okumak için GPU'yu bekler; varsayılanda bekleme yoktur. Adımı atlamak isteyen eğitim bunun yerine sonlu_mu(norm.değer) ile denetler.

```tr
değişken w = tensör([1.0, 2.0], türev_izle: doğru); geri_yayılım(toplam(w * ondalık("inf"))); dene { gradyan_kırp([w], sonlu_değilse_hata: doğru) } yakala h { yazdır("sonlu değil") }
```

Kullananlar: TR_fn_clip_grad `gradyan_kırp` · İlişkili: TR_fn_clip_grad `gradyan_kırp`, TR_fn_is_finite `sonlu_mu`

#### TR_arg_every_minutes `her_dakika`

kontrol_noktası'nın kayıt aralığıdır (dakika, varsayılan 15; 0: her güncelle'de). Süre dolunca sıradaki .güncelle(adım) çağrısı adım sınırında kaydeder; durdurma isteğinde (Ctrl+C) aralık beklenmez.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken kn = kontrol_noktası("koşu_t984", {w: w}, her_dakika: 0.5); yazdır(kn)
```

Kullananlar: TR_fn_checkpoint `kontrol_noktası` · İlişkili: TR_arg_keep `tut`

#### TR_arg_fallback `yedek`



#### TR_arg_fd_step `fark_adımı`

türev_denetle'de sayısal gradyanı veren merkezi farkın adım büyüklüğüdür (h): (f(x + h) - f(x - h)) / 2h. Pozitif olmalıdır; varsayılan 0.001'dir.

```tr
yazdır(türev_denetle(fonksiyon(x) { sonuç x * x }, [tensör([1.0, 2.0], türev_izle: doğru)], fark_adımı: 0.01))
```

Kullananlar: TR_fn_grad_check `türev_denetle` · İlişkili: TR_arg_tol `tolerans`

#### TR_arg_ff_use_bias `ileri_sapmalı`



#### TR_arg_folder `klasör`



#### TR_arg_forward `ileri`

özel_türev'in ileri hesabıdır: türevsiz çalışır ve [çıktı, saklananlar] döndürmelidir. Çıktı bir tensör ya da tensör listesidir.

```tr
değişken f = özel_türev(ileri: fonksiyon(x) { sonuç [x * 3, []] }, geri: fonksiyon(s, g) { sonuç [g * 3] }); yazdır(f(tensör([2.0])).değer)
```

Kullananlar: TR_fn_custom_grad `özel_türev` · İlişkili: TR_arg_backward `geri`

#### TR_arg_g_swiglu `g`



#### TR_arg_gate `kapı`



#### TR_arg_gu `gu`



#### TR_arg_h `h`



#### TR_arg_head_dim `bakış_boyutu`



#### TR_arg_history `geçmiş`



#### TR_arg_history_limit `geçmiş_sınırı`



#### TR_arg_in_features `giriş_boyutu`



#### TR_arg_init_std `başlatma_sapması`



#### TR_arg_intermediate_dim `ara_boyut`



#### TR_arg_k `k`



#### TR_arg_keep `tut`

kontrol_noktası'nın saklayacağı en yeni nokta sayısıdır (pozitif Tam, varsayılan 2); daha eskileri her başarılı kayıttan sonra silinir.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken kn = kontrol_noktası("koşu_t985", {w: w}, tut: 3); döngü i içinde 1..5 { kn.güncelle(i); kn.kaydet() }; yazdır(klasörü_listele("koşu_t985/checkpoints", "step_*").uzunluk)
```

Kullananlar: TR_fn_checkpoint `kontrol_noktası` · İlişkili: TR_arg_every_minutes `her_dakika`

#### TR_arg_keepdim `boyutu_koru`

doğru verilince indirgenen eksen 1 uzunlukta korunur, böylece sonuç girdiye yayınlanabilir; eksen verilmezse bütün eksenler 1 olur. Mantık olmalıdır ve yalnızca tensörlerde geçerlidir.

```tr
yazdır(toplam(tensör([[1, 2], [3, 4]]), eksen: -1, boyutu_koru: doğru).şekil)
```

Kullananlar: TR_fn_max `max`, TR_fn_mean `ortalama`, TR_fn_min `min`, TR_fn_sum `toplam` · İlişkili: TR_arg_dim `eksen`

#### TR_arg_key `anahtar`



#### TR_arg_kind `çeşit`



#### TR_arg_layer_idx `katman_no`



#### TR_arg_layout `düzen`



#### TR_arg_log `günlük`

ilerleme çubuğunun her güncellemesini (adım, ölçüler, hız) ve bitişini verilen Günlük'e de yazar; varsayılan yok.

```tr
değişken g = günlük("kayit/ilerleme.jsonl"); değişken ç = ilerleme(2, günlük: g); ç.güncelle({kayıp: 0.9}); ç.güncelle({kayıp: 0.7})
```

Kullananlar: TR_fn_progress `ilerleme` · İlişkili: TR_fn_logger `günlük`

#### TR_arg_loss `kayıp`



#### TR_arg_lr `öğrenme_oranı`

İyileştiricinin her adımda parametreleri ne kadar değiştireceğini belirleyen orandır. Sonlu ve negatif olmayan bir sayı olmalıdır; varsayılan AdamW'de 0.001, SGD'de 0.01'dir.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken opt = SGD([w], öğrenme_oranı: 0.5); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

Kullananlar: TR_fn_AdamW `AdamW`, TR_fn_SGD `SGD` · İlişkili: TR_arg_weight_decay `ağırlık_çürümesi`, TR_mem_lr `öğrenme_oranı`

#### TR_arg_mask `maske`

[..., H, Tq, Tk] şekline yayınlanabilen 0/1 tensörüdür; sıfırdan farklı yerlere bakılır. Türev izlemez; varsayılan yok'tur.

```tr
değişken q = rastgele_normal([3, 4], tohum: 1); yazdır(dikkat(q, q, q, nedensel: yanlış, maske: tensör([[1, 1, 0], [1, 1, 0], [1, 1, 1]])).şekil)
```

Kullananlar: TR_fn_attention `dikkat` · İlişkili: TR_arg_causal `nedensel`

#### TR_arg_master_weights `ana_kopya`

doğru iken (varsayılan) 16 bitlik parametrelerin float32 ana kopyası tutulur; güncelleme orada yapılıp parametreye yuvarlanır, böylece küçük adımlar kaybolmaz. float32 parametrelerde etkisizdir.

```tr
değişken w = tensör([1.0], türev_izle: doğru, tür: "bf16"); değişken opt = AdamW([w], ana_kopya: doğru); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

Kullananlar: TR_fn_AdamW `AdamW`, TR_fn_SGD `SGD`

#### TR_arg_max `en_fazla`

gradyan_kırp için izin verilen en büyük toplam gradyan normudur (pozitif bir sayı, varsayılan 1.0); sınırla için üst sınırdır (sayı; en_az'dan küçük olamaz).

```tr
değişken w = tensör([3.0, 4.0], türev_izle: doğru); geri_yayılım(toplam(w * w)); yazdır(gradyan_kırp([w], en_fazla: 0.5))
```

Kullananlar: TR_fn_clamp `sınırla`, TR_fn_clip_grad `gradyan_kırp` · İlişkili: TR_arg_min `en_az`

#### TR_arg_max_new_tokens `en_fazla_yeni_parça`



#### TR_arg_merged `birleşik`



#### TR_arg_merged_gate `birleşik_kapı`



#### TR_arg_merged_kv `birleşik_kv`



#### TR_arg_metrics `ölçüler`



#### TR_arg_min `en_az`

sınırla için alt sınırdır: değerden küçük elemanlar bu değere çekilir. Bir sayı olmalıdır ve en_fazla'dan büyük olamaz; en_az ile en_fazla'dan en az biri verilmelidir.

```tr
yazdır(sınırla(-3.5, en_az: 0.0), sınırla(tensör([-1.0, 2.0]), en_az: 0).değer)
```

Kullananlar: TR_fn_clamp `sınırla` · İlişkili: TR_arg_max `en_fazla`

#### TR_arg_model `model`



#### TR_arg_momentum `momentum`

SGD'nin önceki adımların yönünü sürdürme katsayısıdır; varsayılan 0'dır (momentum yok).

```tr
değişken w = tensör([1.0], türev_izle: doğru); yazdır(SGD([w], momentum: 0.9))
```

Kullananlar: TR_fn_SGD `SGD`

#### TR_arg_multiplier `çarpan`



#### TR_arg_muon_lr `muon_öğrenme_oranı`



#### TR_arg_muon_scale `muon_ölçeği`



#### TR_arg_n `n`



#### TR_arg_name `ad`



#### TR_arg_named `adlarıyla`

doğru verilince parametreler liste yerine "bloklar.0.Wq" gibi yol adı → tensör sözlüğü döndürür. Mantık olmalıdır; varsayılan yanlış'tır.

```tr
yazdır(parametreler([tensör([1.0], türev_izle: doğru)], adlarıyla: doğru))
```

Kullananlar: TR_fn_parameters `parametreler`

#### TR_arg_nesterov `nesterov`



#### TR_arg_new_chunk `yeni`



#### TR_arg_norm `norm`



#### TR_arg_normuon `normuon`



#### TR_arg_ns_dtype `ns_türü`



#### TR_arg_ns_iterations `ns_yineleme_sayısı`



#### TR_arg_num_heads `bakış_sayısı`



#### TR_arg_num_kv_heads `kv_bakış_sayısı`



#### TR_arg_num_layers `katman_sayısı`



#### TR_arg_num_steps `adım_sayısı`



#### TR_arg_objects `nesneler`



#### TR_arg_out_features `çıkış_boyutu`



#### TR_arg_output_dtype `çıktı_türü`



#### TR_arg_output_init_std `çıkış_başlatma_sapması`



#### TR_arg_overwrite `üzerine_yaz`

kopyala ve taşı'da hedef zaten varsa kaynağın onun yerine konmasına izin verir; varsayılan yanlıştır (var olan hedef hatadır). Bir dosya bir klasörün, bir klasör bir dosyanın yerine konmaz.

```tr
dosya_yaz("kaynak.txt", "yeni"); dosya_yaz("hedef.txt", "eski"); kopyala("kaynak.txt", "hedef.txt", üzerine_yaz: doğru); yazdır(dosya_oku("hedef.txt"))
```

Kullananlar: TR_fn_copy `kopyala`, TR_fn_move `taşı` · İlişkili: TR_fn_copy `kopyala`, TR_fn_move `taşı`

#### TR_arg_pair `ikinci`

kodla'ya ikinci bir metin vererek çift kodlama yapar. Liste kodlanırken aynı uzunlukta bir metin listesi olmalıdır.

```tr
yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").kodla("soru", ikinci: "cevap"))
```

Kullananlar: TR_mem_encode `kodla`

#### TR_arg_parameters `parametreler`



#### TR_arg_pattern `desen`



#### TR_arg_patterns `desenler`



#### TR_arg_ple_context `ple_bağlamı`



#### TR_arg_ple_contexts `ple_bağlamları`



#### TR_arg_ple_dim `ple_boyutu`



#### TR_arg_position `konum`



#### TR_arg_prompt `istem`



#### TR_arg_qk_norm `qk_normu`



#### TR_arg_quota_gb `kota_gb`

Önbellek klasörünün kullanabileceği en çok disk alanıdır (GB). Pozitif bir sayı olmalıdır ve yalnızca önbellek: ile birlikte geçerlidir.

```tr
değişken küme = veri_kümesi("tests/data/numbers", önbellek: "onbellek/veri", kota_gb: 0.5)
```

Kullananlar: TR_fn_dataset `veri_kümesi` · İlişkili: TR_arg_cache `önbellek`

#### TR_arg_recursive `içindekilerle`

sil'de dolu bir klasörün içindekilerle birlikte silinmesine izin verir; varsayılan yanlıştır (yalnızca boş klasör silinir). İçerideki bağlantıların gösterdiği yerlere dokunulmaz; salt okunur bir dosya varsa hiçbir şey silinmez.

```tr
klasör_oluştur("çöp/alt"); dosya_yaz("çöp/alt/a.txt", "x"); yazdır(sil("çöp", içindekilerle: doğru))
```

Kullananlar: TR_fn_remove `sil` · İlişkili: TR_fn_remove `sil`

#### TR_arg_requires_grad `türev_izle`

Oluşturulan tensörün türevinin izlenip izlenmeyeceğini belirler; varsayılan yanlıştır. Parametreler ve gradyanı istenen girdiler türev_izle: doğru ile oluşturulmalıdır.

```tr
değişken x = tensör([2.0], türev_izle: doğru); geri_yayılım(toplam(x * x)); yazdır(x.gradyan.değer)
```

Kullananlar: TR_fn_load_tensors `tensör_yükle`, TR_fn_ones `birler`, TR_fn_randn `rastgele_normal`, TR_fn_tensor `tensör`, TR_fn_zeros `sıfırlar`

#### TR_arg_rope_theta `rope_tabanı`



#### TR_arg_row `satır`



#### TR_arg_scale `ölçek`



#### TR_arg_seed `tohum`

Rastgele sayı üretecinin başlangıç değeridir (tam sayı); aynı tohum her çalıştırmada aynı sonucu verir. Verilmezse paylaşılan bir üreteç kullanılır.

```tr
yazdır(rastgele_normal([3], tohum: 7).değer)
```

Kullananlar: TR_fn_randn `rastgele_normal`, TR_fn_sample `örnekle`, TR_mem_sampler `örnekleyici`

#### TR_arg_separated `ayrılan`



#### TR_arg_shape `şekil`



#### TR_arg_sin_table `sin_tablosu`



#### TR_arg_sparse `seyrek`



#### TR_arg_sparse_lr `seyrek_öğrenme_oranı`



#### TR_arg_special_tokens `özel_parçalar`

Özel parçaların kodlamaya eklenip eklenmeyeceğini (kodla, varsayılan doğru) ya da çözülen metne yazılıp yazılmayacağını (çöz, varsayılan yanlış) belirler. Yalnızca doğru ya da yanlış olabilir.

```tr
yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").kodla("merhaba dünya", özel_parçalar: yanlış))
```

Kullananlar: TR_mem_decode `çöz`, TR_mem_encode `kodla`

#### TR_arg_start `başlangıç`

yerine_yaz'ın hedef eksende yazmaya başladığı konumdur: tam sayı ya da 0 boyutlu tam sayı tensörü. Kaynak hedefe sığmalıdır; tensörle verilen konum tekrar_oynat kaydında her oynatmada cihazdan okunur.

```tr
değişken t = sıfırlar([3]); yerine_yaz(t, birler([1]), eksen: 0, başlangıç: 2); yazdır(t.değer)
```

Kullananlar: TR_fn_write_slice `yerine_yaz` · İlişkili: TR_fn_write_slice `yerine_yaz`

#### TR_arg_step `adım`

İlerleme.güncelle'de tamamlanan eğitim adımı sayısıdır (kaldığı yerden sürdürmek için; 0 ya da pozitif Tam).

```tr
değişken ç = ilerleme(100); ç.güncelle(adım: 40); ç.durdur()
```

Kullananlar: TR_mem_update `güncelle` · İlişkili: TR_mem_update `güncelle`

#### TR_arg_targets `hedef`



#### TR_arg_temperature `sıcaklık`

örnekle'de logitlerin bölündüğü sıcaklıktır: küçük değer en olasıya yaklaştırır, 0 her zaman en olasıyı seçer. Negatif olmamalıdır; varsayılan 1.0'dır.

```tr
yazdır(örnekle(tensör([1.0, 3.0, 0.5]), sıcaklık: 0).değer)
```

Kullananlar: TR_fn_sample `örnekle` · İlişkili: TR_arg_top_k `en_iyi_k`

#### TR_arg_threshold `eşik`



#### TR_arg_tied `paylaşımlı`



#### TR_arg_token_id `kimlik`



#### TR_arg_token_ids `kimlikler`



#### TR_arg_tol `tolerans`

türev_denetle'de izin verilen göreli farktır: |analitik - sayısal| ≤ tolerans · max(1, |analitik|, |sayısal|). Pozitif olmalıdır; varsayılan 0.01'dir.

```tr
yazdır(türev_denetle(fonksiyon(x) { sonuç sigmoid(x) }, [tensör([0.3], türev_izle: doğru)], tolerans: 0.001))
```

Kullananlar: TR_fn_grad_check `türev_denetle` · İlişkili: TR_arg_fd_step `fark_adımı`

#### TR_arg_top_k `en_iyi_k`

örnekle'de seçimi en olası k sınıfla sınırlar; 0 (varsayılan) bütün sınıflar demektir. Negatif olmayan bir tam sayı olmalıdır.

```tr
yazdır(örnekle(tensör([1.0, 3.0, 0.5]), en_iyi_k: 1).değer)
```

Kullananlar: TR_fn_sample `örnekle` · İlişkili: TR_arg_temperature `sıcaklık`

#### TR_arg_u `u`



#### TR_arg_updates `güncellemeler`



#### TR_arg_use_bias `sapmalı`



#### TR_arg_v `v`



#### TR_arg_value `değer`



#### TR_arg_vocab_size `dağarcık_boyutu`



#### TR_arg_warmup `ısınma`



#### TR_arg_weight `ağırlık`

Normalleştirme sonucunu son eksende eleman bazlı ölçekleyen [D] şeklindeki tensördür; yok verilirse ölçekleme yapılmaz. Konumlu da verilebilir.

```tr
yazdır(rms_normu(tensör([[3.0, 4.0]]), ağırlık: tensör([2.0, 1.0])).değer)
```

Kullananlar: TR_fn_layer_norm `katman_normu`, TR_fn_rms_norm `rms_normu` · İlişkili: TR_arg_bias `sapma`

#### TR_arg_weight_decay `ağırlık_çürümesi`

Her adımda parametreleri sıfıra doğru küçülten düzenlileştirme katsayısıdır. Varsayılan AdamW'de 0.01, SGD'de 0'dır.

```tr
değişken w = tensör([1.0], türev_izle: doğru); yazdır(AdamW([w], ağırlık_çürümesi: 0.1))
```

Kullananlar: TR_fn_AdamW `AdamW`, TR_fn_SGD `SGD` · İlişkili: TR_arg_lr `öğrenme_oranı`

#### TR_arg_weight_dtype `ağırlık_türü`



#### TR_arg_weights `ağırlıklar`



#### TR_arg_window `pencere`

Her sorguyu son w konuma (kendisi dahil) sınırlayan yerel dikkat penceresidir; 0 (varsayılan) pencere yok demektir. nedensel: yanlış ile pencere iki yönlüdür.

```tr
değişken q = rastgele_normal([6, 4], tohum: 2); yazdır(dikkat(q, q, q, pencere: 2).şekil)
```

Kullananlar: TR_fn_attention `dikkat` · İlişkili: TR_arg_causal `nedensel`

#### TR_arg_windows `pencereler`



#### TR_arg_x `x`



## Metot ve özellikler (x.ad)

### Metot ve özellikler

#### TR_mem_absolute `mutlak`

    Yol.mutlak → Yol

Yolu dosya_oku ile aynı kuralla (önce çalışma klasörü, sonra modülün ve programın klasörü) çözüp mutlak bir Yol olarak verir; günlük ve hata ayıklama için.

```tr
yazdır(yol("a.txt").mutlak.dosya_adı)
```

#### TR_mem_adamw `adamw`



#### TR_mem_adamw_lr `adamw_öğrenme_oranı`



#### TR_mem_adamw_no_decay `adamw_çürümesiz`



#### TR_mem_advance `ilerle`



#### TR_mem_append `ekle`

    Liste.ekle(Herhangi...) → Liste
    Günlük.ekle(Sözlük | Metin) → Mantık

Listenin sonuna verilen elemanları ekler; listeyi yerinde değiştirir ve listenin kendisini verir. Günlük'te bir kayıt (Sözlük ya da Metin) yazar ve doğru verir.

```tr
değişken l = [1]; l.ekle(2, 3); yazdır(l)
```

İlişkili: TR_mem_first `ilk`, TR_mem_last `son`

#### TR_mem_append_buffer `tampon_ekle`



#### TR_mem_attention `dikkat`



#### TR_mem_bar `çubuk`



#### TR_mem_beta1 `beta1`



#### TR_mem_beta2 `beta2`



#### TR_mem_bias `sapma`



#### TR_mem_blocks `bloklar`



#### TR_mem_build_cache `önbellek_kur`



#### TR_mem_capacity `kapasite`



#### TR_mem_checkpoint `nokta`



#### TR_mem_close `kapat`

    SatırOkuyucu.kapat() → Mantık

Okuyucunun dosyasını bırakır (sonuna kadar okunmamış dosya böylece silinebilir ve taşınabilir); kapatılmış okuyucudan satır istemek hatadır. Dosyayı bu çağrı kapattıysa doğru, zaten kapalıysa (sonuna kadar okunduysa ya da önceden kapatıldıysa) yanlış verir; iki kez kapatmak hata değildir.

```tr
dosya_yaz("kapat.txt", "a\nb"); değişken o = dosya_satırları("kapat.txt"); yazdır(o.sonraki(), o.kapat(), o.kapat(), sil("kapat.txt"))
```

İlişkili: TR_mem_next `sonraki`, TR_fn_read_lines `dosya_satırları`

#### TR_mem_config `ayar`



#### TR_mem_contains `içerir`

    Sözlük.içerir(Herhangi) → Mantık
    Liste.içerir(Herhangi) → Mantık
    Metin.içerir(Metin) → Mantık

Metinde alt metni, listede değeri (== ile), sözlükte anahtarı arar ve doğru ya da yanlış verir. Metinde argüman Metin olmalıdır.

```tr
yazdır("merhaba".içerir("er"), [1, 2].içerir(2), {"a": 1}.içerir("a"))
```

İlişkili: TR_mem_starts_with `başlar`, TR_mem_ends_with `biter`

#### TR_mem_context_proj `bağlam_katmanı`



#### TR_mem_cos_table `cos_tablosu`



#### TR_mem_decode `çöz`

    Parçalayıcı.çöz(Liste | Tensör, özel_parçalar: Mantık = yanlış) → Metin | Liste

Parça kimliklerini (liste, listelerin listesi ya da tensör) metne geri çevirir. Varsayılan olarak özel parçaları atlar; özel_parçalar: doğru onları da yazar.

```tr
değişken p = parçalayıcı_yükle("tests/data/tokenizer.json"); yazdır(p.çöz(p.kodla("Merhaba dünya!")))
```

İlişkili: TR_mem_encode `kodla`

#### TR_mem_device `cihaz`

    Tensör.cihaz → Metin

Tensörün bulunduğu cihazın adını verir: "cpu", "gpu" (ilk GPU) ya da "gpu:1" gibi.

```tr
yazdır(tensör([1, 2]).cihaz)
```

İlişkili: TR_mem_to_device `cihaza`, TR_fn_default_device `varsayılan_cihaz`

#### TR_mem_device_step `cihaz_adımı`



#### TR_mem_down_proj `aşağı`



#### TR_mem_dry_run `prova`



#### TR_mem_dtype `tür`

    Tensör.tür → Metin

Tensörün sayı türünün adını verir: "float32", "bf16" ya da "fp16".

```tr
yazdır(tensör([1.0, 2.0]).tür)
```

İlişkili: TR_mem_to_dtype `türe`, TR_arg_dtype `tür`

#### TR_mem_embed `göm`



#### TR_mem_embed_dim `boyut`



#### TR_mem_embedding `gömme`



#### TR_mem_encode `kodla`

    Parçalayıcı.kodla(Metin | Liste, özel_parçalar: Mantık = doğru, ikinci: Metin | Liste) → Liste

Metni parça kimliklerinin listesine çevirir; metin listesi verilirse her metni ayrı kodlar. özel_parçalar: yanlış verilmezse parçalayıcının özel parçaları eklenir; ikinci: ile metin çifti kodlanır.

```tr
yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").kodla("Merhaba dünya!"))
```

İlişkili: TR_mem_decode `çöz`

#### TR_mem_ends_with `biter`

    Metin.biter(Metin) → Mantık

Metin verilen sonekle bitiyorsa doğru verir. Argüman Metin olmalıdır.

```tr
yazdır("rapor.tr".biter(".tr"))
```

İlişkili: TR_mem_starts_with `başlar`, TR_mem_contains `içerir`

#### TR_mem_eps `epsilon`



#### TR_mem_evaluated `değerlendirildi`



#### TR_mem_exceeds `aşıyor`



#### TR_mem_exists `var_mı`

    Yol.var_mı → Mantık

Yolda bir dosya ya da klasör var mı söyler; bağlantıyı izler, kırık bağlantıda yanlış verir.

```tr
dosya_yaz("var.txt", "x"); yazdır(yol("var.txt").var_mı, yol("olmayan.txt").var_mı)
```

İlişkili: TR_mem_is_file `dosya_mı`, TR_mem_is_dir `klasör_mü`

#### TR_mem_extension `uzantı`

    Yol.uzantı → Metin

Dosya adının noktasıyla birlikte son uzantısını verir ("derlem.tar.gz" → ".gz"); uzantı yoksa ya da ad nokta ile başlıyorsa (".gitignore") boş metindir.

```tr
yazdır(yol("veri/derlem.tar.gz").uzantı)
```

İlişkili: TR_mem_stem `uzantısız_ad`

#### TR_mem_feed_forward `ileri_besleme`



#### TR_mem_filter `filtrele`

    Liste.filtrele(Fonksiyon) → Liste

Fonksiyonun doğru döndüğü elemanlardan oluşan yeni bir liste verir. Asıl liste değişmez; fonksiyon sondaki blokla da yazılır.

```tr
yazdır([1, 2, 3, 4].filtrele { o > 2 })
```

İlişkili: TR_mem_map `haritala`, TR_mem_reduce `indirge`

#### TR_mem_final_norm `son_norm`



#### TR_mem_finish `bitir`



#### TR_mem_first `ilk`

    Liste.ilk → Herhangi

Listenin ilk elemanını verir; liste boşsa yok verir. Parantezsiz yazılan bir özelliktir.

```tr
yazdır([5, 6, 7].ilk)
```

İlişkili: TR_mem_last `son`

#### TR_mem_folder `klasör`



#### TR_mem_forward `ileri`



#### TR_mem_gate_proj `kapı`



#### TR_mem_gate_up_proj `kapı_yukarı`



#### TR_mem_generate `üret`



#### TR_mem_get `varsa_al`

    Liste.varsa_al(Tam) → Herhangi
    Metin.varsa_al(Tam) → Metin | Yok

i. öğeyi (negatif indeks sondan sayar) verir; öyle bir öğe yoksa yok döner. Köşeli parantezle okuma (l[i]) sınır dışında hata verir; öğenin olmayabileceği bilinçli durumda varsa_al ile ?? birlikte kullanılır.

```tr
değişken l = [10, 20, 30]; yazdır(l.varsa_al(1), l.varsa_al(5) ?? 0, "ab".varsa_al(-1))
```

İlişkili: TR_mem_first `ilk`, TR_mem_last `son`

#### TR_mem_grad `gradyan`

    Tensör.gradyan → Tensör | Yok

geri_yayılım sonrası biriken gradyanı tensör olarak verir; henüz yoksa yok döner. Yalnızca yaprak tensörlerde okunabilir, bir işlemin sonucunda hata verir.

```tr
değişken x = tensör([1.0, 2.0], türev_izle: doğru); geri_yayılım(toplam(x * 3)); yazdır(x.gradyan.değer)
```

İlişkili: TR_fn_backward `geri_yayılım`, TR_arg_requires_grad `türev_izle`

#### TR_mem_groups `gruplar`



#### TR_mem_head_dim `bakış_boyutu`



#### TR_mem_hidden_states `gizli_temsil`



#### TR_mem_history_length `geçmiş_uzunluğu`



#### TR_mem_history_limit `geçmiş_sınırı`



#### TR_mem_input_norm `norm1`



#### TR_mem_is_dir `klasör_mü`

    Yol.klasör_mü → Mantık

Yol bir klasör mü söyler (bağlantıyı izler); yol yoksa yanlış verir.

```tr
klasör_oluştur("bir_klasör"); yazdır(yol("bir_klasör").klasör_mü)
```

İlişkili: TR_mem_exists `var_mı`, TR_mem_is_file `dosya_mı`

#### TR_mem_is_file `dosya_mı`

    Yol.dosya_mı → Mantık

Yol sıradan bir dosya mı söyler (bağlantıyı izler); yol yoksa yanlış verir.

```tr
klasör_oluştur("bir_klasör"); yazdır(yol("bir_klasör").dosya_mı)
```

İlişkili: TR_mem_exists `var_mı`, TR_mem_is_dir `klasör_mü`

#### TR_mem_join `birleştir`

    Liste.birleştir([Metin]) → Metin

Liste elemanlarını metne çevirip aralarına ayırıcıyı koyarak birleştirir (ayırıcı verilmezse boş metin). Sonuç bir metindir.

```tr
yazdır(["a", "b", "c"].birleştir(", "))
```

İlişkili: TR_mem_split `böl`

#### TR_mem_k_buffers `k_tamponları`



#### TR_mem_k_norm `k_norm`



#### TR_mem_k_proj `anahtar_katmanı`



#### TR_mem_keys `anahtarlar`

    Sözlük.anahtarlar() → Liste

Sözlüğün anahtarlarını ekleniş sırasıyla bir liste olarak verir.

```tr
yazdır({"ad": "Ali", "puan": 30}.anahtarlar())
```

İlişkili: TR_mem_values `değerler`

#### TR_mem_kind `çeşit`



#### TR_mem_kv_proj `kv_katmanı`



#### TR_mem_last `son`

    Liste.son → Herhangi

Listenin son elemanını verir; liste boşsa yok verir. Parantezsiz yazılan bir özelliktir.

```tr
yazdır([5, 6, 7].son)
```

İlişkili: TR_mem_first `ilk`

#### TR_mem_last_step `son_adım`



#### TR_mem_length `uzunluk`

    Metin.uzunluk → Tam
    Liste.uzunluk → Tam
    Sözlük.uzunluk → Tam

Metnin karakter sayısını, listenin ya da sözlüğün eleman sayısını verir. Parantezsiz yazılan bir özelliktir.

```tr
yazdır("çay".uzunluk, [1, 2, 3].uzunluk, {"a": 1}.uzunluk)
```

#### TR_mem_load_state `durumu_yükle`

    İyileştirici.durumu_yükle(Sözlük) → Mantık
    Örnekleyici.durumu_yükle(Sözlük) → Mantık
    SıralıOkuyucu.durumu_yükle(Sözlük) → Mantık

durum ile alınmış sözlüğü aynı ayarla kurulmuş bir iyileştiriciye ya da okuyucuya yükler ve doğru verir; tensörler parametrenin cihazına taşınır. Parametre sayısı ve şekiller, okuyucunun türü, toplu, bağlam, tohum ve veri kümesi aynı olmalıdır, değilse hata verir.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken op = AdamW([w]); geri_yayılım(toplam(w * 2)); op.adım(); değişken yeni = AdamW([w]); yazdır(yeni.durumu_yükle(op.durum))
```

İlişkili: TR_mem_state `durum`

#### TR_mem_logger `günlük`



#### TR_mem_logit_layer `logit_katmanı`



#### TR_mem_logits `logit`



#### TR_mem_loss `kayıp`



#### TR_mem_losses `kayıplar`



#### TR_mem_lower `küçük_harf`

    Metin.küçük_harf() → Metin

Metni Unicode'un varsayılan kuralıyla küçük harfe çevirir; sonuç programın dilinden bağımsızdır (I → i). Yeni bir metin verir.

```tr
yazdır("MERHABA".küçük_harf())
```

İlişkili: TR_mem_upper `büyük_harf`

#### TR_mem_lr `öğrenme_oranı`

    İyileştirici.öğrenme_oranı → Ondalık

İyileştiricinin (AdamW, SGD) şimdiki öğrenme oranıdır (Ondalık); okunur ve atanabilir: op.öğrenme_oranı = 0.0005 ataması sonraki adımdan başlayarak yeni oranı kullanır (ısınma, düşürme gibi öğrenme oranı planları). Atanan değer sonlu ve negatif olmayan bir Tam ya da Ondalık olmalıdır; oran 32 bit saklanır. Ayar olduğu için duruma (durum) girmez.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken op = SGD([w], öğrenme_oranı: 0.5); op.öğrenme_oranı = 0.25; geri_yayılım(toplam(w * w)); op.adım(); yazdır(op.öğrenme_oranı, w.değer)
```

İlişkili: TR_arg_lr `öğrenme_oranı`, TR_mem_step `adım`

#### TR_mem_lr_multiplier `öğrenme_oranı_çarpanı`



#### TR_mem_map `haritala`

    Liste.haritala(Fonksiyon) → Liste

Her elemana fonksiyonu uygular ve sonuçların yeni listesini verir. Asıl liste değişmez; fonksiyon sondaki blokla da yazılır.

```tr
yazdır([1, 2, 3].haritala { o * 10 })
```

İlişkili: TR_mem_filter `filtrele`, TR_mem_reduce `indirge`

#### TR_mem_mask `maske`



#### TR_mem_masks `maskeler`



#### TR_mem_momentum `momentum`



#### TR_mem_muon `muon`



#### TR_mem_muon_lr `muon_öğrenme_oranı`



#### TR_mem_name `dosya_adı`

    Yol.dosya_adı → Metin

Yolun son bileşenini verir ("veri/derlem.tar.gz" → "derlem.tar.gz"); . ve kök için boş metindir.

```tr
yazdır(yol("veri/derlem.tar.gz").dosya_adı)
```

İlişkili: TR_mem_extension `uzantı`, TR_mem_stem `uzantısız_ad`

#### TR_mem_nesterov `nesterov`



#### TR_mem_next `sonraki`

    SatırOkuyucu.sonraki() → Metin | Yok
    Örnekleyici.sonraki() → Liste
    SıralıOkuyucu.sonraki() → Liste | Yok

Örnekleyici ve sıralı okuyucuda bir sonraki toplu işi [girdi, hedef] tensörleri olarak (ikisi de [toplu, bağlam] şeklinde, hedef bir parça kaydırılmış), SatırOkuyucu'da sıradaki satırı Metin olarak verir. Sıralı okuyucu veri, SatırOkuyucu dosya bitince yok verir.

```tr
değişken girdi, hedef = veri_kümesi("tests/data/numbers").sıralı(toplu: 2, bağlam: 4).sonraki(); yazdır(girdi.değer, hedef.değer)
```

İlişkili: TR_mem_sampler `örnekleyici`, TR_mem_sequential `sıralı`, TR_fn_read_lines `dosya_satırları`

#### TR_mem_norm `norm`



#### TR_mem_norms `normlar`



#### TR_mem_ns_dtype `ns_türü`



#### TR_mem_ns_iterations `ns_yineleme_sayısı`



#### TR_mem_num_heads `bakış_sayısı`



#### TR_mem_num_kv_heads `kv_bakış_sayısı`



#### TR_mem_num_layers `katman_sayısı`



#### TR_mem_o_proj `çıkış_katmanı`



#### TR_mem_observe `gözle`



#### TR_mem_other `diğer`



#### TR_mem_parameters `parametreler`



#### TR_mem_parent `üst_klasör`

    Yol.üst_klasör → Yol

Yolun bulunduğu klasörü Yol olarak verir; tek bileşenli göreli yolda . olur.

```tr
yazdır(yol("veri/eğitim/a.npy").üst_klasör)
```

İlişkili: TR_mem_name `dosya_adı`

#### TR_mem_permute `eksenleri_sırala`

    Tensör.eksenleri_sırala(Liste | Tam, [Tam...]) → Tensör

Tensörün eksenlerini verilen sıraya dizer. Her eksen tam olarak bir kez yer almalıdır.

```tr
yazdır(sıfırlar([2, 3, 4]).eksenleri_sırala([2, 0, 1]).şekil)
```

İlişkili: TR_mem_transpose `devrik`

#### TR_mem_ple `ple`



#### TR_mem_ple_context `ple_bağlamı`



#### TR_mem_ple_dim `ple_boyutu`



#### TR_mem_position `konum`



#### TR_mem_positions `konumlar`



#### TR_mem_post_attention_norm `norm2`



#### TR_mem_proj `izdüşüm_katmanı`



#### TR_mem_q_norm `q_norm`



#### TR_mem_q_proj `sorgu_katmanı`



#### TR_mem_reduce `indirge`

    Liste.indirge([Herhangi], Fonksiyon) → Herhangi

Listeyi soldan sağa (birikim, eleman) alan bir fonksiyonla tek değere indirger; başlangıç verilmezse ilk eleman başlangıçtır, boş listede başlangıç (yoksa yok) döner. Fonksiyon son argümandır ve iki parametre alır; sondaki blokla yazılınca birikim önceki sonuç, o eleman olur.

```tr
yazdır([1, 2, 3, 4].indirge(0, (a, x) => a + x), [1, 2, 3, 4].indirge(1) { birikim * o })
```

İlişkili: TR_mem_filter `filtrele`, TR_mem_map `haritala`

#### TR_mem_replace `değiştir`

    Metin.değiştir(Metin, Metin) → Metin

Metindeki bütün eski alt metinleri yenisiyle değiştirir. İki argüman da Metin olmalıdır; yeni bir metin döner.

```tr
yazdır("elma armut elma".değiştir("elma", "erik"))
```

#### TR_mem_reshape `şekillendir`

    Tensör.şekillendir(Liste) → Tensör

Tensörü aynı eleman sayısıyla yeni bir şekle sokar; bir eksen -1 verilirse uzunluğu hesaplanır. Sonuç veriyi paylaşan bir görünümdür.

```tr
yazdır(tensör([1, 2, 3, 4, 5, 6]).şekillendir([2, 3]).şekil)
```

İlişkili: TR_mem_shape `şekil`

#### TR_mem_resume_step `kaldığı_adım`

    KontrolNoktası.kaldığı_adım → Tam

Kurulurken yüklenen kontrol noktasının adımıdır (tamamlanan adım sayısı); yeni bir koşuda 0. Eğitim döngüsü buradan başlar: döngü adım içinde kn.kaldığı_adım..toplam.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken kn = kontrol_noktası("koşu_y520", {w: w}); yazdır(kn.kaldığı_adım)
```

İlişkili: TR_mem_save `kaydet`, TR_mem_update `güncelle`

#### TR_mem_reverse `ters_çevir`

    Liste.ters_çevir() → Liste
    Metin.ters_çevir() → Metin

Metnin karakterlerini ya da listenin elemanlarını ters sırayla verir. Asıl değeri değiştirmez, yeni metin ya da liste döner.

```tr
yazdır("kitap".ters_çevir(), [1, 2, 3].ters_çevir())
```

İlişkili: TR_mem_sort `sırala`

#### TR_mem_rollback `geri_dön`

    KontrolNoktası.geri_dön() → Tam

En yeni geçerli kontrol noktasını aynı programda yerinde yükler (parametreler, iyileştiriciler, okuyucular, düz değerler; günlüklere devam kaydı yazılır) ve noktanın adımını Tam olarak verir; eğitim döngüsü o adımdan sürer. Kayıp NaN olunca ya da sıçrayınca son sağlam noktaya dönmek içindir. Nokta yoksa hatadır.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken kn = kontrol_noktası("koşu_y522", {w: w}); kn.güncelle(1); kn.kaydet(); yazdır(kn.geri_dön())
```

İlişkili: TR_mem_save `kaydet`, TR_mem_resume_step `kaldığı_adım`, TR_fn_is_finite `sonlu_mu`

#### TR_mem_row_rms `satır_rms`



#### TR_mem_run_blocks `blokları_işle`



#### TR_mem_sampler `örnekleyici`

    VeriKümesi.örnekleyici([Tam], [Tam], [Tam], toplu: Tam, bağlam: Tam, tohum: Tam = 0) → Örnekleyici

Veri kümesinden rastgele pencereler alan bir örnekleyici kurar; toplu ve bağlam zorunludur, tohum isteğe bağlıdır. Örnekleyici sonsuzdur, dönemler bitince baştan sürer.

```tr
yazdır(veri_kümesi("tests/data/numbers").örnekleyici(toplu: 2, bağlam: 4, tohum: 1))
```

İlişkili: TR_mem_sequential `sıralı`, TR_mem_next `sonraki`

#### TR_mem_save `kaydet`

    KontrolNoktası.kaydet() → Yol

Kontrol noktasını hemen, son güncelle(adım) adımıyla kaydeder ve noktanın klasörünü Yol olarak verir (ör. eğitim sonunda). Yazma hatası açık hatadır; aralıkla kayıttaki gibi eğitim sürmez.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken kn = kontrol_noktası("koşu_y521", {w: w}); kn.güncelle(1); yazdır(kn.kaydet())
```

İlişkili: TR_mem_resume_step `kaldığı_adım`, TR_mem_update `güncelle`

#### TR_mem_scales `ölçekler`



#### TR_mem_sequential `sıralı`

    VeriKümesi.sıralı([Tam], [Tam], toplu: Tam, bağlam: Tam) → SıralıOkuyucu

Veri kümesini baştan sona sırayla okuyan bir okuyucu kurar; toplu ve bağlam zorunludur. Veri bitince sonraki() yok verir.

```tr
yazdır(veri_kümesi("tests/data/numbers").sıralı(toplu: 2, bağlam: 4).sonraki()[0].şekil)
```

İlişkili: TR_mem_sampler `örnekleyici`, TR_mem_next `sonraki`

#### TR_mem_set_multiplier `çarpan_ayarla`



#### TR_mem_shape `şekil`

    Tensör.şekil → Liste

Tensörün her eksendeki uzunluğunu Tam listesi olarak verir; 0 boyutlu tensörde boş listedir.

```tr
yazdır(tensör([[1, 2], [3, 4]]).şekil)
```

İlişkili: TR_mem_reshape `şekillendir`

#### TR_mem_sin_table `sin_tablosu`



#### TR_mem_single_step `tek_adım`



#### TR_mem_size `bayt`

    Yol.bayt → Tam

Dosyanın boyutunu bayt olarak Tam verir (.gz dosyada sıkıştırılmış boyut); klasörde ve olmayan dosyada hata verir.

```tr
dosya_yaz("boyut.txt", "merhaba"); yazdır(yol("boyut.txt").bayt)
```

#### TR_mem_skip `atla`

    Örnekleyici.atla(Tam) → Tam
    SıralıOkuyucu.atla(Tam) → Tam

Sıradaki n toplu işi tensör kurmadan atlar ve atladığı sayıyı Tam olarak verir; sonraki .sonraki() n tane sonraki() çağrısından sonra gelecek toplu işi verir, okuyucunun durumu (sayaç) n artar. Sıralı okuyucu biterse kalan kadar atlar. Kayıp sıçramasından sonra son kontrol noktasına dönen eğitim, sıçramayı tetikleyen toplu işleri böyle atlar.

```tr
değişken ö = veri_kümesi("tests/data/text").örnekleyici(toplu: 2, bağlam: 8, tohum: 1); yazdır(ö.atla(3), ö.durum["counter"])
```

İlişkili: TR_mem_next `sonraki`, TR_mem_state `durum`, TR_mem_rollback `geri_dön`

#### TR_mem_slice `dilim`

    Tensör.dilim(Tam, Tam, Tam) → Tensör

Verilen eksende [baş, son) aralığını keser; negatif konumlar sondan sayılır. Sonuç veriyi paylaşan bir görünümdür.

```tr
yazdır(tensör([[1, 2, 3], [4, 5, 6]]).dilim(1, 0, 2).değer)
```

#### TR_mem_sort `sırala`

    Liste.sırala() → Liste

Listenin küçükten büyüğe sıralanmış yeni bir kopyasını verir. Liste yalnızca sayılardan ya da yalnızca metinlerden oluşmalıdır.

```tr
yazdır([3, 1, 2].sırala())
```

İlişkili: TR_mem_reverse `ters_çevir`

#### TR_mem_sparse_lr `seyrek_öğrenme_oranı`



#### TR_mem_spike_count `sıçrama_sayısı`



#### TR_mem_split `böl`

    Metin.böl([Metin]) → Liste
    Tensör.böl(Tam, [Tam], eksen: Tam = -1) → Liste

Metni ayırıcıdan böler ve parçaların listesini verir (ayırıcı verilmezse tek boşluk). Tensörü bir eksende (varsayılan son eksen) eşit sayıda parçaya böler; eksen uzunluğu parça sayısına tam bölünmelidir.

```tr
yazdır("a,b,c".böl(","), tensör([1, 2, 3, 4]).böl(2)[0].değer)
```

İlişkili: TR_mem_join `birleştir`, TR_fn_concat `bitiştir`

#### TR_mem_starts_with `başlar`

    Metin.başlar(Metin) → Mantık

Metin verilen önekle başlıyorsa doğru verir. Argüman Metin olmalıdır.

```tr
yazdır("merhaba".başlar("mer"))
```

İlişkili: TR_mem_ends_with `biter`, TR_mem_contains `içerir`

#### TR_mem_state `durum`

    İyileştirici.durum → Sözlük
    Örnekleyici.durum → Sözlük
    SıralıOkuyucu.durum → Sözlük

Kaldığı yerden sürdürmek için gereken durumdur: iyileştiricide ad → tensör sözlüğü (momentler, adım sayaçları, ana kopyalar; kopyadır, tensör_kaydet ile yazılabilir), okuyucularda düz değerli bir sözlük (tür, toplu, bağlam, tohum, küme, dönem, sıra, pencere, sayaç). Öğrenme oranı gibi ayarlar duruma girmez.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken op = AdamW([w]); geri_yayılım(toplam(w * 2)); op.adım(); yazdır(op.durum.anahtarlar())
```

İlişkili: TR_mem_load_state `durumu_yükle`, TR_fn_checkpoint `kontrol_noktası`

#### TR_mem_stem `uzantısız_ad`

    Yol.uzantısız_ad → Metin

Dosya adının son uzantısı atılmış hâlini verir ("derlem.tar.gz" → "derlem.tar"); uzantısız_ad + uzantı her zaman dosya_adı'dır.

```tr
yazdır(yol("veri/derlem.tar.gz").uzantısız_ad)
```

İlişkili: TR_mem_extension `uzantı`

#### TR_mem_step `adım`

    İyileştirici.adım() → Yok

İyileştiricinin (AdamW, SGD) bir güncelleme adımını yapar: parametreleri gradyanlarına göre yerinde günceller. Argüman almaz, yok döner.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken op = SGD([w], öğrenme_oranı: 0.1); geri_yayılım(toplam(w * 2)); op.adım(); yazdır(w.değer)
```

İlişkili: TR_mem_zero_grad `sıfırla`, TR_fn_backward `geri_yayılım`

#### TR_mem_step_done `adım_bitti`



#### TR_mem_step_position `adım_konumu`



#### TR_mem_stop `durdur`

    İlerleme.durdur() → Mantık

Çubuğu bitirir ve geçen süreyle son satırı yazar; ilk çağrıda doğru, sonrakilerde yanlış verir. Toplama ulaşınca ve program bitince kendiliğinden çağrılır.

```tr
değişken ç = ilerleme(); ç.güncelle(); yazdır(ç.durdur())
```

İlişkili: TR_mem_update `güncelle`

#### TR_mem_strip `kırp`

    Metin.kırp() → Metin

Metnin başındaki ve sonundaki boşlukları (satır sonları dahil) atar. Yeni bir metin verir.

```tr
yazdır("  merhaba  ".kırp())
```

#### TR_mem_table `tablo`



#### TR_mem_threshold `eşik`



#### TR_mem_to_device `cihaza`

    Tensör.cihaza(Metin) → Tensör

Tensörü verilen cihaza ("cpu", "gpu", "gpu:1") taşınmış olarak verir; kullanılabilir cihazlar cihazlar() ile görülür, olmayan cihaz hatadır. Türev izleyen bir yaprak taşınınca sonuç, hedef cihazda türev izleyen yeni bir yapraktır (gradyanı kendi .gradyan'ında birikir); bir işlemin sonucu taşınınca türev kaynağa geri akar.

```tr
yazdır(tensör([1, 2]).cihaza("cpu").cihaz)
```

İlişkili: TR_mem_device `cihaz`, TR_fn_default_device `varsayılan_cihaz`, TR_fn_devices `cihazlar`

#### TR_mem_to_dtype `türe`

    Tensör.türe(Metin) → Tensör

Tensörün "float32", "bf16" ya da "fp16" türüne çevrilmiş kopyasını verir; gradyan kaynak tensöre akar. Tür zaten aynıysa tensörün kendisi döner.

```tr
yazdır(tensör([1.0, 2.0]).türe("bf16").tür)
```

İlişkili: TR_mem_dtype `tür`, TR_fn_mixed_precision `karışık_hassasiyet`

#### TR_mem_token_id `parça_kimliği`

    Parçalayıcı.parça_kimliği(Metin) → Tam | Yok

Verilen parçanın dağarcıktaki kimliğini verir; parça dağarcıkta yoksa yok döner. Argüman Metin olmalıdır.

```tr
yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").parça_kimliği("<|endoftext|>"))
```

İlişkili: TR_mem_vocab_size `dağarcık_boyutu`

#### TR_mem_tolist `değer`

    Tensör.değer → Liste | Ondalık | Tam

Tensörün değerlerini iç içe liste olarak verir; 0 boyutlu tensörde tek bir sayı döner. Sonuç kopyadır, değiştirilmesi tensörü etkilemez.

```tr
yazdır(tensör([[1, 2], [3, 4]]).değer)
```

#### TR_mem_transpose `devrik`

    Tensör.devrik() → Tensör

Tensörün son iki eksenini yer değiştirir. Argüman almaz ve en az 2 boyutlu tensör ister.

```tr
yazdır(tensör([[1, 2], [3, 4]]).devrik().değer)
```

İlişkili: TR_mem_permute `eksenleri_sırala`

#### TR_mem_truncate `kısalt`



#### TR_mem_unique `benzersiz`

    Liste.benzersiz() → Liste

Tekrar eden elemanları atar, her değerin ilk geçtiği sırayı korur. Yeni bir liste verir.

```tr
yazdır([1, 2, 2, 3, 1].benzersiz())
```

#### TR_mem_up_proj `yukarı`



#### TR_mem_update `güncelle`

    KontrolNoktası.güncelle(Tam) → Mantık
    İlerleme.güncelle([Sözlük], adım: Tam = yok) → Mantık

Her çağrı eğitimin bir adım sınırıdır. İlerleme'de çubuğu bir adım ilerletir, verilen ölçüleri (kayıp, öğrenme oranı ...) gösterir, adım: ile tamamlanan adım sayısını o değere getirir ve toplama ulaşınca çubuğu bitirir. KontrolNoktası'nda güncelle(adım) tamamlanan adım sayısını bildirir; her_dakika süresi dolduysa ya da durdurma istendiyse (Ctrl+C) burada kaydeder, kaydettiyse doğru verir.

```tr
değişken ç = ilerleme(10); ç.güncelle({kayıp: 0.5}, adım: 5); yazdır(ç)
```

İlişkili: TR_mem_stop `durdur`, TR_mem_save `kaydet`

#### TR_mem_upper `büyük_harf`

    Metin.büyük_harf() → Metin

Metni Unicode'un varsayılan kuralıyla büyük harfe çevirir; sonuç programın dilinden bağımsızdır (i → I). Yeni bir metin verir.

```tr
yazdır("merhaba".büyük_harf())
```

İlişkili: TR_mem_lower `küçük_harf`

#### TR_mem_v_buffers `v_tamponları`



#### TR_mem_v_proj `değer_katmanı`



#### TR_mem_values `değerler`

    Sözlük.değerler() → Liste

Sözlüğün değerlerini anahtarların ekleniş sırasıyla bir liste olarak verir.

```tr
yazdır({"ad": "Ali", "puan": 30}.değerler())
```

İlişkili: TR_mem_keys `anahtarlar`

#### TR_mem_vocab_size `dağarcık_boyutu`

    Parçalayıcı.dağarcık_boyutu → Tam

Parçalayıcının dağarcığındaki parça sayısını (eklenmiş özel parçalar dahil) Tam olarak verir. Parantezsiz yazılan bir özelliktir.

```tr
yazdır(parçalayıcı_yükle("tests/data/tokenizer.json").dağarcık_boyutu)
```

İlişkili: TR_mem_token_id `parça_kimliği`

#### TR_mem_weight `ağırlık`



#### TR_mem_weight_decay `ağırlık_çürümesi`



#### TR_mem_window `pencere`



#### TR_mem_zero_grad `sıfırla`

    İyileştirici.sıfırla() → Yok

İyileştiricinin parametrelerindeki biriken gradyanları sıfırlar. Bir sonraki geri_yayılım'dan önce çağrılır; argüman almaz.

```tr
değişken w = tensör([1.0], türev_izle: doğru); geri_yayılım(toplam(w * 2)); AdamW([w]).sıfırla(); yazdır(w.gradyan.değer)
```

İlişkili: TR_mem_step `adım`

## Komutlar (tr, kabuk, hata ayıklayıcı)

### tr komutları

#### TR_cmd_build `derle`

Programı yorumlayıcıyla birlikte tek bir çalıştırılabilir dosyaya paketler.

```tr
tr derle eğit.tr
```

#### TR_cmd_check `kontrol`

Programı çalıştırmadan ayrıştırır ve tip denetiminden geçirir.

```tr
tr kontrol eğit.tr
```

#### TR_cmd_dap `dap`

Düzenleyicinin hata ayıklama protokolü (DAP) sunucusunu başlatır; düzenleyiciler kullanır.

```tr
tr dap
```

#### TR_cmd_debug `hata_ayıkla`

Programı terminal hata ayıklayıcısında başlatır.

```tr
tr hata_ayıkla eğit.tr
```

#### TR_cmd_encode `kodla`

Dosyayı dilden bağımsız kodlu biçime çevirir.

```tr
tr kodla eğit.tr
```

#### TR_cmd_format `biçimle`

TR dosyalarını standart biçime getirir.

```tr
tr biçimle eğit.tr
```

#### TR_cmd_help `yardım`

Komutların ve seçeneklerin kısa yardımını yazar.

```tr
tr yardım
```

#### TR_cmd_index `dizin`

TR kodunun dizinini kurar ve ona sorar. Tek başına dizini kurar ve özetler; altında tanım, kullananlar, sınayanlar ve etki alt komutları vardır.

```tr
tr dizin
```

#### TR_cmd_profile `profil`

Biten ya da süren bir eğitimin profil raporunu yazar.

```tr
tr profil
```

#### TR_cmd_run `çalıştır`

Bir TR programını çalıştırır; komut adı yazılmadan verilen dosya da çalıştırılır.

```tr
tr çalıştır eğit.tr
```

#### TR_cmd_show `göster`

Kodlu dosyayı değiştirmeden etkin yazım dilinde yazar.

```tr
tr göster eğit.tr
```

#### TR_cmd_test `test`

Bir dosyadaki ya da klasördeki *_test.tr testlerini koşar.

```tr
tr test tests
```

#### TR_cmd_version `sürüm`

TR'nin sürümünü yazar.

```tr
tr sürüm
```

#### TR_cmd_watch `izle`

Çalışan bir eğitimin canlı profil tablosunu gösterir.

```tr
tr izle
```

### tr dizin alt komutları

#### TR_sub_definition `tanım`

Bir adın tanımını gösterir: nerede, imzası, belge yorumu, üyeleri (tr dizin altında).

```tr
tr dizin tanım Muon
```

#### TR_sub_impact `etki`

Bir ad ya da dosya değişirse etkilenenleri gösterir: dışarıdan kullananlar, onu aktaran modüller, sınayan testler (tr dizin altında).

```tr
tr dizin etki lib/katman.tr
```

#### TR_sub_references `kullananlar`

Bir ada başvuranları gösterir: çağrı, okuma, yazma, aktarma, test; dosya:satır:sütun ve içeren bildirimle (tr dizin altında).

```tr
tr dizin kullananlar Doğrusal
```

#### TR_sub_rename `yeniden_adlandır`

Bir ad değişirse değişecek yerleri listeler, hiçbir dosyayı değiştirmez: kesin (tanım, parametre, adlı argüman, kendi.x ve yapıcıdan bilinen yerelin üye erişimi, komut_satırı anahtarı) ve olası (alıcısı bilinmeyen üye erişimi, belge, yorum, ileti metni) olarak ikiye ayrılır (tr dizin altında).

```tr
tr dizin yeniden_adlandır baş bakış_sayısı
```

#### TR_sub_tested_by `sınayanlar`

Bir adı sınayan test bloklarını gösterir: önce doğrudan, sonra çağrı zinciriyle dolaylı olanlar (tr dizin altında).

```tr
tr dizin sınayanlar çevre_hesapla
```

### Komut satırı seçenekleri

#### TR_opt_all `tümü`

Dizin sorgularında satır sınırını kaldırır.

```tr
tr dizin kullananlar Muon --tümü
```

#### TR_opt_check `kontrol`

Dosyayı değiştirmeden biçiminin standart olup olmadığını denetler (tr biçimle).

```tr
tr biçimle --kontrol eğit.tr
```

#### TR_opt_clear `temizle`

Dizin komutunda dizin önbelleğini siler.

```tr
tr dizin --temizle
```

#### TR_opt_go_profile `go_profili`

Yorumlayıcının kendi Go profilini de yazar (tr profil).

```tr
tr profil --go-profili go.prof eğit.tr
```

#### TR_opt_help `yardım`

Programın komut satırı seçeneklerinin yardımını yazar (komut_satırı ile tanımlanan seçenekler).

```tr
tr eğit.tr --yardım
```

#### TR_opt_json `json`

Profil raporunu JSON olarak yazar (tr profil).

```tr
tr profil --json
```

#### TR_opt_kind `tür`

Sorgu adını yalnızca bu türdeki düğümlere çözer (function, struct, method, field, constant, variable, test, module, builtin).

```tr
tr dizin tanım ileri --tür method
```

#### TR_opt_lang `dil`

Yazım dilini seçer (tr, en); --dil biçiminde yazılır.

```tr
tr göster --dil en eğit.tr
```

#### TR_opt_limit `sınır`

Dizin sorgularında bir listedeki en çok satır sayısını verir (varsayılan 12).

```tr
tr dizin kullananlar Muon --sınır 5
```

#### TR_opt_no_index `dizin_yok`

Dizini kurma ya da tazeleme, önbellekteki dizini olduğu gibi kullan (dizin sorgularında).

```tr
tr dizin tanım Muon --dizin-yok
```

#### TR_opt_no_text `metin_yok`

Yeniden adlandırma etkisinde belge, yorum ve ileti metni taramasını kapatır; yalnızca kodun anlamsal yerleri listelenir.

```tr
tr dizin yeniden_adlandır baş bakış_sayısı --metin-yok
```

#### TR_opt_output `çıktı`

Canlı tablonun yazılacağı dosyayı seçer (tr izle).

```tr
tr izle --çıktı tablo.txt
```

#### TR_opt_root `kök`

Proje kökünü verir; verilmezse .git bulunan üst klasör, o da yoksa çalışılan klasör alınır.

```tr
tr dizin tanım Muon --kök .
```

#### TR_opt_status `durum`

Dizin komutunda dizini kurmadan durumunu gösterir.

```tr
tr dizin --durum
```

#### TR_opt_term `terim`

Yeniden adlandırma etkisinde sembol aramaz, verilen sözcüğü (ve Türkçe ekli biçimlerini) yalnızca metinde arar: bir terimin çevirisi değişince belge, yorum ve ileti metinlerini listelemek için. Sözcüğün birden çok anlamı varsa araç ayıramaz, her satırın bağlamı gösterilir.

```tr
tr dizin yeniden_adlandır çekirdek kernel --terim
```

### Kabuk komutları

#### TR_repl_exit `çıkış`

Etkileşimli kabuktan çıkar.

```tr
çıkış
```

### Hata ayıklayıcı komutları

#### TR_dbg_break `kesme`

Bir satıra kesme noktası koyar ya da kaldırır.

```tr
kesme 12
```

#### TR_dbg_continue `devam`

Sonraki kesme ya da izleme noktasına kadar devam eder (F5).

```tr
devam
```

#### TR_dbg_eval `hesapla`

Bir ifadeyi o anki ortamda hesaplar ve yazar.

```tr
hesapla x * 2
```

#### TR_dbg_help `yardım`

Hata ayıklayıcı komutlarının yardımını yazar.

```tr
yardım
```

#### TR_dbg_jump `zıpla`

Çalışma sırasını verilen satıra taşır.

```tr
zıpla 15
```

#### TR_dbg_next `geç`

Üstten geçer: fonksiyonu tek adımda çalıştırır (F10).

```tr
geç
```

#### TR_dbg_out `dön`

Çalışan fonksiyondan çıkana kadar çalıştırır (Shift+F11).

```tr
dön
```

#### TR_dbg_quit `çıkış`

Hata ayıklayıcıdan ve programdan çıkar.

```tr
çıkış
```

#### TR_dbg_screen `ekran`

Kaynak kodun duraklanan satır çevresini gösterir.

```tr
ekran
```

#### TR_dbg_set `değiştir`

Çalışma anında bir değişkenin değerini değiştirir.

```tr
değiştir x = 100
```

#### TR_dbg_stack `yığın`

Çağrı yığınını gösterir.

```tr
yığın
```

#### TR_dbg_step `adım`

Tek adım ilerler; fonksiyonların içine girer (F11).

```tr
adım
```

#### TR_dbg_table `tablo`

Değişkenlerin tablosunu gösterir.

```tr
tablo
```

#### TR_dbg_watch `izle`

Bir ifadeyi izleme listesine ekler; değeri her durakta gösterilir.

```tr
izle kayıp
```

## Boru hattıyla kullanılabilen fonksiyonlar

`x |> f(a)` ≡ `f(x, a)` (bkz. `docs/spec/04_operatorler.md` 4.9). Boru yalnızca fonksiyonlarla çalışır; metotlar `.` ile zincirlenir (`l.filtrele { o > 1 }.haritala { o * 2 } |> toplam`). Borudaki değeri ilk konumlu argüman olarak alabilen yerleşik fonksiyonlar (kullanıcı fonksiyonları da kullanılabilir):

`AdamW`, `SGD`, `mutlak`, `acos`, `argmax`, `sıralama_indeksi`, `asin`, `doğrula`, `eşit_mi`, `atan`, `atan2`, `dikkat`, `geri_yayılım`, `tavan`, `kontrol_noktası`, `sınırla`, `gradyan_kırp`, `komut_satırı`, `bitiştir`, `kopyala`, `cos`, `cosh`, `çapraz_entropi`, `birikimli_toplam`, `veri_kümesi`, `varsayılan_cihaz`, `gömme`, `ortam_değişkeni`, `hata_kodu`, `sonlandır`, `exp`, `expm1`, `birim_matris`, `ondalık`, `taban`, `kaynaştır`, `gelu`, `türev_denetle`, `girdi`, `tam`, `sonlu_mu`, `json_çöz`, `json_metin`, `katman_normu`, `doğrusal_çapraz_entropi`, `klasörü_listele`, `parametreleri_yükle`, `tensör_yükle`, `parçalayıcı_yükle`, `log`, `log10`, `log1p`, `log2`, `log_softmax`, `günlük`, `klasör_oluştur`, `max`, `ortalama`, `ortanca`, `ileti`, `min`, `karışık_hassasiyet`, `taşı`, `türevsiz`, `birler`, `parametreler`, `csv_çöz`, `yol`, `yazdır`, `ilerleme`, `rastgele_normal`, `aralık`, `csv_oku`, `dosya_oku`, `dosya_satırları`, `relu`, `sil`, `tekrar_oynat`, `rms_normu`, `yuvarla`, `örnekle`, `tensör_kaydet`, `sigmoid`, `işaret`, `silu`, `sin`, `sinh`, `bekle`, `softmax`, `karekök`, `standart_sapma`, `metin`, `toplam`, `tan`, `tanh`, `tensör`, `en_büyük_k`, `tip`, `koşullu`, `dosya_yaz`, `yerine_yaz`, `sıfırlar`
