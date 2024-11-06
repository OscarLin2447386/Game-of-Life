package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

// worker function to calculate next state for a specific region of the world.
func worker(turn int, startY, endY, startX, endX int, temp_world [][]uint8, world [][]uint8, worldHeight int, worldWidth int, wg *sync.WaitGroup) {
	defer wg.Done()

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
				} else {
					world[i][j] = temp_world[i][j]
				}
			} else {
				if sum == 3*255 {
					world[i][j] = 255
				} else {
					world[i][j] = temp_world[i][j]
				}
			}
		}
	}
}

// mutex for AliveCellsCount
var Keymtx sync.Mutex
var Cellmtx sync.Mutex
var aliveCellsmtx sync.Mutex
var turn int = 0
var pausing bool = false
var closing bool = false
var quitting bool = false
var aliveCells []util.Cell

// Update world by workers and return final result after all turns complete
func UpdateWorld(request gol.Request) gol.FinalResponse {
	var wg sync.WaitGroup
	workerNum := request.Parameters.Threads
	totalTurns := request.Parameters.Turns
	world := request.InitialWorld
	worldHeight := request.Parameters.ImageHeight
	worldWidth := request.Parameters.ImageWidth

	temp_world := make([][]uint8, worldHeight)
	for i := 0; i < worldHeight; i++ {
		temp_world[i] = make([]uint8, worldWidth)
	}
	turn = 0
	closing = false
	quitting = false
	aliveCellsmtx.Lock()

	for turn < totalTurns {
		Keymtx.Lock()
		if !pausing {
			Cellmtx.Lock()
			aliveCells = []util.Cell{}
			// Update temp_world state
			for y := 0; y < worldHeight; y++ {
				copy(temp_world[y], world[y])
			}

			// Distribute work among workers
			unitY := worldHeight / workerNum
			wg.Add(workerNum)
			for i := 1; i <= workerNum; i++ {
				startY := unitY * (i - 1)
				endY := unitY * i
				if i == workerNum {
					leftY := worldHeight - (i-1)*unitY //can change start+leftY to worldHeight
					go worker(turn, startY, startY+leftY, 0, worldWidth, temp_world, world, worldHeight, worldWidth, &wg)
				} else {
					go worker(turn, startY, endY, 0, worldWidth, temp_world, world, worldHeight, worldWidth, &wg)
				}
			}
			// Wait for all workers to complete
			wg.Wait()
			turn++

			//Count Alive Cells
			AliveCellsCount = gol.AliveCellsCount{
				turn,
				CountAliceCells(worldHeight, worldWidth, world),
			}

			// Save current world state
			CurrentWorld = make([][]uint8, worldHeight)
			for i := 0; i < worldHeight; i++ {
				CurrentWorld[i] = make([]uint8, worldWidth)
				copy(CurrentWorld[i], world[i])
			}
			Cellmtx.Unlock()
		}
		Keymtx.Unlock()

		// Quit or Close GolEngine Program if keyPress "q" or "k"
		if quitting || closing {
			break
		}
	}

	// Get final world state
	finalWorld := make([][]uint8, worldHeight)
	for i := 0; i < worldHeight; i++ {
		finalWorld[i] = make([]uint8, worldWidth)
		copy(finalWorld[i], world[i])
	}

	// Get Alive Cells
	aliveCells = []util.Cell{}
	for i := 0; i < worldHeight; i++ {
		for j := 0; j < worldWidth; j++ {
			if finalWorld[i][j] == 255 {
				aliveCells = append(aliveCells, util.Cell{j, i})
			}
		}
	}
	aliveCellsmtx.Unlock()

	finalResponse := gol.FinalResponse{
		finalWorld,
		aliveCells,
		turn,
	}

	return finalResponse
}

// Struct for UpdateWorld_RPC
type UpdateGOLWorld struct{}

// UpdateWorld (RPC)
func (u *UpdateGOLWorld) UpdateWorld_RPC(request gol.Request, response *gol.FinalResponse) error {
	*response = UpdateWorld(request)
	return nil
}

// Count Alive Cells in the World
func CountAliceCells(worldHeight int, worldWidth int, world [][]uint8) int {
	var aliveCells int = 0
	for i := 0; i < worldHeight; i++ {
		for j := 0; j < worldWidth; j++ {
			if world[i][j] == 255 {
				aliveCells++
			}
		}
	}
	return aliveCells
}

// Struct for CountAliveCells_RPC
type CountGOLAliveCells struct{}

var AliveCellsCount gol.AliveCellsCount

// CountAliveCells (RPC)
func (c *CountGOLAliveCells) CountAliveCells_RPC(request struct{}, response *gol.AliveCellsCount) error {
	Cellmtx.Lock()
	*response = AliveCellsCount
	Cellmtx.Unlock()
	return nil
}

// Struct for SaveCurrentWorld_RPC -- KEYPRESS "S"
type SaveCurrentGOLWorld struct{}

var CurrentWorld [][]uint8

// Save current world state -- KEYPRESS "S"
func (s *SaveCurrentGOLWorld) SaveCurrentWorld_RPC(request struct{}, response *gol.CurrentResponse) error {
	Keymtx.Lock()
	*response = gol.CurrentResponse{
		CurrentWorld,
		turn,
	}
	Keymtx.Unlock()
	return nil
}

// Quitting GolEngine Program
func QuitEngine() {
	quitting = true
}

// Struct for QuitGolEngine_RPC
type QuittingGOLEngine struct{}

func (q *QuittingGOLEngine) QuitEngine_RPC(request struct{}, response *gol.FinalTurnComplete) error {
	QuitEngine()
	aliveCellsmtx.Lock()
	*response = gol.FinalTurnComplete{
		turn,
		aliveCells,
	}
	aliveCellsmtx.Unlock()
	return nil
}

// Close GolEngine Program
func CloseEngine() {
	closing = true
}

// Struct for CloseGolEngine_RPC
type ClosingGOLEngine struct{}

// Closing all components of distrubuted system
func (c *ClosingGOLEngine) CloseEngine_RPC(request struct{}, _ *struct{}) error {
	CloseEngine()
	return nil
}

// Pause GolEngine Program
func PauseEngine() {
	if pausing {
		pausing = false
	} else {
		pausing = true
	}

}

// struct for PauseGolEngine_RPC
type PausingGOLEngine struct{}

func (p *PausingGOLEngine) PauseEngine_RPC(request struct{}, response *gol.PausingResponse) error {
	Keymtx.Lock()
	PauseEngine()
	*response = gol.PausingResponse{
		pausing,
		turn,
	}
	Keymtx.Unlock()
	return nil
}

func main() {
	// Listen for connections
	ln, err := net.Listen("tcp", ":8030")

	if err != nil {
		log.Fatal("Listening failed...")
		return
	} else {
		fmt.Println("Listening successed...")
	}

	// Register UpdateGOLWorld
	UpdateGOLWorld := new(UpdateGOLWorld)
	rpc.Register(UpdateGOLWorld)

	// Register CountGOLAliveCells
	CountGOLAliveCells := new(CountGOLAliveCells)
	rpc.Register(CountGOLAliveCells)

	// Register SaveCurrentGOLWorld
	SaveCurrentGOLWorld := new(SaveCurrentGOLWorld)
	rpc.Register(SaveCurrentGOLWorld)

	// Register QuittingGOLEngine
	QuittingGOLEngine := new(QuittingGOLEngine)
	rpc.Register(QuittingGOLEngine)

	// Register ClosingGOLEngine
	ClosingGOLEngine := new(ClosingGOLEngine)
	rpc.Register(ClosingGOLEngine)

	// Register PausingGOLEngine
	PausingGOLEngine := new(PausingGOLEngine)
	rpc.Register(PausingGOLEngine)

	// Iteratally connect to client
	for {
		conn, err := ln.Accept()

		if err != nil {
			log.Fatal("Connection failed with client")
		}

		go func() {
			rpc.ServeConn(conn)
			fmt.Println("Connection successed with client")

		}()

		// Exit GolEngine
		if closing {
			ln.Close()
		}

	}
}
