package tokenize

import (
	"strings"
	"testing"
)

// Kural (README, tek adım bütçesi): ön bölücü zincirinde (Sequence) her adımın bütün parçalardaki
// toplam büyümesi bütçeden bir kez düşülür; dıştaki zincir aynı büyümeyi ikinci kez düşmez. Tek
// başına bütçe içinde kalan Metaspace, Sequence([WhitespaceSplit, Metaspace]) içinde de kalır.
// (Gemini parçalayıcı denetimi, Bulgu 1.)
func TestPreTokenizerSequenceChargesGrowthOnce(t *testing.T) {
	setAndRestore(t, RegexSettings{StepBase: 50, StepFactor: 1, BacktrackLimit: 1024})

	// 100 baytlık girdi ("a " * 50). Düzenli ifade sayısı 0 olduğundan
	// toplam adım bütçesi: StepBase + StepFactor * 100 * (0 + 1) = 150 adım.
	// Metaspace 100 baytlık girdiyi 200 bayta çıkarır (net büyüme: 100 bayt <= 150 bütçe).
	input := strings.Repeat("a ", 50)

	const singleMetaspaceJSON = `{
		"model": {"type": "WordLevel", "vocab": {"<unk>": 0, "▁a": 1}, "unk_token": "<unk>"},
		"pre_tokenizer": {"type": "Metaspace", "replacement": "▁", "prepend_scheme": "always", "split": true}
	}`
	pSingle, err := Parse([]byte(singleMetaspaceJSON))
	if err != nil {
		t.Fatalf("tekil Metaspace yüklenemedi: %v", err)
	}
	if _, err := pSingle.SafeEncode(input, Options{}); err != nil {
		t.Fatalf("tekil Metaspace 100 bayt büyütmeyle 150 adımlık bütçede hata vermemeliydi: %v", err)
	}

	// Aynı tekil Metaspace adımının Sequence içine sarılmış hâli (veya t5-small / xlm-roberta
	// biçimindeki WhitespaceSplit + Metaspace zinciri) yine toplam 200 bayt (100 bayt büyüme)
	// üretir; bütçe 150 adım olduğu hâlde preTokenizeStream büyütmeyi hem iç adımda hem dış
	// Sequence kapsayıcısında iki kez düştüğü için (200 adım > 150) H5289 ile yanlış alarm verir.
	const seqMetaspaceJSON = `{
		"model": {"type": "WordLevel", "vocab": {"<unk>": 0, "▁a": 1}, "unk_token": "<unk>"},
		"pre_tokenizer": {
			"type": "Sequence",
			"pretokenizers": [
				{"type": "WhitespaceSplit"},
				{"type": "Metaspace", "replacement": "▁", "prepend_scheme": "always", "split": true}
			]
		}
	}`
	pSeq, err := Parse([]byte(seqMetaspaceJSON))
	if err != nil {
		t.Fatalf("Sequence(WhitespaceSplit, Metaspace) yüklenemedi: %v", err)
	}
	if _, err := pSeq.SafeEncode(input, Options{}); err != nil {
		t.Fatalf("Sequence(WhitespaceSplit, Metaspace) 100 baytlık gerçek büyütmeyi mükerrer sayarak 150 adımlık bütçeyi yanlış alarmla aştı: %v", err)
	}
}

// Kural (README, tek adım bütçesi): bütçe sayacı ve zincir adımları çağrı başına bir kez kurulur;
// parça başına ayırma yapılmaz (Qwen3, BLOOM, SmolLM2 biçimi Sequence([Split, ByteLevel])). Parça
// başına kapanış ayırması kodlamayı %5-15 yavaşlatmıştı. (Gemini parçalayıcı denetimi, Bulgu 2.)
func TestPreTokenizerSequenceAllocatesPerCallNotPerPiece(t *testing.T) {
	// Qwen3 / BLOOM / SmolLM2 mimarisi: Sequence([Split, ByteLevel(lazy=true)]) + BPE.
	// 4a38c0d~2 sürümünde 2.000 sözcüklük önbellekli bir metnin kodlanması parça başına
	// hiç yığın (heap) ayırması yapmazken (< 50 toplam ayırma), 34065cd sonrasında
	// preTokenizeStream (pretokenizers.go:127-137) her sözcük parçası için sum, grown ve
	// count kapanışını yığında ayırarak > 4.000 ayırma üretir.
	const qwenStyleJSON = `{
		"model": {
			"type": "BPE",
			"vocab": {"Ġ": 0, "m": 1, "e": 2, "r": 3, "h": 4, "a": 5, "b": 6},
			"merges": []
		},
		"pre_tokenizer": {
			"type": "Sequence",
			"pretokenizers": [
				{"type": "Split", "pattern": {"String": " "}, "behavior": "MergedWithNext", "invert": false},
				{"type": "ByteLevel", "add_prefix_space": false, "trim_offsets": false, "use_regex": false}
			]
		}
	}`
	p, err := Parse([]byte(qwenStyleJSON))
	if err != nil {
		t.Fatalf("parçalayıcı yüklenemedi: %v", err)
	}
	text := strings.Repeat("merhaba ", 2000)
	// Önbelleği ısıt.
	_ = p.Encode(text)

	allocs := testing.AllocsPerRun(10, func() {
		_ = p.Encode(text)
	})
	if allocs > 100 {
		t.Fatalf("PreTokenizerSequence.stream parça başına yığın ayırması yapıyor: 2000 parçada %.0f ayırma (beklenen <= 100)", allocs)
	}
}
