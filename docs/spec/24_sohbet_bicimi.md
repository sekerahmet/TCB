# 24. Sohbet biçimi

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "24. Sohbet biçimi".

Her kuralın emsali ve gerekçesi: [`rationale/24_chat.md`](rationale/24_chat.md).

Bir sohbet modeli konuşmayı, eğitildiği biçimde kurulmuş bir parça dizisi olarak görür: rol başlıkları, tur sonları, düşünce ve araç etiketleri. Hugging Face bu diziyi modelin dosyasındaki Jinja şablonunu çalıştırarak kurar. TR dışarıdan gelen şablonu çalıştırmaz: her model ailesinin biçimi `tr/chat` kitaplığında (11.6) TR ile yazılmış bir fonksiyondur ve dizi kimliklerden kurulur.

**24.1** İleti. `tr/chat` `TR_ChatMessage` yapısını (7.1) ve `TR_Role` durumlu tipini (7.7) verir.
- `TR_Role`'ün durumları `TR_System`, `TR_User`, `TR_Assistant` ve `TR_Tool`'dur. Rol bir metin değildir: yanlış yazılan rol denetim hatasıdır. Biçim fonksiyonu rolü modelin kendi protokol metnine çevirir.
- `TR_ChatMessage(TR_role: r, TR_content: metin, TR_reasoning: düşünce, TR_tool_calls: çağrılar)`:
  - `TR_content:` `TR_String`;
  - `TR_reasoning:` asistanın düşüncesi, `TR_String?`, varsayılanı `TR_none`;
  - `TR_tool_calls:` asistanın araç çağrıları, `TR_List[TR_ToolCall]`, varsayılanı `[]`.
- `TR_ToolCall(TR_name: ad, TR_arguments: json)`: `TR_arguments` aracın argümanlarının JSON metnidir; araç onu kendi yapısına `TR_json_parse[T]` ile okur (13.8).
- Yapılar ve rol dışa açıktır: kullanıcı kendi modelinin biçimini aynı yapılarla yazar.

```tr
# TR 4
TR_import "tr/chat"
TR_var iletiler = [
    TR_ChatMessage(TR_role: TR_Role.TR_System, TR_content: "Kısa cevap ver."),
    TR_ChatMessage(TR_role: TR_Role.TR_User, TR_content: "Saat kaç?"),
]
```

- *Test:* gerekmez, henüz uygulanmadı.

**24.2** `TR_qwen3_format(p, iletiler, TR_add_generation_prompt: b, TR_enable_thinking: d, TR_tools: araçlar)` Qwen3 ailesinin biçimiyle bir `TR_List[TR_Int]` verir; `p` modelin parçalayıcısıdır (16.1).
- **Kimlikle kurulur.** Biçimin etiketleri (`<|im_start|>`, `<|im_end|>`, `<think>`, `</think>`, `<tool_call>`, `<tool_response>` ve kapanışları) kimlikleriyle girer (`p.TR_token_id`). İki etiket arasındaki bütün metin, biçimin kendi metni ve iletinin içeriği birlikte, tek bir `p.TR_encode(metin, TR_special_tokens: TR_false)` çağrısıyla kodlanır (16.2). İçerikte yazılan bir etiket etiket olmaz, metin olarak kalır.
- `TR_add_generation_prompt:` zorunludur: `TR_true` sona asistanın başlığını ekler (çıkarım), `TR_false` eklemez (eğitim).
- `TR_enable_thinking:` zorunludur. `TR_false` ile `TR_add_generation_prompt: TR_true` asistan başlığından sonra boş bir düşünce bloğu ekler; model düşünmeden cevap verir.
- `TR_tools:` araçların JSON şemalarıdır (`TR_List[TR_String]`, varsayılanı `[]`); her biri sistem iletisinin araç bloğuna olduğu gibi yazılır.
- **Düşünce:** son kullanıcı sorgusundan önceki asistan iletilerinin `TR_reasoning`'i yazılmaz; sonrakilerinki (aynı sorgunun araç adımları) yazılır. Son sorgu `TR_User` rolündeki son iletidir; araç sonuçları `TR_Tool` rolündedir. Seçim içeriğin metnine bakmaz.
- Ardışık `TR_Tool` iletileri tek bir kullanıcı turunda, her biri kendi `<tool_response>` bloğunda yazılır.
- Sistem iletisi ilk iletiyse ve araçlar varsa araç bloğuyla birleşir. Öbür her şey özgün şablonun yazdığı gibidir.
- Biçim duvar saatini ya da ortamı okumaz: aynı iletiler ve argümanlar her zaman aynı kimlikleri verir.

```tr
# TR 4
TR_import "tr/chat"
TR_var p = TR_load_tokenizer("qwen3/tokenizer.json")
TR_var iletiler = [TR_ChatMessage(TR_role: TR_Role.TR_User, TR_content: "Merhaba")]
TR_var kimlikler = TR_qwen3_format(p, iletiler, TR_add_generation_prompt: TR_true, TR_enable_thinking: TR_false)
```

- *Test:* gerekmez, henüz uygulanmadı. Uygulanınca TR testi: transformers'ın `apply_chat_template(..., tokenize=True)` kimlikleriyle birebir altın testler; tek tur, çok tur ve düşüncenin silinmesi, `enable_thinking=False`, araç tanımı, araç çağrısı ve ardışık araç sonuçları, boşlukla başlayıp biten içerik. Bilinen farklar ayrı testtir: içerikte etiket yazımı; içeriği `<tool_response>` ile sarılmış bir `TR_User` iletisi.

**24.3** `TR_qwen3_reply(metin)` modelin çözülmüş cevabını bir asistan iletisine çevirir: `TR_ChatMessage(TR_role: TR_Role.TR_Assistant, ...)`. `metin` `TR_special_tokens: TR_false` ile çözülmüştür (16.3).
- `<think>` ile `</think>` arasındaki metin `TR_reasoning`, `<tool_call>` blokları sırayla `TR_tool_calls`, geri kalanı `TR_content` olur. Böylece cevap geçmişe 24.2'nin beklediği biçimde konur.
- Her `<tool_call>` bloğunun içi `{"name": ..., "arguments": {...}}` biçiminde bir JSON nesnesidir; `TR_arguments` `arguments` nesnesinin modelin yazdığı JSON metnidir.
- Bozuk JSON, eksik alan ya da kapanmamış blok hatadır (8); ileti bloğun sırasını ve nedenini söyler. Ajan hatayı `TR_try` ile yakalar ve iletisini `TR_Tool` rolünde modele geri verir; ajanın döngüsü durmaz.
- *Test:* gerekmez, henüz uygulanmadı.

**24.4** Sohbet derlemi (ince ayar verisi) iletilere ayrıştırılıp biçim fonksiyonuyla kodlanır.
- Biçimi önceden uygulanmış, güvenilir bir düz metin derlemi kodlanacaksa `TR_special_tokens: TR_true` açıkça yazılır. `TR_false` etiket yazımlarını baytlara böler; model etiketin kimliği yerine baytlarını öğrenir (16.2).
- Güvenilmeyen metin (kullanıcı girdisi, araç sonucu, web sayfası) yalnız ileti içeriği olarak girer.
- *Test:* gerekmez, henüz uygulanmadı.
