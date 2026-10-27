# 3. Tipler ve değerler

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "3. Tipler ve değerler".

Her kuralın emsali ve gerekçesi: [`rationale/03_types.md`](rationale/03_types.md).

TR statik tiplidir (1.2): her değerin tipi çalışmadan önce bilinir. Tip denetimi 3.13'te, değerin metni 3.14'tedir.

**3.1** `TR_Int`: 64 bit işaretli tam sayı (−9223372036854775808 … 9223372036854775807). İşlemin ya da yerleşiğin (`TR_sum`, `TR_abs`) sonucu aralığı aşarsa sarılmaz, "tam sayı taşması" çalışma hatası verir.
- *Test:* TR testi.

**3.2** `TR_Float`: IEEE 754 64 bit. Metne, geri okununca aynı sayıyı veren en kısa rakamlarla çevrilir: tam değerli ondalık bir sıfırla ("1.0", "1234567.0"); üssü −4'ten küçük ya da 16 ve üstüyse bilimsel ("1e-05", "1e+16"; "0.0001", "1000000000000000.0" olduğu gibi). Sonsuz ve tanımsız `TR_inf` ve `TR_nan`dır. 32 ve 16 bit ve nicemlenmiş tipler yalnız tensörün öğe tipidir (sonraki bölümlerde).
- *Test:* TR testi; yazım eşiği uygulanınca TR testi.

**3.3** `TR_String`: değiştirilemez Unicode metni; uzunluk ve indeks kod noktası sayar. Negatif indeks sondan sayar; sınır dışı indeks çalışma hatasıdır, `.TR_get(i)` isteğe bağlı değer verir (3.5). `m[a..b]` alt metin verir; dilimde negatif uç sondan sayılır, uçlar sınıra kırpılır, başlangıç bitişten büyükse sonuç boştur (`"şeker"[-3..5]` → "ker").
- *Test:* TR testi.

```tr
//! TR 4
TR_print("şeker".TR_length, "şeker"[-1], "şeker"[1..3])   // 5 r ek
```

**3.3a** `TR_Bytes`: değiştirilemez bayt dizisi (dosya, ağ, görüntü, ses verisi). `b[i]` 0–255 arası `TR_Int`, `b[a..c]` `TR_Bytes` verir; indeks ve dilim 3.3'teki gibidir. `TR_Bytes("ab")` metnin UTF-8 baytlarını, `TR_Bytes([0, 255])` sayıları verir; 0–255 dışı sayı hatadır. `TR_String(b)` UTF-8 okur; geçersiz UTF-8 hatadır. Metin ile bayt kendiliğinden dönüşmez.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**3.4** `TR_Bool`: `TR_true` ve `TR_false`; koşulda yalnız bu tip kullanılır (3.10).
- *Test:* TR testi.

**3.5** `TR_none` yokluktur, `TR_NoneType`'ın tek değeridir. Değişken, parametre, alan ya da dönüş ancak tipi isteğe bağlıysa (`TR_Int?`) `TR_none` alabilir. İsteğe bağlı değer asıl tipin yerinde kullanılamaz; `??` ile varsayılan verilerek, `?.` ile ya da `TR_none`'la karşılaştırılarak açılır: `x != TR_none` sınamasının dalında `x` asıl tipindedir. Değer döndürmeyen fonksiyonun dönüş tipi `TR_NoneType`'tır.
- *Test:* TR testi; isteğe bağlı tip uygulanınca denetleyicinin testi.

```tr
//! TR 4
TR_var l: TR_List[TR_Int] = [10, 20, 30]
TR_var x: TR_Int? = l.TR_get(5)
TR_print(x ?? 0)                 // çıktı: 0
TR_if x != TR_none { TR_print(x + 1) }   // bu dalda x bir TR_Int
```

**3.6** `TR_List[T]`: tek tipli, sıralı, değiştirilebilir dizi; `[1, 2, 3]` bir `TR_List[TR_Int]`dir, boş liste tipini yazar (`TR_var l: TR_List[TR_Int] = []`). Negatif indeks sondan sayar; sınır dışı okuma ve yazma çalışma hatasıdır, ileti indeksi ve uzunluğu söyler; `.TR_get(i)` `T?` verir. Dilim 3.3'teki gibidir. Liste, sözlük ve yapı nesnesi başvuruyla paylaşılır: `b = a`'dan sonra ikisi aynı listedir.
- *Test:* TR testi; tek tipli liste uygulanınca denetleyicinin testi.

**3.7** `TR_Dict[K, V]`: eklenme sırasını koruyan tablo (`{"k": 1}`); anahtar tipi `TR_Int`, `TR_String` ya da `TR_Bool`. Değişmezdeki anahtar bir ifadedir: `{k: 1}`'de `k` değişkendir. Anahtar yalnız köşeli parantezle okunur ve yazılır (`d["k"]`); nokta yalnız üyeleri verir (`.TR_length`, `.TR_keys()`, `.TR_values()`, `.TR_contains(k)`, `.TR_get(k)`). Olmayan anahtarı köşeli parantezle okumak hatadır; `.TR_get(k)` `V?` verir.
- *Test:* TR testi; anahtar ve erişim kuralları uygulanınca TR testi.

**3.7a** `TR_JSON`: yapısı bilinmeyen veri; JSON'un her değerini tutar (yokluk, mantık, sayı, metin, liste, nesne). `TR_json_parse(m)` onu verir, `TR_json_string(j)` metne yazar. `j["k"]` ve `j[i]` yine `TR_JSON` verir; olmayan alan ya da indeks hatadır, `.TR_get(k)` `TR_JSON?` verir. Asıl değere tip çağrısıyla geçilir: `TR_String(j)`, `TR_Int(j)`, `TR_Float(j)`, `TR_Bool(j)`; tür tutmazsa hatadır. Her değeri tutan genel bir tip yoktur.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**3.8** `TR_Range`: `a..b` (b hariç), `a..=b` (b dahil); uçlar `TR_Int`. `a >= b` ise `a..b`, `a > b` ise `a..=b` boştur; geriye `.TR_reverse()` sayar (`(0..3).TR_reverse()` → 2, 1, 0). Sınırdaki uçlar taşmadan gezilir.
- *Test:* TR testi; boş aralık ve geriye sayma uygulanınca TR testi.

**3.9** Fonksiyon birinci sınıf değerdir: atanır, listeye konur, argüman olur, döndürülür. Tipi parametrelerinin ve dönüşünün tipleridir; yazımı sonraki bölümlerdedir.
- *Test:* TR testi.

**3.10** Koşul `TR_Bool` olmalıdır: `TR_if` ve koşullu `TR_for`'un koşulu, `TR_and`, `TR_or`, `TR_not`'un işlenenleri. Başka tip denetim hatasıdır; boşluk `.TR_length == 0`, yokluk `TR_none`'la karşılaştırma ile sorulur.
- *Test:* TR testi; mantık dışı koşulun hatası uygulanınca denetleyicinin testi.

**3.11** `TR_Type(x)` değerin tipini verir; tip değerlerinin tipi `TR_Type`'tır. Tip değerleri: dilin tipleri (`TR_Int`, `TR_Float`, `TR_String`, `TR_Bytes`, `TR_Bool`, `TR_NoneType`, `TR_JSON`, `TR_Range`, `TR_Function`, `TR_Tensor`, `TR_Type`), parametreli tipler (`TR_List[TR_Int]`, `TR_Int?`), yapılar ve kitaplığın tipleri (`TR_Optimizer`, `TR_Tokenizer`). Tip, tiple karşılaştırılır (`TR_Type(x) == TR_Int`); metinle karşılaştırma denetim hatasıdır. Matris iki boyutlu tensördür.
- *Test:* TR testi.

**3.12** Tip çağrılınca değerini kurar ya da verileni o tipe çevirir; aynı işi gören ayrı fonksiyon yoktur. `TR_Int(x)` ondalığı sıfıra doğru keser, metni tam sayı olarak okur; `TR_Float(x)` tam sayıyı ve metni ondalığa çevirir; `TR_String(x)` değerin metnini verir (3.14); kitaplık tipleri de böyle kurulur (`TR_Tensor([1, 2])`, `TR_Checkpoint("kayit")`). Çevrilemeyen değer (sayı olmayan metin, `TR_nan`, sonsuz, `TR_Int`'e sığmayan ondalık) çalışma hatasıdır. Atamada ve argümanda kendiliğinden dönüşüm yoktur; tek istisna 3.13'ün tam sayı yazımıdır.
- *Test:* TR testi.

```tr
//! TR 4
TR_print(TR_Int("42") + 1, TR_Int(3.9), TR_Int(-3.9), TR_String(12) + "a")   // 43 3 -3 12a
```

**3.13** Tip denetimi çalışmadan önce bütün programa iner (1.3): her ifadenin, atama hedefinin, varsayılanın, alanın ve metot gövdesinin tipi bilinir. Fonksiyon parametrelerinin ve dönüşünün tipini yazar (1.2); yerel değişkenin tipi ilk değerinden çıkar ve değişmez, açıklaması isteğe bağlıdır (`n: TR_Int`). Tam sayı yazımı (2.4) `TR_Float` beklenen yerde ondalıktır (`r: TR_Float = 1` → 1.0); `TR_Int` tipli bir değer `TR_Float` yerine verilmez. Tip uyuşmazlığı denetim hatasıdır (çıkış kodu 1); ileti doğru yazımı ya da dönüşümü önerir (`0.0`, `TR_Float(n)`).
- *Test:* TR testi; kalıcı tip ve tam sayı yazımı uygulanınca denetleyicinin testleri.

```tr hata
//! TR 4
TR_var toplam = 0
toplam += 0.5        // denetim hatası: toplam bir TR_Int; ilk değeri 0.0 yazın
```

**3.14** Değerin metni dilden bağımsızdır: `TR_String(x)`, `TR_print` ve yerleştirme (2.7) her makinede ve her okuyan için aynı metni verir. Tam sayı ondalık rakamla, ondalık 3.2'deki gibi yazılır. Dilin değerleri ve tipleri kimliğin adıyla yazılır: "true", "false", "none", "inf", "-inf", "nan"; tip adıyla ("Int", "List[Int]", "Int?"). Liste "[1, 2]", sözlük "{"k": 1}" biçimindedir; içlerindeki metin çift tırnakla ve 2.6'nın kaçışlarıyla yazılır. Yapı ve tensörün metni sonraki bölümlerdedir. Program çıktısı çevrilmez; kişinin dilinde görünen yalnız iletiler, kabuk ve hata ayıklayıcıdır.
- *Test:* TR testi; dilden bağımsız metin uygulanınca TR testi.

```tr
//! TR 4
TR_print(TR_true, TR_none, 1e16, TR_Type([1]), ["a"], {"k": 1})
// true none 1e+16 List[Int] ["a"] {"k": 1}
```
