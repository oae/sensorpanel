package panel

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"
)

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
	if d.outEP == nil || d.inEP == nil {
		return ErrDeviceNotOpen
	}

	packet := buildLYPacket(payload)
	for offset := 0; offset < len(packet); {
		writeSize := lyUSBWriteSize
		remaining := len(packet) - offset
		if remaining < writeSize {
			writeSize = remaining
		}

		writeCtx, writeCancel := context.WithTimeout(context.Background(), lyWriteTimeout)
		n, err := d.outEP.WriteContext(writeCtx, packet[offset:offset+writeSize])
		writeCancel()
		if err != nil {
			return fmt.Errorf("LY frame write: %w", err)
		}
		if n != writeSize {
			return fmt.Errorf("%w: LY frame wrote %d/%d bytes", ErrWriteIncomplete, n, writeSize)
		}
		offset += writeSize
	}

	ack := make([]byte, lyHandshakeReadSize)
	readCtx, readCancel := context.WithTimeout(context.Background(), lyReadTimeout)
	defer readCancel()
	if _, err := d.inEP.ReadContext(readCtx, ack); err != nil {
		return fmt.Errorf("LY frame ACK read: %w", err)
	}

	return nil
}

func buildLYPacket(payload []byte) []byte {
	totalSize := len(payload)
	numChunks := totalSize/lyChunkDataSize + 1
	chunks := make([]byte, numChunks*lyChunkSize)
	lastChunkData := totalSize % lyChunkDataSize

	for i := 0; i < numChunks; i++ {
		offset := i * lyChunkSize
		dataLen := lyChunkDataSize
		if i == numChunks-1 {
			dataLen = lastChunkData
		}

		chunks[offset] = 0x01
		chunks[offset+1] = 0xff
		binary.LittleEndian.PutUint32(chunks[offset+2:offset+6], uint32(totalSize))
		binary.LittleEndian.PutUint16(chunks[offset+6:offset+8], uint16(dataLen))
		chunks[offset+8] = lyChunkCommand
		binary.LittleEndian.PutUint16(chunks[offset+9:offset+11], uint16(numChunks))
		binary.LittleEndian.PutUint16(chunks[offset+11:offset+13], uint16(i))

		srcOffset := i * lyChunkDataSize
		copy(chunks[offset+lyChunkHeaderSize:offset+lyChunkHeaderSize+dataLen], payload[srcOffset:srcOffset+dataLen])
	}

	paddedChunks := numChunks
	if remainder := paddedChunks % lyPadChunkMultiple; remainder != 0 {
		paddedChunks += lyPadChunkMultiple - remainder
	}
	if paddedChunks == numChunks {
		return chunks
	}

	padded := make([]byte, paddedChunks*lyChunkSize)
	copy(padded, chunks)
	return padded
}
