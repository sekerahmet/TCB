package machine

// Kapalı tablo (19.12): .TR_os "windows", "linux" ya da "macos"; .TR_arch "x86_64" ya da
// "arm64". Tablonun dışındaki makinede değer motorun derlendiği hedefin Go adıdır ve tablodaki
// hiçbir değere eşit değildir (go tool dist list, go1.27.4). Denetleyicinin karşılaştırma kuralı
// (internal/lang/checker/machine_comparison.go) aynı tabloyu bilir.

// osName Go'nun işletim sistemi adını tablonun yazımına çevirir.
func osName(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

// archName Go'nun mimari adını tablonun yazımına çevirir.
func archName(goarch string) string {
	if goarch == "amd64" {
		return "x86_64"
	}
	return goarch
}
