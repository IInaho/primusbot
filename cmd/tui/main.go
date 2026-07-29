package main

import (
	"log"

	"nekocode/interaction/tui"
)

func main() {
	if err := tui.Run(); err != nil {
		log.Fatal(err)
	}
}
