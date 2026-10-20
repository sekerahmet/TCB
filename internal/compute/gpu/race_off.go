//go:build !race

package gpu

// raceEnabled yarış dedektörüyle derlenip derlenmediğidir (race_on.go ile eş). Testler ağır sayısal
// çekirdek sınamalarını yarış dedektörü altında atlar (skipUnderRace).
const raceEnabled = false
