package panel

import (
	"bytes"
	"fmt"
	"sort"
)

const (
	defaultDirtyTileSize  = 16
	regionCommandOverhead = 680
	maxJoinCandidates     = 128
)

var adaptiveDirtyTileSizes = [...]int{1, 2, 4, 8, 16, 32}

// Region is a rectangular area of the display.
type Region struct {
	X      int
	Y      int
	Width  int
	Height int
}

// FrameUpdateStats describes how a frame was transferred.
type FrameUpdateStats struct {
	FullFrame bool
	Skipped   bool
	Regions   int
	Bytes     int
}

type frameDisplay interface {
	DisplayBuffer(buffer []byte) error
	DisplayRegion(x, y, w, h int, buffer []byte) error
}

// FrameUpdater compares native pixel buffers and uses regional blits when they
// cost less than a full-frame transfer. The first frame is always sent in full.
type FrameUpdater struct {
	display         frameDisplay
	width           int
	height          int
	bytesPerPixel   int
	tileSize        int
	adaptiveTiles   bool
	maxRegions      int
	previous        []byte
	regionsDisabled bool
}

// NewFrameUpdater creates a frame updater for a USB display.
func NewFrameUpdater(device *Device) *FrameUpdater {
	return newDeviceFrameUpdater(device, 0)
}

// NewCoherentFrameUpdater creates an updater that sends each logical frame
// with one USB write, avoiding tearing between independently applied regions.
func NewCoherentFrameUpdater(device *Device) *FrameUpdater {
	return newDeviceFrameUpdater(device, 1)
}

func newDeviceFrameUpdater(device *Device, maxRegions int) *FrameUpdater {
	updater := newFrameUpdater(
		device,
		device.Profile.Width(),
		device.Profile.Height(),
		device.Profile.ColorFormat().BytesPerPixel(),
		defaultDirtyTileSize,
	)
	updater.adaptiveTiles = true
	updater.maxRegions = maxRegions
	return updater
}

func newFrameUpdater(display frameDisplay, width, height, bytesPerPixel, tileSize int) *FrameUpdater {
	return &FrameUpdater{
		display:       display,
		width:         width,
		height:        height,
		bytesPerPixel: bytesPerPixel,
		tileSize:      tileSize,
	}
}

// Display transfers a native full-frame buffer, using dirty regions when
// possible. If a regional write fails, it falls back to a full frame and
// disables regional writes for the remainder of this updater's lifetime.
func (u *FrameUpdater) Display(buffer []byte) (FrameUpdateStats, error) {
	expected := u.width * u.height * u.bytesPerPixel
	if len(buffer) != expected {
		return FrameUpdateStats{}, fmt.Errorf("%w: got %d bytes, expected %d",
			ErrBufferSizeMismatch, len(buffer), expected)
	}

	if u.previous == nil || u.regionsDisabled {
		return u.displayFull(buffer)
	}

	var regions []Region
	if u.maxRegions == 1 {
		if bounds, changed := dirtyBoundingRegion(
			u.previous, buffer, u.width, u.height, u.bytesPerPixel,
		); changed {
			regions = []Region{bounds}
		}
	} else {
		regions = dirtyRegions(u.previous, buffer, u.width, u.height, u.bytesPerPixel, u.tileSize)
	}
	if u.adaptiveTiles && u.maxRegions != 1 {
		regions = adaptiveDirtyRegions(u.previous, buffer, u.width, u.height, u.bytesPerPixel)
	}
	if len(regions) == 0 {
		return FrameUpdateStats{Skipped: true}, nil
	}

	regions = chooseTransferRegions(regions, u.width, u.height, u.bytesPerPixel)
	if regions == nil {
		return u.displayFull(buffer)
	}
	if u.maxRegions > 0 && len(regions) > u.maxRegions {
		if u.maxRegions == 1 {
			regions = []Region{boundingRegion(regions)}
		}
	}

	stats := FrameUpdateStats{Regions: len(regions)}
	for _, region := range regions {
		data := extractRegion(buffer, u.width, u.bytesPerPixel, region)
		if err := u.display.DisplayRegion(region.X, region.Y, region.Width, region.Height, data); err != nil {
			u.regionsDisabled = true
			fullStats, fullErr := u.displayFull(buffer)
			if fullErr != nil {
				return FrameUpdateStats{}, fmt.Errorf("regional update failed: %v; full-frame fallback failed: %w", err, fullErr)
			}
			return fullStats, nil
		}
		stats.Bytes += len(data)
	}

	u.remember(buffer)
	return stats, nil
}

func (u *FrameUpdater) displayFull(buffer []byte) (FrameUpdateStats, error) {
	if err := u.display.DisplayBuffer(buffer); err != nil {
		return FrameUpdateStats{}, err
	}
	u.remember(buffer)
	return FrameUpdateStats{FullFrame: true, Regions: 1, Bytes: len(buffer)}, nil
}

func (u *FrameUpdater) remember(buffer []byte) {
	if len(u.previous) != len(buffer) {
		u.previous = make([]byte, len(buffer))
	}
	copy(u.previous, buffer)
}

func dirtyRegions(previous, current []byte, width, height, bytesPerPixel, tileSize int) []Region {
	tilesX := (width + tileSize - 1) / tileSize
	tilesY := (height + tileSize - 1) / tileSize
	active := make(map[[2]int]int)
	regions := make([]Region, 0)

	for tileY := 0; tileY < tilesY; tileY++ {
		runs := changedTileRuns(previous, current, width, height, bytesPerPixel, tileSize, tileY, tilesX)
		next := make(map[[2]int]int, len(runs))
		for _, run := range runs {
			key := [2]int{run[0], run[1]}
			if index, ok := active[key]; ok {
				regions[index].Height += min(tileSize, height-(tileY*tileSize))
				next[key] = index
				continue
			}

			x := run[0] * tileSize
			endX := min(run[1]*tileSize, width)
			regions = append(regions, Region{
				X:      x,
				Y:      tileY * tileSize,
				Width:  endX - x,
				Height: min(tileSize, height-(tileY*tileSize)),
			})
			next[key] = len(regions) - 1
		}
		active = next
	}

	return regions
}

func changedTileRuns(previous, current []byte, width, height, bytesPerPixel, tileSize, tileY, tilesX int) [][2]int {
	var runs [][2]int
	runStart := -1
	for tileX := 0; tileX < tilesX; tileX++ {
		changed := tileChanged(previous, current, width, height, bytesPerPixel, tileSize, tileX, tileY)
		if changed && runStart < 0 {
			runStart = tileX
		}
		if !changed && runStart >= 0 {
			runs = append(runs, [2]int{runStart, tileX})
			runStart = -1
		}
	}
	if runStart >= 0 {
		runs = append(runs, [2]int{runStart, tilesX})
	}
	return runs
}

func tileChanged(previous, current []byte, width, height, bytesPerPixel, tileSize, tileX, tileY int) bool {
	x0 := tileX * tileSize
	y0 := tileY * tileSize
	x1 := min(x0+tileSize, width)
	y1 := min(y0+tileSize, height)
	rowBytes := (x1 - x0) * bytesPerPixel

	for y := y0; y < y1; y++ {
		start := (y*width + x0) * bytesPerPixel
		if !bytes.Equal(previous[start:start+rowBytes], current[start:start+rowBytes]) {
			return true
		}
	}
	return false
}

func chooseTransferRegions(regions []Region, width, height, bytesPerPixel int) []Region {
	if len(regions) == 0 {
		return regions
	}

	regions = joinTransferRegions(regions, bytesPerPixel)
	fullCost := regionCommandOverhead + width*height*bytesPerPixel
	regionalCost := transferCost(regions, bytesPerPixel)

	bounds := boundingRegion(regions)
	boundsCost := transferCost([]Region{bounds}, bytesPerPixel)
	if boundsCost < regionalCost {
		regions = []Region{bounds}
		regionalCost = boundsCost
	}
	if regionalCost >= fullCost {
		return nil
	}
	return regions
}

func adaptiveDirtyRegions(previous, current []byte, width, height, bytesPerPixel int) []Region {
	best := joinTransferRegions(
		costAwareDirtyRegions(previous, current, width, height, bytesPerPixel),
		bytesPerPixel,
	)
	if len(best) == 0 {
		return nil
	}
	bestCost := transferCost(best, bytesPerPixel)
	for _, tileSize := range adaptiveDirtyTileSizes {
		regions := dirtyRegions(previous, current, width, height, bytesPerPixel, tileSize)
		regions = joinTransferRegions(regions, bytesPerPixel)
		cost := transferCost(regions, bytesPerPixel)
		if cost < bestCost {
			best = regions
			bestCost = cost
		}
	}
	return best
}

type rowJoin struct {
	active int
	run    int
	delta  int
	bounds Region
}

// costAwareDirtyRegions sweeps exact changed-pixel runs from top to bottom.
// Runs are extended into rectangles only when the added unchanged pixels cost
// less than starting another USB command.
func costAwareDirtyRegions(previous, current []byte, width, height, bytesPerPixel int) []Region {
	regions := make([]Region, 0)
	var active []int

	for y := 0; y < height; y++ {
		runs := changedPixelRuns(previous, current, width, bytesPerPixel, y)
		joins := make([]rowJoin, 0, len(active)*len(runs))
		for activePosition, regionIndex := range active {
			for runIndex, run := range runs {
				bounds := boundingRegion([]Region{regions[regionIndex], run})
				delta := transferCost([]Region{bounds}, bytesPerPixel) -
					transferCost([]Region{regions[regionIndex], run}, bytesPerPixel)
				if delta < 0 {
					joins = append(joins, rowJoin{
						active: activePosition,
						run:    runIndex,
						delta:  delta,
						bounds: bounds,
					})
				}
			}
		}
		sort.Slice(joins, func(i, j int) bool {
			return joins[i].delta < joins[j].delta
		})

		nextActive := make([]int, 0, len(runs))
		usedActive := make([]bool, len(active))
		usedRuns := make([]bool, len(runs))
		for _, join := range joins {
			if usedActive[join.active] || usedRuns[join.run] {
				continue
			}
			regionIndex := active[join.active]
			regions[regionIndex] = join.bounds
			nextActive = append(nextActive, regionIndex)
			usedActive[join.active] = true
			usedRuns[join.run] = true
		}
		for runIndex, run := range runs {
			if usedRuns[runIndex] {
				continue
			}
			regions = append(regions, run)
			nextActive = append(nextActive, len(regions)-1)
		}
		active = nextActive
	}

	return regions
}

func changedPixelRuns(previous, current []byte, width, bytesPerPixel, y int) []Region {
	var runs []Region
	runStart := -1
	for x := 0; x < width; x++ {
		offset := (y*width + x) * bytesPerPixel
		changed := !bytes.Equal(
			previous[offset:offset+bytesPerPixel],
			current[offset:offset+bytesPerPixel],
		)
		if changed && runStart < 0 {
			runStart = x
		}
		if !changed && runStart >= 0 {
			runs = append(runs, Region{X: runStart, Y: y, Width: x - runStart, Height: 1})
			runStart = -1
		}
	}
	if runStart >= 0 {
		runs = append(runs, Region{X: runStart, Y: y, Width: width - runStart, Height: 1})
	}
	return runs
}

func dirtyBoundingRegion(previous, current []byte, width, height, bytesPerPixel int) (Region, bool) {
	x0, y0 := width, height
	x1, y1 := 0, 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * bytesPerPixel
			if bytes.Equal(
				previous[offset:offset+bytesPerPixel],
				current[offset:offset+bytesPerPixel],
			) {
				continue
			}
			x0 = min(x0, x)
			y0 = min(y0, y)
			x1 = max(x1, x+1)
			y1 = max(y1, y+1)
		}
	}
	if x0 == width {
		return Region{}, false
	}
	return Region{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}, true
}

// joinTransferRegions greedily replaces pairs with their bounding rectangle
// whenever the extra pixels cost less than issuing another USB command.
func joinTransferRegions(regions []Region, bytesPerPixel int) []Region {
	if len(regions) < 2 || len(regions) > maxJoinCandidates {
		return regions
	}

	regions = append([]Region(nil), regions...)
	for {
		bestI, bestJ := -1, -1
		bestDelta := 0
		var bestBounds Region
		for i := 0; i < len(regions); i++ {
			for j := i + 1; j < len(regions); j++ {
				bounds := boundingRegion([]Region{regions[i], regions[j]})
				delta := transferCost([]Region{bounds}, bytesPerPixel) -
					transferCost([]Region{regions[i], regions[j]}, bytesPerPixel)
				if delta < bestDelta {
					bestI, bestJ = i, j
					bestDelta = delta
					bestBounds = bounds
				}
			}
		}
		if bestI < 0 {
			return regions
		}

		regions[bestI] = bestBounds
		regions[bestJ] = regions[len(regions)-1]
		regions = regions[:len(regions)-1]
	}
}

func transferCost(regions []Region, bytesPerPixel int) int {
	cost := len(regions) * regionCommandOverhead
	for _, region := range regions {
		cost += region.Width * region.Height * bytesPerPixel
	}
	return cost
}

func boundingRegion(regions []Region) Region {
	x0, y0 := regions[0].X, regions[0].Y
	x1 := regions[0].X + regions[0].Width
	y1 := regions[0].Y + regions[0].Height
	for _, region := range regions[1:] {
		x0 = min(x0, region.X)
		y0 = min(y0, region.Y)
		x1 = max(x1, region.X+region.Width)
		y1 = max(y1, region.Y+region.Height)
	}
	return Region{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

func extractRegion(buffer []byte, frameWidth, bytesPerPixel int, region Region) []byte {
	rowBytes := region.Width * bytesPerPixel
	result := make([]byte, rowBytes*region.Height)
	for row := 0; row < region.Height; row++ {
		source := ((region.Y+row)*frameWidth + region.X) * bytesPerPixel
		copy(result[row*rowBytes:(row+1)*rowBytes], buffer[source:source+rowBytes])
	}
	return result
}
