package main

import (
	"github.com/sekerahmet/TCB/internal/devtools/testrunner"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/stdlib"
	"os"
)

// testCommand 'trlang test [dosya.tr | klasör]...' komutudur. Verilen bütün hedefler çalışır; hedef
// verilmezse bulunulan klasördeki (alt klasörler dahil) TR_test içeren bütün .tr dosyaları (12.1). Bir
// test bile kalırsa çıkış kodu 1'dir.
// Çıkmadan önce motorların açık kaynakları kapatılır ve geçici klasörler silinir (motor.Kapat).
func testCommand(args []string) {
	targets := fileArguments(codes.NameRef(codes.CmdTest), args, nil, nil)
	if len(targets) == 0 {
		targets = []string{"."}
	}
	summary, err := testrunner.Run(targets, os.Stdout)
	stdlib.Close()
	if err != nil {
		exitWithError(err)
	}
	if !summary.OK() {
		os.Exit(1)
	}
}
