package infra

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// TaskDefResolver registers new ECS task definition revisions with store-specific
// EFS access point configurations.
type TaskDefResolver struct {
	client  *ecs.Client
	baseArn string
}

// NewTaskDefResolver creates a TaskDefResolver using the given base task definition ARN as a template.
func NewTaskDefResolver(client *ecs.Client, baseArn string) *TaskDefResolver {
	return &TaskDefResolver{client: client, baseArn: baseArn}
}

// ResolveWithAccessPoint registers a new task definition revision with the given EFS
// access point injected into the volume configuration, returning the new task definition ARN.
func (r *TaskDefResolver) ResolveWithAccessPoint(ctx context.Context, accessPointID string) (string, error) {
	td, err := r.describeBaseTaskDef(ctx)
	if err != nil {
		return "", err
	}
	return r.registerRevision(ctx, td, accessPointID)
}

func (r *TaskDefResolver) describeBaseTaskDef(ctx context.Context) (*types.TaskDefinition, error) {
	out, err := r.client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(r.baseArn),
	})
	if err != nil {
		return nil, fmt.Errorf("describe task definition: %w", err)
	}
	return out.TaskDefinition, nil
}

func (r *TaskDefResolver) registerRevision(ctx context.Context, td *types.TaskDefinition, accessPointID string) (string, error) {
	input := cloneTaskDef(td, patchVolumes(td.Volumes, accessPointID))
	out, err := r.client.RegisterTaskDefinition(ctx, input)
	if err != nil {
		return "", fmt.Errorf("register task definition: %w", err)
	}
	return aws.ToString(out.TaskDefinition.TaskDefinitionArn), nil
}

func cloneTaskDef(td *types.TaskDefinition, volumes []types.Volume) *ecs.RegisterTaskDefinitionInput {
	return &ecs.RegisterTaskDefinitionInput{
		Family:                  td.Family,
		ContainerDefinitions:    td.ContainerDefinitions,
		Cpu:                     td.Cpu,
		Memory:                  td.Memory,
		NetworkMode:             td.NetworkMode,
		RequiresCompatibilities: td.RequiresCompatibilities,
		ExecutionRoleArn:        td.ExecutionRoleArn,
		TaskRoleArn:             td.TaskRoleArn,
		Volumes:                 volumes,
	}
}

func patchVolumes(volumes []types.Volume, accessPointID string) []types.Volume {
	patched := make([]types.Volume, len(volumes))
	copy(patched, volumes)
	for i, v := range patched {
		if aws.ToString(v.Name) == EfsVolumeName {
			patched[i] = patchEfsVolume(v, accessPointID)
		}
	}
	return patched
}

func patchEfsVolume(v types.Volume, accessPointID string) types.Volume {
	if v.EfsVolumeConfiguration == nil {
		return v
	}
	efsConfig := *v.EfsVolumeConfiguration
	authConfig := types.EFSAuthorizationConfig{}
	if efsConfig.AuthorizationConfig != nil {
		authConfig = *efsConfig.AuthorizationConfig
	}
	authConfig.AccessPointId = aws.String(accessPointID)
	efsConfig.AuthorizationConfig = &authConfig
	v.EfsVolumeConfiguration = &efsConfig
	return v
}
