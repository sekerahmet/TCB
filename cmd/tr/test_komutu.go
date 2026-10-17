package main

import (
	"fmt"
	"os"
	"tr_dili/pkg/testkosucu"
)

// testKomutu 'tr test [dosya.tr | klasör]' komutudur. Hedef verilmezse bulunulan klasördeki
// (alt klasörler dahil) tüm *_test.tr dosyaları çalışır. Bir test bile kalırsa çıkış kodu 1'dir.
func testKomutu(args []string) {
	hedef := "."
	if len(args) > 0 {
		hedef = args[0]
	}
	ozet, err := testkosucu.Calistir(hedef, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
	if !ozet.Basarili() {
		os.Exit(1)
	}
}
