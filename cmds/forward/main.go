package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/YanBC/porward/forward"
	"github.com/YanBC/porward/utils"
)

var (
	containerName string
	srcAddr       string
	desAddr       string
	transport     string
)

func init() {
	flag.StringVar(&containerName, "container", "", "")
	flag.StringVar(&srcAddr, "srcAddr", "", "")
	flag.StringVar(&desAddr, "desAddr", "", "")
	flag.StringVar(&transport, "transport", "tcp", "transport type, tcp or udp")
	flag.Parse()
}

func main() {
	// create docker client
	cli, err := utils.CreateDockerClient()
	if err != nil {
		fmt.Printf("fail to create docker client, %s\n", err)
		os.Exit(2)
	}

	// start forward server
	var server *forward.Server
	switch transport {
	case "tcp":
		server, err = forward.NewTCPServer(desAddr, srcAddr)
	case "udp":
		server, err = forward.NewUDPServer(desAddr, srcAddr)
	default:
		err = errors.New("unsupported transport protocol")
	}
	if server == nil {
		fmt.Printf("fail to create forward server, %s", err)
		os.Exit(2)
	}

	// wait for container to exit
	ctx := context.Background()
	stop := make(chan bool)
	go func() {
		counts := 0
		for counts < 3 {
			if !utils.IsContainerLive(ctx, cli, containerName) {
				counts += 1
			}
			time.Sleep(time.Second)
		}
		stop <- true
	}()

	// wait for stop
	<-stop
}
