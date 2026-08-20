package starlark

import (
	"errors"
	"time"
)

const (
	APIModuleLabel  = "@mamacord//api.star"
	EntrypointLabel = "//:plugin.star"
)

type Limits struct {
	MaxFileBytes        int64
	MaxTotalSourceBytes int64
	MaxModules          int
	MaxLoadDepth        int
	InitSteps           uint64
	InitTimeout         time.Duration
	InvokeSteps         uint64
	InvokeTimeout       time.Duration
	CheckSteps          uint64
	CheckTimeout        time.Duration
	MaxPrints           int
	MaxPrintBytes       int
	MaxHostCalls        int
}

func DefaultLimits() Limits {
	return Limits{
		MaxFileBytes:        256 << 10,
		MaxTotalSourceBytes: 1 << 20,
		MaxModules:          64,
		MaxLoadDepth:        32,
		InitSteps:           250_000,
		InitTimeout:         500 * time.Millisecond,
		InvokeSteps:         1_000_000,
		InvokeTimeout:       2 * time.Second,
		CheckSteps:          100_000,
		CheckTimeout:        250 * time.Millisecond,
		MaxPrints:           20,
		MaxPrintBytes:       1024,
		MaxHostCalls:        100,
	}
}

func (limits Limits) Validate() error {
	if limits.MaxFileBytes <= 0 || limits.MaxTotalSourceBytes < limits.MaxFileBytes {
		return errors.New("source byte limits are invalid")
	}
	if limits.MaxModules <= 0 || limits.MaxLoadDepth <= 0 {
		return errors.New("module limits are invalid")
	}
	if limits.InitSteps == 0 || limits.InvokeSteps == 0 || limits.CheckSteps == 0 {
		return errors.New("execution step limits must be positive")
	}
	if limits.InitTimeout <= 0 || limits.InvokeTimeout <= 0 || limits.CheckTimeout <= 0 {
		return errors.New("execution timeouts must be positive")
	}
	if limits.MaxPrints < 0 || limits.MaxPrintBytes <= 0 || limits.MaxHostCalls <= 0 {
		return errors.New("print or host-call limits are invalid")
	}
	return nil
}
