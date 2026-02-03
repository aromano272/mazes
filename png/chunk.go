package png

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

type Chunk struct {
	chunkType ChunkType
	data      []byte
}

type ChunkType string

const (
	IHDR ChunkType = "IHDR"
	IEND ChunkType = "IEND"
	PLTE ChunkType = "PLTE"
	IDAT ChunkType = "IDAT"
)

func extractChunks(data []byte) ([]*Chunk, error) {
	if binary.BigEndian.Uint64(data) != file_sign {
		return nil, fmt.Errorf("invalid PNG signature")
	}
	data = data[8:]

	chunks := make([]*Chunk, 0)

	for len(data) > 0 {
		chunk, read, err := readChunk(data)
		if err != nil {
			return nil, err
		}
		data = data[read:]
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

func readChunk(data []byte) (*Chunk, int, error) {
	if len(data) < 4 {
		return nil, -1, fmt.Errorf("invalid chunk of length: %d", len(data))
	}
	newData := data
	length := binary.BigEndian.Uint32(newData[:4])
	newData = newData[4:]

	if len(newData) < 4+int(length)+4 {
		return nil, -1, fmt.Errorf("invalid chunk of length: %d, expected length of: %d", len(data), 4+4+length+4)
	}
	chunkType := ChunkType(newData[:4])
	newData = newData[4:]

	chunkData := newData[:length]
	newData = newData[length:]

	expectedChecksum := binary.BigEndian.Uint32(newData[:4])
	newData = newData[4:]

	actualChecksum := crc32.ChecksumIEEE(data[4 : 8+length])
	if expectedChecksum != actualChecksum {
		return nil, -1, fmt.Errorf("checksum mismatch, expected %d, got: %d", expectedChecksum, actualChecksum)
	}

	return &Chunk{
		chunkType: chunkType,
		data:      chunkData,
	}, len(data) - len(newData), nil
}

func decodeIHDRChunk(data []byte) (ihdrData, error) {
	res := ihdrData{}
	if len(data) != 13 {
		return res, fmt.Errorf("expected IHDR chunk to be 13 bytes long, was: %d", len(data))
	}

	res.Width = int(binary.BigEndian.Uint32(data[:4]))
	res.Height = int(binary.BigEndian.Uint32(data[4:8]))
	bitDepth := data[8]
	res.BitDepth = bitDepth

	switch data[9] {
	case 0:
		if bitDepth != 1 && bitDepth != 2 && bitDepth != 4 && bitDepth != 8 && bitDepth != 16 {
			return res, fmt.Errorf("invalid bit depth for grayscale: %d", bitDepth)
		}
		res.ColorType = ColorTypeGrayscale
	case 2:
		if bitDepth != 8 && bitDepth != 16 {
			return res, fmt.Errorf("invalid bit depth for truecolor: %d", bitDepth)
		}
		res.ColorType = ColorTypeTruecolor
	case 3:
		if bitDepth != 1 && bitDepth != 2 && bitDepth != 4 && bitDepth != 8 {
			return res, fmt.Errorf("invalid bit depth for palette: %d", bitDepth)
		}
		res.ColorType = ColorTypePalette
	case 4:
		if bitDepth != 8 && bitDepth != 16 {
			return res, fmt.Errorf("invalid bit depth for grayscale with Alpha: %d", bitDepth)
		}
		res.ColorType = ColorTypeGrayscaleAlpha
	case 6:
		if bitDepth != 8 && bitDepth != 16 {
			return res, fmt.Errorf("invalid bit depth for truecolor with Alpha: %d", bitDepth)
		}
		res.ColorType = ColorTypeTruecolorAlpha
	default:
		return res, fmt.Errorf("invalid color type: %d", data[9])
	}

	if data[10] != 0 {
		return res, fmt.Errorf("invalid compression method: %d", data[10])
	}

	if data[11] != 0 {
		return res, fmt.Errorf("invalid filter method: %d", data[11])
	}

	switch data[12] {
	case 0:
		res.InterlaceMethod = InterlaceMethodNone
	case 1:
		res.InterlaceMethod = InterlaceMethodAdam7
	default:
		return res, fmt.Errorf("invalid interlace method: %d", data[12])
	}

	return res, nil
}

func decodePLTEChunk(ihdrData ihdrData, data []byte) (plteData, error) {
	res := plteData{}

	if len(data)%3 != 0 {
		return res, errors.New("invalid PLTE chunk data, not divisible by 3")
	}
	if len(data)/3 > 1<<ihdrData.BitDepth {
		return res, fmt.Errorf("invalid PLTE chunk data, max samples: %d found: %d samples", 1<<ihdrData.BitDepth, len(data)/3)
	}

	entries := make([]TruecolorPixel, 0)
	for len(data) > 0 {
		pixel := TruecolorPixel{
			Red:   uint(data[0]),
			Green: uint(data[1]),
			Blue:  uint(data[2]),
			Alpha: 0xFF,
		}
		entries = append(entries, pixel)
		data = data[3:]
	}

	res.Entries = entries

	return res, nil
}
