package main

import (
	"fmt"
	"mazes/png"
)

func main() {
	png.Png()
	a := []int{137, 80, 78, 71, 13, 10, 26, 10}
	hexArray := []string{}
	for _, v := range a {
		hexArray = append(hexArray, fmt.Sprintf("%x", v))
	}
	fmt.Println(hexArray)
	asciiString := ""
	for _, hex := range a {
		asciiString += string(hex)
	}
	fmt.Println(asciiString)
}
