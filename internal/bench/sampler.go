package bench

import (
	"context"
	"sync"
	"time"

	"github.com/marcellovictorino/pill/internal/sysinfo"
)

// Stats summarise what the sampler saw.
type Stats struct {
	Samples      int
	MinFreePct   float64
	SwapGrowthMB float64
	MaxPressure  int
	GPUBusyMin   float64
}

// Sampler records free memory, swap, memory pressure and GPU load on a timer
// while a benchmark runs, from its own goroutine.
//
// A goroutine is Go's lightweight thread; this one loops until the context is
// cancelled or Stop is called, and a mutex guards the shared slice because
// the benchmark reads Stats while the goroutine is still writing.
type Sampler struct {
	sys      sysinfo.Info
	interval time.Duration

	mu      sync.Mutex
	samples []sysinfo.Sample

	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// StartSampler takes one sample immediately, then one every interval.
func StartSampler(ctx context.Context, sys sysinfo.Info, interval time.Duration) *Sampler {
	s := &Sampler{sys: sys, interval: interval, stop: make(chan struct{}), done: make(chan struct{})}
	s.take()
	go func() {
		defer close(s.done)
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-tick.C:
				s.take()
			}
		}
	}()
	return s
}

func (s *Sampler) take() {
	sample, err := s.sys.Sample()
	if err != nil {
		return
	}
	s.mu.Lock()
	s.samples = append(s.samples, sample)
	s.mu.Unlock()
}

// Stop ends sampling (after one last reading) and returns the final stats.
// It is safe to call more than once.
func (s *Sampler) Stop() Stats {
	s.stopOnce.Do(func() {
		close(s.stop)
		<-s.done
		s.take()
	})
	return s.Stats()
}

// Stats computes the summary of the samples so far.
func (s *Sampler) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Stats{Samples: len(s.samples)}
	if len(s.samples) == 0 {
		return st
	}
	st.MinFreePct = s.samples[0].FreePct
	busy := 0
	for _, x := range s.samples {
		st.MinFreePct = min(st.MinFreePct, x.FreePct)
		st.MaxPressure = max(st.MaxPressure, x.Pressure)
		if x.GPUUtilPct >= 10 {
			busy++
		}
	}
	growth := s.samples[len(s.samples)-1].SwapUsedBytes - s.samples[0].SwapUsedBytes
	st.SwapGrowthMB = max(float64(growth)/1e6, 0)
	st.GPUBusyMin = float64(busy) * s.interval.Minutes()
	return st
}
