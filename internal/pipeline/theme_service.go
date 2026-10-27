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

// ThemeService manages the lifecycle of Shopify theme pipeline ECS tasks, including
// launching, status polling, theme selection, and user input forwarding.
type ThemeService struct {
	taskResolver
	runtime      *infra.RuntimeClient
	previewRoute *infra.PreviewRouteManager
	config       ThemeConfig
}

// NewThemeService creates a ThemeService with the given AWS clients, runtime poller,
// preview route manager, and theme-specific task configuration.
func NewThemeService(clients infra.AWSClients, runtime *infra.RuntimeClient, previewRoute *infra.PreviewRouteManager, config ThemeConfig) *ThemeService {
	return &ThemeService{
		taskResolver: taskResolver{clients: clients, cluster: config.ClusterName},
		runtime:      runtime,
		previewRoute: previewRoute,
		config:       config,
	}
}

// Provision starts a new theme pipeline ECS task for the given store, or returns the
// current status if a task is already running.
func (s *ThemeService) Provision(ctx context.Context, storeID, shopifyDomain string, envOverrides []types.KeyValuePair) (*ThemeStatusResponse, error) {
	taskArn, err := s.findTask(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	if taskArn != "" {
		return s.GetStatus(ctx, storeID)
	}
	return s.launchTask(ctx, storeID, shopifyDomain, envOverrides)
}

func (s *ThemeService) launchTask(ctx context.Context, storeID, shopifyDomain string, envOverrides []types.KeyValuePair) (*ThemeStatusResponse, error) {
	input := s.buildRunTaskInput(storeID, shopifyDomain, envOverrides)
	if _, err := s.clients.ECS.RunTask(ctx, input); err != nil {
		return nil, fmt.Errorf("run task: %w", err)
	}
	return &ThemeStatusResponse{Status: ThemeStatusProvisioning}, nil
}

func (s *ThemeService) buildRunTaskInput(storeID, shopifyDomain string, envOverrides []types.KeyValuePair) *ecs.RunTaskInput {
	envVars := s.buildEnvVars(storeID, shopifyDomain, envOverrides)
	return &ecs.RunTaskInput{
		Cluster:              aws.String(s.config.ClusterName),
		TaskDefinition:       aws.String(s.config.TaskDefinition),
		LaunchType:           types.LaunchTypeFargate,
		NetworkConfiguration: s.networkConfig(),
		Overrides: &types.TaskOverride{
			ContainerOverrides: []types.ContainerOverride{
				{Name: aws.String(s.config.ContainerName), Environment: envVars},
			},
		},
		PropagateTags: types.PropagateTagsTaskDefinition,
		StartedBy:     aws.String(infra.StartedByValue),
		Group:         aws.String(infra.StoreGroupPrefix + storeID),
		Tags:          storeTags(storeID),
	}
}

func (s *ThemeService) buildEnvVars(storeID, shopifyDomain string, extra []types.KeyValuePair) []types.KeyValuePair {
	envVars := []types.KeyValuePair{
		{Name: aws.String("STORE_ID"), Value: aws.String(storeID)},
		{Name: aws.String("SHOPIFY_URL"), Value: aws.String(shopifyDomain)},
	}
	return append(envVars, extra...)
}

func (s *ThemeService) networkConfig() *types.NetworkConfiguration {
	return &types.NetworkConfiguration{
		AwsvpcConfiguration: &types.AwsVpcConfiguration{
			Subnets:        s.config.Network.SubnetIDs,
			SecurityGroups: []string{s.config.Network.SecurityGroupID},
			AssignPublicIp: types.AssignPublicIpEnabled,
		},
	}
}

// GetStatus returns the current theme pipeline status by inspecting the ECS task
// state and querying the container runtime for progress.
func (s *ThemeService) GetStatus(ctx context.Context, storeID string) (*ThemeStatusResponse, error) {
	taskArn, err := s.findTask(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	if taskArn == "" {
		return &ThemeStatusResponse{Status: ThemeStatusIdle}, nil
	}
	task, err := s.describeTask(ctx, taskArn)
	if err != nil {
		return nil, err
	}
	return s.mapTaskStatus(ctx, *task)
}

func (s *ThemeService) mapTaskStatus(ctx context.Context, task types.Task) (*ThemeStatusResponse, error) {
	switch aws.ToString(task.LastStatus) {
	case "PROVISIONING", "PENDING", "ACTIVATING":
		return &ThemeStatusResponse{Status: ThemeStatusProvisioning}, nil
	case "RUNNING":
		return s.queryRuntime(ctx, task)
	case infra.EcsStatusStopped:
		return themeStoppedStatus(task), nil
	default:
		return &ThemeStatusResponse{Status: ThemeStatusProvisioning}, nil
	}
}

func themeStoppedStatus(task types.Task) *ThemeStatusResponse {
	reason := aws.ToString(task.StoppedReason)
	if reason != "" {
		slog.Warn("theme pipeline task stopped", "reason", reason)
		return &ThemeStatusResponse{Status: ThemeStatusError}
	}
	return &ThemeStatusResponse{Status: ThemeStatusIdle}
}

func (s *ThemeService) queryRuntime(ctx context.Context, task types.Task) (*ThemeStatusResponse, error) {
	ip, err := infra.ResolvePublicIP(ctx, s.clients, task)
	if err != nil {
		slog.Warn("resolvePublicIP for theme", "error", err)
		return &ThemeStatusResponse{Status: ThemeStatusProvisioning}, nil
	}
	rs, err := s.runtime.GetStatus(ctx, ip)
	if err != nil {
		slog.Warn("theme runtime status", "error", err)
		return &ThemeStatusResponse{Status: ThemeStatusProvisioning}, nil
	}
	resp := buildThemeResponse(rs)
	if resp.Status == ThemeStatusRunning && s.previewRoute != nil {
		s.previewRoute.SetupPreviewRoute(ctx, task)
	}
	return resp, nil
}

func buildThemeResponse(rs *infra.RuntimeStatus) *ThemeStatusResponse {
	themes := make([]ThemeInfo, len(rs.Themes))
	for i, t := range rs.Themes {
		themes[i] = ThemeInfo{ID: t.ID, Name: t.Name, Role: t.Role}
	}
	return &ThemeStatusResponse{
		Status:           ThemeStatus(rs.Status),
		PreviewURL:       rs.PreviewURL,
		AuthURL:          rs.AuthURL,
		Themes:           themes,
		ExistingThemeIDs: rs.ExistingThemeIDs,
	}
}

// SelectTheme forwards a theme selection to the running theme pipeline container
// by resolving the task's IP and calling the runtime API.
func (s *ThemeService) SelectTheme(ctx context.Context, storeID string, sel ThemeSelection) error {
	ip, err := s.resolveTaskIP(ctx, storeID)
	if err != nil {
		return err
	}
	return s.runtime.SelectTheme(ctx, ip, infra.ThemeSelection{
		ThemeID:     sel.ThemeID,
		UseExisting: sel.UseExisting,
	})
}

// ProvideInput sends user-provided input (e.g. a store password) to the running
// theme pipeline container via its runtime API.
func (s *ThemeService) ProvideInput(ctx context.Context, storeID, input string) error {
	ip, err := s.resolveTaskIP(ctx, storeID)
	if err != nil {
		return err
	}
	return s.runtime.ProvideInput(ctx, ip, input)
}

func (s *ThemeService) resolveTaskIP(ctx context.Context, storeID string) (string, error) {
	taskArn, err := s.findTask(ctx, storeID)
	if err != nil {
		return "", fmt.Errorf("find task: %w", err)
	}
	if taskArn == "" {
		return "", fmt.Errorf("no running task for store %s", storeID)
	}
	task, err := s.describeTask(ctx, taskArn)
	if err != nil {
		return "", err
	}
	return infra.ResolvePublicIP(ctx, s.clients, *task)
}
