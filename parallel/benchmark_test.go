package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

func BenchmarkGol(b *testing.B) {

	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stdout)

	tests := []gol.Params{
		//{ImageWidth: 16, ImageHeight: 16},
		//{ImageWidth: 64, ImageHeight: 64},
		{ImageWidth: 512, ImageHeight: 512},
	}
	for _, t := range tests {
		for _, turns := range []int{100} {
			t.Turns = turns
			for threads := 1; threads <= 16; threads++ {
				t.Threads = threads
				b.Run(fmt.Sprintf("%dx%dx%d-%d", t.ImageWidth, t.ImageHeight, t.Turns, t.Threads), func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						events := make(chan gol.Event)
						go gol.Run(t, events, nil)

						for range events {
						}
					}
				})
			}
		}
	}
}
