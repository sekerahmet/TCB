package main

import (
	"strings"
	"testing"
)

// Programın kitaplıktan (tr/<ad>) ve kendi klasöründen aktardıkları, dolaylılar dahil, yollara
// girer; yorumdaki aktarım girmez. Dolaylı aktarılan dosyaya dokunan commit satırı eskitir,
// aktarılmayan kitaplık dosyası eskitmez; programsız satırın yollarında bütün lib/ vardır.
func TestProgramImportsJoinThePaths(t *testing.T) {
	fx := newFixture(t, "")
	a := fx.commit("taban")
	fx.publish()
	g := repo{fx.dir}
	files, err := programImports("examples/a.tr", g.readAt)
	if err != nil {
		t.Fatal(err)
	}
	want := "examples/a.tr examples/util/geo.tr lib/base.tr lib/layers.tr"
	if got := strings.Join(files, " "); got != want {
		t.Errorf("aktarımlar %q, beklenen %q", got, want)
	}
	if _, err := programImports("examples/yok.tr", g.readAt); err == nil {
		t.Error("okunamayan program hata vermedi")
	}

	fx.write("lib/base.tr", "# TR 4\n# değişti\n")
	fx.commit("dolaylı aktarım")
	fx.write("lib/other.tr", "# TR 4\n# değişti\n")
	fx.commit("aktarılmayan kitaplık dosyası")
	fx.publish()
	fx.section("text-generation.md", l4,
		validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-3", cCommit: a, cModel: "a.gguf 0123456789ab"}),
		validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-2", cDate: "2026-11-17 14:40 +0300", cCommit: a,
			cModel: "b.gguf 0123456789ab", cPaths: "engine, examples/b.tr"}),
		validCells(map[int]string{cID: "TG-2026-11-17-L4GPU-H1-1", cDate: "2026-11-17 14:30 +0300", cCommit: a,
			cModel: "c.gguf 0123456789ab", cPaths: "engine, — (program yok: Go ölçüsü, kıyas)"}))
	out, _ := fx.run()
	expectLines(t, lines(t, out),
		"TG-2026-11-17-L4GPU-H1-3: eski: 1 commit, 2026-11-17, Colab L4, GPU",
		"TG-2026-11-17-L4GPU-H1-2: güncel",
		"TG-2026-11-17-L4GPU-H1-1: eski: 2 commit, 2026-11-17, Colab L4, GPU")
}
