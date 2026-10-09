//go:build darwin

package sysinfo

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Darwin reads macOS state by running its standard command-line tools.
// Shelling out keeps pill free of cgo (and so trivially cross-buildable).
type Darwin struct{}

// New returns the real implementation for this platform.
func New() Info { return Darwin{} }

func run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func (Darwin) TotalRAMBytes() (int64, error) {
	out, err := run("sysctl", "-n", "hw.memsize")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(out, 10, 64)
}

func (Darwin) OSVersion() string {
	out, err := run("sw_vers", "-productVersion")
	if err != nil {
		return "unknown"
	}
	return out
}

func (Darwin) ProcessTreeRSSBytes(pid int) (int64, error) {
	// `ps -A -o pid=,ppid=,rss=` lists every process; walk the tree in Go.
	out, err := run("ps", "-A", "-o", "pid=,ppid=,rss=")
	if err != nil {
		return 0, err
	}
	rss := map[int]int64{}
	children := map[int][]int{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) != 3 {
			continue
		}
		p, _ := strconv.Atoi(f[0])
		pp, _ := strconv.Atoi(f[1])
		kb, _ := strconv.ParseInt(f[2], 10, 64)
		rss[p] = kb * 1024
		children[pp] = append(children[pp], p)
	}
	var total int64
	stack := []int{pid}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		total += rss[p]
		stack = append(stack, children[p]...)
	}
	return total, nil
}

var (
	freePct  = regexp.MustCompile(`free percentage:\s*(\d+)%`)
	swapUsed = regexp.MustCompile(`used = ([\d.]+)([MGK])`)
	gpuUtil  = regexp.MustCompile(`"Device Utilization %"\s*=\s*(\d+)`)
	gpuMem   = regexp.MustCompile(`"In use system memory"\s*=\s*(\d+)`)
)

func (Darwin) Sample() (Sample, error) {
	s := Sample{GPUUtilPct: -1, Pressure: 1}
	out, err := run("memory_pressure")
	if err != nil {
		return s, fmt.Errorf("memory_pressure: %w", err)
	}
	if m := freePct.FindStringSubmatch(out); m != nil {
		s.FreePct, _ = strconv.ParseFloat(m[1], 64)
	}
	if out, err := run("sysctl", "-n", "vm.swapusage"); err == nil {
		if m := swapUsed.FindStringSubmatch(out); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			switch m[2] {
			case "G":
				v *= 1 << 30
			case "M":
				v *= 1 << 20
			case "K":
				v *= 1 << 10
			}
			s.SwapUsedBytes = int64(v)
		}
	}
	if out, err := run("sysctl", "-n", "kern.memorystatus_vm_pressure_level"); err == nil {
		s.Pressure, _ = strconv.Atoi(out)
	}
	if out, err := run("ioreg", "-r", "-d", "1", "-w", "0", "-c", "IOAccelerator"); err == nil {
		if m := gpuUtil.FindStringSubmatch(out); m != nil {
			s.GPUUtilPct, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := gpuMem.FindStringSubmatch(out); m != nil {
			s.GPUMemBytes, _ = strconv.ParseInt(m[1], 10, 64)
		}
	}
	return s, nil
}
