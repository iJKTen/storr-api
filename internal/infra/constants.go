package infra

import "time"

const (
	// Ports
	RuntimePort     = 8080
	PreviewPort     = 9292
	HealthCheckPort = "8080"

	// Paths
	HealthCheckPath  = "/health"
	StatusPath       = "/status"
	SelectThemePath  = "/select-theme"
	ProvideInputPath = "/provide-input"

	// ECS
	StoreGroupPrefix = "store-"
	StoreIDTagKey    = "StoreID"
	StartedByValue   = "storr-api"
	EcsStatusStopped = "STOPPED"

	// ALB
	TargetGroupPrefix  = "tg-"
	PreviewDomainSuffix = ".preview.storr.build"
	PreviewDomainFmt   = "%s.preview.storr.build"
	TargetGroupMaxLen  = 32
	PriorityModulo     = 49999
	HostHeaderField    = "host-header"

	// EFS
	EfsOwnerUID    = 1000
	EfsOwnerGID    = 1000
	EfsPermissions = "755"
	EfsStorePathFmt = "/stores/%s"
	EfsVolumeName  = "studio-storage"

	// Container names
	PipelineContainerName = "pipeline-env"
	StudioContainerName   = "studio-env"

	// Cleanup
	StoreDeletedReason = "Store deleted"

	// Timeouts
	RuntimeHTTPTimeout = 5 * time.Second
)
