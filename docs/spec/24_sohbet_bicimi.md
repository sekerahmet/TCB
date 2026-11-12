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

- *Test:* TR testi: yapı ve varsayılanlar (`tests/chat_format_test.tr`).

**24.2** `TR_qwen3_format(p, iletiler, TR_add_generation_prompt: b, TR_enable_thinking: d, TR_tools: araçlar)` Qwen3 ailesinin biçimiyle bir `TR_List[TR_Int]` verir; `p` modelin parçalayıcısıdır (16.1).
- **Kimlikle kurulur.** Biçimin etiketleri (`<|im_start|>`, `<|im_end|>`, `<think>`, `</think>`, `<tool_call>`, `<tool_response>` ve kapanışları) kimlikleriyle girer (`p.TR_token_id`). İki etiket arasındaki bütün metin, biçimin kendi metni ve iletinin içeriği birlikte, tek bir `p.TR_encode(metin, TR_special_tokens: TR_false, TR_boundary_tokens: TR_false)` çağrısıyla kodlanır (16.2): parça dizinin ortasındadır, şablonun sınır parçaları ona eklenmez. İçerikte yazılan bir etiket etiket olmaz, metin olarak kalır. Biçimin kendi metnindeki etiketler de (araç yönergesindeki `<tool_call></tool_call>` örneği) kimlikle girer; eklenmiş parça olmayan yazımlar (`<tools>`) metindir.
- `TR_add_generation_prompt:` zorunludur: `TR_true` sona asistanın başlığını ekler (çıkarım), `TR_false` eklemez (eğitim).
- `TR_enable_thinking:` `TR_add_generation_prompt: TR_true`'da zorunludur, `TR_false`'ta verilirse hatadır (yalnız asistan başlığını etkiler). `TR_false` asistan başlığından sonra boş bir düşünce bloğu ekler; model düşünmeden cevap verir.
- `TR_tools:` araçların JSON şemalarıdır (`TR_List[TR_JSON]`, varsayılanı `[]`). Biçim onları özgün şablonun JSON yazımıyla yazar: `", "` ve `": "` ayırıcıları, anahtarlar sırasıyla, ASCII dışı karakterler kaçırılmadan, sayılar Python'un `json.dumps` yazımıyla (`1e-05`). Bu yazım biçimin protokolüdür; `TR_json_string`'in (13.8) boşluksuz yazımı değildir.
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

- *Test:* TR testi (`tests/chat_format_test.tr`): transformers'ın `apply_chat_template(..., tokenize=True)` kimlikleriyle birebir altın testler; tek tur, çok tur ve düşüncenin silinmesi, `enable_thinking=False`, araç tanımı (ondalık sayılı, denetim karakterli ve tırnaklı açıklamalı bir şemayla), araç çağrısı ve ardışık araç sonuçları, boşlukla başlayıp biten içerik, sınırda birleşen satır sonu (metnin tek çağrıyla kodlanması); `TR_enable_thinking:`'in iki hatası. Bilinen farklar ayrı testtir: içerikte etiket yazımı; içeriği `<tool_response>` ile sarılmış bir `TR_User` iletisi.

**24.3** `TR_qwen3_reply(p, kimlikler)` modelin ürettiği kimlikleri bir asistan iletisine çevirir ve `(ileti, hatalar)` demetini verir: `TR_ChatMessage(TR_role: TR_Role.TR_Assistant, ...)` ve `TR_List[TR_String]`.
- **Kimliklerden ayrılır.** Bloklar etiket kimliklerinde (`<think>`, `</think>`, `<tool_call>`, `</tool_call>`; `p.TR_token_id`) bölünür, aradaki parçalar `p.TR_decode` ile çözülür (16.3). Baytlarıyla yazılmış bir etiket (güvenilmeyen bir metinden yinelenen `<tool_call>`) `TR_content`'te metin olarak kalır, çağrı olmaz.
- Düşünce bloğu `TR_reasoning`, `<tool_call>` blokları sırayla `TR_tool_calls`, geri kalanı `TR_content` olur. Sınır boşlukları özgün şablonun koyduğu kadar kırpılır: düşüncenin iki ucundaki satır sonları, içeriğin başındaki satır sonları, araç çağrısından önceki tek satır sonu.
- Her `<tool_call>` bloğunun içi `{"name": ..., "arguments": {...}}` biçiminde bir JSON nesnesidir; `TR_arguments` `arguments` nesnesinin modelin yazdığı JSON metnidir.
- **Bozuk blok** (bozuk JSON, eksik alan, kapanmamış araç ya da düşünce bloğu; en uzun sınıra takılan bir üretim düşüncenin ortasında kesilebilir) çağrı ya da düşünce olmaz: modelin yazdığı metin `TR_content`'te olduğu gibi kalır ve `hatalar`'a bloğun sırasını ve nedenini söyleyen bir ileti girer. Ajan önce `ileti`'yi geçmişe ekler, sonra her hatayı `TR_Tool` rolünde bir iletiyle modele verir: model kendi bozuk çağrısını ve hatasını birlikte görür. Demet açılmadan ileti olarak kullanılamaz (3.6a); hataları modele vermek ajanın işidir, örnekteki gibi.
- **Gidiş-dönüş sözü:** bozuk bloğu olmayan ve kanonik olan bir cevapta `TR_qwen3_format(p, geçmiş + [ileti], TR_add_generation_prompt: TR_false)`'ın sonu, modelin ürettiği kimlikleri ve ardından gelen `<|im_end|>`'i aynen içerir. Kanonik cevap, kimlikleri aradaki metnin `p.TR_encode`'uyla ve şablonun boşluklarıyla aynı olan cevaptır. Model aynı metni başka bir bölmeyle (gpt2'de `"\n\n"` için `[198, 198]`, kanonik `[628]`) ya da başka boşlukla üretirse geçmiş modelin metnini korur, kimliklerini değil.
- Sesli bir döngüde düşünce ve araç blokları sese verilmez; akışta (16.3a) bunlar etiket kimliği gelince bilinir.

```tr
# TR 4
TR_import "tr/chat"
TR_var p = TR_load_tokenizer("qwen3/tokenizer.json")
TR_var geçmiş = [TR_ChatMessage(TR_role: TR_Role.TR_User, TR_content: "Hava nasıl?")]
TR_var üretilen = [151667, 198, 151668]
TR_var cevap, hatalar = TR_qwen3_reply(p, üretilen)
geçmiş.TR_append(cevap)
TR_for h TR_in hatalar {
    geçmiş.TR_append(TR_ChatMessage(TR_role: TR_Role.TR_Tool, TR_content: h))
}
```

- *Test:* TR testi (`tests/chat_format_test.tr`): gidiş-dönüş sözü (içerikli, düşünceli, araç çağrılı cevap); kanonik olmayan bir cevap (söz dışında, metin korunur); baytla yazılmış `<tool_call>`; bozuk araç bloğu; kapanmamış düşünce.

**24.4** Sohbet derlemi (ince ayar verisi) iletilere ayrıştırılıp biçim fonksiyonuyla kodlanır.
- `TR_read_chat(metin)` yaygın JSON biçimindeki bir konuşmayı `TR_List[TR_ChatMessage]` olarak okur. `metin` bir ileti listesidir (`[{"role": "user", "content": ...}, ...]`) ya da onu `"messages"` alanında taşıyan bir nesnedir. Asistanda `reasoning_content` ve `tool_calls` okunur.
  - Rol metni (`"system"`, `"user"`, `"assistant"`, `"tool"`) `TR_Role`'e çevrilir; başka rol hatadır, ileti geçerli rolleri söyler.
  - Araç çağrısı düz (`{"name", "arguments"}`) ya da OpenAI biçiminde iç içe (`{"type": "function", "function": {...}}`) olabilir. Metin olan `arguments` olduğu gibi kalır, nesne olan 24.2'nin JSON yazımıyla metne çevrilir.
- Biçimi önceden uygulanmış, güvenilir bir düz metin derlemi kodlanacaksa `TR_special_tokens: TR_true` açıkça yazılır; biçim BOS'u kendisi taşıyorsa `TR_boundary_tokens: TR_false` da (16.2: yoksa çift BOS hatası). `TR_false` etiket yazımlarını baytlara böler; model etiketin kimliği yerine baytlarını öğrenir (16.2).
- Güvenilmeyen metin (kullanıcı girdisi, araç sonucu, web sayfası) yalnız ileti içeriği olarak girer.

```tr
# TR 4
TR_import "tr/chat"
TR_var p = TR_load_tokenizer("qwen3/tokenizer.json")
TR_for satır TR_in TR_read_lines("sohbetler.jsonl") {
    TR_var iletiler = TR_read_chat(satır)
    TR_print(TR_qwen3_format(p, iletiler, TR_add_generation_prompt: TR_false).TR_length)
}
```

- *Test:* gerekmez, henüz uygulanmadı.

**24.5** `TR_llama3_format(p, iletiler, TR_add_generation_prompt: b, TR_date: tarih, TR_tools: araçlar)` Llama 3.1 ve 3.2 instruct biçimiyle bir `TR_List[TR_Int]` verir.
- **Kimlikle kurulur** (24.2 gibi): `<|begin_of_text|>`, `<|start_header_id|>`, `<|end_header_id|>`, `<|eot_id|>` kimlikleriyle girer; aradaki metin tek `p.TR_encode(metin, TR_special_tokens: TR_false, TR_boundary_tokens: TR_false)` çağrısıyla.
- **BOS'u biçim taşır:** dizi `<|begin_of_text|>` ile başlar. Parçalayıcının sınır parçası eklenmez (`TR_boundary_tokens: TR_false`). Bugünkü motor `TR_false`'ta sınır parçasını zaten kapatıyor; `TR_boundary_tokens:` motorda yok (`docs/status.md`, 16.2'nin Tur 5 satırı); `TR_boundary_tokens:` gelince çağrı onu açıkça yazar, `llama-bpe`'nin `addBOS: true`'su (`gguf.go:75`) çift BOS vermez.
- **Parçalayıcı:** dört etiketten biri dağarcıkta yoksa (Llama olmayan parçalayıcı) çalışma hatasıdır (H7164), ileti eksik etiketi ve biçimin adını söyler.
- **Sistem başlığı her zaman yazılır:** `system` başlığı, araç varsa "Environment: ipython\n", sonra "Cutting Knowledge Date: December 2023\n", "Today Date: " + tarih + "\n\n", ilk iletiyse sistem iletisinin içeriği, `<|eot_id|>`.
- `TR_date:` zorunludur, `TR_String`, biçimi "26 Jul 2024" (iki haneli gün, 01–31; İngilizce üç harfli ay; dört haneli yıl; şablonun `strftime("%d %b %Y")`'ı); başka biçim hatadır (H7165). Biçim duvar saatini okumaz (24.2 ile aynı söz).
- **İçerik kırpılır:** şablon sistem, kullanıcı ve asistan iletilerinin içeriğini `| trim` ile yazar: iki uçtaki boşluk, Python'un `str.strip()`'i (Unicode boşlukları ve U+001C–U+001F); biçim de öyle yapar. Bu Qwen3'ten farklıdır. `TR_Tool` iletisinin içeriği kırpılmaz (aşağıda).
- `TR_tools:` varsa şablonun öntanımlısı gibi (`tools_in_user_message = true`) ilk kullanıcı iletisine yazılır: sabit yönerge metni, her şema `tojson(indent=4)` yazımıyla ve "\n\n", sonra iletinin içeriği. Sistemden sonra ileti yoksa ya da sistemden sonraki ilk ileti `TR_User` değilse hatadır (H7166): şablon o iletiyi rolüne bakmadan kullanıcı iletisi sayar (yoksa `raise_exception`); biçim rolü sessizce değiştirmez.
- Asistanın araç çağrısı: `{"name": "<ad>", "parameters": <argümanlar>}` ve `<|eot_id|>`; argümanlar şablonun `tojson` yazımıyla. Birden çok çağrı hatadır (şablon: "This model only supports single tool-calls at once!", H7167). Çağrılı iletinin `TR_content`'i boştur: şablon çağrılı iletide içeriği yazmaz; dolu içerik hatadır (H7176), sessizce atılmaz.
- `TR_Tool` iletisi `ipython` başlığıyla yazılır; içerik **JSON metni olarak tırnaklanır**: şablon `message.content is mapping or message.content is iterable` ile `tojson` uygular ve Jinja'da metin de "iterable"dır.
- `TR_reasoning` dolu bir asistan iletisi hatadır: ailenin düşünce bloğu yok (H7168); sessizce atılmaz.

```tr
# TR 4
TR_import "tr/chat"
TR_var p = TR_load_tokenizer("llama3/tokenizer.json")
TR_var iletiler = [TR_ChatMessage(TR_role: TR_Role.TR_User, TR_content: "Merhaba")]
TR_var kimlikler = TR_llama3_format(p, iletiler, TR_add_generation_prompt: TR_true, TR_date: "26 Jul 2024")
```

- *Test:* TR testi (`tests/llama3_chat_test.tr`): transformers `apply_chat_template(..., date_string=..., tokenize=True)`'ın kimlikleriyle birebir, 3.1 ve 3.2 şablonlarında ayrı ayrı: tek tur, tarih, sistemli ve sistemsiz, çok tur, ortadaki sistem iletisi, araçlı (tools ilk kullanıcı iletisinde), araç çağrısı, araç sonucu, boşlukla başlayan içerik (U+001C–U+001F dahil); hatalar: eksik etiket, yanlış tarih, araçlı ve kullanıcısız, araçlı ve ilk iletisi asistan, iki çağrı, içerikli çağrı, düşünce; içerikte yazılan etiket metin kalır.

**24.6** `TR_llama3_reply(p, kimlikler, TR_tools: araçlar)` üretilen kimlikleri `(ileti, hatalar)`'a çevirir (24.3 gibi).
- Sondaki durma parçası (`<|eot_id|>`, `<|eom_id|>`) atılır, kalan `p.TR_decode` ile çözülür.
- **Araç çağrısı:** Llama 3'te çağrının etiketi yoktur. Ölçüt kırpılmış içeriktir (iki uçtaki boşluk atılır; şablonun `| trim`'i). Kırpılmış içerik baştan sona tek bir JSON nesnesiyse, anahtarları tam olarak `name` ve `parameters`'sa, `name` `TR_tools:`'taki bir aracın adıysa (şemanın `function.name`'i ya da `name`'i) ve `parameters` bir nesneyse çağrı olur (`TR_arguments` `parameters`'ın metni, `TR_content` boş). Çağrı değilse `TR_content` çözülen metnin kendisidir. `TR_tools:` boşsa çağrı aranmaz.
- `TR_tools:` doluyken içerikte üst düzeyinde `name` anahtarı olan bir JSON nesnesi bulunan (anahtarların sırası önemsizdir) ya da `{`, Unicode boşlukları (Unicode'un White_Space özelliğindeki her karakter: boşluk, sekme, satır sonu, satır başı, dikey sekme, sayfa sonu, U+0085, U+00A0 ve öbürleri), `"name"`, Unicode boşlukları ve `:` geçen ama çağrı sayılmayan her cevap metin olarak kalır ve `hatalar`'a ileti girer: JSON olarak çözülmeyen içerik, aracı listede olmayan ad, `name` ve `parameters`'tan başka anahtar taşıyan nesne, `parameters` yerine `arguments` yazan nesne (ileti doğru anahtarı söyler), JSON nesnesinden önce ya da sonra metin taşıyan cevap. Çağrının ve nesnenin kendi çözümü JSON'un dört boşluğuyla (boşluk, sekme, satır sonu, satır başı) yapılır.
- `<|python_tag|>` (3.1'in yerleşik araçları) desteklenmez: içerikte görünürse (kimliğiyle ya da yazımıyla) metin olarak kalır, `hatalar`'a ileti girer; kimliği varsa içerik özel parçalarıyla çözülür, etiket metinde görünür.
- Gidiş-dönüş sözü 24.3'teki gibi.

```tr
# TR 4
TR_import "tr/chat"
TR_var p = TR_load_tokenizer("llama3/tokenizer.json")
TR_var araçlar: TR_List[TR_JSON] = [TR_json_parse("{\"type\": \"function\", \"function\": {\"name\": \"saat\", \"parameters\": {}}}")]
TR_var üretilen = p.TR_encode("{\"name\": \"saat\", \"parameters\": {}}", TR_special_tokens: TR_false)
TR_var cevap, hatalar = TR_llama3_reply(p, üretilen, TR_tools: araçlar)
```

- *Test:* TR testi (`tests/llama3_chat_test.tr`): transformers'ın 3.1 ve 3.2 şablonlarıyla gidiş-dönüş (içerikli cevap, araç çağrılı cevap: `TR_llama3_format(p, geçmiş + [ileti], TR_add_generation_prompt: TR_false, ...)`'ın sonu üretilen kimlikleri ve `<|eot_id|>`'yi aynen içerir); `TR_tools:` boşken çağrı biçimli JSON metin kalır; aracı listede olmayan ad, arguments yazan nesne, fazladan anahtarlı nesne ve bozuk JSON metin kalır ve `hatalar`'a girer; JSON'dan önce cümle taşıyan çağrı biçimli cevap (Tamam: {"name": ...}) metin kalır ve `hatalar`'a girer; aynı cevap boşluklu (Tamam: { "name": ...}) ve satır sonlu (`{` ile "name" arasında satır sonu) yazımıyla da metin kalır ve `hatalar`'a girer; ters sıralı yanlış çağrı ({"parameters": {...}, "name": "yok"}, ad listede yok) metin kalır ve `hatalar`'a girer; önünde metin olan ters sıralı çağrı (Tamam: {"parameters": {"şehir": "Ankara"}, "name": "hava"}, `hava` listede) metin kalır ve `hatalar`'a girer; dizi içindeki ters sıralı nesne ([{"parameters": {}, "name": "yok"}]) metin kalır ve `hatalar`'a girer; `{` ile "name" arasında dikey sekme ya da U+00A0 varsa ve içerik JSON olarak çözülmüyorsa cevap metin kalır ve `hatalar`'a girer; en çok token sınırında kesilmiş çağrı ({"name": "hava", "parameters": {"şehir": "Ank) metin kalır ve `hatalar`'a girer; iki uçta boşluklu çağrı (kırpma); `;` ile ayrılmış iki çağrı metin kalır ve `hatalar`'a girer; içerikte `<|python_tag|>`.
