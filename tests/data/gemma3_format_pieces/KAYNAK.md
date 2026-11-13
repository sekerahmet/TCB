# Gemma 3'ün biçim parçaları

`tests/veri_parcalayici_test.tr`'nin 16.2 testi, `internal/compute/tokenize/gguf_spm_test.go`, `tests/gemma3_chat_test.tr`'nin eksik etiket durumu (H7182) ve `tools/hf/gemma3_chat_reference.py` bu klasörü okur. Klasör Gemma 3 ailesinin biçim parçalarını (dil tanımı 16.2) gerçek kimlikleriyle taşır: TR'nin aile tablosu bu kümelerin sayısını ve karmasını bekler (`internal/compute/tokenize/family.go`).

- **Kaynak:** `unsloth/gemma-3-1b-it` deposunun `5b11413a10db4e486ef16a20101fd028f8f2499c` commit'indeki `tokenizer.json` (sha256 `4667f2089529e8e7657cfb6d1c19910ae71ff5f28aa7ab2ff2763330affad795`; `unsloth/gemma-3-4b-it` ve `-12b-it`'inki bayt bayt aynı). Google'ın deposu kapılıdır; `unsloth`'unki onun yeniden yayımı olarak bilinir (iki dosya karşılaştırılmadı).
- **Seçim:** dosyanın özel olmayan (`special: false`) eklenmiş parçalarından iki küme, 16.2'nin kuralıyla: boşluk kümesi (içeriği yalnız boşluk, `▁`, sekme, satır sonu ve satır başı olan 92 parça) ve desen kümesi (`^</?[a-z]+>$`'e uyan 59 parça). 151 parça `added_tokens`'ta ve dağarcıkta, kaynaktaki kimlik ve bayraklarıyla durur.
- **Parçalayıcının geri kalanı:** dağarcıkta ayrıca `a`, `b`, `▁` ve 256 bayt parçası (`<0x00>`–`<0xFF>`), kaynaktaki kimlikleriyle. Birleştirme yok, özel parça yok. Normalleştirici, ön ayırıcı ve çözücü kaynağınkiyle aynıdır.
- **`config.json`** (`model_type` `gemma3_text`) ve **`tokenizer_config.json`** (`tokenizer_class` `GemmaTokenizer`) klasörü Gemma 3 ailesiyle açar; ikisi de yalnız bu anahtarı taşır.
- **Kuran betik:** `tools/hf/gemma3_format_pieces.py`; komut `tools/hf/README.md`'de. Betik iki kümenin sayısını ve karmasını kaynakta ve fikstürde `family.go`'nunkilerle karşılaştırır.

**Lisans:** Gemma is provided under and subject to the Gemma Terms of Use found at ai.google.dev/gemma/terms. Bu klasörde Gemma'nın dosyasından yalnız yukarıdaki 151 kısa biçim parçası (boşluk dizileri ve HTML öğe adları) ve kimlikler vardır; Gemma'nın parçalayıcı dosyası, sohbet şablonu ve ağırlığı yoktur.
