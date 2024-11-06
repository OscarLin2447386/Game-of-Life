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

type Controler struct{}

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

var awsNodeIPs [16]string = [16]string{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""}

//var brokers []*rpc.Client = make([]*rpc.Client, 0, 16)

// Global variables
var turn int = 0
var pausing bool = false
var closing bool = false
var quitting bool = false
var currentWorld [][]uint8
var currentAliveCellsCount int
var responsesMtx sync.Mutex
var keyPressMtx sync.Mutex
var countAliveCellsMtx sync.Mutex
var waitRPC sync.WaitGroup

// Run the game
func runGameBrokerCall(controlerRequest gol.Request) gol.FinalResponse {
	waitRPC.Add(1)
	defer waitRPC.Done()

	brokers := make([]*rpc.Client, controlerRequest.Parameters.Threads)

	for nodeIPIndex := 0; nodeIPIndex < controlerRequest.Parameters.Threads; nodeIPIndex++ {
		// Connecting to AWS nodes
		address := awsNodeIPs[nodeIPIndex] + ":8080"
		broker, err := rpc.Dial("tcp", address)
		if err != nil {
			log.Fatal("Dialing failed...")
		} else {
			fmt.Println("Dialing seccessed...")
		}
		brokers[nodeIPIndex] = broker
	}

	// Declare all variable to be used during the process
	var nodeswg sync.WaitGroup
	var currentAliveCells []util.Cell

	currentWorld = controlerRequest.InitialWorld

	closing = false
	quitting = false
	// pausing = false

	// Each turn calling RPC to update world
	for turn < controlerRequest.Parameters.Turns {
		combineResponse := make([]BrokerResponse, 0)

		keyPressMtx.Lock()
		if !pausing {
			countAliveCellsMtx.Lock()
			nodeswg.Add(controlerRequest.Parameters.Threads)
			currentAliveCellsCount = 0

			// Assigning tasks to GolEngine server
			for i := 1; i <= controlerRequest.Parameters.Threads; i++ {
				unitY := controlerRequest.Parameters.ImageHeight / controlerRequest.Parameters.Threads
				var brokerRequest BrokerRequest
				if i < controlerRequest.Parameters.Threads {
					// Create request for RPC
					brokerRequest = BrokerRequest{
						unitY * (i - 1),
						unitY * i,
						0,
						controlerRequest.Parameters.ImageWidth,
						currentWorld,
						controlerRequest.Parameters.ImageHeight,
						controlerRequest.Parameters.ImageWidth,
					}
				} else {
					// Create request for RPC
					brokerRequest = BrokerRequest{
						unitY * (i - 1),
						controlerRequest.Parameters.ImageHeight,
						0,
						controlerRequest.Parameters.ImageWidth,
						currentWorld,
						controlerRequest.Parameters.ImageHeight,
						controlerRequest.Parameters.ImageWidth,
					}
				}

				// Calling RPC to update world
				go func(brokerIndex int) {
					var brokerResponse BrokerResponse
					defer nodeswg.Done()
					brokers[brokerIndex-1].Call("Broker.UpdateWorld_RPC", brokerRequest, &brokerResponse)
					responsesMtx.Lock()
					combineResponse = append(combineResponse, brokerResponse)
					responsesMtx.Unlock()
				}(i)

			}
			nodeswg.Wait()
			// Combine work result from the nodes
			for n := 0; n < controlerRequest.Parameters.Threads; n++ {
				nodeResponse := combineResponse[n]
				sliceWorld := nodeResponse.World
				startY := nodeResponse.StartY
				endY := nodeResponse.EndY
				currentAliveCellsCount += nodeResponse.AliveCellsCount
				// Update a slice of current world handled by a node
				for i := startY; i < endY; i++ {
					copy(currentWorld[i], sliceWorld[i])
				}
			}
			turn++
			countAliveCellsMtx.Unlock()

		}
		keyPressMtx.Unlock()

		// Break Broker game run if keyPress "q" or "k"
		if quitting || closing {
			if closing {
				for i := 0; i < controlerRequest.Parameters.Threads; i++ {
					brokers[i].Call("Broker.CloseAWSNode_RPC", struct{}{}, struct{}{})
				}
			}
			break
		}
	}

	// Construct final alive cells
	for j := 0; j < controlerRequest.Parameters.ImageHeight; j++ {
		for k := 0; k < controlerRequest.Parameters.ImageWidth; k++ {
			if currentWorld[j][k] == 255 {
				currentAliveCells = append(currentAliveCells, util.Cell{k, j})
			}
		}
	}

	// Construct final response to send to client
	controlerResponse := gol.FinalResponse{
		currentWorld,
		currentAliveCells,
		turn,
	}
	pausing = false
	turn = 0

	// Close awsNodes connections
	for _, broker := range brokers {
		if broker != nil {
			defer broker.Close()
		}
	}

	return controlerResponse
}

// PRC for runGameBrokerCall
func (c *Controler) RunGameBrokerCall_RPC(controlerRequest gol.Request, controlerResponse *gol.FinalResponse) error {
	waitRPC.Add(1)
	defer waitRPC.Done()
	*controlerResponse = runGameBrokerCall(controlerRequest)
	return nil
}

// RPC for CountAliveCells
func (c *Controler) CountAliveCells_RPC(controlerRequest struct{}, controlerResponse *gol.AliveCellsCount) error {
	waitRPC.Add(1)
	defer waitRPC.Done()
	countAliveCellsMtx.Lock()
	*controlerResponse = gol.AliveCellsCount{
		turn,
		currentAliveCellsCount,
	}
	countAliveCellsMtx.Unlock()
	return nil
}

// RPC for SaveCurrentWorld
func (c *Controler) SaveCurrentWorld_RPC(controlerRequest struct{}, controlerResponse *gol.CurrentResponse) error {
	waitRPC.Add(1)
	defer waitRPC.Done()
	keyPressMtx.Lock()
	*controlerResponse = gol.CurrentResponse{
		currentWorld,
		turn,
	}
	keyPressMtx.Unlock()
	return nil
}

// RPC for QuitBroker
func (c *Controler) QuitBroker_RPC(controlerRequest struct{}, controlerResponse *struct{}) error {
	waitRPC.Add(1)
	defer waitRPC.Done()
	//keyPressMtx.Lock()
	quitting = true
	//pausing = true
	//keyPressMtx.Unlock()
	return nil
}

// RPC for CloseBroker
func (c *Controler) CloseBroker_RPC(controlerRequest struct{}, controlerResponse *struct{}) error {
	waitRPC.Add(1)
	defer waitRPC.Done()
	closing = true
	return nil
}

// RPC for PauseBroker
func (c *Controler) PauseBroker_RPC(controlerRequest struct{}, controlerResponse *gol.PausingResponse) error {
	waitRPC.Add(1)
	defer waitRPC.Done()
	keyPressMtx.Lock()
	pausing = !pausing
	*controlerResponse = gol.PausingResponse{
		pausing,
		turn,
	}
	keyPressMtx.Unlock()
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

	// Register Contro
	Controler := new(Controler)
	rpc.Register(Controler)

	// Iteratelly connect to local controllers
	for {
		conn, err := ln.Accept()

		if err != nil {
			log.Fatal("Connection failed with client")
		}

		go func() {
			rpc.ServeConn(conn)
			fmt.Println("Connection successed with client")

		}()

		// Stop listening to controler connection
		if closing {
			ln.Close()
			fmt.Println("Closing gracefully the Broker Listener")
			break
		}
	}
	waitRPC.Wait()
	fmt.Println("Closing gracefully the Broker")
}
