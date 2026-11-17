package main

import (
	"strings"
	"testing"
)

const l4 = "Colab L4, GPU"

// Commit'ten sonra satırın yollarına dokunan commit'ler sayılır, yol dışındakiler sayılmaz;
// bilinmeyen commit söylenir.
func TestFreshnessCountsCommitsOnThePaths(t *testing.T) {
	fx := newFixture(t, "")
	a := fx.commit("taban")
	fx.write("internal/compute/a.go", "package compute // değişti\n")
	b := fx.commit("motor")
	fx.write("docs/other.md", "değişti\n")
	fx.commit("belge, yol dışında")
	fx.publish()
	missing := strings.Repeat("0", 40)
	fx.section("text-generation.md", l4,
		validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-3", cCommit: a, cModel: "a.gguf 0123456789ab"}),
		validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-2", cDate: "2026-11-17 14:40 +0300", cCommit: b, cModel: "b.gguf 0123456789ab"}),
		validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-1", cDate: "2026-11-17 14:30 +0300", cCommit: missing, cModel: "c.gguf 0123456789ab"}))
	out, code := fx.run()
	if code != 0 {
		t.Errorf("çıkış %d, beklenen 0 (eski satır uyarıdır)", code)
	}
	expectLines(t, lines(t, out),
		"TG-2026-11-17-L4GPU-H1-3: eski: 1 commit, 2026-11-17, Colab L4, GPU",
		"TG-2026-11-17-L4GPU-H1-2: güncel",
		"TG-2026-11-17-L4GPU-H1-1: eski: commit bulunamadı ("+missing+"), 2026-11-17, Colab L4, GPU")
}

// Emsalin tablodaki sürümü ya da go.mod'un toolchain'i satırınkinden farklıysa satır eskidir;
// tabloda o makine için sürüm yoksa karşılaştırılmaz.
func TestOutOfRepoVersionChangeMakesRowOld(t *testing.T) {
	fx := newFixture(t, "| llama.cpp | L4GPU | b13000 |\n")
	a := fx.commit("taban")
	fx.publish()
	fx.section("text-generation.md", l4,
		validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-3", cCommit: a, cModel: "a.gguf 0123456789ab"}),
		validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-2", cDate: "2026-11-17 14:40 +0300", cCommit: a,
			cModel: "b.gguf 0123456789ab", cOutOfRepo: "llama.cpp b13000; Go go1.26.0"}),
		validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-1", cDate: "2026-11-17 14:30 +0300", cCommit: a,
			cModel: "c.gguf 0123456789ab", cOutOfRepo: "llama.cpp b13000; Go go1.27.4"}))
	fx.section("speech-to-text.md", "Dizüstü i7-13700H",
		validCells(map[int]string{cID: "STT-2026-11-17-LAPTOP-H1-1", cDevice: "işlemci, i7-13700H", cCommit: a}))
	out, _ := fx.run()
	expectLines(t, lines(t, out),
		"TG-2026-11-17-L4GPU-H1-3: eski: depo dışı llama.cpp b12124 → b13000, 2026-11-17, Colab L4, GPU",
		"TG-2026-11-17-L4GPU-H1-2: eski: depo dışı Go go1.26.0 → go1.27.4, 2026-11-17, Colab L4, GPU",
		"TG-2026-11-17-L4GPU-H1-1: güncel",
		"STT-2026-11-17-LAPTOP-H1-1: güncel")
}

// Yerini alanı boş satır için aynı bölümde aynı modelin ve programın daha yeni satırı önerilir:
// taşınan eski satır (sha256'sız, programı bilinmeyen) yeni ölçülmüş satırla eşleşir; sha256'ları
// farklı ya da modeli başka satır eşleşmez; dolu Yerini alan söylenir.
func TestSuggestsAPossibleReplacement(t *testing.T) {
	fx := newFixture(t, "")
	a := fx.commit("taban")
	fx.publish()
	newID := "TG-2026-11-17-L4GPU-H1-1"
	fx.section("text-generation.md", l4,
		validCells(map[int]string{cCommit: a}),
		validCells(map[int]string{cID: "TG-2026-11-12-L4GPU-H1-1", cDate: "2026-11-12 10:00 +0300", cCommit: a, cReplacedBy: newID}),
		validCells(map[int]string{cID: "TG-2026-11-11-L4GPU-H1-1", cDate: "2026-11-11 10:00 +0300", cCommit: a, cModel: "qwen.gguf ffffffffffff"}),
		validCells(map[int]string{cID: "TG-2026-11-10-L4GPU-H1-1", cDate: "2026-11-10 10:00 +0300", cCommit: a, cModel: "gemma.gguf 0123456789ab"}),
		validCells(map[int]string{cID: "TG-2026-11-09-L4GPU-TASINDI-1", cDate: "2026-11-09 (saat yazılmamış)", cSession: "TASINDI",
			cCommit: a, cModel: "qwen.gguf sha256 yazılmamış (taşındı)", cPaths: "engine, " + programUnknown,
			cSource: "taşındı: docs/measurements.md@c061399a:20; günlüğün saat dilimi +0300"}))
	f, err := Load(fx.dir)
	if err != nil {
		t.Fatal(err)
	}
	if ps := Check(f); len(ps) > 0 {
		t.Fatalf("düzenekte biçim yanlışı: %v", ps)
	}
	out, code := fx.run()
	if code != 0 {
		t.Errorf("çıkış %d, beklenen 0 (öneri uyarıdır)", code)
	}
	expectLines(t, lines(t, out),
		newID+": güncel",
		"TG-2026-11-12-L4GPU-H1-1: güncel; yerini alan: "+newID,
		"TG-2026-11-11-L4GPU-H1-1: güncel",
		"TG-2026-11-10-L4GPU-H1-1: güncel",
		"TG-2026-11-09-L4GPU-TASINDI-1: güncel",
		"TG-2026-11-09-L4GPU-TASINDI-1: olası yerini alan: "+newID)
}
