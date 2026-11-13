package machine

// memoryLimit sürecin cgroup bellek sınırını okur (limit.go). Okunamayan sınır ok=false'tur:
// sessizce "sınır yok" denmez.
func memoryLimit() (bytes int64, limited, ok bool) {
	b, l, err := cgroupMemoryLimit("/")
	if err != nil {
		return 0, false, false
	}
	return b, l, true
}
