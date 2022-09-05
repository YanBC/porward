package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/YanBC/porward/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
)

var (
	containerName string
	containerPort int
	hostPort      int
	transport     string
)

func isPortValid(port int) bool {
	return port > 0 && port < 65535
}

func init() {
	var portMapping string
	flag.StringVar(&containerName, "c", "", "target container name or id")
	flag.StringVar(&portMapping, "p", "", "port mapping of the form <host port>:<container port>")
	flag.StringVar(&transport, "t", "tcp", "transport type, tcp or udp")
	flag.Parse()

	if containerName == "" {
		fmt.Println("specify target container, see -h for usage")
		os.Exit(2)
	}

	_, err := fmt.Sscanf(portMapping, "%d:%d", &hostPort, &containerPort)
	if err != nil {
		fmt.Println("fail to parse port mapping, see -h for usage")
		os.Exit(2)
	}

	if !(isPortValid(hostPort) && isPortValid(containerPort)) {
		fmt.Println("invalid port numbers")
		os.Exit(2)
	}

	if transport != "tcp" && transport != "udp" {
		fmt.Println("only supporting tcp or udp at the moment, see -h for usage")
		os.Exit(2)
	}

	fmt.Printf("serving container %s port %d on host machine port %d\n", containerName, containerPort, hostPort)
}

func main() {
	// create docker client
	cli, err := utils.CreateDockerClient()
	if err != nil {
		fmt.Printf("fail to create docker client, %s\n", err)
		return
	}

	// query container info
	ctx := context.Background()
	if !utils.IsContainerLive(ctx, cli, containerName) {
		fmt.Printf("container %s does not exist\n", containerName)
		return
	}
	c_json, _ := cli.ContainerInspect(ctx, containerName)
	network_mode := c_json.HostConfig.NetworkMode

	// pull docker image
	image_name := utils.GetDockerImage()
	// if _, err = cli.ImagePull(ctx, image_name, types.ImagePullOptions{}); err != nil {
	// 	fmt.Println("fail to pull image")
	// 	return
	// }

	// create docker container
	container_config := container.Config{
		Image: image_name,
		Cmd: []string{
			"forward",
			fmt.Sprintf("-container=%s", containerName),
			fmt.Sprintf("-srcAddr=target:%d", containerPort),
			fmt.Sprintf("-desAddr=:%d", hostPort),
			fmt.Sprintf("-transport=%s", transport),
		},
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/%s", hostPort, transport)): {},
		},
	}
	host_config := container.HostConfig{
		NetworkMode: container.NetworkMode(network_mode),
		PortBindings: nat.PortMap{
			nat.Port(fmt.Sprintf("%d/%s", hostPort, transport)): []nat.PortBinding{
				{
					HostIP:   "",
					HostPort: fmt.Sprintf("%d", hostPort),
				},
			},
		},
		Init:       &[]bool{true}[0],
		AutoRemove: true,
		RestartPolicy: container.RestartPolicy{
			Name:              "no",
			MaximumRetryCount: 0,
		},
		Links: []string{fmt.Sprintf("%s:%s", containerName, "target")},
		Mounts: []mount.Mount{
			mount.Mount{
				Type:     mount.TypeBind,
				Source:   "/run/docker.sock",
				Target:   "/run/docker.sock",
				ReadOnly: false,
			},
		},
	}
	create_resp, err := cli.ContainerCreate(
		ctx, &container_config, &host_config, nil, nil, "")
	if err != nil {
		fmt.Printf("fail to create container, %s", err)
		return
	}

	// start container
	if err := cli.ContainerStart(ctx, create_resp.ID, types.ContainerStartOptions{}); err != nil {
		fmt.Printf("fail to start container, %s", err)
		return
	}
}
