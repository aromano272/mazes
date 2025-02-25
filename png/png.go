package png

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const FILE_SIGN = 0x89504E470D0A1A0A

type Chunk struct {
	chunkType ChunkType
	data      []byte
}

type ChunkType string

type Pixel struct {
	R uint
	G uint
	B uint
	A uint
}

type Png struct {
	Width           uint
	Height          uint
	ColorType       ColorType
	BitDepth        uint8
	InterlaceMethod InterlaceMethod

	Pixels [][]Pixel
}

const (
	IHDR ChunkType = "IHDR"
	IEND ChunkType = "IEND"
	IDAT ChunkType = "IDAT"
)

type ColorType int

const (
	ColorTypeGrayscale ColorType = iota
	ColorTypeTruecolor
	ColorTypePalette
	ColorTypeGrayscaleAlpha
	ColorTypeTruecolorAlpha
)

type InterlaceMethod int

const (
	InterlaceMethodNone InterlaceMethod = iota
	InterlaceMethodAdam7
)

type Decoder struct {
	data []byte
}

func DecodePng(data []byte) (*Png, error) {
	decoder := Decoder{data: data}

	err := decoder.readSig()
	if err != nil {
		return nil, err
	}

	chunks := make([]*Chunk, 0)

	for len(decoder.data) > 0 {
		chunk, err := decoder.readChunk()
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}

	png := &Png{}

	hasProcessedIDAT := false
	IDATData := make([]byte, 0)
	for index, chunk := range chunks {
		if index == 0 && chunk.chunkType != IHDR {
			return nil, fmt.Errorf("first chunk should be IHDR, found: %s", chunk.chunkType)
		}
		if index == len(chunks)-1 {
			if chunk.chunkType != IEND {
				return nil, fmt.Errorf("last chunk should be IEND, found: %s", chunk.chunkType)
			}
			if !hasProcessedIDAT {
				return nil, fmt.Errorf("there should be atleast one IDAT chunk")
			}
		}

		switch chunk.chunkType {
		case IHDR:
			res, err := decodeIHDRChunk(chunk.data)
			if err != nil {
				return nil, err
			}
			png.Width = res.Width
			png.Height = res.Height
			png.ColorType = res.ColorType
			png.BitDepth = res.BitDepth
			png.InterlaceMethod = res.InterlaceMethod
		case IDAT:
			if hasProcessedIDAT {
				return nil, fmt.Errorf("IDAT chunks should be contiguous")
			}
			isProcessingIDAT = true

			res, err := decodeIDATChunk(chunk.data)
			if err != nil {
				return nil, err
			}
			IDATData = append(IDATData, res...)
		}
	}

	return png, nil
}

type PalettePixel struct {
	index int
}

type GreyscalePixel struct {
	value int
}

type TruecolorPixel struct {
	red   int
	green int
	blue  int
}

type IHDRData struct {
	Width           uint
	Height          uint
	BitDepth        uint8
	ColorType       ColorType
	InterlaceMethod InterlaceMethod
}

func decodeIHDRChunk(data []byte) (IHDRData, error) {
	res := IHDRData{}
	if len(data) != 13 {
		return res, fmt.Errorf("expected IHDR chunk to be 13 bytes long, was: %d", len(data))
	}

	res.Width = uint(binary.BigEndian.Uint32(data[:4]))
	res.Width = uint(binary.BigEndian.Uint32(data[4:8]))
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
			return res, fmt.Errorf("invalid bit depth for grayscale with alpha: %d", bitDepth)
		}
		res.ColorType = ColorTypeGrayscaleAlpha
	case 6:
		if bitDepth != 8 && bitDepth != 16 {
			return res, fmt.Errorf("invalid bit depth for truecolor with alpha: %d", bitDepth)
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

func (d *Decoder) readSig() error {
	sig := d.data[:8]
	d.data = d.data[8:]

	if binary.BigEndian.Uint64(sig) != FILE_SIGN {
		return fmt.Errorf("invalid PNG signature")
	}

	return nil
}

func (d *Decoder) readChunk() (*Chunk, error) {
	newData := d.data
	length := binary.BigEndian.Uint32(newData[:4])
	newData = newData[4:]

	chunkType := ChunkType(newData[:4])
	newData = newData[4:]

	chunkData := newData[:length]
	newData = newData[length:]
	fmt.Printf("%x\n", chunkData)

	expectedChecksum := binary.BigEndian.Uint32(newData[:4])
	newData = newData[4:]

	actualChecksum := crc32.ChecksumIEEE(d.data[4 : 8+length])
	if expectedChecksum != actualChecksum {
		return nil, fmt.Errorf("checksum mismatch, expected %d, got: %d", expectedChecksum, actualChecksum)
	}

	d.data = newData

	return &Chunk{
		chunkType: chunkType,
		data:      chunkData,
	}, nil
}
