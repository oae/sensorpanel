package panel

import (
	"bytes"
	"errors"
	"testing"
)

type recordedRegion struct {
	Region
	Data []byte
}

type recordingDisplay struct {
	fullWrites   [][]byte
	regionWrites []recordedRegion
	regionError  error
}

func (d *recordingDisplay) DisplayBuffer(buffer []byte) error {
	d.fullWrites = append(d.fullWrites, append([]byte(nil), buffer...))
	return nil
}

func (d *recordingDisplay) DisplayRegion(x, y, w, h int, buffer []byte) error {
	if d.regionError != nil {
		return d.regionError
	}
	d.regionWrites = append(d.regionWrites, recordedRegion{
		Region: Region{X: x, Y: y, Width: w, Height: h},
		Data:   append([]byte(nil), buffer...),
	})
	return nil
}

func TestFrameUpdaterUsesRegionalWrites(t *testing.T) {
	display := &recordingDisplay{}
	updater := newFrameUpdater(display, 32, 32, 2, 8)
	first := make([]byte, 32*32*2)

	stats, err := updater.Display(first)
	if err != nil {
		t.Fatalf("first Display() error = %v", err)
	}
	if !stats.FullFrame || len(display.fullWrites) != 1 {
		t.Fatalf("first Display() stats = %+v, full writes = %d", stats, len(display.fullWrites))
	}

	second := append([]byte(nil), first...)
	setPixel(second, 32, 2, 9, 10, 0x12, 0x34)
	stats, err = updater.Display(second)
	if err != nil {
		t.Fatalf("second Display() error = %v", err)
	}
	if stats.FullFrame || stats.Regions != 1 {
		t.Fatalf("second Display() stats = %+v", stats)
	}
	if len(display.regionWrites) != 1 {
		t.Fatalf("region writes = %d, want 1", len(display.regionWrites))
	}
	got := display.regionWrites[0]
	if got.Region != (Region{X: 8, Y: 8, Width: 8, Height: 8}) {
		t.Errorf("region = %+v, want tile at 8,8", got.Region)
	}
	if !bytes.Equal(got.Data, extractRegion(second, 32, 2, got.Region)) {
		t.Error("regional pixel data does not match the source frame")
	}
}

func TestFrameUpdaterSkipsUnchangedFrames(t *testing.T) {
	display := &recordingDisplay{}
	updater := newFrameUpdater(display, 16, 16, 2, 8)
	frame := make([]byte, 16*16*2)
	if _, err := updater.Display(frame); err != nil {
		t.Fatal(err)
	}

	stats, err := updater.Display(append([]byte(nil), frame...))
	if err != nil {
		t.Fatal(err)
	}
	if !stats.Skipped {
		t.Fatalf("stats = %+v, want skipped update", stats)
	}
	if len(display.fullWrites) != 1 || len(display.regionWrites) != 0 {
		t.Fatalf("unexpected writes: full=%d regional=%d", len(display.fullWrites), len(display.regionWrites))
	}
}

func TestCoherentFrameUpdaterUsesChangedBoundingBox(t *testing.T) {
	display := &recordingDisplay{}
	updater := newFrameUpdater(display, 100, 100, 2, 8)
	updater.maxRegions = 1
	first := make([]byte, 100*100*2)
	if _, err := updater.Display(first); err != nil {
		t.Fatal(err)
	}

	second := append([]byte(nil), first...)
	setPixel(second, 100, 2, 10, 20, 1, 1)
	setPixel(second, 100, 2, 69, 79, 1, 1)
	stats, err := updater.Display(second)
	if err != nil {
		t.Fatal(err)
	}
	if stats.FullFrame || stats.Regions != 1 {
		t.Fatalf("stats = %+v, want one regional write", stats)
	}
	want := Region{X: 10, Y: 20, Width: 60, Height: 60}
	if got := display.regionWrites[0].Region; got != want {
		t.Fatalf("region = %+v, want %+v", got, want)
	}
}

func TestFrameUpdaterFallsBackAfterRegionalFailure(t *testing.T) {
	display := &recordingDisplay{}
	updater := newFrameUpdater(display, 32, 32, 2, 8)
	first := make([]byte, 32*32*2)
	if _, err := updater.Display(first); err != nil {
		t.Fatal(err)
	}

	display.regionError = errors.New("unsupported")
	second := append([]byte(nil), first...)
	setPixel(second, 32, 2, 1, 1, 0xff, 0xff)
	stats, err := updater.Display(second)
	if err != nil {
		t.Fatalf("Display() error = %v", err)
	}
	if !stats.FullFrame || len(display.fullWrites) != 2 {
		t.Fatalf("fallback stats = %+v, full writes = %d", stats, len(display.fullWrites))
	}

	display.regionError = nil
	third := append([]byte(nil), second...)
	setPixel(third, 32, 2, 20, 20, 0xaa, 0xbb)
	stats, err = updater.Display(third)
	if err != nil {
		t.Fatal(err)
	}
	if !stats.FullFrame || len(display.regionWrites) != 0 {
		t.Fatalf("regional writes were not disabled: stats=%+v writes=%d", stats, len(display.regionWrites))
	}
}

func TestDirtyRegionsMergeVerticalRuns(t *testing.T) {
	previous := make([]byte, 32*32*2)
	current := append([]byte(nil), previous...)
	setPixel(current, 32, 2, 2, 2, 1, 1)
	setPixel(current, 32, 2, 2, 10, 1, 1)

	regions := dirtyRegions(previous, current, 32, 32, 2, 8)
	want := []Region{{X: 0, Y: 0, Width: 8, Height: 16}}
	if len(regions) != len(want) || regions[0] != want[0] {
		t.Fatalf("regions = %+v, want %+v", regions, want)
	}
}

func TestChooseTransferRegionsUsesFullFrameForBroadChanges(t *testing.T) {
	regions := []Region{{X: 0, Y: 0, Width: 32, Height: 32}}
	if got := chooseTransferRegions(regions, 32, 32, 2); got != nil {
		t.Fatalf("chooseTransferRegions() = %+v, want full frame", got)
	}
}

func setPixel(buffer []byte, width, bytesPerPixel, x, y int, values ...byte) {
	offset := (y*width + x) * bytesPerPixel
	copy(buffer[offset:offset+bytesPerPixel], values)
}
