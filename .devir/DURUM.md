# DURUM — Faz 0.6b: demet

- **Başlama:** 2026-10-29 16:03 UTC
- **Dal:** `claude/magical-feynman-mgjmfn` (taban `main` 4eb9a44)

## Biten adımlar

- 16:40 sözcük çözümleyici ve ayrıştırıcı (`698ab10`): `t.0.1` iki indeks; `(a, b)` demet değişmezi; `ast.ElementExpression`; tipte tek öğeli parantez gruplar. Biçimleyici zaten doğru biçimliyordu, tablo satırı eklendi.
- 17:20 değer ve işleçler (`383c265`): `TupleVal`, `MultiVal` ve H2708 kalktı; `==`, `<`, sıralama, `TR_min`/`TR_max`, sözlük anahtarı, metin, tip metni; hata ayıklayıcıda öğeler `.0`. Değer ve işleçler aynı ileti dosyalarına dokunduğu için tek commit.
- 17:20 JSON yazımı (`01cd4be`): `TR_json_string` demeti dizi yazar.

- 17:45 sahiplik (`55e45eb`): demet `Owned`; `moveTo`, `adopt`, `escape` liste ve demeti `ownedElements` ile gezer; `ownList` → `ownContainer`. Testler (a)–(e), liste değişmezi ve indeksle atama `behavior_ownership_test.go`'da. Liste testi gerçek bir eksiği gösterdi (aşağıda, Kararsız 1): düzeltildi.
- 17:55 denetleyici (`f0fafb7`; H1026, `checker/unpack.go`): sayı `TypeInfo.TupleLen`'de, tip dizesinde değil; `tests/negative/demet_acma.tr`.
- 18:10 (`2a4f3f1`) TR testleri (`tests/tipler_test.tr`'nin 8 "3.6a ..." testi), `bind` ve `codeindex` denemeleri, dil tanımı (3.6a, 6.3, 13.8), gerekçe, status (3.6a, 5.3, 6.3).

## Kalanlar

- bitiş ölçütünün komutları ve hız ölçümü (sürüyor)

## `*ListVal` taraması (`git grep -nE '\*ListVal\b' -- internal`)

Demetin girdiği dallar: `ownership.go` (`moveTo`, `adopt`, `escape`; `ownedElements`), `equality.go` (`comparisonError`), `struct_ops.go` (`walk`), `operators.go` (`valuesEqual`'ın başı), `printing.go` (`writeContainer`, `writeItem`), `type_value.go` (`TypeOfValue`), `values.go` (`DictKey`, `DictKeyError`, `ValueType.MessageText`), `sorting.go`, `numbers.go` (`extreme`), `compiler_stmt.go` (açma), `jsonwrite`, hata ayıklayıcının `children`, `expandable`, `valueKind`'ı.

Bilerek dışarıda bırakılanlar (dil tanımında demet için yazmıyor, bugün hata):
- `compiler_loop.go:113, 264`, `expressions.go:35` (dolaşma): `TR_for x TR_in (1, 2)` H2464. Döngüde açma 16.8'in işi.
- `expressions.go:81` (`t[0]`): H2465; `expressions.go:163` ve `methods.go` (üyeler): `t.TR_length` bilinmeyen üye.
- `sequence_ops.go` (`+`, `*`), `assign.go:62, 70` (`+=`), `assign.go:91` (indeksle atama): genel işleç hatası (H2469) ya da H2465.
- `builtins.go:85` (`TR_sum`), `stats.go:65`, `bytes_value.go:151`, `type_call.go:180, 191` (`TR_List(t)`), `json_value.go` (`TR_JSON(t)`, H2654), `json_use.go` (JSON'un iç değerleri demet tutmaz): liste parametresi demeti almaz, tip hatası (`bind`'ın testi).
- `builtins.go:292` (`extremeArgs`): tek demet argüman açılmaz, tek değerdir (`TR_min((3, 1))` demetin kendisi).
- `arg_errors.go:155` (iletideki özet), `values.go:52` (`Shown`, listeyi seçenek listesi yazar): demet değerin metniyle yazılır, kısaltma gerekmiyor.
- `compiler_call.go:40` (`.TR_append`'in hızlı yolu): yalnız liste; burada 15.43 için değişti (Kararsız 1).
- hata ayıklayıcının `dap_table.go` (`recordList`, `TableData`, `typeLabel`) ve `debugger.go:500` (`inspectTable`): tablo gösterimi listeye özgü; demet `setVariable`'da H8153 (değiştirilemez).
- `stdlib/checkpoint/plain.go`: değişmedi, demet H7457 (20.2).

## Kararsız kaldıklarım

1. **15.43, listeye sonradan yazma.** Liste değişmezi testi bugünkü motorun `.TR_append`'de ve indeksle atamada bütün listeyi (içindeki değişkene bağlı öğelerle) kaçırdığını gösterdi; dil tanımı "yalnız ... sonradan yazılan tensör kaçar" diyor. Bu ikisinde artık yalnız yazılan değer kaçıyor, liste öbür öğelerinin sahibi kalıyor, yerine yazılan eski öğenin listedeki bağı kalkıyor (`writeToList`). Öbür değiştiren metotlar (`TR_insert`, `TR_extend`, `+=` ...) bugünkü gibi bütün listeyi kaçırıyor (güvenli yan). `assign.go` "dokunulacak yerler"de yoktu; değişiklik iki satır.
2. **İndeksin yazımı:** `t.01`, `t.1_0`, `x.0e1` sözcük çözümleyici hatası (H0036); dil tanımı yalnız `t.0`, `t.1` diyor. Rust da `t.01`'i reddeder.
3. **Tensör tutan demetin `==`'si** (H2777) yalnız doğrudan öğede ve iç içe demette yakalanıyor; demetin içindeki listenin tensörü bugünkü liste gibi yakalanmıyor (listede de yok; denetimi 4a2).
4. **Sıralama:** demette 13.10'un sırası (−0.0 < 0.0, NaN en büyük) uygulandı; düz sayı listesinin `.TR_sort`'u bugün −0.0 ile 0.0'ı ayırmıyor (13.10'un status'undaki iş, dokunmadım). İkisi bu yüzden şimdilik ayrı davranıyor.
5. **Yeni ileti kodları:** H0036 (indeks yazımı), H1026 (denetimde öğe sayısı), H2772 (sıralanamayan öğe), H2773 (anahtar olamayan öğe), H2774 (sınır dışı), H2775 (demet olmayan değerde `.0`; listede `[i]`'yi önerir), H2776 (atama), H2777 (tensörlü `==`). H2709'un metni demete göre, H2492/H2493'ünki demeti de sayar. H2708 `retired.json`'da.
6. **Commit bölümü:** değer ve işleçler aynı ileti dosyalarına ve `values.go`'ya dokunduğu için tek commit.
7. `docs/status.md:216` (6. bölümün başı) eski H2708 testini anıyor; "yalnız 3.6a, 5.3, 6.3" dendiği için dokunmadım.

## Sorular

1. **`TR_json_parse[T]` motorda yok** (13.8'in ikinci maddesi; status 13.8: "Gemini Koşu 6, uygulanacak"). Bu yüzden "dizi demete kurulur" uygulanamadı ve JSON gidiş-dönüş TR testi tipsiz okumayla yazıldı: `(TR_Int(j[0]), TR_String(j[1]))`. Dil tanımına cümleler eklendi. Tipli okuma kendi izinde mi gelecek, yoksa bu dala mı?
2. Demetin `+`'sı, `*`'ı, `.TR_length`'i, dolaşılması, `t[0]`'ı, `TR_List(t)`'si ve `TR_JSON` içinde demet: dil tanımında yok, hata olarak bırakıldı. `t[0]` için H2465'in demette `t.0`'ı önermesi istenir mi?
3. `t?.0` (isteğe bağlı demette güvenli öğe okuma) dil tanımında yok: bugün sözdizimi hatası.
4. Demetin mantık değeri (`TR_if (1, 2)`): dil tanımında yok; motor `IsTruthy` doğru veriyor (eski `MultiVal` gibi).
