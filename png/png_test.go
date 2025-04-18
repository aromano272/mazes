package png

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
	if chunk.chunkType != IHDR {
		t.Fatalf("Expected chunk type IHDR, but got: %s", chunk.chunkType)
	}
	if read != len(REAL_IHDR_CHUNK) {
		t.Fatalf("Expected read count to be %d, but got: %d", len(REAL_IHDR_CHUNK), read)
	}
}
