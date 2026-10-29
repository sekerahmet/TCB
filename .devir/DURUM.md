# Faz 0.6a — tip yazımı

- Başlama: 2026-10-29 14:41 UTC
- Bitiş: — (ölçütler koşuluyor)

## Biten adımlar

1. **ast** (`b0ed1b4`): `ast.TypeRef` özyinelemeli: `Name`, `Token`, `Args` (tip argümanları), `Optional` (`?`), `Elems` (demet), `Params` + `Return` (fonksiyon tipi; `Return != nil` fonksiyon tipidir). `VarStatement.Type`, `Param.Type`, `StructField.Type` `*TypeRef` (yazılmamışsa nil); `FunctionLiteral`'ın `ReturnType`/`ReturnToken`/`ReturnTypes`'ı tek `ReturnType *TypeRef` oldu: birden çok değerin dönüşü (6.3) bir demet tipidir. Yorumlayıcı bu alanları okumuyor; ona dokunulmadı.
2. **ayrıştırıcı** (`68df37b`, `60573a5`): `typeAfter` bütün yazımları okur, iç içe: ad ve tip argümanları (`TR_List[T]`, `TR_Dict[K, V]`, `Kutu[T]`, `TR_Tensor[TR_Int]`), demet `(A, B)` (en az iki öğe), fonksiyon `(T1, T2) -> R`, `() -> R` (sağdan birleşir), `T?` (ad ve demetten sonra; fonksiyon tipinin `?`'si dönüşündür). Parantezden sonra `->` varsa fonksiyon tipi, yoksa demet. Fonksiyon ifadesinin dönüşü ve arayüz metodunun dönüşü de `typeAfter`'dan geçer. Tip okuyucu `parser/types.go`'ya taşındı ve ifadelerin iç içelik sınırına (1000, H0155) sayılır. Tipin içindeki her ad `RoleType` alır.
3. **denetleyici** (`af04482`, bu commit): yazılan tip asıl adına indirgenir: `TR_List[...]` → `TR_List`, `TR_Dict[...]` → `TR_Dict`, `TR_Tensor[...]` → `TR_Tensor`, `T?` → `T`, demet → Herhangi, fonksiyon tipi → `TR_Function` (bkz. Kararsız 1). İçteki her ad bugünkü gibi denetlenir (`TR_List[tam]` → H1006).
4. **biçimlendirici** (`702c01e`): `?` tipine bitişik kalır (`TR_Int ?` olmuyordu).
5. **testler** (`36805d7`, bu commit):
   - `parser/types_test.go`: `Dump` biçimli tablo (26 yazım, iç içe olanlar dahil), her konumda aynı okuyucu, 17 yanlış yazım hatası (`TR_List[`, `TR_Int??`, `-> {`...), tipin içindeki adların görevi; `context_test.go`'da iki derinlik girdisi.
   - `checker/type_names_test.go`: bileşik tipli program (değişken, parametre, alan, dönüş, `TR_none`'lu isteğe bağlı) hatasız; indirgenmiş tip ve içteki adlar bugünkü gibi hata verir.
   - `formatter/format_test.go`: bileşik tip aralıksız ve iki koşu aynı.
   - `doc_blocks_known.txt`: `docs/spec/03_tipler.md:40` ve `docs/spec/09_ertele.md:10` artık geçiyor, silindi (`-update`). `03_tipler.md:70` kalıyor: bloğun hatası tip yazımı değil, `TR_JSON`'a sözlük değişmezi (3.7a).

## Sınama (bitti ölçütü 3)

- **3 dosya, 55 bileşik tip açıklaması** (geçici klasörde, depoya konmadı): `lib/katman.tr` 29, `lib/iyilestirici.tr` 16, `lib/egitim.tr` 10. Sayı kopyalardaki `[`, `?` ya da demet içeren açıklamaların düzenli ifadeyle sayımıdır, elle verilen sayı değil.
- `lib/katman.tr`'de istenenler: `TR_shape: TR_List[TR_Int]` (`TR_init_weight`'in parametresi), `TR_bias: TR_Tensor?` (`TR_bias` parametre değil, `TR_Linear`'ın alanıdır; alana yazıldı), `TR_output_dtype: TR_String? = TR_none`. Ötekiler: `TR_Tensor[TR_Float]` parametre ve dönüşleri, `TR_Int? = TR_none` kurucu parametreleri, `-> TR_List[TR_Tensor[TR_Float]]`, `TR_Dict[TR_String, TR_List[TR_Tensor[TR_Float]]]`, `TR_Dict[TR_String, (TR_Int, TR_String)]?`.
- Üçü de `tr kontrol`'den hatasız. Biçimlendirici üçünde de tipleri değiştirmedi, ikinci koşu aynı.
- Bir yanlışım oldu ve düzeltildi: `TR_only_2d(TR_name: TR_String, ...)`'e yazdığım `TR_String` doğru hatayı verdi (`TR_name` bir tip değeri); bileşik tiple ilgisi yok, açıklama kaldırıldı.

## Kalanlar

- Ölçüt komutlarının son koşusu (aşağıda sonuç yazılacak).

## Kararsız kaldıklarım

1. **Fonksiyon tipi `TR_Function`'a indirgendi, görevdeki "her şeyle uyumlu" yerine.** Ölçtüm [ölçüldü]: `main`'de `TR_var f: (TR_Int) -> TR_Bool = 3` H1001, `TR_func g() -> (TR_Int) -> TR_Bool { TR_return 3 }` H1003 veriyor; fonksiyon tipi o zaman da `TR_Function` adını taşıyordu. "Her şeyle uyumlu" bu iki var olan denetimi düşürürdü. `TR_Function` hiçbir kapıda yeni hata vermiyor (`main` zaten böyleydi). Ana oturum görevdeki gibi isterse değişiklik tek satır: `checker/type_names.go` `writtenType`'ta `return typeFunction` → `return typeAny` (ve `normalizedType`), testte iki satır.
2. **Tek öğeli dönüş `-> (TR_Int)` artık sözdizimi hatası.** `main`'de geçiyordu (tek öğeli `ReturnTypes`). Görevin dediği gibi tek öğeli parantez hata bırakıldı; depoda hiçbir `.tr` dosyası ve belge bloğu bunu kullanmıyor [ölçüldü: `git grep`, DocTRCodeBlocks]. Soru 2'ye bağlı.
3. **6.10'un tip parametresi tanımı (`TR_func baştaki[T](...)`) bu işe alınmadı.** Bu bir tip yazımı değil, tanım sözdizimidir; bugünkü gibi hata ("'(' bekleniyordu, ancak '[' bulundu"). Tip yerindeki `T[...]` ve `T` okunuyor. Kayıtlı olmayan `T` adı bugünkü her kayıtsız ad gibi davranır (yalnız uyuşmazlıkta hata).
4. **`TR_AnyTensor` ve `TR_Task` henüz kod değil**: sözcük çözümleyici "bilinen bir kimlik değil" diyor. Görev "kendi izinde kaydedilir" dediği için eklenmedi; ayrıştırıcı testinden çıkarıldı.
5. **Yeni ileti eklenmedi.** Hatalar var olan H0140'la: tek öğeli/boş parantez "'->' bekleniyordu", boş `[]` "bir ad bekleniyordu". `'->'` iletide H0117 ile tek işaret olarak yazılıyor (`parser/types.go` `arrowType`). Eski kod aynı yerde yanlışlıkla `'=>' bekleniyordu` diyordu; bu da düzeldi.
6. Denetleyici yapı alanlarının tipini bugün de denetlemiyor; buna dokunmadım (yeni hata olurdu).

## Sorular

1. **Parantezli tip** `((TR_Int) -> TR_Bool)?`: 3.6a "`(x)` parantezdir" der, tip için söylemez. Tip yerinde tek öğeli parantez gruplama mı (o zaman bu yazım isteğe bağlı fonksiyon tipidir), yoksa yasak mı? Bugün hata: "'->' bekleniyordu, ancak '?' bulundu". Not: gruplama yoksa isteğe bağlı fonksiyon tipi hiç yazılamaz (`(TR_Int) -> TR_Bool?`'nin `?`'si dönüşündür).
2. **Tek öğeli `(TR_Int)`**: tip yerinde (değişken, parametre, `-> (TR_Int)` dönüşü) `TR_Int` mi, hata mı? Bugün hata. Soru 1'in yanıtı bunu da belirler.

## status.md için satırlar (ana oturum birleştirir)

- **3.5 Mevcut durum:** `T?` ayrıştırılıyor; denetleyici onu `T`'ye indirgiyor: isteğe bağlı tipin anlamı yok, `TR_Int` açıklamalı değişkene `TR_none` hâlâ atanabiliyor [ölçüldü]. Yapılacaklar değişmedi.
- **3.6 Mevcut durum:** karışık tipli liste çalışıyor; `TR_List[T]` ayrıştırılıyor, denetleyici `TR_List`'e indirgiyor: `TR_var l: TR_List[TR_Int] = ["a"]` hatasız [ölçüldü]. **Yapılacaklar:** `TR_List[T]`'nin denetimi; öğelerin tipi aynı, boş liste tipini yazar; `TR_none` içeren değişmez `TR_List[T?]`.
- **3.6a ve 3.7 (ek):** demet tipi `(A, B)` ve `TR_Dict[K, V]` ayrıştırılıyor; demet tipi her şeyle uyumlu, sözlük `TR_Dict`'e indirgeniyor [ölçüldü].
- **6.9 Mevcut durum:** fonksiyon tipinin yazımı (`(T) -> R`, `() -> R`, `-> (A, B) -> C`) ayrıştırılıyor, parametre ve dönüş tipleri ağaçta; denetleyici onu `TR_Function`'a indirgiyor, parametre ve dönüşü denetlemiyor. Fonksiyonun metni dosyanın dilinde ve tipsiz: ok fonksiyonu için "<ok fonksiyonu>" [ölçüldü]. **Yapılacaklar:** fonksiyon tipinin denetimi (4a); metin tipin metni. Kimlik listesindeki genel fonksiyon tipi kalkar (3.11).
- **6.10 Mevcut durum:** `TR_func f[T](l)` sözdizimi hatası veriyor; tip yerinde `Kutu[TR_Int]` ve `T` ayrıştırılıyor, denetleyici `Kutu`'ya indirgiyor [ölçüldü]. Yapılacaklar değişmedi.
- **15.1 (ek):** `TR_Tensor[E]` ayrıştırılıyor, `TR_Tensor`'a indirgeniyor; `TR_AnyTensor` kod değil ("bilinen bir kimlik değil") [ölçüldü].

## Kapsam dışı bulgu

- Sözcük çözümleyicinin `paramInScope`'u (`internal/lang/lexer/identity.go`) iç içe parantezde her `TR_` adı için bütün yığını tarıyor: `main`'de `[TR_true, ` ×30000 1.3 sn, `TR_List[` ×100000 14 sn [ölçüldü]. Ayrıştırıcının derinlik sınırı korumuyor, çünkü dosya önce baştan sona sözcüklere ayrılıyor. Ayrı iş olarak önerildi; derinlik testinde bu yüzden 2000 düzey kullanıldı.
