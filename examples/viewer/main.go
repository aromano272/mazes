package main

import (
	"fmt"
	"github.com/veandco/go-sdl2/sdl"
	"image"
	"image/color"
	"image/draw"
	imagePng "image/png"
	"log"
	"os"
	"pngo/pngo"
	"unsafe"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: viewer <path-to-png-file>")
	}

	filePath := os.Args[1]
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("failed to read file %s: %v", filePath, err)
	}

	res, err := pngo.DecodePng(data)
	if err != nil {
		log.Fatal(err)
	}
	width := res.Width
	height := res.Height

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		log.Fatal(err)
	}
	defer sdl.Quit()

	// Create a window
	window, err := sdl.CreateWindow("Window", sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED, int32(width), int32(height), sdl.WINDOW_SHOWN)
	if err != nil {
		log.Fatal(err)
	}
	defer window.Destroy()

	surface, err := window.GetSurface()
	if err != nil {
		log.Fatal(err)
	}

	surface.FillRect(nil, sdl.MapRGB(surface.Format, 0, 0, 0)) // Black background

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixel := res.Pixels[y][x]
			if res.BitDepth > 8 {
				convert16BitDepthPixelTo8Bit(pixel)
			}

			switch p := pixel.(type) {
			case *pngo.TruecolorPixel:
				c := color.RGBA{
					R: uint8(p.Red),
					G: uint8(p.Green),
					B: uint8(p.Blue),
					A: uint8(p.Alpha),
				}
				img.Set(x, y, c)
			case *pngo.GreyscalePixel:
				c := color.RGBA{
					R: uint8(p.Value),
					G: uint8(p.Value),
					B: uint8(p.Value),
					A: uint8(0xFF),
				}
				img.Set(x, y, c)
			case *pngo.PalettePixel:
				truePixel := res.PlteEntries[p.Index]
				c := color.RGBA{
					R: uint8(truePixel.Red),
					G: uint8(truePixel.Green),
					B: uint8(truePixel.Blue),
					A: uint8(truePixel.Alpha),
				}
				img.Set(x, y, c)
			}
		}
	}

	// Create SDL Surface
	overlaySurface, err := sdl.CreateRGBSurfaceWithFormatFrom(
		unsafe.Pointer(&img.Pix[0]), // Convert to pointer
		int32(img.Rect.Dx()),
		int32(img.Rect.Dy()),
		32, // 32-bit color depth
		int32(img.Stride),
		sdl.PIXELFORMAT_ABGR8888, // Format that supports transparency
	)
	if err != nil {
		panic(err)
	}

	overlayRect := sdl.Rect{X: 0, Y: 0, W: overlaySurface.W, H: overlaySurface.H}
	// Blit (blend) the overlay image onto the background surface
	err = overlaySurface.Blit(nil, surface, &overlayRect)
	if err != nil {
		fmt.Println("Failed to blit overlay image:", err)
		return
	}
	bg := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill the background with a solid color (black)
	draw.Draw(bg, bg.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)
	draw.Draw(bg, img.Bounds(), img, image.Point{}, draw.Over)

	// Enable per-pixel alpha blending
	overlaySurface.SetBlendMode(sdl.BLENDMODE_BLEND)

	window.UpdateSurface()

	file, _ := os.Create("output.png")
	defer file.Close()
	imagePng.Encode(file, bg)

	// Handle events and keep window open
	for {
		// Poll events (close window)
		event := sdl.WaitEvent()
		switch event.(type) {
		case *sdl.QuitEvent:
			// Close the window when the user clicks the close button
			return
		}
	}
}

func convert16BitDepthPixelTo8Bit(pixel pngo.Pixel) pngo.Pixel {
	switch p := pixel.(type) {
	case *pngo.TruecolorPixel:
		p.Red = p.Red >> 8
		p.Green = p.Green >> 8
		p.Blue = p.Blue >> 8
		p.Alpha = p.Alpha >> 8
	case *pngo.GreyscalePixel:
		p.Value = p.Value >> 8
	}

	return pixel
}
