# 6. Fonksiyonlar

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "6. Fonksiyonlar".

Her kuralın emsali ve gerekçesi: [`rationale/06_functions.md`](rationale/06_functions.md).

**6.1** `TR_func f(p: T, q: U) -> R { ... }` bir fonksiyon tanımlar. Her parametrenin tipi yazılır (1.2). Değer döndüren fonksiyon dönüş tipini `->` ile yazar; değer döndürmeyende `->` yazılmaz (`-> TR_NoneType` sözdizimi hatasıdır, ileti onu silmeyi önerir). Ad, bulunduğu bloğun her yerinden görünür (5.14). Aynı kapsamda aynı adla ikinci bir tanım sözdizimi hatasıdır (1.3): bir ad tek fonksiyondur, parametre tipine göre ayrışma yoktur. Başka dillerin yazımları sözdizimi hatasıdır ve ileti TR'deki yazımı söyler: `def f(x):`, dönüş tipinin `: R` ile yazılması, Python'un süsleyicisi (satır başında `@f`; ileti sondaki bloğu önerir, 6.7).
- *Test:* TR testi; tipli parametre zorunluluğu uygulanınca denetleyicinin testi.

```tr
# TR 4
TR_func ölçekle(x: TR_Float, k: TR_Float = 2.0) -> TR_Float {
    TR_return x * k
}
TR_print(ölçekle(1.5), ölçekle(1.5, k: 10.0))   # çıktı: 3.0 15.0
```

**6.2** `TR_return ifade` fonksiyondan değer döndürür; değer döndürmeyen fonksiyonda `TR_return` değersiz yazılır. Dönüş tipi yazılmış fonksiyonda ve dönüş tipi beklenen tipten gelen fonksiyon ifadesinin blok gövdesinde (6.6) her yol biter: `TR_return`, `TR_throw` (8.2), `TR_exit`, çıkışı olmayan sonsuz döngü (5.9), bütün dalları biten `TR_if`/`TR_else` ya da bütün kolları biten `TR_match` ile. Bitmeyen yol denetim hatasıdır, ileti `TR_return`'ü önerir. Gövdenin son ifadesi dönüş değeri değildir. Programın en dışında `TR_return` yazılmaz (1.10).
- *Test:* TR testi; eksik dönüş hatası uygulanınca denetleyicinin testi.

**6.3** Birden çok değer bir demettir (3.6a): dönüş tipi `-> (TR_Float, TR_Int)` diye yazılır, `TR_return a, b` bir demet döndürür. Sonuç 5.3'teki gibi açılır (`TR_var a, b = f(x)`) ya da bir değer olarak tutulur (`TR_var t = f(x)`, `t.0`), argüman olur (`g(f())`), listeye konur. Açmada değer sayısı tutmazsa denetim hatasıdır.
- *Test:* TR testi.

**6.4** Varsayılan değer `p: T = ifade` diye yazılır; varsayılanlı bir parametreden sonraki parametrelerin de varsayılanı vardır. Varsayılan, argümanı verilmediğinde her çağrıda yeniden, soldan sağa hesaplanır: önceki parametreleri ve fonksiyonun tanımlandığı kapsamı görür (`TR_func f(a: TR_Int, b: TR_Int = a * 2)` ile `f(1)` → `b` 2). Parametrenin kendisi ve sonraki parametreler varsayılanda kullanılamaz (denetim hatası). Varsayılanı `[]` olan parametre her çağrıda yeni bir liste alır.
- *Test:* TR testi.

**6.5** Argümanlar önce sırayla, sonra adla verilir: `f(10, k: 2)`; adlı argümanların kendi arasındaki sırası serbesttir.
- Python'un `k=2` yazımı sözdizimi hatasıdır; ileti `k: 2`'yi önerir.
- Eksik zorunlu argüman, fazla argüman, bilinmeyen ad, hem sırayla hem adla verilen parametre ve tipi tutmayan argüman denetim hatasıdır; ileti parametrenin adını söyler.
- Değişken sayıda parametre (Python'un `*args` ve `**kwargs`'ı) yoktur; liste ya da sözlük parametresi alınır. Yerleşiklerin bazıları (`TR_print`) değişken sayıda argüman alır.
- Liste ve sözlük argümanı paylaşılır (3.6). Parametre gövdenin yerel değişkenidir ve atanabilir.
- *Test:* TR testi; denetim hataları uygulanınca denetleyicinin testi.

**6.6** Fonksiyon ifadesi `(a, b) => ifade` ya da `(a) => { ... }` bir fonksiyon değeridir; blok gövdesi değerini `TR_return` ile verir (6.2). Parametre tipleri beklenen fonksiyon tipinden çıkar (`l.TR_filter((x) => x > 2)`); beklenen tip yoksa yazılır (`(x: TR_Int) => x * 2`). Parantez her zaman yazılır. Değeri gerekmeyen parametrenin yerine `_` yazılabilir (`(_) => 0`); `_` okunamaz (5.3). Fonksiyon ifadesinin tek yazımı budur: adsız `TR_func(x) { ... }`, parantezsiz `x => ...` ve Python'un `lambda x: ...`'i sözdizimi hatasıdır, ileti `(x) => ...`'i önerir. `TR_func` tanımı yalnız deyim olarak yazılır; ifade yerinde sözdizimi hatasıdır.
- *Test:* TR testi; parametre tipinin çıkarımı ve zorunluluğu uygulanınca denetleyicinin testi.

**6.7** Sondaki blok: bir çağrının ardından aynı satırda yazılan `{ ... }` (2.8) parametresiz bir fonksiyondur ve çağrının son argümanı olur: `TR_mixed_precision("bf16") { ... }`. Blok tek argümansa parantez yazılmaz (`TR_error_code { f(1) }`).
- Bloğun değeri son ifadesidir (5.5'teki gibi). İçinde `TR_return` sözdizimi hatasıdır; `TR_break` ve `TR_continue` 5.13'teki gibi sınırı geçemez.
- Son argüman parametresiz bir fonksiyon ifadesiyse yalnız böyle yazılır: `f(() => { ... })` ve `f(() => x)` sözdizimi hatasıdır, ileti sondaki bloğu önerir. Parametreli fonksiyon ok ile verilir; sondaki blokta parametre yoktur.
- `TR_if`, `TR_for` ve `TR_match` başlığında `{` her zaman gövdeyi başlatır; orada sondaki bloklu çağrı paranteze alınır.
- *Test:* TR testi.

**6.8** Fonksiyon ve fonksiyon ifadesi tanımlandıkları kapsamın değişkenlerini başvuruyla yakalar (kapanış): onları okur ve atar, değişiklik iki yanda da görünür. Döngünün her turu kendi değişkenini taşır (5.12); turda kurulan kapanış o turun değerini görür. Dıştaki değişkene atamak için ek bir sözcük gerekmez, çünkü yeni yerel ad `TR_var` ile tanımlanır (5.1). Fonksiyon dıştaki bir değişkene, onu bu fonksiyonda hiç okumadan önce atıyorsa uyarı verilir; ileti yerel bir ad isteniyorsa `TR_var`'ı önerir. Özyineleme serbesttir.
- *Test:* TR testi.

```tr
# TR 4
TR_func sayaç_kur() -> () -> TR_Int {
    TR_var n = 0
    TR_return () => {
        n += 1
        TR_return n
    }
}
TR_var say = sayaç_kur()
say()
TR_print(say())   # çıktı: 2
```

**6.9** Fonksiyon tipi `(T1, T2) -> R` diye yazılır: `(TR_Int) -> TR_Bool`, parametresizde `() -> R`. `(TR_Int) -> TR_Bool?`'nin `?`'si dönüşündür; isteğe bağlı fonksiyon tipi parantezle yazılır (`((TR_Int) -> TR_Bool)?`, 3.6a). `-> (A, B) -> C` sağdan birleşir: dönüşü fonksiyon olan fonksiyon. Değer döndürmeyen fonksiyonun tipi `(T) -> TR_NoneType`, birden çok değer döndürenin `(T) -> (A, B)`'dir. `->` sağdan birleşir: `-> (A, B) -> C` iki parametreli bir fonksiyon döndürür. Fonksiyon değeri sırayla ve bütün argümanlarıyla çağrılır; adlı argüman ve varsayılan yalnız tanımın adıyla yapılan çağrıda geçerlidir. Fonksiyonlar karşılaştırılamaz (4.5); fonksiyonun metni tipinin metnidir ("(Int) -> Bool", 3.14).
- *Test:* TR testi; fonksiyonun metni ("(Int) -> Bool") ve fonksiyon tipinin parametre ve dönüş tiplerinin denetimi tip denetimiyle (4a) uygulanınca TR testi.

**6.10** Tip parametresi adla parametreler arasında köşeli parantezle yazılır: `TR_func baştaki[T](l: TR_List[T]) -> T? { ... }`. Çağrıda argümanlardan çıkar, çıkmıyorsa yazılır (`baştaki[TR_Int]([])`; tek istisna: `TR_Tensor`'un cinsi çıkmıyorsa `TR_Float`'tır, 15.2); bu yazım ile bir listenin öğesinin çağrısı (`fs[0](x)`) tiple ayrılır.
- `T` tipindeki değerle yalnız her tipte geçerli işler yapılır: atamak, argüman olarak vermek, döndürmek, listeye ve sözlüğe değer olarak koymak, `TR_print`, `TR_String`, `TR_Type` ve yerleştirme (`\(x)`); `T?` değeri `TR_none`'la karşılaştırılır ve `??` ile açılır.
- Öbür işlemler bir kısıt ister: `==`, `+`, üye, eşitlik isteyen liste üyeleri (`.TR_contains`, `.TR_sort()`), sözlük anahtarı olmak (3.7). Yerleşiklerin kısıtları 13.10'da, arayüz kısıtı 7.8'dedir.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.

**6.11** İç içe çağrı derinliği 10000'i aşarsa çalışma hatasıdır.
- *Test:* TR testi.
