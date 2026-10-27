package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"storr-api/internal/infra"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// Service manages the lifecycle of Hydrogen pipeline ECS tasks, including launching,
// status polling, and runtime querying for store builds.
type Service struct {
	taskResolver
	runtime      *infra.RuntimeClient
	previewRoute *infra.PreviewRouteManager
	config       Config
}

// NewService creates a pipeline Service with the given AWS clients, runtime poller,
// preview route manager, and task configuration.
func NewService(clients infra.AWSClients, runtime *infra.RuntimeClient, previewRoute *infra.PreviewRouteManager, config Config) *Service {
	return &Service{
		taskResolver: taskResolver{clients: clients, cluster: config.Task.ClusterName},
		runtime:      runtime,
		previewRoute: previewRoute,
		config:       config,
	}
}

// Provision starts a new Hydrogen pipeline ECS task for the given store, or returns the
// current status if a task is already running.
func (s *Service) Provision(ctx context.Context, storeID string, envOverrides []types.KeyValuePair) (*HydrogenStatusResponse, error) {
	taskArn, err := s.findTask(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	if taskArn != "" {
		return s.GetStatus(ctx, storeID)
	}
	return s.launchTask(ctx, storeID, envOverrides)
}

func (s *Service) launchTask(ctx context.Context, storeID string, envOverrides []types.KeyValuePair) (*HydrogenStatusResponse, error) {
	input := s.buildRunTaskInput(storeID, envOverrides)
	out, err := s.clients.ECS.RunTask(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("run task: %w", err)
	}
	logTaskLaunch(out, storeID)
	return &HydrogenStatusResponse{Status: HydrogenStatusProvisioning}, nil
}

func logTaskLaunch(out *ecs.RunTaskOutput, storeID string) {
	if len(out.Tasks) > 0 {
		slog.Info("launched pipeline task", "taskArn", aws.ToString(out.Tasks[0].TaskArn), "storeId", storeID)
	}
	if len(out.Failures) > 0 {
		slog.Warn("pipeline task launch failures", "storeId", storeID, "failures", out.Failures)
	}
}

func (s *Service) buildRunTaskInput(storeID string, extraEnv []types.KeyValuePair) *ecs.RunTaskInput {
	return &ecs.RunTaskInput{
		Cluster:              aws.String(s.config.Task.ClusterName),
		TaskDefinition:       aws.String(s.config.Task.TaskDefinition),
		LaunchType:           types.LaunchTypeFargate,
		NetworkConfiguration: s.networkConfig(),
		Overrides:            s.containerOverrides(extraEnv),
		PropagateTags:        types.PropagateTagsTaskDefinition,
		StartedBy:            aws.String(infra.StartedByValue),
		Group:                aws.String(infra.StoreGroupPrefix + storeID),
		Tags:                 storeTags(storeID),
	}
}

func (s *Service) networkConfig() *types.NetworkConfiguration {
	return &types.NetworkConfiguration{
		AwsvpcConfiguration: &types.AwsVpcConfiguration{
			Subnets:        s.config.Network.SubnetIDs,
			SecurityGroups: []string{s.config.Network.SecurityGroupID},
			AssignPublicIp: types.AssignPublicIpEnabled,
		},
	}
}

func (s *Service) containerOverrides(envVars []types.KeyValuePair) *types.TaskOverride {
	return &types.TaskOverride{
		ContainerOverrides: []types.ContainerOverride{
			{Name: aws.String(infra.PipelineContainerName), Environment: envVars},
		},
	}
}

func storeTags(storeID string) []types.Tag {
	return []types.Tag{
		{Key: aws.String(infra.StoreIDTagKey), Value: aws.String(storeID)},
	}
}

// GetStatus returns the current Hydrogen pipeline status by inspecting the ECS task
// state and querying the container runtime for build progress.
func (s *Service) GetStatus(ctx context.Context, storeID string) (*HydrogenStatusResponse, error) {
	taskArn, err := s.findTask(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	if taskArn == "" {
		return s.checkStoppedTask(ctx, storeID)
	}
	task, err := s.describeTask(ctx, taskArn)
	if err != nil {
		return nil, err
	}
	return s.mapTaskStatus(ctx, *task)
}

func (s *Service) checkStoppedTask(ctx context.Context, storeID string) (*HydrogenStatusResponse, error) {
	taskArn, err := s.findTaskIncludingStopped(ctx, storeID)
	if err != nil {
		slog.Warn("find stopped task", "error", err)
		return &HydrogenStatusResponse{Status: HydrogenStatusIdle}, nil
	}
	if taskArn == "" {
		return &HydrogenStatusResponse{Status: HydrogenStatusIdle}, nil
	}
	task, err := s.describeTask(ctx, taskArn)
	if err != nil {
		return nil, err
	}
	return s.mapTaskStatus(ctx, *task)
}

func (s *Service) mapTaskStatus(ctx context.Context, task types.Task) (*HydrogenStatusResponse, error) {
	switch aws.ToString(task.LastStatus) {
	case "PROVISIONING", "PENDING", "ACTIVATING":
		return &HydrogenStatusResponse{Status: HydrogenStatusProvisioning}, nil
	case "RUNNING":
		return s.queryPipelineRuntime(ctx, task)
	case infra.EcsStatusStopped:
		return hydrogenStoppedStatus(task), nil
	default:
		return &HydrogenStatusResponse{Status: HydrogenStatusProvisioning}, nil
	}
}

func hydrogenStoppedStatus(task types.Task) *HydrogenStatusResponse {
	reason := aws.ToString(task.StoppedReason)
	if reason != "" {
		slog.Warn("pipeline task stopped", "reason", reason)
		return &HydrogenStatusResponse{Status: HydrogenStatusError, Message: "Pipeline stopped unexpectedly"}
	}
	return &HydrogenStatusResponse{Status: HydrogenStatusIdle}
}

func (s *Service) queryPipelineRuntime(ctx context.Context, task types.Task) (*HydrogenStatusResponse, error) {
	rs, err := s.fetchRuntimeStatus(ctx, task)
	if err != nil {
		return &HydrogenStatusResponse{Status: HydrogenStatusProvisioning}, nil
	}
	status := resolveHydrogenStatus(rs, task)
	if status == HydrogenStatusServing && s.previewRoute != nil {
		s.previewRoute.SetupPreviewRoute(ctx, task)
	}
	return buildHydrogenResponse(status, rs, task), nil
}

func (s *Service) fetchRuntimeStatus(ctx context.Context, task types.Task) (*infra.RuntimeStatus, error) {
	ip, err := infra.ResolvePublicIP(ctx, s.clients, task)
	if err != nil {
		slog.Warn("resolvePublicIP for pipeline", "error", err)
		return nil, err
	}
	rs, err := s.runtime.GetStatus(ctx, ip)
	if err != nil {
		slog.Warn("pipeline runtime status", "error", err)
		return nil, err
	}
	return rs, nil
}

func resolveHydrogenStatus(rs *infra.RuntimeStatus, task types.Task) HydrogenStatus {
	status := rs.Status
	if status == "" {
		status = rs.State
	}
	slog.Info("pipeline runtime status", "status", rs.Status, "state", rs.State, "taskArn", aws.ToString(task.TaskArn))
	if status == "" {
		return HydrogenStatusProvisioning
	}
	if status == "running" {
		return HydrogenStatusServing
	}
	if status == "error" {
		slog.Error("pipeline runtime error", "error", rs.Error)
	}
	return HydrogenStatus(status)
}

func buildHydrogenResponse(status HydrogenStatus, rs *infra.RuntimeStatus, task types.Task) *HydrogenStatusResponse {
	previewURL := resolvePreviewURL(status, rs, task)
	message := ""
	if status == HydrogenStatusError {
		if rs.Error != "" {
			slog.Error("pipeline runtime error", "taskArn", aws.ToString(task.TaskArn), "error", rs.Error)
		}
		message = "build failed"
	}
	return &HydrogenStatusResponse{
		Status:     status,
		PreviewURL: previewURL,
		Message:    message,
	}
}

func resolvePreviewURL(status HydrogenStatus, rs *infra.RuntimeStatus, task types.Task) string {
	if rs.PreviewURL != "" {
		return rs.PreviewURL
	}
	if status != HydrogenStatusServing {
		return ""
	}
	storeID := infra.ExtractStoreIDFromTags(task.Tags)
	if storeID == "" {
		return ""
	}
	return fmt.Sprintf("https://%s", infra.FormatPreviewDomain(storeID))
}
