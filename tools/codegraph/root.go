package main

import "github.com/sekerahmet/TCB/tools/internal/repodir"

// repoModule TR deposunun modül yoludur; kök, go.mod'u bunu söyleyen klasördür.
const repoModule = repodir.Module

// resolveRoot --root değerini mutlak yola çevirir; verilmemişse depo kökü bugünkü klasörden yukarı
// doğru aranır (ayrıntı repodir.Resolve'da: önce go.mod'u repoModule olan klasör, yoksa git, yoksa hata).
func resolveRoot(flagValue string) (string, error) { return repodir.Resolve(flagValue) }
