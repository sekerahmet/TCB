package tokenize

import (
	"sort"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// GGUF'ta SentencePiece parçalayıcısı (dil tanımı 16.1): tokenizer.ggml.model "llama". Yalnız Gemma 3'te
// (general.architecture "gemma3") okunur: önek ve normalleştirme ailelerde ayrıdır, ölçülmeden başka
// mimariye açmak sessiz yanlış olurdu. Dosyada birleştirme yoktur, puan vardır; birleştirmeler Hugging
// Face'in SentencePieceExtractor'ının kuralıyla puanlardan kurulur (spmMerges) ve parçalayıcı GPT-2 yolu
// gibi bellekte kurulan tokenizer.json'dan Parse ile açılır. Sabitler Gemma 3'ün tokenizer.json'undandır
// (unsloth/gemma-3-1b-it @5b11413a): normalleştirici, ön ayırıcı, çözücü ve BPE'nin byte_fallback'i ve
// fuse_unk'u. Önek eklenmez (dosyada add_space_prefix yanlış). Eklenmiş parçalar (tür 3 ve 4) ham metinde,
// normalleştirmeden önce eşlenir: GGUF'ta boşluk parçaları ham boşlukla yazılıdır ("  ", kimlik 138).

// ggufModelSPM SentencePiece dosyasının tokenizer.ggml.model değeridir (llama.cpp'nin adı).
const ggufModelSPM = "llama"

// ggufSPMArch SentencePiece dosyasının okunduğu tek mimaridir.
const ggufSPMArch = "gemma3"

// ggufTokenNormal normal parçanın türüdür (llama.h, LLAMA_TOKEN_TYPE_NORMAL).
const ggufTokenNormal = 1

// spmTags SentencePiece dosyasının etiketleridir: Gemma 3'ün dosyası "default" yazar.
var spmTags = map[string]ggufTag{
	"default": {
		normalizer:   `{"type":"Replace","pattern":{"String":" "},"content":"▁"}`,
		preTokenizer: `{"type":"Split","pattern":{"String":" "},"behavior":"MergedWithPrevious","invert":false}`,
		decoder: `{"type":"Sequence","decoders":[{"type":"Replace","pattern":{"String":"▁"},"content":" "},` +
			`{"type":"ByteFallback"},{"type":"Fuse"}]}`,
		spm: true,
	},
}

// spmTagOf SentencePiece dosyasının mimarisini ve etiketini denetler.
func spmTagOf(f *gguf.File) (ggufTag, error) {
	if arch, _ := f.Values[ggufKeyArch].Data.(string); arch != ggufSPMArch {
		return ggufTag{}, messages.New(messages.H6678, ggufModelSPM)
	}
	name, ok := f.Values[ggufKeyPre].Data.(string)
	if !ok {
		return ggufTag{}, messages.New(messages.H6681, ggufKeyPre)
	}
	tag, known := spmTags[name]
	if !known {
		return ggufTag{}, messages.New(messages.H6679, gguf.Clip(name))
	}
	return tag, nil
}

// ggufMerges dosyanın birleştirmeleridir: GPT-2 dosyasında tokenizer.ggml.merges, SentencePiece dosyasında
// puanlardan kurulanlar (puan dizisi zorunludur; boyunu ggufTokenKinds denetledi).
func ggufMerges(f *gguf.File, tag ggufTag, tokens []string, kinds []int32) (any, error) {
	if !tag.spm {
		merges, ok := f.Values[ggufKeyMerges].Data.([]string)
		if !ok {
			return nil, messages.New(messages.H6681, ggufKeyMerges)
		}
		return merges, nil
	}
	scores, ok := f.Values[ggufKeyScores].Data.([]float32)
	if !ok {
		return nil, messages.New(messages.H6681, ggufKeyScores)
	}
	return spmMerges(tokens, scores, kinds), nil
}

// spmMerges SentencePiece'in parçalarından BPE birleştirmelerini kurar (Hugging Face'in
// SentencePieceExtractor'ının kuralı): her normal parçanın iki normal parçaya her bölünüşü (kod noktası
// sınırında) bir birleştirmedir; sıra birleşen parçanın puanına göre azalan, eşit puanda dosyadaki sıra,
// parçanın içinde (sol kimlik, sağ kimlik). Yalnız normal parçalar (tür 1): denetim ve eklenmiş parçalar
// metinden birleştirmeyle doğmaz. Birleştirme [sol, sağ] dizisidir: parçada ham boşluk olabilir.
// Koşu 4a'da (D1 Gemma 3 tasarımı 1.7) puan sıralı SPM birleştirmesi tokenizer.json'la 64 metinde aynıydı;
// bu kural o birleştirmeyle 12B'nin sözlüğünde 81 metinde aynı.
func spmMerges(tokens []string, scores []float32, kinds []int32) [][2]string {
	normal := make(map[string]int, len(tokens))
	for id, t := range tokens {
		if kinds[id] == ggufTokenNormal {
			normal[t] = id
		}
	}
	type split struct {
		left, right int
		score       float32
	}
	var all, local []split
	for id, t := range tokens {
		if kinds[id] != ggufTokenNormal {
			continue
		}
		local = local[:0]
		for i := range t {
			if i == 0 {
				continue
			}
			l, okL := normal[t[:i]]
			r, okR := normal[t[i:]]
			if okL && okR {
				local = append(local, split{l, r, scores[id]})
			}
		}
		sort.Slice(local, func(a, b int) bool {
			if local[a].left != local[b].left {
				return local[a].left < local[b].left
			}
			return local[a].right < local[b].right
		})
		all = append(all, local...)
	}
	sort.SliceStable(all, func(a, b int) bool { return all[a].score > all[b].score })
	out := make([][2]string, len(all))
	for i, s := range all {
		out[i] = [2]string{tokens[s.left], tokens[s.right]}
	}
	return out
}
