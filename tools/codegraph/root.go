package main

import (
	"os"
	"path/filepath"

	"github.com/sekerahmet/TCB/tools/internal/repodir"
)

// resolveRoot --root değerini mutlak yola çevirir. Verilmemişse (boş) bugünkü klasörden
// yukarı doğru depo kökü aranır, böylece alt klasörden de sorulabilir: önce go.mod'u
// `module github.com/sekerahmet/TCB` olan klasör, yoksa `git rev-parse --show-toplevel`,
// o da yoksa açık bir hata.
func resolveRoot(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return repodir.FindFrom(cwd)
}
