package gguf_test

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"math"
	"runtime"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func decode(data []byte) (*gguf.File, error) {
	return gguf.Decode(bufio.NewReader(bytes.NewReader(data)), int64(len(data)))
}

func TestDecodeReadsKeysAndTensors(t *testing.T) {
	b := &gguftest.Builder{}
	b.KV("general.architecture", "qwen3").KV("a.u32", uint32(7)).KV("a.f", float32(1.5)).
		KV("a.ok", true).KV("a.names", []string{"x", "", "ü"}).KV("a.ints", []int32{-1, 2})
	b.Tensor("w", gguf.TypeF32, []uint64{2, 3}, make([]byte, 24))
	b.Tensor("v", gguf.TypeF16, []uint64{5}, make([]byte, 10))
	f, err := decode(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.Keys, []string{"general.architecture", "a.u32", "a.f", "a.ok", "a.names", "a.ints"}) {
		t.Errorf("anahtarlar: %v", f.Keys)
	}
	if f.Values["general.architecture"].Data != "qwen3" || f.Values["a.u32"].Data != uint32(7) ||
		f.Values["a.f"].Data != float32(1.5) || f.Values["a.ok"].Data != true {
		t.Errorf("değerler: %v", f.Values)
	}
	if names := f.Values["a.names"]; names.Type != gguf.Array || names.Elem != gguf.String ||
		!slices.Equal(names.Data.([]string), []string{"x", "", "ü"}) {
		t.Errorf("metin dizisi: %+v", names)
	}
	if !slices.Equal(f.Values["a.ints"].Data.([]int32), []int32{-1, 2}) {
		t.Errorf("sayı dizisi: %+v", f.Values["a.ints"])
	}
	w := f.Tensors[0]
	if !slices.Equal(w.Shape(), []uint64{3, 2}) || w.Size != 24 || w.Offset != 0 {
		t.Errorf("w: %+v, şekil %v", w, w.Shape())
	}
	if v := f.Tensors[1]; v.Offset != 32 || v.Size != 10 {
		t.Errorf("v hizalı değil: %+v", v)
	}
	if f.DataStart%32 != 0 || f.DataStart < f.HeaderEnd {
		t.Errorf("veri başı %d, başlık sonu %d", f.DataStart, f.HeaderEnd)
	}
}

func TestDecodeHonoursAlignment(t *testing.T) {
	b := &gguftest.Builder{Alignment: 64}
	b.KV("general.alignment", uint32(64))
	b.Tensor("a", gguf.TypeF32, []uint64{1}, make([]byte, 4)).Tensor("b", gguf.TypeF32, []uint64{1}, make([]byte, 4))
	f, err := decode(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if f.DataStart%64 != 0 || f.Tensors[1].Offset != 64 {
		t.Errorf("veri başı %d, ikinci kayma %d", f.DataStart, f.Tensors[1].Offset)
	}
}

// TestDecodeAcceptsEveryKnownType: okunmayan türlerin de boyu bilinir; üstveri ve parçalayıcı
// bütün dosyanın düzenini denetler.
func TestDecodeAcceptsEveryKnownType(t *testing.T) {
	for _, c := range []struct {
		kind  gguf.TensorType
		name  string
		dim   uint64
		bytes int
	}{
		{gguf.TypeQ8_0, "Q8_0", 32, 34}, {gguf.TypeQ4_0, "Q4_0", 32, 18}, {gguf.TypeQ5_0, "Q5_0", 64, 44},
		{gguf.TypeQ4_K, "Q4_K", 256, 144}, {gguf.TypeQ6_K, "Q6_K", 512, 420}, {gguf.TypeBF16, "BF16", 3, 6},
	} {
		b := (&gguftest.Builder{}).Tensor("w", c.kind, []uint64{c.dim}, make([]byte, c.bytes))
		f, err := decode(b.Bytes())
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := f.Tensors[0]; got.Type.String() != c.name || got.Size != uint64(c.bytes) {
			t.Errorf("%s: %+v", c.name, got)
		}
	}
}

// oneTensor anahtarsız, tek tensörlü, verisiz bir dosyanın başıdır.
func oneTensor(name string, dims []uint64, kind uint32, offset uint64) []byte {
	return gguftest.AppendTensor(gguftest.Header(3, 1, 0), name, dims, kind, offset)
}

// TestDecodeRejects raporun yedi sınamasından dosya düzenini ilgilendiren altısıdır (yedincisi
// parçalayıcıda) ve yayımlanmış açıkların biçimleri.
func TestDecodeRejects(t *testing.T) {
	huge := uint64(math.MaxUint64)
	le := binary.LittleEndian
	arrayOf := func(elem uint32, n uint64) []byte {
		out := gguftest.AppendString(nil, "k")
		out = le.AppendUint32(out, uint32(gguf.Array))
		out = le.AppendUint32(out, elem)
		return le.AppendUint64(out, n)
	}
	withKV := func(records ...[]byte) []byte {
		out := gguftest.Header(3, 0, uint64(len(records)))
		for _, r := range records {
			out = append(out, r...)
		}
		return append(out, make([]byte, 64)...)
	}
	rawKV := func(kind uint32, body ...byte) []byte {
		out := gguftest.AppendString(nil, "k")
		return append(le.AppendUint32(out, kind), body...)
	}
	cases := []struct {
		name string
		data []byte
		code messages.Code
	}{
		// 1. başlık
		{"CVE-2024-39720 yalnız sihirli baytlar", []byte("GGUF"), messages.H6658},
		{"boş", nil, messages.H6658},
		{"sihirli yanlış", []byte("GGUX\x03\x00\x00\x00"), messages.H6676},
		{"sürüm 2", gguftest.Header(2, 0, 0), messages.H6656},
		{"sürüm 0", gguftest.Header(0, 0, 0), messages.H6656},
		{"büyük uçlu", gguftest.Header(0x03000000, 0, 0), messages.H6656},
		{"kesik sayılar", gguftest.Header(3, 0, 0)[:12], messages.H6658},
		// 2. ayırmadan önce sınır
		{"dev anahtar sayısı", gguftest.Header(3, 0, huge), messages.H6657},
		{"dev tensör sayısı", gguftest.Header(3, huge, 0), messages.H6657},
		{"taşan toplam", gguftest.Header(3, huge/25, huge/13), messages.H6657},
		{"CVE-2025-0315 sınırsız dizi", withKV(arrayOf(uint32(gguf.Uint8), 1<<40)), messages.H6659},
		{"taşan dizi boyu", withKV(arrayOf(uint32(gguf.Float64), huge/4)), messages.H6659},
		{"metin dizisi boyu", withKV(arrayOf(uint32(gguf.String), huge/8)), messages.H6659},
		{"dev metin", withKV(le.AppendUint64(rawKV(uint32(gguf.String)), huge)), messages.H6659},
		{"dev anahtar adı", append(le.AppendUint64(gguftest.Header(3, 0, 1), 1<<40), make([]byte, 16)...), messages.H6659},
		// 3. değerler
		{"tür 13", withKV(rawKV(13, 0)), messages.H6660},
		{"öğe türü 13", withKV(arrayOf(13, 0)), messages.H6660},
		{"iç içe dizi", withKV(arrayOf(uint32(gguf.Array), 1)), messages.H6661},
		{"mantık 2", withKV(rawKV(uint32(gguf.Bool), 2)), messages.H6662},
		{"bozuk UTF-8", withKV(rawKV(uint32(gguf.String), 1, 0, 0, 0, 0, 0, 0, 0, 0xFF)), messages.H6663},
		{"yinelenen anahtar", withKV(rawKV(uint32(gguf.Uint8), 1), rawKV(uint32(gguf.Uint8), 2)), messages.H6664},
		{"dosya değerde biter", append(gguftest.Header(3, 0, 1), rawKV(uint32(gguf.Uint64), 1)...), messages.H6658},
		// 4. hizalama
		{"CVE-2025-0317 sıfır hizalama", (&gguftest.Builder{}).KV("general.alignment", uint32(0)).Bytes(), messages.H6665},
		{"üç hizalama", (&gguftest.Builder{}).KV("general.alignment", uint32(3)).Bytes(), messages.H6665},
		{"hizalama metin", (&gguftest.Builder{}).KV("general.alignment", "32").Bytes(), messages.H6665},
		{"hizalama uint64", (&gguftest.Builder{}).KV("general.alignment", uint64(32)).Bytes(), messages.H6665},
		// 5. tensör kaydı
		{"boş ad", oneTensor("", []uint64{1}, 0, 0), messages.H6666},
		{"uzun ad", oneTensor(string(make([]byte, 64)), []uint64{1}, 0, 0), messages.H6666},
		{"beş eksen", oneTensor("w", []uint64{1, 1, 1, 1, 1}, 0, 0), messages.H6667},
		{"eksi eksen", oneTensor("w", []uint64{huge}, 0, 0), messages.H6668},
		{"taşan şekil", oneTensor("w", []uint64{1 << 40, 1 << 40}, 0, 0), messages.H6669},
		{"sıfır eksenle gizlenen taşma", oneTensor("w", []uint64{0, 1 << 40, 1 << 40}, 0, 0), messages.H6669},
		{"CVE-2026-33298 bayt boyu taşar", oneTensor("w", []uint64{1 << 62}, 0, 0), messages.H6669},
		{"CVE-2024-42477 bilinmeyen tür", oneTensor("w", []uint64{1}, 44, 0), messages.H6670},
		{"kaldırılmış tür 4", oneTensor("w", []uint64{32}, 4, 0), messages.H6670},
		{"blok katı değil", oneTensor("w", []uint64{33}, uint32(gguf.TypeQ8_0), 0), messages.H6671},
		{"eksensiz bloklu", oneTensor("w", nil, uint32(gguf.TypeQ8_0), 0), messages.H6671},
		{"kesik tensör kaydı", oneTensor("w", []uint64{1}, 0, 0)[:52], messages.H6658},
		// 6. kayma ve veri
		{"kayma yanlış", oneTensor("w", []uint64{1}, 0, 32), messages.H6672},
		{"kesik veri", oneTensor("w", []uint64{1}, 0, 0), messages.H6673},
		{"veri dosyadan büyük", (&gguftest.Builder{}).Tensor("w", gguf.TypeF32, []uint64{1 << 40}, nil).Bytes(), messages.H6673},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := decode(c.data); !messages.Has(err, c.code) {
				t.Errorf("hata %v; %s olmalı", err, c.code)
			}
		})
	}
}

func TestDecodeRejectsDuplicateTensorName(t *testing.T) {
	b := (&gguftest.Builder{}).Tensor("w", gguf.TypeF32, []uint64{1}, make([]byte, 4)).
		Tensor("w", gguf.TypeF32, []uint64{1}, make([]byte, 4))
	if _, err := decode(b.Bytes()); !messages.Has(err, messages.H6664) {
		t.Errorf("hata %v", err)
	}
}

// TestDecodeTruncatedEverywhere: geçerli bir dosya her uzunlukta kesilince hata verir, çökmez
// (veri bölümünden önceki her kesik).
func TestDecodeTruncatedEverywhere(t *testing.T) {
	b := (&gguftest.Builder{}).KV("a", "metin").KV("b", []string{"x", "y"}).KV("c", []int32{1})
	b.Tensor("w", gguf.TypeF32, []uint64{4}, make([]byte, 16))
	data := b.Bytes()
	for n := range len(data) {
		if _, err := decode(data[:n]); err == nil {
			t.Errorf("%d baytta kesilen dosya kabul edildi", n)
		}
	}
	if _, err := decode(data); err != nil {
		t.Fatal(err)
	}
}

// TestDecodeAllocatesWithinTheFile: küçük dosya büyük boylar bildirince ayırma dosyayla
// orantılı kalır (15.46, 15.55). Bildirilen boya güvenilseydi her biri gigabaytlar isterdi.
func TestDecodeAllocatesWithinTheFile(t *testing.T) {
	le := binary.LittleEndian
	filler := make([]byte, 1<<20)
	array := func(elem gguf.ValueType, n uint64) []byte {
		out := gguftest.Header(3, 0, 1)
		out = gguftest.AppendString(out, "k")
		out = le.AppendUint32(out, uint32(gguf.Array))
		out = le.AppendUint32(out, uint32(elem))
		return append(le.AppendUint64(out, n), filler...)
	}
	const slack = 1 << 16
	for _, c := range []struct {
		name string
		data []byte
		// bound dosya baytı başına izin verilen ayırmadır: metin dizisinin Go başlığı öğe
		// başına 16 bayt, dosyadaki en küçük öğe 8 bayttır.
		bound uint64
	}{
		{"yalan uint8 dizisi", array(gguf.Uint8, 1<<50), 1},
		{"tam boy uint8 dizisi", array(gguf.Uint8, 1<<20), 3},
		{"tam boy metin dizisi", array(gguf.String, 1<<17), 3},
		{"yalan metin", le.AppendUint64(gguftest.AppendString(gguftest.Header(3, 0, 1), "k"), 1<<50), 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, _ = decode(c.data)
			runtime.ReadMemStats(&after)
			if got := after.TotalAlloc - before.TotalAlloc; got > c.bound*uint64(len(c.data))+slack {
				t.Errorf("%d baytlık dosyadan %d bayt ayrıldı", len(c.data), got)
			}
		})
	}
}
