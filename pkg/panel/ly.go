package panel

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"
)

// DisplayMetrics contains the expensive stages of one LY frame presentation.
type DisplayMetrics struct {
	Rotate    time.Duration `json:"rotate"`
	Encode    time.Duration `json:"encode"`
	Packetize time.Duration `json:"packetize"`
	USBWrite  time.Duration `json:"usb_write"`
	ACK       time.Duration `json:"ack"`
	JPEGBytes int           `json:"jpeg_bytes"`
	WireBytes int           `json:"wire_bytes"`
}

const (
	lyHandshakeReadSize  = 512
	lyHandshakeTimeout   = time.Second
	lyWriteTimeout       = 5 * time.Second
	lyReadTimeout        = time.Second
	lyChunkSize          = 512
	lyChunkHeaderSize    = 16
	lyChunkDataSize      = lyChunkSize - lyChunkHeaderSize
	lyUSBWriteSize       = 4096
	lyChunkCommand       = 1
	lyPadChunkMultiple   = 4
	lyHandshakePayloadSz = 2048
)

var lyHandshakeHeader = []byte{
	0x02, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

func (d *Device) lyHandshake() error {
	payload := make([]byte, lyHandshakePayloadSz)
	copy(payload, lyHandshakeHeader)

	writeCtx, writeCancel := context.WithTimeout(context.Background(), lyHandshakeTimeout)
	defer writeCancel()
	n, err := d.outEP.WriteContext(writeCtx, payload)
	if err != nil {
		return fmt.Errorf("LY handshake write: %w", err)
	}
	if n != len(payload) {
		return fmt.Errorf("%w: LY handshake wrote %d/%d bytes", ErrWriteIncomplete, n, len(payload))
	}

	response := make([]byte, lyHandshakeReadSize)
	readCtx, readCancel := context.WithTimeout(context.Background(), lyHandshakeTimeout)
	defer readCancel()
	n, err = d.inEP.ReadContext(readCtx, response)
	if err != nil {
		return fmt.Errorf("LY handshake read: %w", err)
	}
	if n < 37 {
		return fmt.Errorf("%w: LY handshake got %d bytes, expected at least 37", ErrReadIncomplete, n)
	}
	if response[0] != 0x03 || response[1] != 0xff || response[8] != 0x01 {
		return fmt.Errorf("invalid LY handshake response: [0]=0x%02x [1]=0x%02x [8]=0x%02x",
			response[0], response[1], response[8])
	}

	return nil
}

func (d *Device) lySend(payload []byte) error {
	_, err := d.lySendTimed(payload)
	return err
}

func (d *Device) lySendTimed(payload []byte) (DisplayMetrics, error) {
	metrics := DisplayMetrics{JPEGBytes: len(payload)}
	if d.outEP == nil || d.inEP == nil {
		return metrics, ErrDeviceNotOpen
	}

	started := time.Now()
	packet := buildLYPacketInto(payload, d.lyPacket)
	metrics.Packetize = time.Since(started)
	metrics.WireBytes = len(packet)
	d.lyPacket = packet
	writeCtx, writeCancel := context.WithTimeout(context.Background(), lyWriteTimeout)
	defer writeCancel()
	started = time.Now()
	for offset := 0; offset < len(packet); {
		writeSize := lyUSBWriteSize
		remaining := len(packet) - offset
		if remaining < writeSize {
			writeSize = remaining
		}

		n, err := d.outEP.WriteContext(writeCtx, packet[offset:offset+writeSize])
		if err != nil {
			return metrics, fmt.Errorf("LY frame write: %w", err)
		}
		if n != writeSize {
			return metrics, fmt.Errorf("%w: LY frame wrote %d/%d bytes", ErrWriteIncomplete, n, writeSize)
		}
		offset += writeSize
	}
	metrics.USBWrite = time.Since(started)

	if cap(d.lyACK) < lyHandshakeReadSize {
		d.lyACK = make([]byte, lyHandshakeReadSize)
	} else {
		d.lyACK = d.lyACK[:lyHandshakeReadSize]
	}
	readCtx, readCancel := context.WithTimeout(context.Background(), lyReadTimeout)
	defer readCancel()
	started = time.Now()
	if _, err := d.inEP.ReadContext(readCtx, d.lyACK); err != nil {
		return metrics, fmt.Errorf("LY frame ACK read: %w", err)
	}
	metrics.ACK = time.Since(started)

	return metrics, nil
}

func buildLYPacket(payload []byte) []byte {
	return buildLYPacketInto(payload, nil)
}

func buildLYPacketInto(payload, reuse []byte) []byte {
	totalSize := len(payload)
	numChunks := totalSize/lyChunkDataSize + 1
	paddedChunks := numChunks
	if remainder := paddedChunks % lyPadChunkMultiple; remainder != 0 {
		paddedChunks += lyPadChunkMultiple - remainder
	}
	packetSize := paddedChunks * lyChunkSize
	var chunks []byte
	if cap(reuse) < packetSize {
		chunks = make([]byte, packetSize)
	} else {
		chunks = reuse[:packetSize]
	}
	lastChunkData := totalSize % lyChunkDataSize

	for i := 0; i < numChunks; i++ {
		offset := i * lyChunkSize
		dataLen := lyChunkDataSize
		if i == numChunks-1 {
			dataLen = lastChunkData
		}

		clear(chunks[offset : offset+lyChunkHeaderSize])
		chunks[offset] = 0x01
		chunks[offset+1] = 0xff
		binary.LittleEndian.PutUint32(chunks[offset+2:offset+6], uint32(totalSize))
		binary.LittleEndian.PutUint16(chunks[offset+6:offset+8], uint16(dataLen))
		chunks[offset+8] = lyChunkCommand
		binary.LittleEndian.PutUint16(chunks[offset+9:offset+11], uint16(numChunks))
		binary.LittleEndian.PutUint16(chunks[offset+11:offset+13], uint16(i))

		srcOffset := i * lyChunkDataSize
		copy(chunks[offset+lyChunkHeaderSize:offset+lyChunkHeaderSize+dataLen], payload[srcOffset:srcOffset+dataLen])
		if dataLen < lyChunkDataSize {
			clear(chunks[offset+lyChunkHeaderSize+dataLen : offset+lyChunkSize])
		}
	}

	if paddedChunks > numChunks {
		clear(chunks[numChunks*lyChunkSize:])
	}
	return chunks
}
