//go:build !darwin

package sysinfo

import "errors"

// New is a placeholder: pill v1 supports Apple Silicon macOS only.
func New() Info { return unsupported{} }

type unsupported struct{}

var errUnsupported = errors.New("pill supports macOS only in this version")

func (unsupported) TotalRAMBytes() (int64, error)          { return 0, errUnsupported }
func (unsupported) ProcessTreeRSSBytes(int) (int64, error) { return 0, errUnsupported }
func (unsupported) Sample() (Sample, error)                { return Sample{}, errUnsupported }
func (unsupported) OSVersion() string                      { return "unsupported" }
