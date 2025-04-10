package png

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"math"
	"runtime"
	"slices"
)

const FILE_SIGN = 0x89504E470D0A1A0A

type Chunk struct {
	chunkType ChunkType
	data      []byte
}

type ChunkType string

type Pixel interface {
	pixel()
}

type PalettePixel struct {
	Index uint
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

type PLTEData struct {
	Entries []TruecolorPixel
}

type Png struct {
	Width           int
	Height          int
	ColorType       ColorType
	BitDepth        uint8
	InterlaceMethod InterlaceMethod

	Pixels      [][]Pixel
	PlteEntries []TruecolorPixel
}

const (
	IHDR ChunkType = "IHDR"
	IEND ChunkType = "IEND"
	PLTE ChunkType = "PLTE"
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

type Scanline struct {
	index      int
	unfData    []byte
	data       []byte
	filterType FilterType
}

func (sl Scanline) unfilteringRequiresPrevScanline() bool {
	return sl.filterType >= 2
}

const (
	FilterTypeNone FilterType = iota
	FilterTypeSub
	FilterTypeUp
	FilterTypeAverage
	FilterTypePaeth
)

func DecodePng(data []byte) (*Png, error) {
	read, err := readSig(data)
	if err != nil {
		return nil, err
	}
	data = data[read:]

	chunks := make([]*Chunk, 0)

	for len(data) > 0 {
		chunk, read, err := readChunk(data)
		if err != nil {
			return nil, err
		}
		data = data[read:]
		chunks = append(chunks, chunk)
	}

	png := &Png{}

	var ihdrData IHDRData
	var plteData PLTEData
	idatData := make([]byte, 0)
	for index, chunk := range chunks {
		if index == 0 && chunk.chunkType != IHDR {
			return nil, fmt.Errorf("first chunk should be IHDR, found: %s", chunk.chunkType)
		}
		if index == len(chunks)-1 {
			if chunk.chunkType != IEND {
				return nil, fmt.Errorf("last chunk should be IEND, found: %s", chunk.chunkType)
			}
			if len(idatData) == 0 {
				return nil, fmt.Errorf("there should be atleast one IDAT chunk")
			}
			if ihdrData.ColorType == ColorTypePalette && len(plteData.Entries) == 0 {
				return nil, fmt.Errorf("palette color type missing PLTR chunk")
			}

			uncompressedIdatData := uncompressIDATData(idatData)
			pixels, err := processIDATData(uncompressedIdatData, ihdrData)
			if err != nil {
				return nil, err
			}

			png.Pixels = pixels
			png.PlteEntries = plteData.Entries
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
			ihdrData = res
		case PLTE:
			res, err := decodePLTEChunk(ihdrData, chunk.data)
			if err != nil {
				return nil, err
			}
			plteData = res
		case IDAT:
			idatData = append(idatData, chunk.data...)
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

func (h IHDRData) pixelNumChannels() int {
	pixelNumChannels := 0
	switch h.ColorType {
	case ColorTypeGrayscale:
		pixelNumChannels = 1
	case ColorTypeTruecolor:
		pixelNumChannels = 3
	case ColorTypePalette:
		pixelNumChannels = 1
	case ColorTypeGrayscaleAlpha:
		pixelNumChannels = 2
	case ColorTypeTruecolorAlpha:
		pixelNumChannels = 4
	}
	return pixelNumChannels
}
func (h IHDRData) pixelBitSize() int {
	return h.pixelNumChannels() * int(h.BitDepth)
}
func (h IHDRData) pixelByteSize() int {
	return h.pixelBitSize() / 8
}
func (h IHDRData) scanlineBitSize() int {
	return h.pixelBitSize() * h.Width
}
func (h IHDRData) scanlineBitPadding() int {
	return h.scanlineBitSize() % 8
}
func (h IHDRData) scanlineByteSize() int {
	return (h.scanlineBitSize() + h.scanlineBitPadding()) / 8
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
		pixelNumChannels = 1
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

	// todo: we also need to break/continue on:
	//      Scanlines always begin on byte boundaries.  When pixels have fewer
	//      than 8 bits and the scanline width is not evenly divisible by the
	//      number of pixels per byte, the low-order bits in the last byte of
	//      each scanline are wasted.  The contents of these wasted bits are
	//      unspecified.

	scanlineCount := header.Height
	minScanlinesPerWorker := 100
	optimalNumWorkers := (scanlineCount / minScanlinesPerWorker) + 1
	numWorkers := minInt(runtime.NumCPU(), optimalNumWorkers)
	scanlinesPerWorker := scanlineCount / numWorkers

	scanlines := make([]*Scanline, 0)
	for i := 0; i < scanlineCount; i++ {
		slLen := scanlineByteSize + 1
		start := slLen * i
		end := start + slLen
		if end > len(idatData) {
			return nil, fmt.Errorf("couldn't parse IDAT idatData, not enough idatData to read all scanlines, expected: %d scanlines, got: %d", header.Height, i)
		}
		if i == scanlineCount-1 && len(idatData[end:]) > 0 {
			return nil, fmt.Errorf("there was idatData left in the IDAT chunks after reading all scanlines, remaining idatData: %v", idatData)
		}
		unpData := idatData[start:end]
		filterType := FilterType(unpData[0])
		scanline := &Scanline{
			index:      i,
			unfData:    unpData[1:],
			filterType: filterType,
		}
		scanlines = append(scanlines, scanline)
	}

	results := make(chan Result, numWorkers)

	for i := 0; i < numWorkers; i++ {
		start := scanlinesPerWorker * i
		end := start + scanlinesPerWorker
		if i == numWorkers-1 {
			end = scanlineCount
		}
		work := scanlines[start:end]
		go worker(
			header,
			work,
			scanlineByteSize,
			pixelBitSize,
			pixelByteSize,
			results,
		)
	}

	unprocessedScanlinesIndices := make([]int, 0)

	for i := 0; i < numWorkers; i++ {
		result := <-results
		if result.err != nil {
			return nil, result.err
		}
		unprocessedScanlinesIndices = append(unprocessedScanlinesIndices, result.unprocessedIndices...)
		for _, sl := range result.scanlines {
			pixels[sl.index] = sl.pixels
		}

		if i == numWorkers-1 {
			close(results)
		}
	}

	slices.Sort(unprocessedScanlinesIndices)

	for _, index := range unprocessedScanlinesIndices {
		sl := scanlines[index]

		res, err := processScanline(
			header,
			scanlines[index-1],
			sl,
			scanlineByteSize,
			pixelBitSize,
			pixelByteSize,
		)

		if err != nil {
			return nil, err
		}

		pixels[index] = res
	}

	return pixels, nil
}

type ScanlineResult struct {
	index  int
	pixels []Pixel
}

type Result struct {
	unprocessedIndices []int
	scanlines          []ScanlineResult
	err                error
}

func worker(
	header IHDRData,
	scanlines []*Scanline,
	scanlineByteSize int,
	pixelBitSize int,
	pixelByteSize int,
	results chan<- Result,
) {
	result := Result{}
	for i, sl := range scanlines {
		var prevScanline *Scanline
		if sl.unfilteringRequiresPrevScanline() {
			if sl.index == 0 {
				prevScanline = &Scanline{data: make([]byte, scanlineByteSize)}
			} else if i == 0 || len(scanlines[i-1].data) == 0 {
				result.unprocessedIndices = append(result.unprocessedIndices, sl.index)
				continue
			} else {
				prevScanline = scanlines[i-1]
			}
		}

		pixels, err := processScanline(
			header,
			prevScanline,
			sl,
			scanlineByteSize,
			pixelBitSize,
			pixelByteSize,
		)

		if err != nil {
			result.err = err
			results <- result
			return
		}

		scanlineResult := ScanlineResult{index: sl.index, pixels: pixels}
		result.scanlines = append(result.scanlines, scanlineResult)
	}
	results <- result
}

func processScanline(
	header IHDRData,
	prevScanline *Scanline,
	scanline *Scanline,
	scanlineByteSize int,
	pixelBitSize int,
	pixelByteSize int,
) ([]Pixel, error) {
	// also strips filter type byte
	err := unfilterScanline(
		prevScanline,
		scanline,
		scanlineByteSize,
		pixelByteSize,
	)
	if err != nil {
		return nil, err
	}
	data := scanline.data

	pixels := make([]Pixel, header.Width)
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
			var value uint
			if pixelBitSize < 8 {
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
			} else {
				value = uint(data[0])
			}
			pixel = &PalettePixel{
				Index: value,
			}
		case ColorTypeGrayscaleAlpha:
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
		pixels[x] = pixel
	}

	return pixels, nil
}

func unfilterScanline(
	previousScanline *Scanline,
	scanline *Scanline,
	scanlineByteSize int,
	pixelByteSize int,
) error {
	if pixelByteSize == 0 {
		pixelByteSize = 1
	}

	result := make([]byte, scanlineByteSize)

	switch scanline.filterType {
	case FilterTypeNone:
		scanline.data = scanline.unfData
	case FilterTypeSub:
		for i := 0; i < scanlineByteSize; i++ {
			subX := uint(scanline.unfData[i])
			rawXminusBpp := uint(0)
			if i-pixelByteSize >= 0 {
				rawXminusBpp = uint(result[i-pixelByteSize])
			}
			rawX := (subX + rawXminusBpp) % 256
			result[i] = byte(rawX)
		}
	case FilterTypeUp:
		if len(previousScanline.data) == 0 {
			return fmt.Errorf("couldn't unfilter scanline, prev scanline not unfiltered yet, curr scanline: %d", scanline.index)
		}
		for i := 0; i < scanlineByteSize; i++ {
			upX := uint(scanline.unfData[i])
			priorX := uint(previousScanline.data[i])
			rawX := (upX + priorX) % 256
			result[i] = byte(rawX)
		}
	case FilterTypeAverage:
		if len(previousScanline.data) == 0 {
			return fmt.Errorf("couldn't unfilter scanline, prev scanline not unfiltered yet, curr scanline: %d", scanline.index)
		}
		for i := 0; i < scanlineByteSize; i++ {
			//Average(x) + floor((Raw(x-bpp)+Prior(x))/2)
			avgX := uint(scanline.unfData[i])
			rawXminusBpp := uint(0)
			if i-pixelByteSize >= 0 {
				rawXminusBpp = uint(result[i-pixelByteSize])
			}
			priorX := uint(previousScanline.data[i])
			rawX := (avgX + uint(math.Floor(float64(rawXminusBpp+priorX)/2))) % 256
			result[i] = byte(rawX)
		}
	case FilterTypePaeth:
		if len(previousScanline.data) == 0 {
			return fmt.Errorf("couldn't unfilter scanline, prev scanline not unfiltered yet, curr scanline: %d", scanline.index)
		}
		for i := 0; i < scanlineByteSize; i++ {
			paethX := uint(scanline.unfData[i])
			rawXminusBpp := uint(0)
			priorX := previousScanline.data[i]
			priorXminusBpp := uint(0)
			if i-pixelByteSize >= 0 {
				rawXminusBpp = uint(result[i-pixelByteSize])
				priorXminusBpp = uint(previousScanline.data[i-pixelByteSize])
			}

			predictor := paethPredictor(int(rawXminusBpp), int(priorX), int(priorXminusBpp))
			rawX := (paethX + uint(predictor)) % 256

			result[i] = byte(rawX)
		}
	default:
		return fmt.Errorf("unknown filter type %v", scanline.filterType)
	}

	scanline.data = result

	return nil
}

func paethPredictor(a int, b int, c int) int {
	// a = left, b = above, c = upper left
	p := a + b - c      // initial estimate
	pa := absInt(p - a) // distances to a, b, c
	pb := absInt(p - b)
	pc := absInt(p - c)
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

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func decodePLTEChunk(ihdrData IHDRData, data []byte) (PLTEData, error) {
	res := PLTEData{}

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

func readSig(data []byte) (int, error) {
	sig := data[:8]

	if binary.BigEndian.Uint64(sig) != FILE_SIGN {
		return -1, fmt.Errorf("invalid PNG signature")
	}

	return 8, nil
}

func readChunk(data []byte) (*Chunk, int, error) {
	newData := data
	length := binary.BigEndian.Uint32(newData[:4])
	newData = newData[4:]

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
