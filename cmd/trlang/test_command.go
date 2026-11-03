package main

import (
	"github.com/sekerahmet/TCB/internal/devtools/testrunner"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"os"
)

// testCommand 'tr test [dosya.tr | klasör]' komutudur. Hedef verilmezse bulunulan klasördeki
// (alt klasörler dahil) tüm *_test.tr dosyaları çalışır. Bir test bile kalırsa çıkış kodu 1'dir.
// Çıkmadan önce motorların açık kaynakları kapatılır ve geçici klasörler silinir (motor.Kapat).
func testCommand(args []string) {
	target := "."
	if file, _, ok := programFile(codes.NameRef(codes.CmdTest), args); ok {
		target = file
	}
	summary, err := testrunner.Run(target, os.Stdout)
	stdlib.Close()
	if err != nil {
		exitWithError(err)
	}
	if !summary.OK() {
		os.Exit(1)
	}
}
