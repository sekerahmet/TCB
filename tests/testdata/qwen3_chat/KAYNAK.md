# Qwen3 sohbet biçiminin altın testi

`tests/chat_format_test.tr` ve `examples/tr/53_sohbet_bicimi.tr` bu klasörü okur.

- **`tokenizer.json`, `tokenizer_config.json`:** `tools/hf/chat_reference.py --tokenizer-out` ile bu depo için kuruldu: `tokenizers` 0.23.2'nin BPE eğiticisi betikteki küçük bir metinle, 600 parçalık dağarcık. Ön ayırıcı Qwen2'nin deseni ve bayt düzeyi, normalleştirici NFC: Qwen3'ün `tokenizer.json`'u gibi. Qwen3'ün etiketleri eklenmiş parçadır: `<|endoftext|>`, `<|im_start|>`, `<|im_end|>` özel; `<tool_call>`, `</tool_call>`, `<tool_response>`, `</tool_response>`, `<think>`, `</think>` özel değil, yayımlanmış parçalayıcıdaki gibi. Dağarcık ve birleştirmeler bu deponundur, yayımlanmış bir modelden kopyalanmaz. Aynı sürümle yeniden kurmak aynı baytları verir.
- **`cases.json`:** `tools/hf/chat_reference.py --cases-out`'un bu parçalayıcıyla verdiği durumlar: iletiler, argümanlar ve transformers 5.17.0'ın `apply_chat_template(..., tokenize=True)` kimlikleri. `known_` ile başlayan iki durum dil tanımı 24.2'nin bilinen farklarıdır; TR'nin kimlikleri başkadır, test farkı sınar.
- **Şablon:** Qwen/Qwen3-4B'nin `tokenizer_config.json`'undaki `chat_template`, revizyon `1cfa9a7208912126459214e8b04321603b3df60c` (2025-07-26), sha256 `a55ee1b1660128b7098723e0abcd92caa0788061051c62d51cbe87d9cf1974d8`. Şablon depoya kopyalanmaz; betik onu `--template` ile verilen dosyadan okur, karmasını `cases.json`'un `meta`'sına yazar. Qwen3-4B'nin bir GGUF'undaki `tokenizer.chat_template` (sha256 `57f1fd00f0013a2be96aa79b857391f27e23df5b5f847072b524c897e24d0361`) daha eski bir sürümdür: sarılı kullanıcı iletisini `startswith`/`endswith` yerine `in` ile tanır, metin olmayan içeriği boş saymaz; öbür yazımı aynıdır.
- Yeniden üretmenin komutları `tools/hf/README.md`'dedir.

**Lisans:** parçalayıcı ve `cases.json` bu deponun lisansındadır. Kimlikleri hesaplayan yazılım `transformers` ve `tokenizers`'tır (Apache 2.0); şablon Qwen3'ündür (Apache 2.0).
