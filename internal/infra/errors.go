package infra

import "errors"

var (
	// ErrTaskNotFound is returned when a described ECS task ARN yields no results.
	ErrTaskNotFound = errors.New("task not found")
	// ErrNoENI is returned when no Elastic Network Interface attachment is found on an ECS task.
	ErrNoENI = errors.New("no ENI found for task")
	// ErrNoPublicIP is returned when the task's ENI has no associated public IP address.
	ErrNoPublicIP = errors.New("no public IP for ENI")
	// ErrNoPrivateIP is returned when the task's ENI has no private IP address.
	ErrNoPrivateIP = errors.New("no private IP for ENI")
	// ErrNoStoreIDTag is returned when the StoreID tag is missing from an ECS task.
	ErrNoStoreIDTag = errors.New("no StoreID tag found on task")
)
