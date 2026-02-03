package pngo

import "testing"

var PNG_SIGN = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
var REAL_PNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x05,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x8d, 0x6f, 0x26,
	0xe5, 0x00, 0x00, 0x00, 0x01, 0x73, 0x52, 0x47,
	0x42, 0x00, 0xae, 0xce, 0x1c, 0xe9, 0x00, 0x00,
	0x00, 0x2a, 0x49, 0x44, 0x41, 0x54, 0x18, 0x57,
	0x63, 0x64, 0x60, 0x60, 0xf8, 0xcf, 0x80, 0x06,
	0x18, 0xff, 0xff, 0xff, 0x8f, 0x22, 0xc8, 0xc8,
	0xc8, 0xc8, 0xc0, 0x08, 0x52, 0x09, 0x12, 0x07,
	0x71, 0x60, 0x00, 0x2e, 0x88, 0x6c, 0x02, 0x58,
	0x10, 0xdd, 0x4c, 0x00, 0x34, 0x02, 0x0d, 0xfe,
	0xa4, 0x8d, 0x71, 0xf6, 0x00, 0x00, 0x00, 0x00,
	0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

var REAL_IHDR_CHUNK = []byte{
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x05,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x8d, 0x6f, 0x26,
	0xe5,
}

func TestFirstBytesAreFileSign(t *testing.T) {
	data := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	_, err := DecodePng(data)
	if err == nil {
		t.Fatalf("Expected file sign error, but got no error")
	}
}

func TestLastChunkIsNotIEND(t *testing.T) {
	data := make([]byte, 0)
	data = append(data, PNG_SIGN...)
	data = append(data, REAL_IHDR_CHUNK...)

	_, err := DecodePng(data)
	if err == nil {
		t.Fatal("Expected error due to missing IEND chunk, but got no error")
	}
}

func TestFirstChunkIsIHDR(t *testing.T) {
	data := []byte{
		0x00, 0x00, 0x00, 0x00, // Length of the chunk data
		0x00, 0x00, 0x00, 0x00, // Chunk type IHDR
		0x00, 0x00, 0x00, 0x00, // CRC (dummy Value)
	}

	_, err := DecodePng(data)
	if err == nil {
		t.Fatal("Expected error that first chunk should be IHDR, but got no error")
	}
}

func TestDecodeChunkWithRealIHDR(t *testing.T) {
	chunk, read, err := readChunk(REAL_IHDR_CHUNK)
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
	if chunk.chunkType != ihdr {
		t.Fatalf("Expected chunk type ihdr, but got: %s", chunk.chunkType)
	}
	if read != len(REAL_IHDR_CHUNK) {
		t.Fatalf("Expected read count to be %d, but got: %d", len(REAL_IHDR_CHUNK), read)
	}
}

func TestDecodeRealPNG(t *testing.T) {
	png, err := DecodePng(REAL_PNG)
	if err != nil {
		t.Fatalf("Expected no error decoding real PNG, but got: %v", err)
	}
	if png.Width != 5 || png.Height != 5 {
		t.Fatalf("Expected 5x5 image, got %dx%d", png.Width, png.Height)
	}
	if png.ColorType != ColorTypeTruecolorAlpha {
		t.Fatalf("Expected ColorTypeTruecolorAlpha, got %v", png.ColorType)
	}
	if png.BitDepth != 8 {
		t.Fatalf("Expected bit depth 8, got %d", png.BitDepth)
	}
}

func TestDecodeIHDRChunk(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantWidth  int
		wantHeight int
		wantDepth  uint8
		wantColor  ColorType
		wantErr    bool
	}{
		{
			name: "valid grayscale 8-bit",
			data: []byte{
				0x00, 0x00, 0x00, 0x0A, // width: 10
				0x00, 0x00, 0x00, 0x14, // height: 20
				0x08,             // bit depth: 8
				0x00,             // color type: grayscale
				0x00, 0x00, 0x00, // compression, filter, interlace
			},
			wantWidth:  10,
			wantHeight: 20,
			wantDepth:  8,
			wantColor:  ColorTypeGrayscale,
			wantErr:    false,
		},
		{
			name: "valid truecolor 16-bit",
			data: []byte{
				0x00, 0x00, 0x00, 0x64, // width: 100
				0x00, 0x00, 0x00, 0x64, // height: 100
				0x10, // bit depth: 16
				0x02, // color type: truecolor
				0x00, 0x00, 0x00,
			},
			wantWidth:  100,
			wantHeight: 100,
			wantDepth:  16,
			wantColor:  ColorTypeTruecolor,
			wantErr:    false,
		},
		{
			name: "invalid bit depth for grayscale",
			data: []byte{
				0x00, 0x00, 0x00, 0x0A,
				0x00, 0x00, 0x00, 0x0A,
				0x0C, // bit depth: 12 (invalid)
				0x00, // color type: grayscale
				0x00, 0x00, 0x00,
			},
			wantErr: true,
		},
		{
			name:    "invalid chunk length",
			data:    []byte{0x00, 0x00, 0x00}, // too short
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodeIHDRChunk(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeIHDRChunk() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if result.Width != tt.wantWidth {
					t.Errorf("Width = %d, want %d", result.Width, tt.wantWidth)
				}
				if result.Height != tt.wantHeight {
					t.Errorf("Height = %d, want %d", result.Height, tt.wantHeight)
				}
				if result.BitDepth != tt.wantDepth {
					t.Errorf("BitDepth = %d, want %d", result.BitDepth, tt.wantDepth)
				}
				if result.ColorType != tt.wantColor {
					t.Errorf("ColorType = %v, want %v", result.ColorType, tt.wantColor)
				}
			}
		})
	}
}

func TestPaethPredictor(t *testing.T) {
	tests := []struct {
		name    string
		a, b, c int
		want    int
	}{
		// a=left, b=above, c=upper-left
		// p = a + b - c, then return whichever of {a,b,c} is closest to p

		{"all zeros", 0, 0, 0, 0},
		// p=0, pa=0, pb=0, pc=0 → a (tie-breaker)

		{"left closest", 10, 5, 3, 10},
		// p=10+5-3=12, pa=|12-10|=2, pb=|12-5|=7, pc=|12-3|=9 → a

		{"above closest", 5, 10, 3, 10},
		// p=5+10-3=12, pa=|12-5|=7, pb=|12-10|=2, pc=|12-3|=9 → b

		{"upper left closest", 100, 200, 149, 149},
		// p=100+200-149=151, pa=|151-100|=51, pb=|151-200|=49, pc=|151-149|=2 → c

		{"tie a and b, a wins", 10, 10, 5, 10},
		// p=10+10-5=15, pa=|15-10|=5, pb=|15-10|=5, pc=|15-5|=10 → a (tie-breaker)

		{"tie b and c, b wins", 40, 10, 30, 10},
		// p=40+10-30=20, pa=|20-40|=20, pb=|20-10|=10, pc=|20-30|=10 → b (tie-breaker)

		{"tie all three, a wins", 5, 5, 5, 5},
		// p=5+5-5=5, pa=0, pb=0, pc=0 → a (tie-breaker)

		{"upper left is exact match", 10, 20, 15, 15},
		// p=10+20-15=15, pa=|15-10|=5, pb=|15-20|=5, pc=|15-15|=0 → c
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := paethPredictor(tt.a, tt.b, tt.c)
			if got != tt.want {
				// Calculate and show the distances for debugging
				p := tt.a + tt.b - tt.c
				pa := absInt(p - tt.a)
				pb := absInt(p - tt.b)
				pc := absInt(p - tt.c)
				t.Errorf("paethPredictor(%d, %d, %d) = %d, want %d (p=%d, pa=%d, pb=%d, pc=%d)",
					tt.a, tt.b, tt.c, got, tt.want, p, pa, pb, pc)
			}
		})
	}
}

func TestAbsInt(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 0},
		{5, 5},
		{-5, 5},
		{-100, 100},
		{100, 100},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := absInt(tt.input)
			if got != tt.want {
				t.Errorf("absInt(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b int
		want int
	}{
		{5, 10, 5},
		{10, 5, 5},
		{-5, 5, -5},
		{0, 0, 0},
		{-10, -20, -20},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := minInt(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("minInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestReadChunkInvalidChecksum(t *testing.T) {
	data := []byte{
		0x00, 0x00, 0x00, 0x00, // Length: 0
		0x49, 0x48, 0x44, 0x52, // Type: IHDR
		0xFF, 0xFF, 0xFF, 0xFF, // Invalid CRC
	}

	_, _, err := readChunk(data)
	if err == nil {
		t.Fatal("Expected checksum error, but got no error")
	}
}

func TestReadChunkTooShort(t *testing.T) {
	data := []byte{0x00, 0x00} // Only 2 bytes

	_, _, err := readChunk(data)
	if err == nil {
		t.Fatal("Expected error for too short chunk, but got no error")
	}
}

func TestDecodePLTEChunk(t *testing.T) {
	t.Run("valid palette", func(t *testing.T) {
		header := ihdrData{BitDepth: 8}
		data := []byte{
			0xFF, 0x00, 0x00, // Red
			0x00, 0xFF, 0x00, // Green
			0x00, 0x00, 0xFF, // Blue
		}

		result, err := decodePLTEChunk(header, data)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if len(result.Entries) != 3 {
			t.Fatalf("Expected 3 entries, got %d", len(result.Entries))
		}
		if result.Entries[0].Red != 0xFF || result.Entries[0].Green != 0 {
			t.Errorf("First entry incorrect: %+v", result.Entries[0])
		}
	})

	t.Run("invalid length not divisible by 3", func(t *testing.T) {
		header := ihdrData{BitDepth: 8}
		data := []byte{0xFF, 0x00} // Only 2 bytes

		_, err := decodePLTEChunk(header, data)
		if err == nil {
			t.Fatal("Expected error for invalid palette length")
		}
	})

	t.Run("too many entries for bit depth", func(t *testing.T) {
		header := ihdrData{BitDepth: 1} // Max 2 entries
		data := make([]byte, 3*10)      // 10 entries

		_, err := decodePLTEChunk(header, data)
		if err == nil {
			t.Fatal("Expected error for too many palette entries")
		}
	})
}

func TestExtractChunks(t *testing.T) {
	t.Run("valid PNG with chunks", func(t *testing.T) {
		chunks, err := extractChunks(REAL_PNG)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if len(chunks) < 2 {
			t.Fatalf("Expected at least 2 chunks, got %d", len(chunks))
		}
		if chunks[0].chunkType != ihdr {
			t.Errorf("First chunk should be ihdr, got %s", chunks[0].chunkType)
		}
		if chunks[len(chunks)-1].chunkType != iend {
			t.Errorf("Last chunk should be iend, got %s", chunks[len(chunks)-1].chunkType)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		invalidData := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		_, err := extractChunks(invalidData)
		if err == nil {
			t.Fatal("Expected error for invalid PNG signature")
		}
	})
}
