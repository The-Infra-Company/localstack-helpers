package localstack

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	// the image we pull, including tag
	defaultImageURL = "docker.io/localstack/localstack:latest"
	// the image reference used when creating the container
	defaultImage = "localstack/localstack"
	// the single service port that LocalStack exposes by default
	entryPort = "4566/tcp"
	// range of additional service ports
	portRangeStart = 4510
	portRangeEnd   = 4559
	// how long we’ll wait for pull+create operations
	operationTimeout = 2 * time.Minute
)

// Runner encapsulates all config needed to start LocalStack in Docker.
type Runner struct {
	Cli          *client.Client
	ImageURL     string
	Image        string
	PortBindings nat.PortMap
}

// NewRunner returns a Runner, initializing the Docker client if nil.
func NewRunner(cli *client.Client) (*Runner, error) {
	if cli == nil {
		var err error
		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, fmt.Errorf("docker client init failed: %w", err)
		}
	}

	return &Runner{
		Cli:          cli,
		ImageURL:     defaultImageURL,
		Image:        defaultImage,
		PortBindings: buildPortMap(),
	}, nil
}

// buildPortMap generates the same 4566 + 4510-4559 => 127.0.0.1 bindings.
func buildPortMap() nat.PortMap {
	pm := nat.PortMap{
		nat.Port(entryPort): {{HostIP: "127.0.0.1", HostPort: "4566"}},
	}
	for p := portRangeStart; p <= portRangeEnd; p++ {
		port := nat.Port(fmt.Sprintf("%d/tcp", p))
		pm[port] = []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: fmt.Sprint(p)}}
	}
	return pm
}

// Start pulls the image, creates & starts the container, and returns its ID.
func (r *Runner) Start(ctx context.Context) (string, error) {
	// enforce a deadline for pull + create operations
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	if err := r.pullImage(ctx); err != nil {
		return "", err
	}
	return r.createAndStart(ctx)
}

func (r *Runner) pullImage(ctx context.Context) error {
	reader, err := r.Cli.ImagePull(ctx, r.ImageURL, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", r.ImageURL, err)
	}
	defer reader.Close()

	if _, err := io.Copy(os.Stdout, reader); err != nil {
		return fmt.Errorf("failed streaming pull output: %w", err)
	}
	return nil
}

func (r *Runner) createAndStart(ctx context.Context) (string, error) {
	cfg := &container.Config{
		Image:     r.Image,
		Tty:       true,
		OpenStdin: true,
	}
	hostCfg := &container.HostConfig{
		PortBindings: r.PortBindings,
		AutoRemove:   true,
	}

	resp, err := r.Cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("container create failed: %w", err)
	}
	if err := r.Cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("container start failed: %w", err)
	}
	return resp.ID, nil
}

func (r *Runner) StreamLogs(ctx context.Context, containerID string) error {
	opts := container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true}
	reader, err := r.Cli.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return fmt.Errorf("cannot fetch logs: %w", err)
	}
	defer reader.Close()
	_, err = io.Copy(os.Stdout, reader)
	return err
}
