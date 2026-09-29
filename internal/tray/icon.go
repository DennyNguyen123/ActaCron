package tray

import (
	"bytes"
	"encoding/binary"
)

// generateDefaultIcon creates a 16x16 32-bit RGBA ICO in pure Go (Green cron dot)
func generateDefaultIcon() []byte {
	width := 16
	height := 16
	bpp := 32

	headerSize := 40
	pixelDataSize := width * height * 4
	andMaskSize := ((width + 31) / 32) * 4 * height
	imageSize := headerSize + pixelDataSize + andMaskSize

	buf := new(bytes.Buffer)

	// ICONDIR (6 bytes)
	binary.Write(buf, binary.LittleEndian, uint16(0)) // Reserved
	binary.Write(buf, binary.LittleEndian, uint16(1)) // ICO
	binary.Write(buf, binary.LittleEndian, uint16(1)) // 1 image

	// ICONDIRENTRY (16 bytes)
	buf.WriteByte(byte(width))
	buf.WriteByte(byte(height))
	buf.WriteByte(0) // 0 for >= 8bpp
	buf.WriteByte(0) // Reserved
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint16(bpp))
	binary.Write(buf, binary.LittleEndian, uint32(imageSize))
	binary.Write(buf, binary.LittleEndian, uint32(22)) // Offset

	// BITMAPINFOHEADER (40 bytes)
	binary.Write(buf, binary.LittleEndian, uint32(headerSize))
	binary.Write(buf, binary.LittleEndian, int32(width))
	binary.Write(buf, binary.LittleEndian, int32(height*2)) // Doubled for icon
	binary.Write(buf, binary.LittleEndian, uint16(1))       // Planes
	binary.Write(buf, binary.LittleEndian, uint16(bpp))     // BitCount
	binary.Write(buf, binary.LittleEndian, uint32(0))       // BI_RGB
	binary.Write(buf, binary.LittleEndian, uint32(pixelDataSize+andMaskSize))
	binary.Write(buf, binary.LittleEndian, int32(0))
	binary.Write(buf, binary.LittleEndian, int32(0))
	binary.Write(buf, binary.LittleEndian, uint32(0))
	binary.Write(buf, binary.LittleEndian, uint32(0))

	// Pixel data: bottom-to-top RGBA (Green dot in center)
	center := 7.5
	radius := 6.0
	for y := height - 1; y >= 0; y-- {
		for x := 0; x < width; x++ {
			dx := float64(x) - center
			dy := float64(y) - center
			distSq := dx*dx + dy*dy
			if distSq <= radius*radius {
				// Accent green: #22C55E (B: 0x5E, G: 0xC5, R: 0x22, A: 0xFF)
				buf.WriteByte(0x5E)
				buf.WriteByte(0xC5)
				buf.WriteByte(0x22)
				buf.WriteByte(0xFF)
			} else {
				// Transparent
				buf.WriteByte(0)
				buf.WriteByte(0)
				buf.WriteByte(0)
				buf.WriteByte(0)
			}
		}
	}

	// AND mask (1 bit per pixel: 0 for opaque, 1 for transparent)
	for y := height - 1; y >= 0; y-- {
		var rowMask uint32 = 0
		for x := 0; x < width; x++ {
			dx := float64(x) - center
			dy := float64(y) - center
			distSq := dx*dx + dy*dy
			if distSq > radius*radius {
				rowMask |= (1 << (31 - x))
			}
		}
		binary.Write(buf, binary.BigEndian, rowMask)
	}

	return buf.Bytes()
}
