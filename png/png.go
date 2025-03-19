package png

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"math"
)

const FILE_SIGN = 0x89504E470D0A1A0A

type Chunk struct {
	chunkType ChunkType
	data      []byte
}

type ChunkType string

//type Pixel struct {
//	R uint
//	G uint
//	B uint
//	A uint
//}

type Pixel interface {
	pixel()
}

type Png struct {
	Width           int
	Height          int
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

type FilterType int

const (
	FilterTypeNone FilterType = iota
	FilterTypeSub
	FilterTypeUp
	FilterTypeAverage
	FilterTypePaeth
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

	var IHDRData IHDRData
	IDATData := make([]byte, 0)
	for index, chunk := range chunks {
		if index == 0 && chunk.chunkType != IHDR {
			return nil, fmt.Errorf("first chunk should be IHDR, found: %s", chunk.chunkType)
		}
		if index == len(chunks)-1 {
			if chunk.chunkType != IEND {
				return nil, fmt.Errorf("last chunk should be IEND, found: %s", chunk.chunkType)
			}
			// todo: implement
			if !true {
				return nil, fmt.Errorf("there should be atleast one IDAT chunk")
			}

			uncompressedIDATData := uncompressIDATData(IDATData)
			pixels, err := processIDATData(uncompressedIDATData, IHDRData)
			if err != nil {
				return nil, err
			}

			png.Pixels = pixels
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
			IHDRData = res
		case IDAT:
			IDATData = append(IDATData, chunk.data...)
		}
	}

	return png, nil
}

func uncompressIDATData(data []byte) []byte {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}
	var out bytes.Buffer
	_, err = io.Copy(&out, r)
	if err != nil {
		log.Fatal(err)
	}
	r.Close()
	return out.Bytes()
}

func processIDATData(idatData []byte, header IHDRData) ([][]Pixel, error) {
	pixels := make([][]Pixel, header.Height)

	pixelNumChannels := 0
	switch header.ColorType {
	case ColorTypeGrayscale:
		pixelNumChannels = 1
	case ColorTypeTruecolor:
		pixelNumChannels = 3
	case ColorTypePalette:
		// todo: implement

	case ColorTypeGrayscaleAlpha:
		pixelNumChannels = 2
	case ColorTypeTruecolorAlpha:
		pixelNumChannels = 4
	}

	pixelBitSize := pixelNumChannels * int(header.BitDepth)
	pixelByteSize := pixelBitSize / 8
	scanlineBitSize := pixelBitSize * header.Width
	scanlineBitPadding := scanlineBitSize % 8
	scanlineByteSize := (scanlineBitSize + scanlineBitPadding) / 8

	currScanline := 0
	// todo: we also need to break/continue on:
	//      Scanlines always begin on byte boundaries.  When pixels have fewer
	//      than 8 bits and the scanline width is not evenly divisible by the
	//      number of pixels per byte, the low-order bits in the last byte of
	//      each scanline are wasted.  The contents of these wasted bits are
	//      unspecified.

	previousScanlineData := make([]byte, scanlineByteSize)
	for ; currScanline < header.Height && len(idatData) >= scanlineByteSize+1; currScanline++ {
		// also strips filter type byte
		data, err := unfilterScanline(
			previousScanlineData,
			idatData[:scanlineByteSize+1],
			scanlineByteSize,
			pixelByteSize,
		)
		if err != nil {
			return nil, err
		}
		previousScanlineData = data
		idatData = idatData[scanlineByteSize+1:]

		pixels[currScanline] = make([]Pixel, header.Width)

		for x := 0; x < header.Width; x++ {
			var smallPixelData uint
			if pixelBitSize < 8 {
				currDataByteIndex := (x * pixelBitSize) / 8
				smallPixelData = uint(data[currDataByteIndex])
			}

			var pixel Pixel
			switch header.ColorType {
			case ColorTypeGrayscale:
				var value uint
				switch pixelByteSize {
				case 0:
					// in pixels smaller than a byte, this is used to place the mask correctly within the byte to get the correct pixel
					// so if pixelBitSize == 2, we want a mask of 0b11000000, for the first pixel in the byte, 0b00110000 for the second, etc..
					pixelsPerByte := 8 / pixelBitSize
					// pixelsperbyte == 8 && x == 0, shiftMaskBy = 7 0b1000 0000
					// pixelsperbyte == 8 && x == 1, shiftMaskBy = 6 0b0100 0000
					// pixelsperbyte == 4 && x == 0, shiftMaskBy = 6 0b1100 0000
					// pixelsperbyte == 4 && x == 2, shiftMaskBy = 2 0b0000 1100
					// pixelsperbyte == 4 && x == 3, shiftMaskBy = 0 0b0000 0011
					// pixelsperbyte == 2 && x == 0, shiftMaskBy = 4 0b1111 0000
					// pixelsperbyte == 2 && x == 1, shiftMaskBy = 0 0b1111 0000
					shiftMaskBy := (8 / pixelsPerByte) * (pixelsPerByte - 1 - x)
					mask := uint((1<<pixelBitSize)-1) << shiftMaskBy
					value = smallPixelData & uint(mask)
				case 1:
					value = uint(data[0])
				case 2:
					value = uint(binary.BigEndian.Uint16(data))
				}
				pixel = &GreyscalePixel{
					Value: value,
				}
			case ColorTypeTruecolor:
				switch header.BitDepth {
				case 8:
					pixel = &TruecolorPixel{
						Red:   uint(data[0]),
						Green: uint(data[1]),
						Blue:  uint(data[2]),
						Alpha: math.MaxUint,
					}
				case 16:
					pixel = &TruecolorPixel{
						Red:   uint(binary.BigEndian.Uint16(data)),
						Green: uint(binary.BigEndian.Uint16(data[2:])),
						Blue:  uint(binary.BigEndian.Uint16(data[4:])),
						Alpha: math.MaxUint,
					}
				}
			case ColorTypePalette:
				// todo: implement
				panic("implement")
			case ColorTypeGrayscaleAlpha:
				pixelNumChannels = 2
			case ColorTypeTruecolorAlpha:
				switch header.BitDepth {
				case 8:
					pixel = &TruecolorPixel{
						Red:   uint(data[0]),
						Green: uint(data[1]),
						Blue:  uint(data[2]),
						Alpha: uint(data[3]),
					}
				case 16:
					pixel = &TruecolorPixel{
						Red:   uint(binary.BigEndian.Uint16(data)),
						Green: uint(binary.BigEndian.Uint16(data[2:])),
						Blue:  uint(binary.BigEndian.Uint16(data[4:])),
						Alpha: uint(binary.BigEndian.Uint16(data[6:])),
					}
				}
			}

			if pixelBitSize < 8 {
				pixelsPerByte := 8 / pixelBitSize
				if x%pixelsPerByte == pixelsPerByte-1 {
					data = data[1:]
				}
			} else {
				data = data[pixelByteSize:]
			}
			pixels[currScanline][x] = pixel
		}
	}

	if currScanline < header.Height {
		return nil, fmt.Errorf("couldn't parse IDAT idatData, not enough idatData to read all scanlines, expected: %d scanlines, got: %d", header.Height, currScanline)
	}
	if len(idatData) != 0 {
		return nil, fmt.Errorf("there was idatData left in the IDAT chunks after reading all scanlines, remaining idatData: %v", idatData)
	}

	return pixels, nil
}

func unfilterScanline(
	previousScanlineData []byte,
	data []byte,
	scanlineByteSize int,
	pixelByteSize int,
) ([]byte, error) {
	if pixelByteSize == 0 {
		pixelByteSize = 1
	}

	filterType := FilterType(data[0])
	data = data[1:]
	result := make([]byte, scanlineByteSize)

	switch filterType {
	case FilterTypeNone:
		return data, nil
	case FilterTypeSub:
		for i := 0; i < scanlineByteSize; i++ {
			subX := uint(data[i])
			rawXminusBpp := uint(0)
			if i-pixelByteSize >= 0 {
				rawXminusBpp = uint(result[i-pixelByteSize])
			}
			rawX := (subX + rawXminusBpp) % 256
			result[i] = byte(rawX)
		}
	case FilterTypeUp:
		for i := 0; i < scanlineByteSize; i++ {
			upX := uint(data[i])
			priorX := uint(previousScanlineData[i])
			rawX := (upX + priorX) % 256
			result[i] = byte(rawX)
		}
	case FilterTypeAverage:
		for i := 0; i < scanlineByteSize; i++ {
			//Average(x) + floor((Raw(x-bpp)+Prior(x))/2)
			avgX := uint(data[i])
			rawXminusBpp := uint(0)
			if i-pixelByteSize >= 0 {
				rawXminusBpp = uint(result[i-pixelByteSize])
			}
			priorX := uint(previousScanlineData[i])
			rawX := (avgX + uint(math.Floor(float64(rawXminusBpp+priorX)/2))) % 256
			result[i] = byte(rawX)
		}
	case FilterTypePaeth:
		for i := 0; i < scanlineByteSize; i++ {
			paethX := uint(data[i])
			rawXminusBpp := uint(0)
			priorX := previousScanlineData[i]
			priorXminusBpp := uint(0)
			if i-pixelByteSize >= 0 {
				rawXminusBpp = uint(result[i-pixelByteSize])
				priorXminusBpp = uint(previousScanlineData[i-pixelByteSize])
			}

			predictor := PaethPredictor(int(rawXminusBpp), int(priorX), int(priorXminusBpp))
			rawX := (paethX + uint(predictor)) % 256

			result[i] = byte(rawX)
		}
	default:
		return nil, fmt.Errorf("unknown filter type %v", filterType)
	}

	return result, nil
}

func PaethPredictor(a int, b int, c int) int {
	// a = left, b = above, c = upper left
	p := a + b - c      // initial estimate
	pa := AbsInt(p - a) // distances to a, b, c
	pb := AbsInt(p - b)
	pc := AbsInt(p - c)
	// return nearest of a,b,c,
	// breaking ties in order a,b,c.
	if pa <= pb && pa <= pc {
		return a
	} else if pb <= pc {
		return b
	} else {
		return c
	}
}

func AbsInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

type PalettePixel struct {
	index uint
}

func (p *PalettePixel) pixel() {}

type GreyscalePixel struct {
	Value uint
}

func (p *GreyscalePixel) pixel() {}

type TruecolorPixel struct {
	Red   uint
	Green uint
	Blue  uint
	Alpha uint
}

func (p *TruecolorPixel) pixel() {}

type IHDRData struct {
	Width           int
	Height          int
	BitDepth        uint8
	ColorType       ColorType
	InterlaceMethod InterlaceMethod
}

func decodeIHDRChunk(data []byte) (IHDRData, error) {
	res := IHDRData{}
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
