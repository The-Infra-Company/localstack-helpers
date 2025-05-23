# `localstack-helpers`

Go client for managing LocalStack containers during Terraform tests.

## Example

```go
package main

import (
	"context"
	"testing"

	"github.com/The-Infra-Company/localstack-helpers/pkg/localstack"
	"github.com/docker/docker/client"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

func TestTerraformWithLocalStack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	assert.NoError(t, err)
	defer func() { _ = cli.Close() }()

	runner, err := localstack.NewRunner(cli)
	assert.NoError(t, err)

	containerID, err := runner.Start(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, containerID)

	tfOptions := &terraform.Options{
		TerraformDir: "../../examples/complete",
		Upgrade:      true,
		VarFiles:     []string{"fixtures.us-east-2.tfvars"},
	}

	defer terraform.Destroy(t, tfOptions)
	terraform.InitAndApply(t, tfOptions)
}
