//go:build !linux

package cmd

import "time"

func processCPUTime() time.Duration { return 0 }
