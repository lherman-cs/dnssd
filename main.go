package main

import (
	"fmt"
	"time"
)

func main() {
	in, _ := Discover("_googlecast._tcp.local.")

	go func() {
		for resp := range in {
			fmt.Println(resp.DeviceName)
		}
	}()

	time.Sleep(time.Second * 3)
}
