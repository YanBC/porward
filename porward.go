package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func printUsage() {
	executable := os.Args[0]
	fmt.Printf("see %s -h for usage\n", executable)
}

func main() {
	// parse arguments
	target_name := flag.String("c", "", "container name")
	port := flag.String("p", "", "port mapping of format <host_port>:<container_port>")
	flag.Parse()

	if *target_name == "" {
		fmt.Println("you must provide a container name")
		printUsage()
		return
	}
	ports := strings.Split(*port, ":")
	if len(ports) != 2 {
		fmt.Println("you must provide port mapping")
		printUsage()
		return
	}
	host_port := ports[0]
	target_port := ports[1]
	if host_port == "" || target_port == "" {
		fmt.Println("you must provide port mapping")
		printUsage()
		return
	}

	// create docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	// query docker container info
	ctx := context.Background()
	running_containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	target_container := types.Container{}
	container_found := false
	for _, target_container = range running_containers {
		if target_container.Names[0] == fmt.Sprintf("/%s", *target_name) {
			container_found = true
			break
		}
	}
	if !container_found {
		panic(fmt.Sprintf("no container by the name %s", *target_name))
	}

	target_network := ""
	target_ip4_addr := ""
	networks := target_container.NetworkSettings.Networks
	for key, value := range networks {
		if key != "host" {
			target_network = key
			target_ip4_addr = value.IPAddress
			break
		}
	}
	if target_network == "" || target_ip4_addr == "" {
		panic(fmt.Sprintf("fail to get network settings for %s", *target_name))
	}
	target_networkMode := target_container.HostConfig.NetworkMode

	// pull gost docker image
	image_pull_resp, err := cli.ImagePull(ctx, "docker.io/ginuerzh/gost:2.11.1", types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, image_pull_resp)

	// create gost docker container
	proxy_config := container.Config{
		Image: "ginuerzh/gost:2.11.1",
		Cmd:   []string{"-L", fmt.Sprintf("tcp://:%s/%s:%s", host_port, target_ip4_addr, target_port)},
		Healthcheck: &container.HealthConfig{
			Test:     []string{"CMD-SHELL", fmt.Sprintf("ping -c 1 %s", target_ip4_addr)},
			Interval: 2 * time.Second,
			Retries:  3,
			Timeout:  1 * time.Second,
		},
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%s/tcp", host_port)): {},
		},
	}
	proxy_host_config := container.HostConfig{
		NetworkMode: container.NetworkMode(target_networkMode),
		PortBindings: nat.PortMap{
			nat.Port(fmt.Sprintf("%s/tcp", target_port)): []nat.PortBinding{
				nat.PortBinding{
					HostIP:   "",
					HostPort: host_port,
				},
			},
		},
		Init:       &[]bool{true}[0],
		AutoRemove: true,
		RestartPolicy: container.RestartPolicy{
			Name:              "no",
			MaximumRetryCount: 0,
		},
	}
	proxy_create_resp, err := cli.ContainerCreate(ctx, &proxy_config, &proxy_host_config, nil, nil, "")
	if err != nil {
		panic(err)
	}

	// start gost docker container
	if err := cli.ContainerStart(ctx, proxy_create_resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}
}