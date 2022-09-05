package utils

import (
	"context"

	"github.com/docker/docker/client"
)

func CreateDockerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	return cli, err
}

func IsContainerLive(ctx context.Context, cli *client.Client, name_or_id string) bool {
	container, err := cli.ContainerInspect(ctx, name_or_id)
	if err != nil {
		return false
	}
	// container status can be one of "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	status := container.State.Status
	if status != "running" && status != "paused" && status != "restarting" {
		return false
	}
	return true
}

func GetDockerImage() string {
	return "docker.io/yanbc/porward:latest"
}
