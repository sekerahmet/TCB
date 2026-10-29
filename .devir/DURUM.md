# BİTTİ

# DURUM — status 5, 6, 7'nin yeniden ölçülmesi (Faz 0.1)

- Başlama: 2026-10-29 14:41 UTC (konteynerin saati)
- Bitiş: 2026-10-29 14:58 UTC (konteynerin saati; önceki devirde bu saatin gerçek süreyi az gösterdiği görülmüştü, oturumun kendi zaman damgalarıyla karşılaştırın)
- Taban: main `1073aea`; ölçümden sonra main `59efe46`'ya ilerledi (yalnız 8, 12, 16'nın belgeleri ve status'un başka bölümleri, motor aynı), dala birleştirildi.
- Ölçüm: depodan kurulan `tr` ikilisiyle, her kural için geçici klasörde küçük programlar; her biri hem denetimden (TR_cmd_check) hem çalıştırmadan geçirildi. Üç test dosyası da koşuldu: `deyimler_test.tr` 23/23, `fonksiyonlar_test.tr` 20/20, `yapilar_test.tr` 24/24 geçiyor.

## Biten bölümler

### 5 (commit: status 5)

- Silinen: 0. Değişen: 8 (5.1, 5.3, 5.4, 5.5, 5.6, 5.7, 5.12, 5.14; hepsi yeniden ölçüldü, eskimiş olanlar düzeltildi). Eklenen: 1 (5.11, sözlük üreteci yok).
- Uyuyor: 5.2, 5.8, 5.9, 5.10, 5.13.
- Eskimiş olanlar (status'un dediği, bugün öyle değil): `:=` ve ilk değersiz `TR_var` artık sözdizimi hatası (5.1); öğe sayısı tutmazsa H2700/H2701, `_` okunamıyor (5.3); on iki bileşik atama ve yerinde `+=` çalışıyor (5.4); `TR_else`'siz değer veren `TR_if` hata (5.5); eşleşmeyen eşleme H2702, `TR_default` iletisi (5.6); dolaşırken ekleme H2703/H2704 (5.7); `TR_while` iletisi (5.8). Girişteki "testler eski kuralları sınıyor" listesi de eskimişti, çıkarıldı.
- Yeni bulgu (status'ta yoktu): aktarılan modülün fonksiyonu aktaran dosyanın en dış adını okuyor (5.14); `TR_elif`'ten sonra ikinci, yanıltıcı sözdizimi hatası (5.5).

### 6 (commit: status 6)

- Silinen: 2 (6.7 kurala uyuyor; 6.11 kurala uyuyor, "Açık" notu aşağıda Sorular'a taşındı). Değişen: 9 (6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.8, 6.9, 6.10). Eklenen: 0.
- Uyuyor: 6.7, 6.11.
- Eskimiş olanlar: dönüş tipinin `:` yazımı, `def`, aynı adla ikinci tanım artık hata (6.1); örtük dönüş kalktı (6.2); `TR_return 1, 2` çalışıyor (6.3); varsayılanlıdan sonra varsayılansız parametre hata (6.4); `k=2` iletisi (6.5); parantezsiz ok, adsız `TR_func`, `lambda` hata (6.6); `TR_it`/`TR_acc` kalktı, `f(() => ...)` hata (6.7); ok fonksiyonunun metni artık "<lambda>" (6.9). Girişteki göç sayıları (`TR_it` 44 vb.) ve test listesi eskimişti: depoda kullanım kalmadı.
- Yeni bulgu: adlı argümanın tipi hiç denetlenmiyor (6.5).
- 6.9 ve 6.10 main `1073aeab`'nin ayrıştırıcısıyla ölçüldü, etiketleri istendiği gibi.

### 7 (commit: status 7)

- Silinen: 0. Değişen: 8 (7.1–7.8; hepsinde bir ayrılık kaldı). Eklenen: 0.
- Uyuyor: yok.
- Eskimiş olanlar: yinelenen metottan sonraki ikinci H0141 düzelmiş (7.1); metotta düz alan adı dıştaki ada çözülse de denetim hatası (7.3); `p?.z` artık H2468 (7.5); metin artık kurma yazımı "Nokta(x: 1, y: 2)" (7.6); `TR_enum` (7.7) ve `TR_interface` (7.8) uygulanmış, ikisi de kimlik listesinde (girişteki "kimlik listesine eklenir" ve 7.8'deki "onaylı yeni adlar listesine girer" çıkarıldı). Girişteki göç sayıları yeniden sayıldı: `.tr`'de 63 yapı (eski 38 başka bir sayımdı ya da testler arttı), `lib/`'de 16; `lib/`'deki `TR_self.alan != TR_none` 15 değil 9 (benim kalıbım: aynı satırda ikisi).
- Yeni bulgular: düz self parametresi (7.3, aşağıda kararsız); alanla eşit uzaklıktaki metodun önerilmesi (7.5); fonksiyon alanlı nesnenin kendisiyle `==`'si hatasız doğru (7.6); imzası tutmayan değerin arayüzden hatasız geçmesi ve arayüz değerinin `==`'si (7.8).

## Kapsam ölçütü

```
grep -oE '^\*\*[567]\.[0-9]+[a-z]?\*\*' docs/spec/0[567]_*.md   # 33 kural
grep -oE '^### [567]\.[0-9]+[a-z]?' docs/status.md                 # 26 madde
```

- İkinci liste (status maddeleri, 26): 5.1 5.3 5.4 5.5 5.6 5.7 5.11 5.12 5.14 6.1 6.2 6.3 6.4 6.5 6.6 6.8 6.9 6.10 7.1 7.2 7.3 7.4 7.5 7.6 7.7 7.8
- Üçüncü liste ("uyuyor", 7): 5.2 5.8 5.9 5.10 5.13 6.7 6.11
- Birinci liste = ikinci ∪ üçüncü (33 = 26 + 7), kesişim boş [`comm` ile denetlendi].
- `awk '/^## 5\./,/^## 8\./' docs/status.md | grep '^- \*\*Mevcut durum:\*\*' | grep -cE '\[ölçüldü: |\[ölçülmedi: '` → 26; Mevcut durum satırı sayısı da 26. `[ölçülmedi: ...]` gerekmedi: bütün maddeler bu makinede ölçülebildi.
- Testler her commit'ten önce: dört komut geçti, spec-refs "0 error(s), 0 note(s)", `dups --changed main` 0 aday.

## Kalanlar

Yok.

## Kararsız kaldıklarım

- 5.2 "uyuyor" sayıldı, ama kuralın örneği `TR_const l: TR_List[TR_Int] = []` bugün ayrışmıyor ("'=' bekleniyordu"). Bu parametreli tipin yazımıdır, 3.6'nın maddesinde (`TR_List[T]`) zaten var; 5.2'ye ayrıca madde açmadım.
- 7.3: dil tanımı "Python'un self parametresi sözdizimi hatasıdır" diyor; motor yalnız parametre olarak yazılan `TR_self`'i yakalıyor. Kimlik kodlu dilde Python alışkanlığı `TR_func m(self, x)` (düz self sözcüğü) yazar; bunu da kuralın içinde sayıp madde açtım. Kural yalnız `TR_self`'i kastediyorsa 7.3 "uyuyor" olur ve madde silinir.
- 6.7 ve 6.1: "denetim hatası" ile "sözdizimi hatası" farkını ayrılık saymadım (Sorular 1).
- 6.11: motor uyuyor, madde silindi; gerekçe eksiği Sorular 2'de.
- 5.8 "uyuyor" sayıldı: koşulun `TR_Bool` olması yalnız çalışırken denetleniyor, o 3.10'un maddesinde.

## Kapsam dışı gözlemler (status'a yazmadım)

- Çalışma hatalarının ve bazı sözdizimi iletilerinin içinde insan dilinde kod adları geçiyor, sözdizimi iletileri ise kimlik yazıyor: "'sonlandır(0)' yazın" (`TR_return` iletisi), "'x.uzunluk == 0', 'x != yok'" (koşul iletisi, H2600), "eşle'nin hiçbir koluyla" (H2702), "eşle kolunun koşulu" (H0292), tip adları "Tam", "Metin". 2. ya da 17. bölümün işi olabilir.

## Sorular

1. **"Sözdizimi hatası" ile "denetim hatası" ayrımı.** 6.1 aynı adla ikinci tanımı, 6.7 sondaki bloktaki `TR_return`'ü "denetim hatası" diye yazıyor; motor ikisini "Sözdizimi hatası" olarak veriyor (H0260, H0263). Davranış aynı (hiçbir satır çalışmaz, çıkış 1, 1.3 ve 1.9). Dil tanımının başka yerleri (5.13 "sözdizimi hatasıdır: dosya okunurken bulunur", 7.1 "sözdizimi hatasıdır (tanımda bilinir)") ayrımı bilerek yapıyor gibi. Ayrım kullanıcıya görünen bir şey mi (ileti başlığı, `TR_cmd_check --json`'un türü), yoksa ikisi de "çalışmadan önce" mi? Görünürse 6.1 ve 6.7 için motor ya da tanım düzelir; ben bu yüzden madde açmadım, 6.7'yi uyuyor saydım.
2. **6.11'in dayanağı.** Eski status'taki "Açık: 10000'in dayanağı (Go yığınına sığan derinlik mi, ML kodunun gereksinimi mi) ölçülmedi; gerekçeye yazılacak" notu motorun ayrılığı değil, gerekçenin eksiği; motor kurala uyduğu için maddeyi sildim. Gerekçede (`rationale/06_functions.md` 6.11) hâlâ yalnız Python'un 1000'i var, 10000'in nedeni yok. Bu bir gerekçe işi olarak başka bir listeye mi girmeli?
