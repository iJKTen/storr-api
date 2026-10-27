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

const describeTasksBatchSize = 100

type taskResolver struct {
	clients infra.AWSClients
	cluster string
}

func (r *taskResolver) findTask(ctx context.Context, storeID string) (string, error) {
	arns, err := r.listAllTaskArns(ctx, "")
	if err != nil {
		return "", err
	}
	if len(arns) == 0 {
		return "", nil
	}
	return r.matchStoreTask(ctx, arns, storeID)
}

func (r *taskResolver) listAllTaskArns(ctx context.Context, desiredStatus string) ([]string, error) {
	var (
		allArns   []string
		nextToken *string
	)
	for {
		input := &ecs.ListTasksInput{Cluster: aws.String(r.cluster), NextToken: nextToken}
		if desiredStatus != "" {
			input.DesiredStatus = types.DesiredStatus(desiredStatus)
		}
		out, err := r.clients.ECS.ListTasks(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("list tasks: %w", err)
		}
		allArns = append(allArns, out.TaskArns...)
		if out.NextToken == nil {
			return allArns, nil
		}
		nextToken = out.NextToken
	}
}

func (r *taskResolver) matchStoreTask(ctx context.Context, taskArns []string, storeID string) (string, error) {
	group := infra.StoreGroupPrefix + storeID
	for i := 0; i < len(taskArns); i += describeTasksBatchSize {
		end := i + describeTasksBatchSize
		if end > len(taskArns) {
			end = len(taskArns)
		}
		desc, err := r.clients.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(r.cluster),
			Tasks:   taskArns[i:end],
		})
		if err != nil {
			return "", fmt.Errorf("describe tasks: %w", err)
		}
		for _, task := range desc.Tasks {
			if aws.ToString(task.Group) == group && aws.ToString(task.LastStatus) != infra.EcsStatusStopped {
				return aws.ToString(task.TaskArn), nil
			}
		}
	}
	return "", nil
}

func (r *taskResolver) findTaskIncludingStopped(ctx context.Context, storeID string) (string, error) {
	arns, err := r.listAllTaskArns(ctx, string(types.DesiredStatusStopped))
	if err != nil {
		return "", err
	}
	if len(arns) == 0 {
		return "", nil
	}
	return r.matchStoreTaskAny(ctx, arns, storeID)
}

func (r *taskResolver) matchStoreTaskAny(ctx context.Context, taskArns []string, storeID string) (string, error) {
	group := infra.StoreGroupPrefix + storeID
	for i := 0; i < len(taskArns); i += describeTasksBatchSize {
		end := i + describeTasksBatchSize
		if end > len(taskArns) {
			end = len(taskArns)
		}
		desc, err := r.clients.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(r.cluster),
			Tasks:   taskArns[i:end],
		})
		if err != nil {
			return "", fmt.Errorf("describe tasks: %w", err)
		}
		for _, task := range desc.Tasks {
			if aws.ToString(task.Group) == group {
				return aws.ToString(task.TaskArn), nil
			}
		}
	}
	return "", nil
}

func (r *taskResolver) describeTask(ctx context.Context, taskArn string) (*types.Task, error) {
	out, err := r.clients.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(r.cluster),
		Tasks:   []string{taskArn},
		Include: []types.TaskField{types.TaskFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("describe task: %w", err)
	}
	if len(out.Tasks) == 0 {
		return nil, fmt.Errorf("%w: %s", infra.ErrTaskNotFound, taskArn)
	}
	return &out.Tasks[0], nil
}

func (r *taskResolver) StopTask(ctx context.Context, storeID string) {
	taskArn, err := r.findTask(ctx, storeID)
	if err != nil || taskArn == "" {
		return
	}
	if _, err := r.clients.ECS.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: aws.String(r.cluster),
		Task:    aws.String(taskArn),
		Reason:  aws.String(infra.StoreDeletedReason),
	}); err != nil {
		slog.Error("failed to stop task", "storeId", storeID, "error", err)
	}
}
