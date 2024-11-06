package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"sync"
	"time"

	// "uk.ac.bris.cs/gameoflife/gol"
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

// Request sent to the GOLEngine
type Request struct {
	Parameters   Params
	InitialWorld [][]uint8
}

// Response from the GOLEngine
type CurrentResponse struct {
	CurrentWorld [][]uint8
	CurrentTurns int
}

// Pausing Response from GOLEngine
type PausingResponse struct {
	PausingState bool
	Turn         int
}

// Final response from the GOLEngine
type FinalResponse struct {
	FinalWorld          [][]uint8
	FinalAliveCellCount []util.Cell
	CompleteTurns       int
}

var pausing bool = false

// Create ticker to control sending alive cell each 2 sec
func createAliveCellTicker(c distributorChannels, client *rpc.Client, quitTicker chan bool) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var response AliveCellsCount
	for {
		select {
		case <-ticker.C:
			if !pausing {
				client.Call("Controler.CountAliveCells_RPC", struct{}{}, &response)
				c.events <- response
			}
		case <-quitTicker:
			return
		}
	}
}

// Makes a call to run the world update
func runGameCall(p Params, c distributorChannels, client *rpc.Client, world [][]uint8) FinalResponse {
	request := Request{
		p,
		world,
	}
	var finalResponse FinalResponse
	var UpdateWorldBrokerwg sync.WaitGroup
	UpdateWorldBrokerwg.Add(1)
	go func() {
		defer UpdateWorldBrokerwg.Done()
		client.Call("Controler.RunGameBrokerCall_RPC", request, &finalResponse)
	}()

	// Establish a ticker for alive cell count
	quitTicker := make(chan bool)
	go createAliveCellTicker(c, client, quitTicker)

	UpdateWorldBrokerwg.Wait()
	quitTicker <- true
	close(quitTicker)
	return finalResponse
}

// Makes a call to detect the key presses
func detectKeyPressesCall(p Params, c distributorChannels, client *rpc.Client, quitDetector chan bool) {
	for {
		select {
		case key := <-c.keyPresses:
			if key == 's' {
				var currentResponse CurrentResponse
				client.Call("Controler.SaveCurrentWorld_RPC", struct{}{}, &currentResponse)

				currentImgFilename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, currentResponse.CurrentTurns)

				currentWorld := ImageOutputComplete{
					currentResponse.CurrentTurns,
					currentImgFilename,
				}

				c.ioCommand <- ioOutput
				c.ioFilename <- currentImgFilename
				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						c.ioOutput <- currentResponse.CurrentWorld[y][x]
					}
				}

				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- currentWorld

			} else if key == 'q' {
				client.Call("Controler.QuitBroker_RPC", struct{}{}, &struct{}{})

			} else if key == 'k' {
				client.Call("Controler.CloseBroker_RPC", struct{}{}, &struct{}{})

			} else if key == 'p' {
				pausing = !pausing
				var pausingResponse PausingResponse
				client.Call("Controler.PauseBroker_RPC", struct{}{}, &pausingResponse)
				if !pausingResponse.PausingState {
					fmt.Println(pausingResponse.Turn)
					c.events <- StateChange{pausingResponse.Turn, Executing}
				} else {
					fmt.Println(pausingResponse.Turn)
					c.events <- StateChange{pausingResponse.Turn, Paused}
				}
			}
		case <-quitDetector:
			return
		}
	}
}

// Distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// Create 2D slice to initialise world
	world := make([][]uint8, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		world[i] = make([]uint8, p.ImageWidth)
	}

	// Get filename and load initial world state
	filename := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	// Initialising world
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			cell := <-c.ioInput
			world[y][x] = cell
		}
	}

	// Initialise state of running game
	c.events <- StateChange{0, Executing}

	// Create another local controller connection for keyPresses detection
	quitDetector := make(chan bool)
	go func(chan bool) {
		client, err := rpc.Dial("tcp", "127.0.0.1:8030")
		if err != nil {
			log.Fatal("Dialing failed...")
			return
		} else {
			fmt.Println("Dialing successed...")
		}
		defer client.Close()
		detectKeyPressesCall(p, c, client, quitDetector)

	}(quitDetector)

	// Create a local controller connection
	client, err := rpc.Dial("tcp", "127.0.0.1:8030")
	if err != nil {
		log.Fatal("Dialing failed...")
		return
	} else {
		fmt.Println("Dialing successed...")
	}
	defer client.Close()

	response := runGameCall(p, c, client, world)
	quitDetector <- true

	// Report the final state using FinalTurnCompleteEvent.
	turn := response.CompleteTurns
	aliveCellsCount := response.FinalAliveCellCount
	finalTurnComplete := FinalTurnComplete{
		turn,
		aliveCellsCount,
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- finalTurnComplete

	// Output the state of the board as final PGM image
	c.ioCommand <- ioOutput
	finalImgFilename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)
	c.ioFilename <- finalImgFilename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- response.FinalWorld[y][x]
		}
	}

	finalWorld := ImageOutputComplete{
		turn,
		finalImgFilename,
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- finalWorld
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
	close(quitDetector)
}
