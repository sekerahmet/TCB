package main

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Commit kaydın commit bilgisi (pytest-benchmark commit_info gibi: id, branch, dirty).
type Commit struct {
	ID      string `json:"id"`
	Branch  string `json:"branch"`
	Dirty   bool   `json:"dirty"`
	Subject string `json:"subject"`
}

// Machine kaydın makine bilgisi; okunamayan alan boş kalır.
type Machine struct {
	Host     string `json:"host"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPU      string `json:"cpu"`
	Cores    int    `json:"cores"`
	MemoryGB int    `json:"memory_gb,omitempty"`
	GPUs     []GPU  `json:"gpus,omitempty"`
	Go       string `json:"go"`
}

// GPU nvidia-smi'nin bildirdiği bir kart.
type GPU struct {
	Name      string `json:"name"`
	MemoryMiB int    `json:"memory_mib"`
	Driver    string `json:"driver"`
}

// commitInfo çalışma klasöründeki deponun commit bilgisi (git yoksa boş).
func commitInfo() Commit {
	return Commit{
		ID:      gitOut("rev-parse", "HEAD"),
		Branch:  gitOut("rev-parse", "--abbrev-ref", "HEAD"),
		Dirty:   gitOut("status", "--porcelain", "--untracked-files=no") != "",
		Subject: gitOut("log", "-1", "--format=%s"),
	}
}

// machineInfo koşunun makinesi.
func machineInfo() Machine {
	host, _ := os.Hostname()
	return Machine{Host: host, OS: runtime.GOOS, Arch: runtime.GOARCH, CPU: cpuModel(),
		Cores: runtime.NumCPU(), MemoryGB: memoryGB(), GPUs: gpus(), Go: runtime.Version()}
}

// cpuModel işlemcinin adı: Linux'ta /proc/cpuinfo, Windows'ta kayıt defterindeki
// ProcessorNameString (ör. "13th Gen Intel(R) Core(TM) i7-13700H"), o da yoksa
// PROCESSOR_IDENTIFIER.
func cpuModel() string {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("reg", "query", `HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "/v", "ProcessorNameString").Output()
		if err == nil {
			if _, v, ok := strings.Cut(string(out), "REG_SZ"); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			if k, v, ok := strings.Cut(s.Text(), ":"); ok && strings.TrimSpace(k) == "model name" {
				return strings.TrimSpace(v)
			}
		}
	}
	return os.Getenv("PROCESSOR_IDENTIFIER")
}

// memoryGB toplam bellek (Linux'ta /proc/meminfo; başka sistemde 0, kayda girmez).
func memoryGB() int {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "MemTotal:" {
			kb, _ := strconv.Atoi(f[1])
			return (kb + 512*1024) / (1024 * 1024)
		}
	}
	return 0
}

// gpus nvidia-smi'nin listelediği kartlar (araç yoksa boş).
func gpus() []GPU {
	b, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total,driver_version",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil
	}
	return parseGPUs(string(b))
}

// parseGPUs nvidia-smi'nin CSV çıktısını ayrıştırır ("ad, bellek MiB, sürücü" satırları).
func parseGPUs(out string) []GPU {
	var list []GPU
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Split(line, ",")
		if len(f) != 3 {
			continue
		}
		mib, _ := strconv.Atoi(strings.TrimSpace(f[1]))
		list = append(list, GPU{Name: strings.TrimSpace(f[0]), MemoryMiB: mib, Driver: strings.TrimSpace(f[2])})
	}
	return list
}
