package infra

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// PreviewRouteConfig holds the ALB listener ARN and VPC ID needed for preview route management.
type PreviewRouteConfig struct {
	ListenerArn string
	VpcID       string
}

// PreviewRouteManager creates and manages ALB target groups and listener rules that route
// preview subdomain traffic to running pipeline containers.
type PreviewRouteManager struct {
	elbv2       *elasticloadbalancingv2.Client
	awsClients  AWSClients
	listenerArn string
	vpcID       string
}

// NewPreviewRouteManager creates a PreviewRouteManager with the given AWS clients and ALB configuration.
func NewPreviewRouteManager(clients AWSClients, cfg PreviewRouteConfig) *PreviewRouteManager {
	return &PreviewRouteManager{
		elbv2:       clients.ELBV2,
		awsClients:  clients,
		listenerArn: cfg.ListenerArn,
		vpcID:       cfg.VpcID,
	}
}

// SetupPreviewRoute ensures an ALB target group and listener rule exist for the store's
// preview subdomain, pointing to the pipeline container's private IP.
func (m *PreviewRouteManager) SetupPreviewRoute(ctx context.Context, task types.Task) {
	storeID := ExtractStoreIDFromTags(task.Tags)
	if storeID == "" {
		slog.Warn("setupPreviewRoute: no StoreID tag on task", "taskArn", aws.ToString(task.TaskArn))
		return
	}
	privateIP, err := ResolvePrivateIP(ctx, m.awsClients, task)
	if err != nil {
		slog.Error("resolvePrivateIP failed", "storeId", storeID, "error", err)
		return
	}
	if err := m.reconcileRoute(ctx, storeID, privateIP); err != nil {
		slog.Error("reconcilePreviewRoute failed", "storeId", storeID, "error", err)
	}
}

func (m *PreviewRouteManager) reconcileRoute(ctx context.Context, storeID, privateIP string) error {
	tgArn, err := m.ensureTargetGroup(ctx, storeID)
	if err != nil {
		return fmt.Errorf("ensureTargetGroup: %w", err)
	}
	m.registerTarget(ctx, tgArn, privateIP)
	return m.ensureListenerRule(ctx, storeID, tgArn)
}

func (m *PreviewRouteManager) ensureTargetGroup(ctx context.Context, storeID string) (string, error) {
	tgName := formatTargetGroupName(storeID)
	if arn, err := m.findTargetGroup(ctx, tgName); arn != "" || err != nil {
		return arn, err
	}
	return m.createTargetGroup(ctx, tgName)
}

func (m *PreviewRouteManager) findTargetGroup(ctx context.Context, tgName string) (string, error) {
	out, err := m.elbv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		Names: []string{tgName},
	})
	if err == nil && len(out.TargetGroups) > 0 {
		return aws.ToString(out.TargetGroups[0].TargetGroupArn), nil
	}
	if err != nil && !strings.Contains(err.Error(), "TargetGroupNotFound") {
		return "", err
	}
	return "", nil
}

func (m *PreviewRouteManager) createTargetGroup(ctx context.Context, tgName string) (string, error) {
	out, err := m.elbv2.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:            aws.String(tgName),
		Protocol:        elbv2types.ProtocolEnumHttp,
		Port:            aws.Int32(PreviewPort),
		VpcId:           aws.String(m.vpcID),
		TargetType:      elbv2types.TargetTypeEnumIp,
		HealthCheckPath: aws.String(HealthCheckPath),
		HealthCheckPort: aws.String(HealthCheckPort),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.TargetGroups[0].TargetGroupArn), nil
}

func (m *PreviewRouteManager) registerTarget(ctx context.Context, tgArn, privateIP string) {
	_, err := m.elbv2.RegisterTargets(ctx, &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: aws.String(tgArn),
		Targets: []elbv2types.TargetDescription{
			{Id: aws.String(privateIP), Port: aws.Int32(PreviewPort)},
		},
	})
	if err != nil {
		slog.Warn("failed to register target", "privateIP", privateIP, "targetGroupArn", tgArn, "error", err)
	}
}

func (m *PreviewRouteManager) ensureListenerRule(ctx context.Context, storeID, tgArn string) error {
	hostHeader := formatHostHeader(storeID)
	exists, err := m.findListenerRule(ctx, hostHeader)
	if err != nil || exists {
		return err
	}
	return m.createListenerRule(ctx, hostHeader, tgArn)
}

func (m *PreviewRouteManager) findListenerRule(ctx context.Context, hostHeader string) (bool, error) {
	out, err := m.elbv2.DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: aws.String(m.listenerArn),
	})
	if err != nil {
		return false, err
	}
	return matchesHostRule(out.Rules, hostHeader), nil
}

func (m *PreviewRouteManager) createListenerRule(ctx context.Context, hostHeader, tgArn string) error {
	priority, err := m.nextAvailablePriority(ctx)
	if err != nil {
		priority = int32(time.Now().UnixNano()%PriorityModulo) + 1
	}
	_, err = m.elbv2.CreateRule(ctx, &elasticloadbalancingv2.CreateRuleInput{
		ListenerArn: aws.String(m.listenerArn),
		Priority:    aws.Int32(priority),
		Conditions: []elbv2types.RuleCondition{
			{
				Field:            aws.String(HostHeaderField),
				HostHeaderConfig: &elbv2types.HostHeaderConditionConfig{Values: []string{hostHeader}},
			},
		},
		Actions: []elbv2types.Action{
			{Type: elbv2types.ActionTypeEnumForward, TargetGroupArn: aws.String(tgArn)},
		},
	})
	return err
}

func (m *PreviewRouteManager) nextAvailablePriority(ctx context.Context) (int32, error) {
	out, err := m.elbv2.DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: aws.String(m.listenerArn),
	})
	if err != nil {
		return 0, err
	}
	var maxPriority int32
	for _, rule := range out.Rules {
		p := aws.ToString(rule.Priority)
		if p == "default" {
			continue
		}
		var val int64
		if _, err := fmt.Sscanf(p, "%d", &val); err == nil && int32(val) > maxPriority {
			maxPriority = int32(val)
		}
	}
	return maxPriority + 1, nil
}

// DeletePreviewRoute removes the ALB listener rule and target group for a store's preview subdomain.
func (m *PreviewRouteManager) DeletePreviewRoute(ctx context.Context, storeID string) {
	m.deleteListenerRule(ctx, storeID)
	m.deleteTargetGroup(ctx, storeID)
}

func (m *PreviewRouteManager) deleteListenerRule(ctx context.Context, storeID string) {
	hostHeader := formatHostHeader(storeID)
	rulesOut, err := m.elbv2.DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: aws.String(m.listenerArn),
	})
	if err != nil {
		return
	}
	for _, rule := range rulesOut.Rules {
		if !matchesHostRuleSingle(rule, hostHeader) {
			continue
		}
		if _, err := m.elbv2.DeleteRule(ctx, &elasticloadbalancingv2.DeleteRuleInput{
			RuleArn: rule.RuleArn,
		}); err != nil {
			slog.Error("failed to delete listener rule", "storeId", storeID, "error", err)
		}
		return
	}
}

func (m *PreviewRouteManager) deleteTargetGroup(ctx context.Context, storeID string) {
	tgName := formatTargetGroupName(storeID)
	tgOut, err := m.elbv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		Names: []string{tgName},
	})
	if err != nil || len(tgOut.TargetGroups) == 0 {
		return
	}
	if _, err := m.elbv2.DeleteTargetGroup(ctx, &elasticloadbalancingv2.DeleteTargetGroupInput{
		TargetGroupArn: tgOut.TargetGroups[0].TargetGroupArn,
	}); err != nil {
		slog.Error("failed to delete target group", "storeId", storeID, "error", err)
	}
}

func formatTargetGroupName(storeID string) string {
	name := TargetGroupPrefix + storeID
	if len(name) > TargetGroupMaxLen {
		name = name[:TargetGroupMaxLen]
	}
	return name
}

func formatHostHeader(storeID string) string {
	return FormatPreviewDomain(storeID)
}

// FormatPreviewDomain returns the full preview subdomain for a store (e.g. "abc123.preview.storr.build").
func FormatPreviewDomain(storeID string) string {
	return fmt.Sprintf(PreviewDomainFmt, storeID)
}

func matchesHostRule(rules []elbv2types.Rule, hostHeader string) bool {
	for _, rule := range rules {
		if matchesHostRuleSingle(rule, hostHeader) {
			return true
		}
	}
	return false
}

func matchesHostRuleSingle(rule elbv2types.Rule, hostHeader string) bool {
	for _, condition := range rule.Conditions {
		if aws.ToString(condition.Field) != HostHeaderField {
			continue
		}
		for _, val := range condition.HostHeaderConfig.Values {
			if val == hostHeader {
				return true
			}
		}
	}
	return false
}

// ExtractStoreIDFromTags finds and returns the StoreID value from an ECS task's tag list.
func ExtractStoreIDFromTags(tags []types.Tag) string {
	for _, tag := range tags {
		if aws.ToString(tag.Key) == StoreIDTagKey {
			return aws.ToString(tag.Value)
		}
	}
	return ""
}
