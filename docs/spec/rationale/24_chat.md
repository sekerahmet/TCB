# 24. Sohbet biçimi: emsal ve gerekçe

Kurallar: [`24_sohbet_bicimi.md`](../24_sohbet_bicimi.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

Araştırma: Gemini Jarvis B (2026-11-02, taban `b2892d87`). Raporu DENETİM doğruladı (istek 42), kararları ikinci kez okudu ve ölçtü (istek 43). Kaynak etiketleri 21'in gerekçe dosyasındaki gibidir: **[DENETİM açtı]** ve **[rapordan]**.

Karşılaştırma:

| Konu | transformers v4.51.3 | llama.cpp | Ollama v0.6.2 | TGI v3.2.1 | TR |
| :-- | :-- | :-- | :-- | :-- | :-- |
| Şablon | modelin Jinja şablonu, `ImmutableSandboxedEnvironment` | eski sürümde `<\|im_start\|>` görünce düz ChatML; sonra C++ Jinja yorumlayıcısı | şablonu düzenleme mesafesiyle gömülü Go şablonlarına eşler | minijinja ve Python yöntemleri eklentisi, bazı şablonlara yama | `tr/chat`'te TR fonksiyonu; şablon çalışmaz (24.2) |
| İleti | sözlük listesi | C yapısı | Go yapısı | Rust yapısı | yapı ve durumlu rol (24.1) |
| Tarih | `strftime_now`: duvar saati | `std::time` | — | — | duvar saati okunmaz (24.2) |
| İçerikteki etiket | kimlik olur | kimlik olur | — | — | metin kalır (16.2) |

### 24.1

- *Emsal:* transformers iletileri `{"role": ..., "content": ...}` sözlükleriyle alır [rapordan]. Ollama `api.Message` (`Role`, `Content`, `ToolCalls`), TGI Rust yapısı kullanır [rapordan]. Qwen3'ün şablonu düşünceyi `reasoning_content` alanından ya da içerikteki `</think>`'ten alır, araç çağrısının `arguments`'ını metinse olduğu gibi, değilse `tojson` ile yazar [DENETİM açtı: Qwen3 `tokenizer_config.json`'un `chat_template`'i].
- *Gerekçe:* sözlük homojendir (3.7); içerik metin, araç çağrıları liste olunca tek bir değer tipi yetmez, `TR_JSON` ise tip denetimini kaybettirir. Yapı alan adlarını denetler: `"content"` yerine `"text"` yazmak denetim hatası olur. Rol değerleri modelin protokol verisidir (`<|im_start|>user`), ama metin olarak yazılırsa `"kullanıcı"` ya da `"human"` gibi bir yazım ancak çalışırken yakalanır; durumlu tip onu denetime çevirir (DENETİM istek 42, Konu 4, öneri 6). Protokol metni biçim fonksiyonunun içindedir, programın metninde değil (kimlik kodlu dil). Araç argümanları metin olarak durur, çünkü her aracın şeması ayrıdır ve Qwen3 metin argümanı olduğu gibi yazar: geçmişteki çağrı modelin yazdığı kimliklere döner. Yapıların dışa açık olması kullanıcının kendi modeline biçim yazmasını sağlar (DENETİM istek 42, §5d).
- Eğitimin asistan maskesi (hangi kimliklerin kayba girdiği) bugün kullanan bir yer olmadığı için yok; ince ayar adımıyla gelir.

### 24.2

- *Emsal:* Jinja korumalı alanından kaçışlar: CVE-2024-56326 ve CVE-2024-56201 (jinja2 3.1.5'te düzeldi), CVE-2025-27516 (3.1.6'da düzeldi); llama-cpp-python'da korumasız Jinja ile model dosyasından kod çalıştırma: CVE-2024-34359, CVSS puanı 9,6 [DENETİM açtı: GHSA kayıtları]. transformers şablonu metne çevirip `add_special_tokens=False` ile kodlar; Llama 3.2'nin şablonu da `bos_token` yazdığı için aksi çift BOS verir [rapordan]. Qwen3: `enable_thinking` kapalıyken `<think>\n\n</think>\n\n`; geçmiş turun düşüncesi `content.split('</think>')[-1].lstrip('\n')` ile silinir; son sorgu `ns.last_query_index` ile, içeriği `<tool_response>` ile başlayıp `</tool_response>` ile biten `user` iletisi sayılmadan seçilir [DENETİM açtı: Qwen3-8B `chat_template`, depo `b968826d`]. Hugging Face eklenmiş parçalarda böler, aradaki metni tek parça ön ayırıcıdan geçirir: gpt2'de `"Answer: Hi"` `[33706, 25, 15902]`, ayrı kodlanınca `[33706, 25, 220, 17250]` [ölçüldü: DENETİM istek 43, tokenizers 0.23.2]. transformers'ın `strftime_now`'ı ve llama.cpp'nin şablon yorumlayıcısı duvar saatini okur [rapordan].
- *Gerekçe:* model dosyası dışarıdan gelir; onun şablonunu çalıştırmak dışarıdan gelen kodu çalıştırmaktır ve dört CVE bunun bedelini gösterir. Biçim kimliklerden kurulunca içerik bir etiket üretemez (16.2): kullanıcının ya da bir aracın metnindeki `</tool_response>` sarmalı erkenden kapatamaz. Etiketler arası metnin tek çağrıyla kodlanması özgün uygulamayla aynı kimlikleri verir (DENETİM istek 43, Ş1); parça parça kodlamak sınırda başka parçalar üretir. Duvar saati okunmaz, çünkü aynı program ertesi gün başka bir istem kurardı (CLAUDE.md, "Güvenilir"); tarih isteyen bir biçim (Llama 3) geldiğinde tarihi zorunlu argümanla alır.
- `TR_add_generation_prompt:` zorunludur: transformers'ın varsayılanı `False`'tur ve çıkarımda unutulursa model asistan olarak değil kullanıcının cümlesini sürdürerek yazar [rapordan]. `TR_enable_thinking:` de zorunludur: Qwen3'ün varsayılanı düşünmektir; sesli bir yardımcıda istenmeyen gecikme sessizce gelir.
- **Son sorgu rolle seçilir.** Özgün şablon araç sonucunu içeriğin metninden tanır; TR rolden tanır, çünkü kullanıcının yazdığı bir metin hangi düşüncenin silineceğini değiştirmemeli (16.2'nin ilkesi: içerik yalnız metindir). Bilinen fark: içeriği `<tool_response>` ile sarılmış bir `TR_User` iletisi; altın testte ayrı bir durumdur (DENETİM istek 43, K13).
- **İçerikteki etiket yazımı** da bilinen bir farktır: TR baytlarını, Hugging Face kimliğini verir (16.2).
- Araç şemaları `TR_JSON` olarak verilir ve biçim onları şablonun yazımıyla yazar. Şablon `{{- tool | tojson }}` ve `tool_call.arguments | tojson` kullanır; transformers'ın `tojson`'u `json.dumps(x, ensure_ascii=False, ...)`'tur, ayırıcıları `", "` ve `": "`'dir [DENETİM açtı: Qwen3-8B `chat_template`, `b968826d`; transformers 5.17.0 `utils/chat_template_utils.py`]. `TR_json_string` boşluksuz yazar (13.8); şemayı onunla kuran ajan modelin eğitildiğinden sessizce ayrılan bir istem verirdi (DENETİM istek 44, Ş2). Yazım rol adları gibi biçimin protokolüdür, onun içinde durur; genel bir JSON seçeneği bugün başka bir yerde kullanılmıyor. `TR_enable_thinking:` şablonda yalnız üretim başlığının dalında okunur; eğitimde verilmesi hatadır, ajan etkisini sanmasın diye (DENETİM istek 44, K7).
- İlk biçim yalnız Qwen3'tür: Jarvis'in aday modelleri bu ailededir [kanıtlı: `docs/roadmap.md`, "Model seçimi"]. Başka aileler aynı yapılarla, kendi altın testleriyle eklenir.

### 24.3

- *Emsal:* Qwen3'ün şablonu modele çağrıyı `<tool_call>\n{"name": <function-name>, "arguments": <args-json-object>}\n</tool_call>` biçiminde yazdırır [rapordan: Qwen3 `tokenizer_config.json`]. llama.cpp `common/chat.cpp`'de aileye göre ayrıştırıcılar taşır [rapordan].
- *Gerekçe:* araç kullanan ajan amacın işlerindendir (CLAUDE.md, "Uzaktaki modeli kullanmak") ve Jarvis bir araç ajanıdır. Cevap geçmişe geri konacağı için düşünce, çağrılar ve metin aynı fonksiyonda ayrılır: düşünceyi `TR_content`'te bırakmak 24.2'nin silme kuralını boşa çıkarırdı.
- **Kimliklerden ayırmak** 16.2'nin girdide kapattığı kanalı çıktıda da kapatır: güvenilmeyen bir sayfadaki `<tool_call>` modele baytlarıyla girer; model onu yinelerse yine bayt üretir ve çözülmüş metinde gerçek etiketten ayırt edilemez (DENETİM istek 44, Ş1a; modelin yinelemesi ölçülmedi, ayırt edilemezlik 16.3'ten).
- **Sınır boşlukları:** şablon düşünceyi `reasoning_content.strip('\n') + '\n</think>\n\n' + content.lstrip('\n')` diye, araç çağrısını içerik varsa önüne `'\n'` koyarak yazar [DENETİM açtı: Qwen3-8B `chat_template`]. Kırpma bu kadardır; gidiş-dönüş sözü bunun sınanmasıdır (Ş1b; şablondan çıkarım, test ölçer).
- **Bozuk blok hata değil, demetin ikinci öğesidir:** hata fırlatılsaydı cevap geçmişe girmez, model kendi bozuk çağrısını görmeden "blok bozuk" iletisini alırdı (Ş1c). Bozuk metin içerikte kaldığı için geçmiş modelin yazdığıdır; demet açılmadan ileti olarak kullanılamaz. Hataları modele vermek ajanın işidir; dil bunu zorlamaz, çünkü TR'de kullanılmayan değişken hatası yok (DENETİM istek 45, D2). JSON 13.8 ile okunur.

### 24.4

- *Emsal (okuma):* sohbet derlemleri yaygın olarak `{"role": ..., "content": ...}` iletileriyle, asistanda nesne olarak `arguments` ile tutulur [hipotez: HF veri kümelerinin alışılmış biçimi, açılmadı]. 13.8'in `TR_json_parse[T]`'si durumlu tipi okumaz ve nesneyi metin alanına koymaz [kanıtlı: `13_yerlesikler.md` 13.8]; `TR_read_chat` bu iki çevirmeyi biçimin yanında yapar, rol metni de orada durur (DENETİM istek 44, Ş3).
- *Emsal:* tiktoken metindeki özel parçayı varsayılanla hata sayar; `encode_ordinary` düz metin yapar [DENETİM açtı: tiktoken 0.14.0 `core.py`].
- *Gerekçe:* 16.2'nin genişlemesi (DENETİM istek 43, Ş2) biçimi önceden uygulanmış düz metin derlemini bozar: `TR_false` ile `<think>` baytlara bölünür ve model Qwen3'ün düşünce kimliği yerine baytları öğrenir, sessizce. Yapılandırılmış veri (ileti listesi) biçim fonksiyonundan geçince sorun kalmaz. Düz metin derlemi güvenilirse onu bilerek `TR_true` ile kodlamak açık bir yazımdır.
