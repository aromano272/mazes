package main

import (
	"fmt"
	"github.com/veandco/go-sdl2/sdl"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	mazesPng "mazes/png"
	"os"
	"unsafe"
)

func main() {
	//png.Png()
	//a := []int{137, 80, 78, 71, 13, 10, 26, 10}
	//hexArray := []string{}
	//for _, v := range a {
	//	hexArray = append(hexArray, fmt.Sprintf("%x", v))
	//}
	//fmt.Println(hexArray)
	//asciiString := ""
	//for _, hex := range a {
	//	asciiString += string(hex)
	//}
	//fmt.Println(asciiString)

	//filePath := "mazediag10001x10001.png"
	//filePath := "mazediag21x21.png"
	//filePath := "palette.png"
	filePath := "pngtest.png"
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("failed to read file: %v", err)
	}

	res, err := mazesPng.DecodePng(data)
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
			bckg := color.RGBA{
				R: 0x00,
				G: 0x00,
				B: 0x00,
				A: 0xFF,
			}
			img.Set(x, y, bckg)
			if y == 20 && x == 20 {
				fmt.Printf("ldsa")
			}
			if y == 95 && x == 175 {
				fmt.Printf("ldsa")
			}
			pixel := res.Pixels[y][x]
			if res.BitDepth > 8 {
				convert16BitDepthPixelTo8Bit(pixel)
			}

			switch p := pixel.(type) {
			case *mazesPng.TruecolorPixel:
				c := color.RGBA{
					R: uint8(p.Red),
					G: uint8(p.Green),
					B: uint8(p.Blue),
					A: uint8(p.Alpha),
				}
				surface.Set(x, y, c)
				img.Set(x, y, c)
			case *mazesPng.GreyscalePixel:
				c := color.RGBA{
					R: uint8(p.Value),
					G: uint8(p.Value),
					B: uint8(p.Value),
					A: uint8(0xFF),
				}
				surface.Set(x, y, c)
			case *mazesPng.PalettePixel:
				truePixel := res.PlteEntries[p.Index]
				c := color.RGBA{
					R: uint8(truePixel.Red),
					G: uint8(truePixel.Green),
					B: uint8(truePixel.Blue),
					A: uint8(truePixel.Alpha),
				}
				surface.Set(x, y, c)
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
	png.Encode(file, bg)

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

	/*

		maze := make([][]bool, len(res.Pixels))

		for y, scanline := range res.Pixels {
			maze[y] = make([]bool, len(scanline))

			for x, pixel := range scanline {
				// Process each pixel here

				switch p := pixel.(type) {
				case *mazesPng.TruecolorPixel:
					maze[y][x] = p.Blue > 0
					img.Set(x, y, color.RGBA{R: uint8(p.Red), G: uint8(p.Green), B: uint8(p.Blue), A: uint8(p.Alpha)})
				case *mazesPng.GreyscalePixel:
					maze[y][x] = p.Value > 0
					img.Set(x, y, color.Gray{Y: uint8(p.Value)})
				case *mazesPng.PalettePixel:
					pltePixel := res.PlteEntries[p.Index]
					img.Set(x, y, color.RGBA{R: uint8(pltePixel.Red), G: uint8(pltePixel.Green), B: uint8(pltePixel.Blue), A: uint8(pltePixel.Alpha)})
				}
				if p, ok := pixel.(*mazesPng.TruecolorPixel); ok {
					maze[y][x] = p.Blue > 0
					img.Set(x, y, color.RGBA{R: uint8(p.Red), G: uint8(p.Green), B: uint8(p.Blue), A: uint8(p.Alpha)})

					if p.Red == 0 {
						//fmt.Print("0")
					} else {
						//fmt.Print("1")
					}
				}
			}
		}

		//for y := range maze {
		//	for _, val := range maze[y] {
		//		if val {
		//			fmt.Print("1")
		//		} else {
		//			fmt.Print("0")
		//		}
		//	}
		//	fmt.Println("")
		//}

		//endY := len(maze) - 1
		//endX := len(maze[endY]) - 1

		start := time.Now()
		//solution := path.DijkstraOrAStar(true, maze, 0, 1, endX, endY-1)
		solution := make([]path.Node2, 0)
		elapsed := time.Since(start)

		log.Printf("Solution took %s", elapsed)

		for _, node := range solution {
			img.Set(node.X, node.Y, color.RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF})
		}

	*/
}

func convert16BitDepthPixelTo8Bit(pixel mazesPng.Pixel) mazesPng.Pixel {
	switch p := pixel.(type) {
	case *mazesPng.TruecolorPixel:
		p.Red = p.Red >> 8
		p.Green = p.Green >> 8
		p.Blue = p.Blue >> 8
		p.Alpha = p.Alpha >> 8
	case *mazesPng.GreyscalePixel:
		p.Value = p.Value >> 8
	}

	return pixel
}
