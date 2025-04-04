package gol

import (
	"fmt"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

// worker function to calculate next state for a specific region of the world.
func worker(c distributorChannels, turn int, startY, endY, startX, endX int, temp_world [][]uint8, world [][]uint8, worldHeight int, worldWidth int, wg *sync.WaitGroup) {
	defer wg.Done()
	//initialize cellsFlipped struct
	cellsFlipped := CellsFlipped{
		turn,
		nil,
	}

	for i := startY; i < endY; i++ {
		for j := startX; j < endX; j++ {
			sum := int(temp_world[(i-1+worldHeight)%worldHeight][(j-1+worldWidth)%worldWidth]) +
				int(temp_world[i%worldHeight][(j-1+worldWidth)%worldWidth]) +
				int(temp_world[(i+1)%worldHeight][(j-1+worldWidth)%worldWidth]) +
				int(temp_world[(i-1+worldHeight)%worldHeight][j%worldWidth]) +
				int(temp_world[(i+1)%worldHeight][j%worldWidth]) +
				int(temp_world[(i-1+worldHeight)%worldHeight][(j+1)%worldWidth]) +
				int(temp_world[i%worldHeight][(j+1)%worldWidth]) +
				int(temp_world[(i+1)%worldHeight][(j+1)%worldWidth])

			if temp_world[i][j] == 255 {
				if sum < 2*255 || sum > 3*255 {
					world[i][j] = 0
					cellsFlipped.Cells = append(cellsFlipped.Cells, util.Cell{j, i})
				} else {
					world[i][j] = temp_world[i][j]
				}
			} else {
				if sum == 3*255 {
					world[i][j] = 255
					cellsFlipped.Cells = append(cellsFlipped.Cells, util.Cell{j, i})
				} else {
					world[i][j] = temp_world[i][j]
				}
			}
		}
	}
	c.events <- cellsFlipped
}

// count currentAliveCells function
func currentAliveCells(imageHeight int, imageWidth int, world [][]uint8) int {
	var aliveCells int
	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			if world[i][j] == 255 {
				aliveCells++
			}
		}
	}
	return aliveCells
}

// Save the current state of the board as a PGM image
func saveCurrentWorld(p Params, c distributorChannels, turn int, currentWorld ImageOutputComplete, world [][]uint8) {
	currentImageFile := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)
	currentWorld = ImageOutputComplete{
		turn,
		currentImageFile,
	}
	c.ioCommand <- ioOutput
	c.ioFilename <- currentImageFile
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- currentWorld
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	world := make([][]uint8, p.ImageHeight)
	temp_world := make([][]uint8, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		world[i] = make([]uint8, p.ImageWidth)
		temp_world[i] = make([]uint8, p.ImageWidth)
	}

	// Get filename and load initial world state
	filename := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	//initialize cellsFlipped struct
	cellsFlipped := CellsFlipped{
		0,
		nil,
	}
	//initializing world
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			cell := <-c.ioInput
			world[y][x] = cell
			// Check if cell is alive for cellsFlipped event
			if cell == 255 {
				cellsFlipped.Cells = append(cellsFlipped.Cells, util.Cell{x, y})
			}
		}
	}
	c.events <- cellsFlipped

	turn := 0
	c.events <- StateChange{turn, Executing}

	// Copy world to temp_world
	for y := 0; y < p.ImageHeight; y++ {
		copy(temp_world[y], world[y])
	}

	//Start ticker for reporting alive cells every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var tickerWg sync.WaitGroup

	//Stop ticker goroutine
	quit_ticker := make(chan bool)

	//Start a goroutine for periodic alive cell counting
	tickerWg.Add(1)
	var currentAliveCellCount AliveCellsCount
	//Save the current state of the board as a PGM image
	var currentWorld ImageOutputComplete
	saveCurrentState := make(chan bool)
	stopCurrentTurn := make(chan bool)
	pauseExecution := make(chan bool)
	pauseStatus := false
	var pausemtx sync.Mutex
	var alivemtx sync.Mutex
	go func() {
		defer tickerWg.Done()
		for {
			select {
			case <-ticker.C:
				alivemtx.Lock()
				c.events <- currentAliveCellCount
				alivemtx.Unlock()
			case key := <-c.keyPresses:
				if key == 's' {
					saveCurrentState <- true
				} else if key == 'q' {
					stopCurrentTurn <- true
				} else if key == 'p' {
					pausemtx.Lock()
					if pauseStatus == false {
						pauseExecution <- true
						pauseStatus = true
					} else {
						pauseExecution <- false
						pauseStatus = false
					}
					pausemtx.Unlock()
				}
			case ticker_close := <-quit_ticker:
				if ticker_close {
					return //close this goroutine
				}
			}
		}
	}()

	// Execute all turns of the Game of Life
	var wg sync.WaitGroup
	workerNum := p.Threads
	for turn < p.Turns {
		if pauseStatus == false {
			// Update temp_world state
			for y := 0; y < p.ImageHeight; y++ {
				copy(temp_world[y], world[y])
			}

			// Distribute work among workers
			unitY := p.ImageHeight / workerNum
			wg.Add(workerNum)
			for i := 1; i <= workerNum; i++ {
				startY := unitY * (i - 1)
				endY := unitY * i
				if i == workerNum {
					leftY := p.ImageHeight - (i-1)*unitY
					go worker(c, turn, startY, startY+leftY, 0, p.ImageWidth, temp_world, world, p.ImageHeight, p.ImageWidth, &wg)
				} else {
					go worker(c, turn, startY, endY, 0, p.ImageWidth, temp_world, world, p.ImageHeight, p.ImageWidth, &wg)
				}
			}

			// Wait for all workers to complete
			wg.Wait()

			turn++
			//Send TurnComplete events
			turnComplete := TurnComplete{
				turn,
			}
			c.events <- turnComplete

			// Report alive cells after each turn
			alivemtx.Lock()
			currentAliveCellCount = AliveCellsCount{
				turn,
				currentAliveCells(p.ImageHeight, p.ImageWidth, world),
			}
			alivemtx.Unlock()
		}

		//KeyPress
		select {
		case save := <-saveCurrentState:
			{
				if save == true {
					saveCurrentWorld(p, c, turn, currentWorld, world)
				}
			}
		case stop := <-stopCurrentTurn:
			{
				if stop == true {
					var aliveCells []util.Cell
					for i := 0; i < p.ImageHeight; i++ {
						for j := 0; j < p.ImageWidth; j++ {
							if world[i][j] == 255 {
								aliveCells = append(aliveCells, util.Cell{j, i})
							}
						}
					}
					final := FinalTurnComplete{
						turn,
						aliveCells,
					}
					c.events <- final
					saveCurrentWorld(p, c, turn, currentWorld, world)
					c.ioCommand <- ioCheckIdle
					<-c.ioIdle
					c.events <- StateChange{turn, Quitting}
					return
				}
			}
		case pause := <-pauseExecution:
			{
				pausemtx.Lock()
				if pause == true {
					c.events <- StateChange{turn, Paused}
				} else {
					c.events <- StateChange{turn, Executing}
				}
				pausemtx.Unlock()
			}
		default:
		}

	}

	quit_ticker <- true

	// Report the final state using FinalTurnCompleteEvent
	var aliveCells []util.Cell
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] == 255 {
				aliveCells = append(aliveCells, util.Cell{j, i})
			}
		}
	}

	final := FinalTurnComplete{
		turn,
		aliveCells,
	}
	c.events <- final

	//Output final image
	c.ioCommand <- ioOutput
	finalImgFilename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)
	c.ioFilename <- finalImgFilename
	finalWorld := ImageOutputComplete{
		turn,
		finalImgFilename,
	}
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.events <- finalWorld

	//Make sure that the IO has finished any output before exiting
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	//Close the channel to stop the SDL goroutine gracefully
	close(c.events)
	close(quit_ticker)
	close(saveCurrentState)
	close(stopCurrentTurn)
	close(pauseExecution)

	//Wait for ticker goroutine to finish
	tickerWg.Wait()
}
