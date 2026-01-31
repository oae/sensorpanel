package cmd

import (
	"fmt"
	"time"

	"github.com/oae/sensorpanel/pkg/panel"
	"github.com/spf13/cobra"
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run FPS benchmark on the panel",
	Long: `Runs a frame rate benchmark by sending frames to the panel as fast as possible.

The USB Full Speed (12 Mbps) connection limits maximum throughput to about
500 KB/s, which translates to approximately 1.67 FPS for the 480x320 display
(307,200 bytes per frame).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		frames, _ := cmd.Flags().GetInt("frames")
		warmup, _ := cmd.Flags().GetInt("warmup")

		dev, err := openConfiguredDevice()
		if err != nil {
			return fmt.Errorf("failed to open device: %w", err)
		}
		defer dev.Close()

		fmt.Printf("Panel: %s %s\n", dev.Info.Manufacturer, dev.Info.Product)
		fmt.Printf("Resolution: %dx%d (%d bytes/frame)\n",
			dev.Info.Width, dev.Info.Height, dev.Info.BufferSize)
		fmt.Printf("USB Speed: %s, Max Packet: %d bytes\n", dev.Info.Speed, dev.Info.MaxPacketSize)
		fmt.Printf("Running benchmark: %d warmup + %d measured frames\n", warmup, frames)
		fmt.Println()

		// Create alternating test patterns for benchmark
		pattern1 := panel.CreateTestPatternBuffer()
		pattern2 := panel.CreateColorBarsBuffer()

		// Warmup frames
		if warmup > 0 {
			fmt.Printf("Warmup: ")
			for i := 0; i < warmup; i++ {
				var buf []byte
				if i%2 == 0 {
					buf = pattern1
				} else {
					buf = pattern2
				}
				if err := dev.DisplayBuffer(buf); err != nil {
					return fmt.Errorf("warmup frame %d failed: %w", i, err)
				}
				fmt.Print(".")
			}
			fmt.Println(" done")
		}

		// Measured frames
		fmt.Printf("Benchmark: ")
		start := time.Now()
		var totalBytes int64

		for i := 0; i < frames; i++ {
			var buf []byte
			if i%2 == 0 {
				buf = pattern1
			} else {
				buf = pattern2
			}
			if err := dev.DisplayBuffer(buf); err != nil {
				return fmt.Errorf("benchmark frame %d failed: %w", i, err)
			}
			totalBytes += int64(len(buf))
			fmt.Print(".")
		}

		elapsed := time.Since(start)
		fmt.Println(" done")
		fmt.Println()

		// Calculate results
		fps := float64(frames) / elapsed.Seconds()
		throughputKBs := float64(totalBytes) / elapsed.Seconds() / 1024
		msPerFrame := elapsed.Seconds() / float64(frames) * 1000

		fmt.Println("=== Results ===")
		fmt.Printf("Frames:      %d\n", frames)
		fmt.Printf("Time:        %.2f seconds\n", elapsed.Seconds())
		fmt.Printf("FPS:         %.2f\n", fps)
		fmt.Printf("ms/frame:    %.1f\n", msPerFrame)
		fmt.Printf("Throughput:  %.1f KB/s\n", throughputKBs)
		fmt.Println()

		// Theoretical limits
		theoreticalMaxFPS := dev.Info.TheoreticalFPS()
		theoreticalMaxKBs := float64(dev.Info.BufferSize) * theoreticalMaxFPS / 1024
		efficiency := (throughputKBs / theoreticalMaxKBs) * 100

		fmt.Println("=== Analysis ===")
		fmt.Printf("Theoretical max: %.2f FPS (USB Full Speed limit)\n", theoreticalMaxFPS)
		fmt.Printf("Efficiency:      %.1f%% of theoretical max\n", efficiency)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(benchmarkCmd)

	benchmarkCmd.Flags().IntP("frames", "f", 20, "Number of frames to benchmark")
	benchmarkCmd.Flags().IntP("warmup", "w", 5, "Number of warmup frames")
}
