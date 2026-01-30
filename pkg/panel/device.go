// Package panel provides the USB device interface for AX206 displays.
package panel

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/google/gousb"
)

// Errors
var (
	ErrDeviceNotFound     = errors.New("no AX206 device found")
	ErrDeviceNotOpen      = errors.New("device not open")
	ErrDeviceAlreadyOpen  = errors.New("device already open")
	ErrWriteIncomplete    = errors.New("USB write incomplete")
	ErrReadIncomplete     = errors.New("USB read incomplete")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrDeviceBusy         = errors.New("device busy")
	ErrNoDeviceConfigured = errors.New("no device configured - run 'sensorpanel device select' first")
)

// Device represents an AX206 USB display.
type Device struct {
	mu sync.Mutex

	ctx      *gousb.Context
	device   *gousb.Device
	intf     *gousb.Interface
	intfDone func()
	outEP    *gousb.OutEndpoint
	inEP     *gousb.InEndpoint

	// Target device identifiers (set before Open)
	targetVID    uint16
	targetPID    uint16
	targetSerial string // Optional: match specific serial number

	// Device info populated on Open()
	Info *DeviceInfo
}

// NewDeviceWithID creates a Device targeting a specific VID/PID.
func NewDeviceWithID(vid, pid uint16) *Device {
	return &Device{
		targetVID: vid,
		targetPID: pid,
		Info:      NewDeviceInfo(),
	}
}

// NewDeviceWithSerial creates a Device targeting a specific VID/PID and serial.
func NewDeviceWithSerial(vid, pid uint16, serial string) *Device {
	return &Device{
		targetVID:    vid,
		targetPID:    pid,
		targetSerial: serial,
		Info:         NewDeviceInfo(),
	}
}

// IsOpen returns true if the device is currently open.
func (d *Device) IsOpen() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.device != nil
}

// FindDevices returns a list of connected devices matching the given VID/PID.
// VID and PID must be provided (non-zero).
func FindDevices(vid, pid uint16) ([]*gousb.Device, error) {
	if vid == 0 || pid == 0 {
		return nil, ErrNoDeviceConfigured
	}

	ctx := gousb.NewContext()
	defer ctx.Close()

	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return desc.Vendor == gousb.ID(vid) && desc.Product == gousb.ID(pid)
	})
	if err != nil {
		return nil, fmt.Errorf("error scanning USB devices: %w", err)
	}

	return devices, nil
}

// IsDeviceConnected checks if a device with the given VID/PID is connected.
// VID and PID must be provided (non-zero).
func IsDeviceConnected(vid, pid uint16) bool {
	if vid == 0 || pid == 0 {
		return false
	}

	ctx := gousb.NewContext()
	defer ctx.Close()

	var found bool
	ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if desc.Vendor == gousb.ID(vid) && desc.Product == gousb.ID(pid) {
			found = true
			return false // Don't actually open
		}
		return false
	})

	return found
}

// GetDeviceInfo returns information about a connected device without fully opening it.
// VID and PID must be provided (non-zero).
func GetDeviceInfo(vid, pid uint16) (*DeviceInfo, error) {
	if vid == 0 || pid == 0 {
		return nil, ErrNoDeviceConfigured
	}

	ctx := gousb.NewContext()
	defer ctx.Close()

	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return desc.Vendor == gousb.ID(vid) && desc.Product == gousb.ID(pid)
	})
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, ErrDeviceNotFound
	}

	dev := devices[0]
	defer dev.Close()

	// Close any extra devices
	for i := 1; i < len(devices); i++ {
		devices[i].Close()
	}

	// Build DeviceInfo from USB descriptors
	info := NewDeviceInfo()
	info.VendorID = uint16(dev.Desc.Vendor)
	info.ProductID = uint16(dev.Desc.Product)
	info.Manufacturer, _ = dev.Manufacturer()
	info.Product, _ = dev.Product()
	info.Serial, _ = dev.SerialNumber()
	info.Speed = dev.Desc.Speed.String()

	// Get max packet size from first bulk OUT endpoint
	for _, cfg := range dev.Desc.Configs {
		for _, intf := range cfg.Interfaces {
			for _, alt := range intf.AltSettings {
				for _, ep := range alt.Endpoints {
					if ep.Direction == gousb.EndpointDirectionOut {
						info.MaxPacketSize = ep.MaxPacketSize
						break
					}
				}
			}
		}
	}

	return info, nil
}

// CheckDeviceAccess attempts to open and claim a USB device to verify permissions.
// Returns nil if access is granted, or an error describing the permission issue.
// This fully opens the device, claims the interface, then closes everything.
func CheckDeviceAccess(vid, pid uint16) error {
	if vid == 0 || pid == 0 {
		return ErrNoDeviceConfigured
	}

	ctx := gousb.NewContext()
	defer ctx.Close()

	// Try to open the device
	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return desc.Vendor == gousb.ID(vid) && desc.Product == gousb.ID(pid)
	})
	if err != nil {
		// Check for permission-related errors
		if isPermissionError(err) {
			return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
		}
		return fmt.Errorf("failed to open device: %w", err)
	}
	if len(devices) == 0 {
		return ErrDeviceNotFound
	}

	// Close extra devices
	for i := 1; i < len(devices); i++ {
		devices[i].Close()
	}
	dev := devices[0]
	defer dev.Close()

	// Try to set auto-detach (needed on Linux to detach kernel driver)
	_ = dev.SetAutoDetach(true) // Ignore errors, not all platforms support this

	// Try to claim the interface - this is where permission errors usually occur
	intf, done, err := dev.DefaultInterface()
	if err != nil {
		if isPermissionError(err) {
			return fmt.Errorf("%w: cannot claim interface: %v", ErrPermissionDenied, err)
		}
		if isDeviceBusyError(err) {
			return fmt.Errorf("%w: another process may be using the device", ErrDeviceBusy)
		}
		return fmt.Errorf("failed to claim interface: %w", err)
	}
	done()
	_ = intf // We just need to verify we can claim it

	return nil
}

// isPermissionError checks if an error is related to USB permissions.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// libusb error strings that indicate permission issues
	permissionIndicators := []string{
		"access denied",
		"permission denied",
		"LIBUSB_ERROR_ACCESS",
		"operation not permitted",
		"insufficient permissions",
	}
	for _, indicator := range permissionIndicators {
		if containsIgnoreCase(errStr, indicator) {
			return true
		}
	}
	return false
}

// isDeviceBusyError checks if an error indicates the device is in use.
func isDeviceBusyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	busyIndicators := []string{
		"device or resource busy",
		"resource busy",
		"LIBUSB_ERROR_BUSY",
		"code -6",
	}
	for _, indicator := range busyIndicators {
		if containsIgnoreCase(errStr, indicator) {
			return true
		}
	}
	return false
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	sLower := make([]byte, len(s))
	substrLower := make([]byte, len(substr))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		sLower[i] = c
	}
	for i := 0; i < len(substr); i++ {
		c := substr[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		substrLower[i] = c
	}
	return bytesContains(sLower, substrLower)
}

// bytesContains checks if b contains sub.
func bytesContains(b, sub []byte) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(b) {
		return false
	}
	for i := 0; i <= len(b)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if b[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// PermissionFixInstructions returns platform-specific instructions for fixing USB permission issues.
func PermissionFixInstructions(vid, pid uint16) string {
	return fmt.Sprintf(`USB permission denied. To fix this:

Linux (udev rules):
  1. Create /etc/udev/rules.d/99-sensorpanel.rules with:
     SUBSYSTEM=="usb", ATTR{idVendor}=="%04x", ATTR{idProduct}=="%04x", MODE="0666"
  2. Reload rules: sudo udevadm control --reload-rules
  3. Reconnect the USB device

Linux (quick fix):
  Run with sudo, or add yourself to the 'plugdev' group:
    sudo usermod -aG plugdev $USER
  Then log out and back in.

macOS:
  No special permissions usually needed. If issues persist,
  check System Preferences > Security & Privacy.

Windows:
  Install libusb driver using Zadig (https://zadig.akeo.ie/):
  1. Run Zadig as Administrator
  2. Select your device (%04x:%04x)
  3. Install WinUSB or libusb-win32 driver`, vid, pid, vid, pid)
}

// DeviceBusyInstructions returns instructions for when the device is in use.
func DeviceBusyInstructions() string {
	return `Device is busy (another process is using it).

To fix this:
  1. Close any other programs that might be using the USB display
     (e.g., another instance of sensorpanel, AIDA64, etc.)
  2. Try unplugging and reconnecting the USB device
  3. If on Linux, check if a kernel driver has claimed the device:
     lsusb -t
     If so, the sensorpanel should auto-detach it, but you may need
     to unload the driver manually: sudo modprobe -r usblp (example)`
}

// Open connects to the AX206 device and prepares it for use.
func (d *Device) Open() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.device != nil {
		return ErrDeviceAlreadyOpen
	}

	// VID/PID must be configured
	vid := d.targetVID
	pid := d.targetPID
	if vid == 0 || pid == 0 {
		return ErrNoDeviceConfigured
	}

	// Create USB context
	d.ctx = gousb.NewContext()

	// Find and open device
	devices, err := d.ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return desc.Vendor == gousb.ID(vid) && desc.Product == gousb.ID(pid)
	})
	if err != nil {
		d.ctx.Close()
		d.ctx = nil
		return fmt.Errorf("error scanning USB devices: %w", err)
	}
	if len(devices) == 0 {
		d.ctx.Close()
		d.ctx = nil
		return ErrDeviceNotFound
	}

	// If serial is specified, find matching device
	var selectedDevice *gousb.Device
	if d.targetSerial != "" {
		for _, dev := range devices {
			serial, _ := dev.SerialNumber()
			if serial == d.targetSerial {
				selectedDevice = dev
				break
			}
		}
		if selectedDevice == nil {
			// Close all devices
			for _, dev := range devices {
				dev.Close()
			}
			d.ctx.Close()
			d.ctx = nil
			return fmt.Errorf("%w: no device with serial %s", ErrDeviceNotFound, d.targetSerial)
		}
		// Close non-selected devices
		for _, dev := range devices {
			if dev != selectedDevice {
				dev.Close()
			}
		}
	} else {
		// Use first device, close any extras
		selectedDevice = devices[0]
		for i := 1; i < len(devices); i++ {
			devices[i].Close()
		}
	}

	d.device = selectedDevice

	// Populate device info from USB descriptors
	d.Info.VendorID = uint16(d.device.Desc.Vendor)
	d.Info.ProductID = uint16(d.device.Desc.Product)
	d.Info.Manufacturer, _ = d.device.Manufacturer()
	d.Info.Product, _ = d.device.Product()
	d.Info.Serial, _ = d.device.SerialNumber()
	d.Info.Speed = d.device.Desc.Speed.String()

	// Set auto-detach for kernel driver
	if err := d.device.SetAutoDetach(true); err != nil {
		// Not fatal - some platforms don't support this
	}

	// Claim interface 0
	intf, done, err := d.device.DefaultInterface()
	if err != nil {
		d.device.Close()
		d.device = nil
		d.ctx.Close()
		d.ctx = nil
		return fmt.Errorf("failed to claim interface: %w", err)
	}
	d.intf = intf
	d.intfDone = done

	// Get endpoints
	for _, epDesc := range intf.Setting.Endpoints {
		if epDesc.Direction == gousb.EndpointDirectionOut {
			ep, err := intf.OutEndpoint(epDesc.Number)
			if err == nil {
				d.outEP = ep
				d.Info.MaxPacketSize = epDesc.MaxPacketSize
			}
		} else if epDesc.Direction == gousb.EndpointDirectionIn {
			ep, err := intf.InEndpoint(epDesc.Number)
			if err == nil {
				d.inEP = ep
			}
		}
	}

	if d.outEP == nil || d.inEP == nil {
		d.closeInternal()
		return errors.New("failed to get USB endpoints")
	}

	// Flush any pending data
	d.flush()

	// Turn on backlight at max brightness
	if err := d.setBacklightInternal(BacklightMax); err != nil {
		// Not fatal, continue anyway
	}

	return nil
}

// Close releases the device and turns off the backlight.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.closeInternal()
}

func (d *Device) closeInternal() error {
	if d.device == nil {
		return nil
	}

	// Turn off backlight
	d.setBacklightInternal(BacklightOff)

	// Release interface
	if d.intfDone != nil {
		d.intfDone()
		d.intfDone = nil
	}
	d.intf = nil
	d.outEP = nil
	d.inEP = nil

	// Close device
	if d.device != nil {
		d.device.Close()
		d.device = nil
	}

	// Close context
	if d.ctx != nil {
		d.ctx.Close()
		d.ctx = nil
	}

	return nil
}

// flush reads and discards any pending data from the IN endpoint.
func (d *Device) flush() {
	if d.inEP == nil {
		return
	}
	buf := make([]byte, 64)
	// Read with short timeout, ignore errors
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	d.inEP.ReadContext(ctx, buf)
}

// scsiCommand executes a SCSI command with CBW/CSW protocol.
func (d *Device) scsiCommand(cbw []byte, dataOut []byte) (byte, error) {
	if d.outEP == nil || d.inEP == nil {
		return 0, ErrDeviceNotOpen
	}

	// Use contexts with timeouts for USB operations
	cmdCtx, cmdCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cmdCancel()

	// Step 1: Send CBW
	n, err := d.outEP.WriteContext(cmdCtx, cbw)
	if err != nil {
		return 0, fmt.Errorf("CBW write error: %w", err)
	}
	if n != len(cbw) {
		return 0, fmt.Errorf("%w: wrote %d/%d bytes", ErrWriteIncomplete, n, len(cbw))
	}

	// Step 2: Send data (if any)
	if len(dataOut) > 0 {
		// Use longer timeout for data transfer (300KB at 500KB/s = ~600ms, use 10s for safety)
		dataCtx, dataCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dataCancel()

		n, err = d.outEP.WriteContext(dataCtx, dataOut)
		if err != nil {
			return 0, fmt.Errorf("data write error: %w", err)
		}
		if n != len(dataOut) {
			return 0, fmt.Errorf("%w: wrote %d/%d bytes", ErrWriteIncomplete, n, len(dataOut))
		}
	}

	// Step 3: Read CSW
	cswCtx, cswCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cswCancel()

	csw := make([]byte, CSWLength)
	n, err = d.inEP.ReadContext(cswCtx, csw)
	if err != nil {
		return 0, fmt.Errorf("CSW read error: %w", err)
	}
	if n != CSWLength {
		return 0, fmt.Errorf("CSW read incomplete: got %d bytes, expected %d", n, CSWLength)
	}

	// Parse CSW
	_, status, err := ParseCSW(csw)
	if err != nil {
		return 0, fmt.Errorf("CSW parse error: %w", err)
	}

	return status, nil
}

// SetBacklight sets the backlight brightness level (0-7).
func (d *Device) SetBacklight(level int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.setBacklightInternal(level)
}

func (d *Device) setBacklightInternal(level int) error {
	if d.device == nil {
		return ErrDeviceNotOpen
	}

	cbw, err := BuildSetBacklightCmd(level)
	if err != nil {
		return err
	}

	status, err := d.scsiCommand(cbw, nil)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("set backlight failed with status %d", status)
	}

	return nil
}

// DisplayBuffer sends raw RGB565 data to the display.
func (d *Device) DisplayBuffer(buffer []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.device == nil {
		return ErrDeviceNotOpen
	}

	expectedSize := d.Info.BufferSize
	if len(buffer) != expectedSize {
		return fmt.Errorf("%w: got %d bytes, expected %d", ErrBufferSizeMismatch, len(buffer), expectedSize)
	}

	cbw, err := BuildFullScreenBlitCmd()
	if err != nil {
		return err
	}

	status, err := d.scsiCommand(cbw, buffer)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("blit failed with status %d", status)
	}

	return nil
}

// DisplaySolidColor fills the screen with a solid color.
func (d *Device) DisplaySolidColor(r, g, b uint8) error {
	buffer := CreateSolidColorBuffer(r, g, b)
	return d.DisplayBuffer(buffer)
}

// DisplayTestPattern shows a 4-color quadrant test pattern.
func (d *Device) DisplayTestPattern() error {
	buffer := CreateTestPatternBuffer()
	return d.DisplayBuffer(buffer)
}

// DisplayColorBars shows an 8-color bar test pattern.
func (d *Device) DisplayColorBars() error {
	buffer := CreateColorBarsBuffer()
	return d.DisplayBuffer(buffer)
}

// DisplayImage converts and displays a Go image.
func (d *Device) DisplayImage(img image.Image) error {
	buffer := ImageToRGB565Buffer(img)
	return d.DisplayBuffer(buffer)
}

// BacklightOn turns the backlight on at maximum brightness.
func (d *Device) BacklightOn() error {
	return d.SetBacklight(BacklightMax)
}

// BacklightOff turns the backlight off.
func (d *Device) BacklightOff() error {
	return d.SetBacklight(BacklightOff)
}
