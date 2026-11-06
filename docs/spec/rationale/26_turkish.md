# 26. Türkçe sözcük çözümleme: emsal ve gerekçe

Kurallar: [`26_turkce_sozcuk.md`](../26_turkce_sozcuk.md). Bu dosya her kuralın emsalini ve seçimin gerekçesini tutar.

Kaynak: ajan denemelerindeki Gemini kök bulucusu ve Zemberek `master-dictionary.dict`, commit `6b90eac24bfb`, Apache-2.0. Arama yarışında fark kökten geliyordu [ölçüldü: `docs/agent_trials.md`, DENETİM 255].

### 26.1

- *Emsal:* sözcük `lemma`dır: ölçü sözlük maddesidir, yapım eki sökülmez.

| Emsal | Ad | Ne yapar |
| :-- | :-- | :-- |
| Zemberek | `stem`, `lemma` | ikisi ayrı |
| spaCy | `lemma_` | sözlük maddesi |
| Snowball | `stem` | yapım ekini de atar |
| SciPy | `optimize.root` | sayısal kök bulma: `root` bu yüzden kullanılmaz |

- *Gerekçe:* modül `tr/turkish`, çünkü bir kodun tek modülü vardır; başka dil kendi kodlarını alır. Düzeltme işareti kalkar: `kâğıt` ile `kağıt` aramada aynı sözcüktür, tek biçim sözlüğü de küçültür. `-î` sıfatının düz yazımı bir adın çekimiyse (`kalbî`, `kalbi`) sıfat sözlüğe girmez: `kalbi` `kalp`in çekimidir.
- En uzun kökü seçmek eşitlik ekini zarf fiilin önüne geçirirdi (`gelince`, `gelin`+`ce`): eşitlik eki aynı sınıftaki adaydan sonra gelir. `-ki`nin önünde yalnız bulunma ve tamlayan eki durur (`masadaki`, `masat`+`a`+`ki` değil).
- Üst üste binen eklerde (`amamama`) dallanma sınırı üç iç ektir: Türkçe en çok iki alır (`gel-eme-yebil-ir`).

### 26.2

- *Emsal:* Zemberek çözümü de kök, tür ve ek listesi verir. Ses olayı metinleri dışa çıkmaz: değerler dile bağlı değildir; tür metin değil durumlu tiptir.
- *Gerekçe:* sözlük dışı sözcük kısa köke kırpılmaz, aramada `blokzincirde` ile `blokzincir` eşleşsin yeter. Bilinmeyen gövdeye değen en iç ek kesin bir çekim eki olmalıdır: yoksa `koala`, `melodi` gibi sözlükte olmayan adlar kırpılırdı.

### 26.3

- *Emsal:* araştırılmadı; arayüz ajan denemelerindeki kullanıcı sözlüğünden geliyor (Arena).
- *Gerekçe:* Gemini'nin `yapım_ekleri` seçeneği ve `metin_kökleri` çıkarıldı, arayüz küçük kalsın. Kullanıcı sözlüğü alan adı içindir; ek sözcük ünlü düşmesi ve yumuşama işareti taşımaz.

### 26.4

- *Emsal:* yok: 11.1'in sonucudur.
- *Gerekçe:* modülün en dışı kod koşturmaz (11.1). Sözlük 23 bin ad ve 3 bin fiildir.
