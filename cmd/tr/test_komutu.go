package main

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/devtools/testrunner"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"os"
)

// testKomutu 'tr test [dosya.tr | klasör]' komutudur. Hedef verilmezse bulunulan klasördeki
// (alt klasörler dahil) tüm *_test.tr dosyaları çalışır. Bir test bile kalırsa çıkış kodu 1'dir.
// Çıkmadan önce motorların açık kaynakları kapatılır ve geçici klasörler silinir (motor.Kapat).
func testKomutu(args []string) {
	hedef := "."
	if len(args) > 0 {
		hedef = args[0]
	}
	ozet, err := testrunner.Run(hedef, os.Stdout)
	stdlib.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	if !ozet.OK() {
		os.Exit(1)
	}
}
