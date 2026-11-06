//go:build windows

package process_test

// killSelf Windows'ta sinyal olmadığından çağrılmaz (TestKilledBySignal).
func killSelf() {}
