package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"mazes/path"
	mazesPng "mazes/png"
	"os"
	"time"
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

	filePath := "mazediag10001x10001.png"
	//filePath := "mazediag21x21.png"
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("failed to read file: %v", err)
	}

	res, err := mazesPng.DecodePng(data)
	if err != nil {
		log.Fatal(err)
	}

	img := image.NewRGBA(image.Rect(0, 0, res.Width, res.Height))

	maze := make([][]bool, len(res.Pixels))

	for y, scanline := range res.Pixels {
		maze[y] = make([]bool, len(scanline))

		for x, pixel := range scanline {
			// Process each pixel here
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

	endY := len(maze) - 1
	endX := len(maze[endY]) - 1

	start := time.Now()
	solution := path.DijkstraOrAStar(true, maze, 0, 1, endX, endY-1)
	elapsed := time.Since(start)

	log.Printf("Solution took %s", elapsed)

	for _, node := range solution {
		img.Set(node.X, node.Y, color.RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF})
	}

	file, _ := os.Create("output.png")
	defer file.Close()
	png.Encode(file, img)
}
