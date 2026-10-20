//go:build !race

package gpu

// raceEnabled yarış dedektörüyle derlenip derlenmediğidir (race_on_test.go ile eş).
const raceEnabled = false
