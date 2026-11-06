# TR: kodlama modelleri için başvuru (dil tanımının 1–9. bölümü ve sık yerleşikler)

TR, yapay zekâ işleri için statik tipli bir dildir. Bütün dil sözcükleri `TR_` ile başlayan kimliklerdir. Yanlış yazım çoğunlukla program çalışmadan önce hata verir ve ileti doğrusunu söyler.

Bu belge bugün çalışan dili anlatır. Kuralların tam metni [`docs/spec`](spec/README.md)'tedir; orada yazıp henüz uygulanmamış adlar da vardır, araç onları tanımaz. Buradaki kuralların bazılarını araç henüz denetlemez, geçirmesi kuralı kaldırmaz. Kurulum, komutlar ve çalışan bir eğitim programı: [`AGENTS.md`](../AGENTS.md).

## Dosya, komutlar ve iletiler
- İlk satır tam olarak `# TR 4` (önünde bir `#!` satırı olabilir). Kod yalnız `TR_` kimlikleriyle yazılır; görünüm (`trlang show`) yalnız okunur, çalışmaz.
- Yorum `#` ile başlar. `//` tam bölme işlecidir; `/* */` yoktur.
- Program en dıştan yukarıdan aşağı çalışır; `main` yoktur. Erken çıkış `TR_exit(kod)` ile olur; en dışta `TR_return` yazılmaz.
- Deyimler satır sonuyla ya da `;` ile ayrılır. Satırı bölmek için işleç satır sonunda kalır ya da parantez açılır. Satır başında `.` ve `?.` önceki satırı sürdürür.
- **Komutlar:** `trlang check dosya.tr` (çalıştırmadan sözdizimi ve tip denetimi), `trlang check --json dosya.tr` (JSON kayıtları), `trlang run dosya.tr` (denetler ve çalıştırır). İleti dili `TR_LANG=en` ortam değişkeniyle seçilir; program çıktısı dilden bağımsızdır.
- **İletileri okuma:** Sözdizimi, denetim ve çalışma hataları çıkış kodu 1 ile biter; her ileti konumunu (`dosya:satır:sütun`), `H...` ileti kodunu, hatanın nedenini ve varsa önerilen düzeltmeyi (yakın `TR_` kimliği, doğru yazım ya da TR karşılığı) yazar. `trlang check --json dosya.tr` satır başına `code`, `severity`, `file`, `start`, `end`, `message`, `related`, `fixes` alanlarını verir.

## Kimlikler ve adlar
- Dilin her sözcüğü `TR_` ve adıdır: `TR_if`, `TR_print(x)`, `x.TR_length`, `l.TR_append(3)`, `TR_Int`. Sözcüğün sınıfını yeri belirler: noktadan sonra üye, çağrıda `:`'dan önce adlı argüman, öbür yerlerde anahtar kelime, fonksiyon, tip ya da değer.
- Kullanıcının adı `TR_` ile başlayamaz.
- Ad yazılışı:
  - dosyanın en dışındaki sabit `BÜYÜK_YILAN`;
  - tip, yapı ve durum adı `BüyükDeve`;
  - öbür adlar, yerel sabitler dahil, `küçük_yılan`.

## Değerler ve tipler
- **Temel tipler:**
  - `TR_Int`: 64 bit; taşma çalışma hatasıdır.
  - `TR_Float`, `TR_String`.
  - `TR_Bool`: `TR_true`, `TR_false`.
  - `TR_none`: yokluk (`TR_NoneType`).
- **Kap tipleri ve demet:**
  - `TR_List[T]`: öğelerin hepsi aynı tipte; öğe tipi her zaman yazılır (`TR_List[TR_Int]`).
  - `TR_Dict[K, V]`: anahtar ve değer tipi yazılır; anahtar `TR_Int`, `TR_String`, `TR_Bool`, `TR_Bytes`, öğeleri anahtar olabilen demet ya da alanları anahtar olabilen durumlu tip olabilir.
  - **Demet `(A, B)`:** sabit sayıda, her biri kendi tipinde değerlerden oluşan değiştirilemez değer: `(1, "a")` bir `(TR_Int, TR_String)`'dir (en az iki öğeli; `(x)` parantezdir). Öğeler `t.0`, `t.1` ile okunur (atanamaz, `t[0]` yazılmaz); `TR_var a, b = t` ile açılır; `==` öğe öğe, `<` ve `.TR_sort()` sözlük sırasıyla karşılaştırır.
  - `TR_Range`: `0..n` n hariç, `0..=n` dahil; başlangıç bitişten büyük ya da eşitse boş.
  - `TR_JSON`, `TR_Bytes`. Genel bir `Herhangi` tipi yoktur.
- **Metin:**
  - Yalnız çift tırnakla yazılır; ham metin `` `...` `` ile, birden çok satır olabilir.
  - Yerleştirme `"toplam \(x + 1)"`; `{` sıradan karakterdir.
  - Kaçışlar: `\n \t \r \" \\ \u{...}`.
- **Statik tip:**
  - Fonksiyon/metot parametrelerinin, değer döndüren fonksiyon dönüşünün (`-> R`) ve yapı/durum alanlarının tipi her zaman yazılır.
  - Yerel değişkenin tipi ilk değerinden çıkar ve değişmez: `TR_var t = 0` bir `TR_Int`'tir; ondalık için `TR_var t = 0.0`.
  - Boş değişmezle ya da `TR_none`'la başlayan değişken tipini yazar: `TR_var l: TR_List[TR_Int] = []`, `TR_var s: TR_Dict[TR_String, TR_Int] = {}`.
  - Tam sayı yazımı, `TR_Float` beklenen yerde ondalıktır (`r: TR_Float = 1`). Bunun dışında kendiliğinden dönüşüm yoktur.
- **İsteğe bağlı `T?`:**
  - `TR_none` alabilir. Açmak için `x ?? varsayılan`, `x?.alan` ya da `TR_if x != TR_none { ... }` yazılır; dalda `x` asıl tiptedir, yeter ki yerel olsun ve dalda değişmesin.
  - `TR_if x == TR_none { x = 0 }`'dan sonra `x` asıl tiptedir.
  - Alan ve indeks daraltılmaz; yerel kopyaya alınır (`TR_var v = p.alan`).
  - İsteğe bağlı olmayan bir değeri `TR_none`'la karşılaştırmak hatadır.
- **Koşul** yalnız `TR_Bool` olur: `TR_if l { }` hatadır; doğrusu `TR_if l.TR_length > 0 { }`.
- **Dönüşüm** tip çağrısıyla yapılır:
  - `TR_Int("42")`; `TR_Int(3.9)` → 3 (sıfıra doğru keser);
  - `TR_Float(n)`, `TR_String(x)`, `TR_List(aralık)`.
  - `TR_Type(x)` tipi verir ve tiple karşılaştırılır (`TR_Type(x) == TR_Int`).
- **Değerin metni:**
  - "true", "false", "none", "[1, 2]", `{"k": 1}`, "(1, \"a\")";
  - ondalık "3.0", "0.5";
  - yapı "Nokta(x: 1.0, y: 2.0)".
- **JSON:**
  - Çözme: `TR_json_parse(m)` bir `TR_JSON` verir. `j["k"]` ve `j[0]` yine `TR_JSON`'dur; olmayan alan çalışma hatasıdır. `j.TR_get("k")` `TR_JSON?` verir.
  - Tür sorma: `TR_Type(j) == TR_String` (türler: `TR_NoneType`, `TR_Bool`, `TR_Int`, `TR_Float`, `TR_String`, `TR_List`, `TR_Dict`).
  - Açma: `TR_String(j)`, `TR_Int(j)`, `TR_Float(j)` (tam ve ondalık her JSON sayısını açar), `TR_Bool(j)`, `TR_List(j)`. Tür tutmazsa çalışma hatasıdır.
  - Yazma: `TR_json_string(j)`. Beklenen tipi `TR_JSON` olan değişmez de JSON kurar: `TR_var g: TR_JSON = {"k": 1}`.

## İşleçler
- **Öncelik**, gevşekten sıkıya:
  - `TR_or`, `TR_and`, `TR_not`;
  - karşılaştırmalar, `??`, `..` / `..=`;
  - bit işleçleri `|`, `^`, `&`, `<<`, `>>`;
  - `+ -`, sonra `* / // % @`;
  - tekli `-`, en sıkı `**`.
- **Bölme ve üs:**
  - `/` her zaman `TR_Float` verir: `7 / 2` = 3.5, `6 / 3` = 2.0.
  - `//` aşağı yuvarlar: `-7 // 2` = -4. `%`'nin işareti bölenin işaretidir: `-7 % 3` = 2.
  - İki `TR_Int`'in `**`'ı `TR_Int`'tir ve eksi üs hatadır; `2.0 ** -1` yazılır.
  - Sıfıra bölme çalışma hatasıdır.
- **Karşılaştırma:**
  - `==` aynı tipten iki değeri karşılaştırır; ayrı tipler hatadır (`"1" == 1`). Sayılar arasında tipten bağımsızdır (`1 == 1.0`).
  - `<`, `>`, `<=`, `>=` sayılar, metinler ve öğeleri sıralanabilen demetler arasında tanımlıdır. `0 <= i < n` geçerlidir; `a == b == c` hatadır.
- **Mantık:** `TR_and`, `TR_or`, `TR_not` yalnız `TR_Bool` alır; `TR_and` ve `TR_or` kısa devre yapar. `&&`, `||`, `!` ve `|>` yoktur.
- **`+` ve `*`:**
  - `+` aynı tipten iki değeri ekler (metin + metin, liste + liste). `"a" + 1` hatadır; `"a\(1)"` yazılır.
  - `*` yineler: `"-" * 40`, `[0] * n`. `[[0] * 3] * 3` hatadır; `[[0] * 3 TR_for _ TR_in 0..3]` yazılır.
- **Üyelik:** `l.TR_contains(x)`. `TR_in` yalnız döngüde ve üreteçte geçer.

## Deyimler
- **Tanım:**
  - `TR_var x = 1`: ilk değer zorunlu, `:=` yoktur.
  - `TR_const X = 1`: bağ sabittir.
- **Çoklu atama ve açma:** `TR_var a, b = 1, 2`, `a, b = b, a`, `TR_var x, y = demet_ya_da_liste`. Gerekmeyen değer `_` ile atılır.
- **Atama:**
  - `=`, `+=`, `-=`, `*=`, `/=`, `//=`, `%=`, `**=`, `&=`, `|=`, `^=`, `<<=`, `>>=`.
  - Tip değişmez: `TR_Int` değişkene `/=` hatadır, `//=` yazılır. Listede `+=` listeyi yerinde uzatır.
  - Öğeye atama: `l[i] = x`, `d[k] = v` (sözlükte anahtar yoksa eklenir).
- **Koşul:**
  - `TR_if k { } TR_else TR_if k { } TR_else { }`. Süslü parantez zorunludur; `TR_elif` yoktur.
  - İfade olarak: `TR_var s = TR_if k { "a" } TR_else { "b" }`; `TR_else` zorunludur. Python'un `a if c else b`'si yoktur.
- **Eşleme:** `TR_match x {` ↵ `1 => ...` ↵ `2, 3 => ...` ↵ `10..20 => ...` ↵ `_ => ... }`. Her kol kendi satırındadır; kol gövdesi ifade, `{ blok }` ya da `TR_return ifade` olabilir.
  - Eşleme her durumu kapsamalıdır.
  - Desende düz ad yalnız bir sabittir; değişkenle karşılaştırma `(x)` diye yazılır.
- **Döngü:**
  - `TR_for x TR_in kaynak { }`: liste, aralık, metin; sözlükte anahtarlar.
  - `TR_for i, x TR_in l { }`: indeks ve öğe; sözlükte anahtar ve değer.
  - `TR_for koşul { }`: koşullu döngü; `TR_while` yoktur.
  - `TR_for { }`: sonsuz döngü. `TR_break` ve `TR_continue` vardır.
  - Dolaşılan liste ya da sözlüğe öğe eklemek ya da ondan öğe çıkarmak çalışma hatasıdır.
- **Üreteç:** `[x * x TR_for x TR_in l TR_if x > 0]`.
- **Kapsam:**
  - Her `{ }` yeni bir kapsam açar. Aynı fonksiyonda, dıştaki ya da aynı kapsamdaki bir yerel adla `TR_var` hatadır (gölgeleme yok). Programın en dışı da bu kural için bir fonksiyondur.
  - Değişken tanımından önce kullanılamaz.
  - Fonksiyon ve tip tanımı, onu çalıştıran satırdan önce gelir: tanımları dosyanın başına yaz. Fonksiyon gövdesi sonraki tanımları görür. Bir fonksiyonu çağıran satır, fonksiyonun kullandığı değişkenlerden sonra gelmelidir.

## Fonksiyonlar
- **Tanım:** `TR_func f(x: TR_Int, k: TR_Float = 2.0) -> TR_Float { TR_return x * k }`.
  - Her parametrenin tipi yazılır; varsayılanlı parametreler sondadır.
  - Değer döndürmeyen fonksiyonda `->` yazılmaz.
  - Her yol `TR_return` ile biter; son ifade dönüş değeri değildir.
  - Bir ad tek fonksiyondur.
- **Varsayılan** her çağrıda yeniden hesaplanır: `[]` her çağrıda yeni bir listedir.
- **Birden çok değer (demet):** `-> (TR_Float, TR_Int)`, `TR_return a, b`; sonuç bir demettir: `TR_var a, b = f()` ile açılır ya da `TR_var t = f()` ile tutulup `t.0`, `t.1` diye okunur.
- **Çağrı:** `f(1, k: 3.0)`. Adlı argüman `:` ile yazılır; `k=3.0` hatadır. `*args` yoktur.
- **Fonksiyon ifadesi:** `(x) => x * 2` ya da `(x) => { ...; TR_return y }`.
  - Parametre tipi beklenen yerden çıkar; yoksa `(x: TR_Int) => x * 2` yazılır.
  - `lambda`, `x => ...` ve adsız `TR_func(...)` yoktur.
- **Değer vermeyen çağrı:** değer yerinde duramaz, kendi satırında yazılır: `TR_var x = l.TR_append(1)` hatadır. `.TR_map`'e verilen fonksiyon değer vermelidir.
- **Sondaki blok:** parametresiz bir fonksiyondur ve son argüman olur: `TR_error_code { f(1) }`.
  - Bloğun değeri son ifadesidir; içinde `TR_return` yazılmaz.
  - `f(() => ...)` yerine sondaki blok yazılır.
- **Tipler:** fonksiyon tipi `(TR_Int) -> TR_Bool`, çoklu dönüşte `(TR_Int) -> (TR_Float, TR_Int)`, değer döndürmeyende `(TR_Int) -> TR_NoneType`; `->` sağdan birleşir.
- **Kapanış:** dıştaki değişkeni okur ve atar; döngünün her turu kendi değişkenini taşır, turda kurulan kapanış o turun değerini görür.

## Yapılar, durumlu tipler, arayüzler
- **Yapı:**
  - Tanım: `TR_struct Nokta { x: TR_Float = 0.0` ↵ `y: TR_Float = 0.0` ↵ `TR_func boy() -> TR_Float { TR_return TR_sqrt(TR_self.x ** 2 + TR_self.y ** 2) } }`.
  - Her alanın tipi yazılır, varsayılanlılar sondadır. Kalıtım yoktur.
  - Kurma: `Nokta(3.0, 4.0)` ya da `Nokta(y: 4.0)`.
  - Kurucu `TR_func TR_init(...)`'tir ve her alanı atar; kurucu varsa nesne onun parametreleriyle kurulur.
  - Alan her zaman `TR_self.x` diye yazılır, metot `TR_self.m()` diye çağrılır; self parametresi yoktur. Metot `TR_self` kullanmayabilir (alansız yapı bir arayüzü böyle karşılar).
  - Nesne başvuruyla paylaşılır (`TR_var q = p` aynı nesne). `==` alanlardan karşılaştırır.
- **Durumlu tip:**
  - Tanım: `TR_enum Durum { Bekliyor` ↵ `Çalışıyor(adım: TR_Int) }`. Kurma: `Durum.Çalışıyor(5)`. Değer değiştirilemez.
  - Eşleme: `TR_match d { Durum.Bekliyor => ...` ↵ `Durum.Çalışıyor(a) => ... }`; `a` alanı konumlu olarak bağlar, gerekmeyen alan `_` ile atılır. Bütün durumlar kapsanır.
- **Arayüz:**
  - Tanım: `TR_interface Katman { TR_func uygula(x: TR_Float) -> TR_Float }`. Bu metotları taşıyan yapı arayüzü kendiliğinden karşılar.
  - `TR_List[Katman]` gibi bir tip olarak kullanılır.
  - Asıl tipe inmek: `TR_if TR_Type(k) == Ölçek { k.kat }` (dalda `k` bir `Ölçek`'tir).

## Hatalar ve erteleme
- **Yakalama:** `TR_try { ... } TR_catch e { ... }`. `TR_catch` ve ad her zaman yazılır; adsız `TR_catch { }` hatadır.
  - İfade olarak da kullanılır: `TR_var n = TR_try { TR_Int(m) } TR_catch e { 0 }`.
  - Tek `TR_catch` vardır; ayırma onun içinde yapılır.
- **Fırlatma:** `TR_throw ifade`. Değer metin, durumlu tip değeri, `TR_Message` ya da `TR_Error` olabilir. Yakalanan hata `TR_throw e` ile olduğu gibi yeniden fırlatılır.
- **`TR_Error`:**
  - `e.TR_code`: çalışma hatasında iletinin kodu (`"H2470"` gibi, metin), kodu olmayan hatada `TR_none`.
  - `e.TR_value`: fırlatılan değer. Yalnız `TR_match` deseniyle ya da `TR_Type` sınamasıyla açılır.
  - `e.TR_cause` (`TR_Error?`), `e.TR_suppressed` (`TR_List[TR_Error]`).
  - `TR_String(e)` fırlatılan değerin metnidir: düz metinde metnin kendisi, çalışma hatasında `Message("H2132")` gibi kodu. Okunur ileti gerekiyorsa programın kendisi yazar. Hatayı metnine göre ayırma, koduna ya da değerine göre ayır.
- **Çalışma hataları da yakalanır:** sıfıra bölme, olmayan anahtar, tutmayan JSON açması.
- **Durumlu tiple ayırma:**
```tr
# TR 4
TR_enum AyarHatası {
    Yok(yol: TR_String)
    Bozuk(satır: TR_Int)
}
TR_func oku() -> TR_Int {
    TR_throw AyarHatası.Yok("ayar.json")
}
TR_var n = TR_try { oku() } TR_catch e {
    TR_match e.TR_value {
        AyarHatası.Yok(y) => 0
        _ => { TR_throw e }
    }
}
```
- **Erteleme:** `TR_defer { ... }` yalnız blokla yazılır; `TR_defer f()` hatadır.
  - İş, en yakın `{ }` kapsamı bitince çalışır: döngünün her turunun sonunda, fonksiyonun sonunu beklemeden.
  - Kapsamdan her çıkışta çalışır: `TR_return`, `TR_break`, `TR_continue`, hata.
  - Aynı kapsamdakiler ters sırayla çalışır, son kaydedilen önce.
  - Blok adları çalıştığı andaki değerleriyle okur; bloktan `TR_return` ile çıkılmaz.

## Sık yerleşikler
- **Genel:** `TR_print(a, b)` (değerleri boşlukla yazar), `TR_abs`, `TR_floor` ve `TR_ceil` (`TR_Float` verir), `TR_round(x)`, `TR_round(x, basamak)`, `TR_sqrt`, `TR_log`, `TR_exp`, `TR_min`, `TR_max`, `TR_sum(l)`, `TR_mean(l)`, `TR_json_parse(m)`, `TR_json_string(j)`, `TR_time_ms()`, `TR_sleep(saniye)`, `TR_exit(kod)`.
- **Yuvarlama:** `TR_round(x)` bir `TR_Int` verir, `TR_round(x, basamak)` girdinin tipini. Yarım çifte gider, yazılan ondalığa göre: `TR_round(2.5)` = 2, `TR_round(3.5)` = 4, `TR_round(0.125, 2)` = 0.12, `TR_round(2.675, 2)` = 2.68.
- **Metin:** `m[0..3]` dilimdir, `.TR_length`, `.TR_lower()`, `.TR_upper()`, `.TR_strip()`, `.TR_split()` (boşluk, sekme ve satır sonunda böler), `.TR_split(ayırıcı)`, `.TR_replace(a, b)`, `.TR_starts_with(m)`, `.TR_ends_with(m)`, `.TR_contains(m)`, `.TR_index_of(m)` (`TR_Int?`, bayt değil karakter sırası), `.TR_lines()` (`TR_read_lines` gibi böler), `.TR_get(i)`.
- **Liste:**
  - `.TR_length`, `.TR_append(x)`, `.TR_contains(x)`, `.TR_index_of(x)` (`TR_Int?`).
  - `.TR_map(f)`, `.TR_filter(f)`, `.TR_reduce(başlangıç, (a, x) => ...)`.
  - `.TR_sort()` ve `.TR_reverse()` yeni bir kopya verir, listeyi değiştirmez. Değeri atılırsa denetim hatasıdır: `l.TR_sort()` tek başına yazılmaz; `l = l.TR_sort()` ya da `TR_var s = l.TR_sort()`.
  - Anahtarlı sıralama `.TR_sort((x) => anahtar)`: anahtar sayı, metin ya da bunların demetidir; eşitlerin sırası korunur. Büyükten küçüğe sayı için `-x`; birden çok ölçüt için demet: `.TR_sort((x) => (-x.puan, x.ad))`.
  - `.TR_join(ayırıcı)`, `.TR_unique()`.
  - `.TR_first`, `.TR_last` ve `.TR_get(i)` isteğe bağlı değer verir. `l[i]` sınır dışında çalışma hatasıdır; negatif indeks sondan sayar.
- **Sözlük:** `d[k]` (olmayan anahtar çalışma hatasıdır), `d.TR_get(k)` (`V?`), `.TR_keys()`, `.TR_values()`, `.TR_contains(k)`, `.TR_length`.

## Dosya, klasör ve komut satırı
- **Dosya:** `TR_read_file(yol)` metni verir, `TR_write_file(yol, metin)` yazar. `TR_read_lines(yol)` satır satır okur: `TR_for satır TR_in TR_read_lines(yol) { }`.
- **Yol:** `TR_Path("klasör", "a.txt")` yolu birleştirir, `yol.TR_is_file` dosya mı diye sorar, `TR_String(yol)` metnini verir. `TR_make_dir(yol)` klasör kurar.
- **Klasör:** `TR_list_dir(klasör, "**/*.txt")` kalıba uyanları `TR_Path` listesi olarak verir; `**` sıfır ya da çok klasördür, klasörün kendisindeki dosyalar da gelir.
- **Komut satırı:** `TR_var ayar = TR_command_line({"adet": 5, "soru": ""})` argümanları varsayılanlarıyla okur: `trlang run a.tr --adet 3`, `ayar["adet"]`. `TR_input("soru: ")` bir satır okur, `TR_env("AD")` ortam değişkenini verir.

## Başka dillerden gelen sık yanlışlar
| Yazılan | TR'de |
| :-- | :-- |
| `def f(x):`, `fn`, `function` | `TR_func f(x: T) -> R { }` |
| `elif`, `while`, `switch`/`default` | `TR_else TR_if`, `TR_for koşul { }`, `TR_match` ve `_` |
| `x = 5` (yeni ad), `x := 5` | `TR_var x = 5` |
| `if l:` / `TR_if l { }` | `TR_if l.TR_length > 0 { }` |
| `x in l` | `l.TR_contains(x)` |
| `a if c else b` | `TR_if c { a } TR_else { b }` |
| `f(k=2)` | `f(k: 2)` |
| `"a" + str(n)`, f-string | `"a\(n)"` |
| `self.x` ve `def m(self)` | `TR_self.x` ve `TR_func m()` |
| `class A(B):` | `TR_struct A { }` ve arayüz |
| `None`, `True`, `is None` | `TR_none`, `TR_true`, `== TR_none` |
| `// yorum`, `/* */` | `# yorum` |
| `lambda x: x * 2` | `(x) => x * 2` |
| `l.sort()` (yerinde) | `l = l.TR_sort()` |
| `sorted(l, key=f, reverse=True)` | `l.TR_sort((x) => -f(x))` |
| `try:` / `except E as e:` | `TR_try { } TR_catch e { }` ve içinde `TR_match e.TR_value` |
| `raise E(x)` | `TR_throw Hata.E(x)` |
| `finally:`, `with` | `TR_defer { }` |
| `[[0] * 3] * 3` | `[[0] * 3 TR_for _ TR_in 0..3]` |
| `n / 2` ile tam sayı, `t[0]` (demette) | `n // 2`, `t.0` |
