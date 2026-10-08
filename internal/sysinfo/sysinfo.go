// Package sysinfo answers questions about the machine and its processes.
//
// Everything platform-specific sits behind the Info interface so the rest of
// pill (and its tests) never shells out to macOS tools directly. The real
// implementation lives in sysinfo_darwin.go; Go only compiles a file ending in
// _darwin.go on macOS, which is how Linux support can be added later as
// sysinfo_linux.go without touching callers.
package sysinfo

// Sample is one reading of memory, swap, pressure and GPU load.
type Sample struct {
	FreePct       float64 // system-wide free memory percentage (as Activity Monitor)
	SwapUsedBytes int64
	Pressure      int     // kern.memorystatus_vm_pressure_level: 1 normal, 2 warn, 4 critical
	GPUUtilPct    float64 // IOAccelerator device utilisation; -1 when unavailable
	// GPUMemBytes is system-wide GPU memory in use. Model weights live in
	// Metal buffers, which a process's resident size does not show, so this
	// is the better "how big is the loaded model" signal.
	GPUMemBytes int64
}

// Info is the platform interface.
type Info interface {
	TotalRAMBytes() (int64, error)
	// ProcessTreeRSSBytes sums the resident memory of pid and its descendants
	// (the router spawns one child per loaded model).
	ProcessTreeRSSBytes(pid int) (int64, error)
	Sample() (Sample, error)
	OSVersion() string
}

// RAMGB converts a byte count to whole gigabytes (binary, rounded).
func RAMGB(bytes int64) int {
	const gb = 1 << 30
	return int((bytes + gb/2) / gb)
}

// Fake is a scriptable Info for tests.
type Fake struct {
	RAM     int64
	RSS     int64
	Samples []Sample
	OS      string
	next    int
}

func (f *Fake) TotalRAMBytes() (int64, error)          { return f.RAM, nil }
func (f *Fake) ProcessTreeRSSBytes(int) (int64, error) { return f.RSS, nil }
func (f *Fake) OSVersion() string                      { return f.OS }
func (f *Fake) Sample() (Sample, error) {
	if len(f.Samples) == 0 {
		return Sample{FreePct: 50, Pressure: 1, GPUUtilPct: -1}, nil
	}
	s := f.Samples[min(f.next, len(f.Samples)-1)]
	f.next++
	return s, nil
}
