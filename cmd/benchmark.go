package cmd

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
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
		regionWidth, _ := cmd.Flags().GetInt("region-width")
		regionHeight, _ := cmd.Flags().GetInt("region-height")
		animation, _ := cmd.Flags().GetBool("animation")
		duration, _ := cmd.Flags().GetDuration("duration")
		targetFPS, _ := cmd.Flags().GetFloat64("target-fps")

		dev, err := openConfiguredDevice()
		if err != nil {
			return fmt.Errorf("failed to open device: %w", err)
		}
		defer dev.Close()

		fmt.Printf("Panel: %s %s\n", dev.Info.Manufacturer, dev.Info.Product)
		fmt.Printf("Resolution: %dx%d (%d bytes/frame)\n",
			dev.Info.Width, dev.Info.Height, dev.Info.BufferSize)
		fmt.Printf("USB Speed: %s, Max Packet: %d bytes\n", dev.Info.Speed, dev.Info.MaxPacketSize)
		if animation {
			return runAnimationBenchmark(dev, regionWidth, regionHeight, duration, targetFPS)
		}

		if regionWidth <= 0 {
			regionWidth = dev.Info.Width
		}
		if regionHeight <= 0 {
			regionHeight = dev.Info.Height
		}
		if regionWidth > dev.Info.Width || regionHeight > dev.Info.Height {
			return fmt.Errorf("benchmark region %dx%d exceeds display size %dx%d",
				regionWidth, regionHeight, dev.Info.Width, dev.Info.Height)
		}

		regionX := (dev.Info.Width - regionWidth) / 2
		regionY := (dev.Info.Height - regionHeight) / 2
		regional := regionWidth != dev.Info.Width || regionHeight != dev.Info.Height
		fmt.Printf("Update area: %dx%d at (%d,%d)\n", regionWidth, regionHeight, regionX, regionY)
		fmt.Printf("Running benchmark: %d warmup + %d measured frames\n", warmup, frames)
		fmt.Println()

		// Create alternating test patterns for benchmark
		pattern1 := panel.CreateSolidColorBufferWithSize(255, 0, 255, regionWidth, regionHeight)
		pattern2 := panel.CreateSolidColorBufferWithSize(0, 255, 255, regionWidth, regionHeight)
		display := func(buffer []byte) error {
			if regional {
				return dev.DisplayRegion(regionX, regionY, regionWidth, regionHeight, buffer)
			}
			return dev.DisplayBuffer(buffer)
		}

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
				if err := display(buf); err != nil {
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
			if err := display(buf); err != nil {
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
		fullFrameMaxFPS := dev.Info.TheoreticalFPS()
		theoreticalMaxKBs := float64(dev.Info.BufferSize) * fullFrameMaxFPS / 1024
		theoreticalMaxFPS := theoreticalMaxKBs * 1024 / float64(len(pattern1))
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
	benchmarkCmd.Flags().Int("region-width", 0, "Benchmark a centered region of this width")
	benchmarkCmd.Flags().Int("region-height", 0, "Benchmark a centered region of this height")
	benchmarkCmd.Flags().Bool("animation", false, "Run a moving regional-update animation")
	benchmarkCmd.Flags().Duration("duration", 10*time.Second, "Animation benchmark duration")
	benchmarkCmd.Flags().Float64("target-fps", 60, "Animation target frame rate")
}

func runAnimationBenchmark(dev *panel.Device, width, height int, duration time.Duration, targetFPS float64) error {
	if width <= 0 {
		width = min(240, dev.Info.Width)
	}
	if height <= 0 {
		height = min(32, dev.Info.Height)
	}
	if width > dev.Info.Width || height > dev.Info.Height || width <= 0 || height <= 0 {
		return fmt.Errorf("animation region %dx%d exceeds display size %dx%d",
			width, height, dev.Info.Width, dev.Info.Height)
	}
	if duration <= 0 {
		return fmt.Errorf("duration must be greater than zero")
	}
	if targetFPS <= 0 {
		return fmt.Errorf("target FPS must be greater than zero")
	}

	fmt.Printf("Animation: %dx%d striped band moving across the full framebuffer\n", width, height)
	fmt.Printf("Target: %.1f FPS for %s\n\n", targetFPS, duration)

	canvas := image.NewRGBA(image.Rect(0, 0, dev.Info.Width, dev.Info.Height))
	updater := panel.NewFrameUpdater(dev)
	frameInterval := time.Duration(float64(time.Second) / targetFPS)
	started := time.Now()
	deadline := started.Add(duration)
	nextFrame := started
	frames := 0
	bytesSent := 0
	regionalWrites := 0
	fullFrames := 0

	for time.Now().Before(deadline) {
		drawAnimationFrame(canvas, width, height, frames)
		stats, err := updater.Display(dev.Profile.ConvertImage(canvas))
		if err != nil {
			return fmt.Errorf("animation frame %d failed: %w", frames, err)
		}
		frames++
		bytesSent += stats.Bytes
		regionalWrites += stats.Regions
		if stats.FullFrame {
			fullFrames++
		}

		nextFrame = nextFrame.Add(frameInterval)
		if wait := time.Until(nextFrame); wait > 0 {
			time.Sleep(wait)
		}
	}

	elapsed := time.Since(started)
	fps := float64(frames) / elapsed.Seconds()
	fmt.Println("=== Animation Results ===")
	fmt.Printf("Frames:          %d\n", frames)
	fmt.Printf("Time:            %.2f seconds\n", elapsed.Seconds())
	fmt.Printf("Measured FPS:    %.2f\n", fps)
	fmt.Printf("Transferred:     %.1f KB/s\n", float64(bytesSent)/elapsed.Seconds()/1024)
	fmt.Printf("Regional writes: %d\n", regionalWrites)
	fmt.Printf("Full frames:     %d\n", fullFrames)
	return nil
}

func drawAnimationFrame(canvas *image.RGBA, width, height, frame int) {
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{R: 8, G: 10, B: 16, A: 255}), image.Point{}, draw.Src)

	travel := canvas.Bounds().Dx() - width
	x := 0
	if travel > 0 {
		position := (frame * 4) % (travel * 2)
		if position > travel {
			position = travel*2 - position
		}
		x = position
	}
	y := (canvas.Bounds().Dy() - height) / 2
	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			stripe := ((px + frame*3) / 8) % 2
			if stripe == 0 {
				canvas.SetRGBA(x+px, y+py, color.RGBA{R: 80, G: 180, B: 255, A: 255})
			} else {
				canvas.SetRGBA(x+px, y+py, color.RGBA{R: 240, G: 80, B: 220, A: 255})
			}
		}
	}
}
