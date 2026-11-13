# tokenize

`tokenizer.json` biçimindeki parçalayıcıları yükler ve çalıştırır: normalleştiriciler, ön
bölücüler, modeller (BPE, WordPiece, Unigram, WordLevel), son işleyiciler, çözücüler ve
kendi düzenli ifade motoru.

## Standartlar

**GGUF'ta ikinci kurucu yok.** GGUF dosyasında `tokenizer.json` yoktur; `FromGGUF` etiketten bellekte onun parçasını kurar ve `Parse`'tan geçirir: NFC, `ignore_merges` ve özel parçalar aynı yolda çözülür. Kalıplar modellerin kendi `tokenizer.json`'undandır, ölçüt modelin parçalayıcısıdır; llama.cpp'nin parçalayıcısı değil. Denetim: `TestGGUFQwen2TagBringsNFC`, `TestGGUFQwen2TagNeedsQwenArchitecture`, `TestGGUFDigitsFollowTheTag`, `TestGGUFLlamaBPEIgnoresMerges`, `TestGGUFBosAndEos`, `TestGGUFAddedTokens`, `TestGGUFQwen2SpecialNames`, `TestGGUFRejects`, `FuzzGGUFTokenizer`.

**Ailenin biçim parçaları (16.2).** `TR_special_tokens: TR_false` (`DontParseAdded`) metindeki hiçbir eklenmiş parçayı tanımaz; tek istisna, parçalayıcı Go'daki aile tablosundan (`family.go`) bir aileyle açıldıysa ailenin iki kümesinin özel olmayan parçalarıdır (yalnız boşluk karakterlerinden oluşanlar ve `^</?[a-z]+>$`'e uyanlar): onlar tek kimlik olur. Aile GGUF'ta (mimari, ön bölme etiketi), klasörde (`model_type`, `tokenizer_config.json`'un işareti) ile seçilir; tablo kitaplığın sohbet tablosuyla (`lib/model_family.tr`) aynı satırlardır. Küme kapalıdır: dosyanın kümesi ailenin resmî dosyasının sayısı ve karmasıyla tutmazsa parçalayıcı ailesiz açılır (fark `Mismatch`'te); küme bir protokol etiketiyle kesişirse yükleme durur (`FamilyOverlapError`, H7185). Ailesiz parçalayıcıda yalnız boşluk karakterlerinden oluşan özel olmayan eklenmiş parça varsa `TR_false` ile kodlama hatadır (`FamilylessError`: kümesi uymayan dosyada H7184, öbüründe H7178'in `alone`, `gguf` ya da `folder` kolu); böyle parça yoksa kapatma bugünkü gibidir. Denetim: `TestFormatPieceRules`, `TestSetHashMatchesPython`, `TestFamilyFormatPiecesAreOneIDUnderTRFalse`, `TestFamilyMismatchLeavesTheTokenizerFamilyless`, `TestFamilyOverlapIsAnError`, `TestFamilylessMessages`, `TestFamilylessErrorBranches`, `TestFamilyTableMatchesTheLibraryChatTable`, `TestEmptyWhitespaceSetFamilies`.

**Başvuruyla birebir.** Kodlama ve çözme, başvuru uygulamasının ürettiği altın çıktılarla
birebir aynıdır; desteklenmeyen bir bileşen sessizce yok sayılmaz, hata verir. Denetim: `TestSyntheticTokenizers`, `TestComponents`, `TestUnsupportedConfig`.

**Girdi güvenilmez.** Parçalayıcı dosyası da kodlanan metin de dışarıdan gelir. Düzenli ifade
motoru adım bütçesiyle çalışır; üstel ya da karesel büyüyebilecek desenler reddedilir. Hiçbir
girdi çökmeye yol açmaz (bulanıklaştırma testleriyle sınanır). Denetim: `TestRiskyPatternsRejected`, `TestStepLimitTriggers`, `FuzzRegexNoCrash`, `FuzzTokenizer`.

**Tek adım bütçesi: düzenli ifade, çıktı büyütmesi ve model araması.** Bir kodlama ya da çözme çağrısının (ve yüklemede ek tokenların normalleştirilmesinin) tek bir adım bütçesi vardır: `StepBase + StepFactor × bayt × (düzenli ifade sayısı + 1)`. Üç iş aynı bütçeden düşer: (1) düzenli ifade eşleştirme adımları; (2) normalleştirici, ön bölücü ve çözücü adımlarının girdisinden fazla ürettiği her bayt (bayt başına bir adım; küçülen ya da aynı boyda kalan adım ücretsizdir, düz metinli `Replace` üretmeden önce hesaplanır); (3) WordPiece sözlük aramaları ve Unigram ağaç ilerlemeleri (adım başına bir adım; WordPiece ayrıca sözlükteki en uzun tokenı aşan adayları aramaz). Bütçe aşılınca `*StepLimitError` döner (çıktı büyümesinde `H5289`, model aramasında `H5290`). Gerekçe: üç iş de girdiyle orantısız büyüyebiliyordu ve düzenli ifade bütçesinin dışındaydı (`Sequence` zincirinde 16 katlayan `Replace` adımları ya da iki katlayan `ByteLevel` adımları bir baytı üstel büyütür). Yeni bir sınır kavramı yoktur: var olan bütçe ve ayarlar (`SetRegexSettings`) geçerlidir; ölçülü büyütmeler (iki katı, bayt başına çevirme) varsayılan 100 kat payın çok altındadır. Ön bölücü zincirinde her adımın bütün parçalardaki toplam büyümesi bir kez düşülür (normalleştirici ve çözücü zincirleri gibi); sayaçlar çağrı başına kurulur, parça başına ayırma yoktur. Denetim: `TestPreTokenizerSequenceChargesGrowthOnce`, `TestPreTokenizerSequenceAllocatesPerCallNotPerPiece`, `TestChainOutputGrowthSharesTheStepBudget`, `TestModestGrowthStaysUnderTheDefaultBudget`, `TestWordPieceSearchIsBoundedByVocabularyAndBudget`, `TestUnigramSearchSharesTheBudget`.

**Önbellek sonucu değiştirmez.** Önbellekli ve önbelleksiz kodlama aynı sonucu verir; toplu ve
paralel kodlama tek tek kodlamayla aynıdır. Denetim: `TestUncachedAndLong`, `TestBatchAndParallel`.

**Geçersiz UTF-8 tanımlı biçimde işlenir.** Kayıplı çözme kuralı bellidir ve sınanır. Denetim: `TestLossyUTF8`.

## Testler

- `behavior_pretokenizer_budget_test.go`: ön bölücü zincirinde büyümenin bir kez düşülmesi ve parça başına ayırma yapılmaması
