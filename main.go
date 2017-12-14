package main

import (
	"fmt"
	"time"
)

var (
	MAX_HOPS         = 64
	RING_BUFFER_SIZE = 8
)

func main() {
	fmt.Println("Start:", time.Now())
	m := NewMTR("8.8.8.8")
	m.Run()
	m.Run()
	m.Run()
	m.Render()
}
