package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/util"
)

type BrokerRequest struct {
	StartY      int
	EndY        int
	StartX      int
	EndX        int
	World       [][]uint8
	WorldHeight int
	WorldWidth  int
}

type BrokerResponse struct {
	World           [][]uint8
	StartY          int
	EndY            int
	AliveCellsCount int
}

var waitRPC sync.WaitGroup

// worker function to calculate next state for a specific region of the world.
func worker(startY, endY, startX, endX int, temp_world [][]uint8, world [][]uint8, worldHeight int, worldWidth int, aliveCells *[]util.Cell, aliveCellsCount *int, workermtx *sync.Mutex) {
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
					workermtx.Lock()
					*aliveCells = append(*aliveCells, util.Cell{j, i})
					(*aliveCellsCount)++
					workermtx.Unlock()
				}
			} else {
				if sum == 3*255 {
					world[i][j] = 255
					workermtx.Lock()
					*aliveCells = append(*aliveCells, util.Cell{j, i})
					(*aliveCellsCount)++
					workermtx.Unlock()
				} else {
					world[i][j] = temp_world[i][j]
				}
			}
		}
	}
}

type Broker struct{}

func (b *Broker) UpdateWorld_RPC(brokerRequest BrokerRequest, brokerResponse *BrokerResponse) error {
	waitRPC.Add(1)
	world := brokerRequest.World
	temp_world := make([][]uint8, brokerRequest.WorldHeight)
	for i := 0; i < brokerRequest.WorldHeight; i++ {
		temp_world[i] = make([]uint8, brokerRequest.WorldWidth)
		copy(temp_world[i], world[i])
	}

	unitX := (brokerRequest.EndX - brokerRequest.StartX) / 8
	var aliveCells []util.Cell = make([]util.Cell, 0)
	var aliveCellsCount int = 0
	var workerwg sync.WaitGroup
	var workermtx sync.Mutex
	workerwg.Add(8)
	for i := 0; i < 8; i++ {
		startY := brokerRequest.StartY
		endY := brokerRequest.EndY
		startX := unitX * i
		endX := unitX * (i + 1)
		go func(startX int, endX int) {
			defer workerwg.Done()
			worker(startY, endY, startX, endX, temp_world, world, brokerRequest.WorldHeight, brokerRequest.WorldWidth, &aliveCells, &aliveCellsCount, &workermtx)
		}(startX, endX)
	}

	workerwg.Wait()
	*brokerResponse = BrokerResponse{
		world,
		brokerRequest.StartY,
		brokerRequest.EndY,
		aliveCellsCount,
	}
	waitRPC.Done()
	return nil
}

func main() {
	// Listen to broker connection
	f := flag.String("ip", "0.0.0.0", "ip to listen on")
	p := flag.String("port", "8080", "ip to listen on")
	flag.Parse()
	ip := fmt.Sprintf("%s", *f)
	port := fmt.Sprintf("%s", *p)
	address := ip + ":" + port
	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Listening failed...")
		return
	} else {
		fmt.Println("Listening successed...")
	}

	// Register Broker
	broker := new(Broker)
	rpc.Register(broker)

	// Accept iteratelly new connection from broker
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal("Connetion failed...")
		}
		go func() {
			fmt.Println("Connection successed...")
			rpc.ServeConn(conn)
		}()

	}
	waitRPC.Wait()
}
