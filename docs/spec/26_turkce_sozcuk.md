# 26. Türkçe sözcük çözümleme

Bu bölüm `tr/turkish` kitaplığını yazar: bir Türkçe sözcüğün sözlük biçimini bulmak. Metin aramasında `kitaplarımızdan` ile `kitap` böyle eşleşir. Yeni yerleşik yoktur, kitaplık `.tr`'dir.

Emsal ve gerekçe: [`rationale/26_turkish.md`](rationale/26_turkish.md).

```tr
# TR 4
TR_import "tr/turkish"
TR_print(TR_lemma("kitaplarımızdan"))
TR_var çözüm = TR_analyze_word("gidiyorum")
TR_print(çözüm.TR_lemma, çözüm.TR_suffixes)
TR_var bulucu = TR_Lemmatizer()
bulucu.TR_add_lemma("blokzincir", TR_WordKind.TR_Noun)
TR_print(bulucu.TR_analyze_word("blokzincirde").TR_known)
```

**26.1** `TR_lemma(sözcük)` tek bir sözcük alır, sözlük biçimini `TR_String` verir.
- Sözlük maddesidir: ad yalın, fiil köktür (`gidiyorum`, `git`). Sözlükte duran sözcükten yapım eki sökülmez (`öğretmen` kalır); sözlükte olmayan türemiş biçimden üretken yapım eki sökülür (`renkli`, `renk`).
- Çekim ekleri, ek eylem, `-ki`, fiilimsi ve ses olayları (yumuşama, ünlü düşmesi, ikizleşme, daralma) geri alınır.
- Yazım aynıysa ad önce gelir (`yemek`), öbür adaylarda en az yapım eki, sonra en uzun kök seçilir.
- Girdi Türkçe küçük harfe çevrilir (`ISI`, `ısı`), düzeltme işareti kalkar (`kâğıdı`, `kağıt`), iki yanındaki boşluk ve noktalama kırpılır.
- Kesme işaretinin solu sözlük biçimi, sağı ektir (`Ankara'dan`, `ankara`).
- Boş girdi boş metin verir, hata değildir.
- *Test:* TR testi.

**26.2** `TR_analyze_word(sözcük)` bir `TR_WordAnalysis` verir. Alanlar:
- `TR_lemma`: 26.1'in sonucu;
- `TR_word_kind`: `TR_WordKind`, durumlu tiptir (7.7): `TR_Noun`, `TR_Verb`, `TR_UnknownKind`. Fiil dışı sözlük maddesi `TR_Noun`'dur;
- `TR_suffixes`: sözcüğün ekleri, soldan sağa, `TR_List[TR_String]`;
- `TR_known`: sözlük biçimi sözlükteyse doğru.

Sözlük dışı sözcükten yalnız güvenli çekim eki soyulur, kısa kök üretilmez, `TR_known` yanlıştır: `blokzincirde`, `blokzincir`. Soyulacak güvenli ek yoksa küçültülmüş girdi döner. Tür `TR_UnknownKind`'dır; kesin fiil eki alan sözcük `TR_Verb`'tür.
- *Test:* TR testi.

**26.3** `TR_Lemmatizer()` kendi sözlüğü olan bir bulucu verir. `.TR_lemma` ve `.TR_analyze_word` 26.1 ve 26.2'dir, yerleşik sözlük de geçerlidir.
- `b.TR_add_lemma(sözcük, tür)` sözlüğe ad ya da fiil kökü ekler; tür `TR_WordKind.TR_Noun` ya da `TR_WordKind.TR_Verb`'tür. Fiil mastar ekiyle değil kökle yazılır.
- Boş sözcük ve `TR_UnknownKind` hatadır.
- Eklenen sözcük yalnız o bulucuda geçerlidir.
- *Test:* TR testi.

**26.4** Sözlük ilk çağrıda bir kez kurulur: kitaplığı aktaran program kullanmadıkça bedel ödemez (11.1). Aynı sözcük aynı sonucu verir.
- *Test:* TR testi.
