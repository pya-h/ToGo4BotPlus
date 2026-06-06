package main

import (
	"fmt"
	"slices"
)

func main() {
	x := make([]int, 0)
	x = append(x, 10)
	x = append(x, 20)
	x = append(x, 25)
	x = slices.Delete(x, 2, 3)
	fmt.Println(x)
}
