package main

import "github.com/sekerahmet/TCB/tools/internal/repodir"

// resolveRoot --root değerini mutlak yola çevirir; verilmemişse depo kökü bugünkü klasörden yukarı
// doğru aranır (ayrıntı repodir.Resolve'da: önce go.mod'u repodir.Module olan klasör, yoksa git, yoksa hata).
func resolveRoot(flagValue string) (string, error) { return repodir.Resolve(flagValue) }
