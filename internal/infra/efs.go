package infra

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
)

// EfsProvisioner manages per-store EFS access points for persistent storage in pipeline containers.
type EfsProvisioner struct {
	client       *efs.Client
	fileSystemID string
}

// NewEfsProvisioner creates an EfsProvisioner for the given EFS file system.
func NewEfsProvisioner(client *efs.Client, fileSystemID string) *EfsProvisioner {
	return &EfsProvisioner{client: client, fileSystemID: fileSystemID}
}

// EnsureAccessPoint returns an existing access point for the store, or creates one if none exists.
func (p *EfsProvisioner) EnsureAccessPoint(ctx context.Context, storeID string) (string, error) {
	apID, err := p.FindAccessPoint(ctx, storeID)
	if err != nil {
		return "", err
	}
	if apID != "" {
		return apID, nil
	}
	return p.CreateAccessPoint(ctx, storeID)
}

// FindAccessPoint searches for an existing EFS access point matching the store's root directory path.
func (p *EfsProvisioner) FindAccessPoint(ctx context.Context, storeID string) (string, error) {
	out, err := p.client.DescribeAccessPoints(ctx, &efs.DescribeAccessPointsInput{
		FileSystemId: aws.String(p.fileSystemID),
	})
	if err != nil {
		return "", fmt.Errorf("describe access points: %w", err)
	}
	path := buildStoreRootDir(storeID)
	return matchAccessPointByPath(out.AccessPoints, path), nil
}

// CreateAccessPoint creates a new EFS access point with a store-specific root directory and POSIX user.
func (p *EfsProvisioner) CreateAccessPoint(ctx context.Context, storeID string) (string, error) {
	input := p.buildAccessPointInput(storeID)
	out, err := p.client.CreateAccessPoint(ctx, input)
	if err != nil {
		return "", fmt.Errorf("create access point: %w", err)
	}
	return aws.ToString(out.AccessPointId), nil
}

// DeleteAccessPoint removes the EFS access point associated with the given store during cleanup.
func (p *EfsProvisioner) DeleteAccessPoint(ctx context.Context, storeID string) {
	apID, err := p.FindAccessPoint(ctx, storeID)
	if err != nil {
		slog.Error("failed to find access point", "storeId", storeID, "error", err)
		return
	}
	if apID == "" {
		return
	}
	if _, err := p.client.DeleteAccessPoint(ctx, &efs.DeleteAccessPointInput{
		AccessPointId: aws.String(apID),
	}); err != nil {
		slog.Error("failed to delete access point", "storeId", storeID, "error", err)
	}
}

func (p *EfsProvisioner) buildAccessPointInput(storeID string) *efs.CreateAccessPointInput {
	return &efs.CreateAccessPointInput{
		FileSystemId:  aws.String(p.fileSystemID),
		RootDirectory: buildRootDirectory(storeID),
		PosixUser: &efstypes.PosixUser{
			Uid: aws.Int64(EfsOwnerUID),
			Gid: aws.Int64(EfsOwnerGID),
		},
		Tags: []efstypes.Tag{
			{Key: aws.String(StoreIDTagKey), Value: aws.String(storeID)},
			{Key: aws.String("Name"), Value: aws.String(StoreGroupPrefix + storeID)},
		},
	}
}

func buildRootDirectory(storeID string) *efstypes.RootDirectory {
	return &efstypes.RootDirectory{
		Path: aws.String(buildStoreRootDir(storeID)),
		CreationInfo: &efstypes.CreationInfo{
			OwnerUid:    aws.Int64(EfsOwnerUID),
			OwnerGid:    aws.Int64(EfsOwnerGID),
			Permissions: aws.String(EfsPermissions),
		},
	}
}

func buildStoreRootDir(storeID string) string {
	return fmt.Sprintf(EfsStorePathFmt, storeID)
}

func matchAccessPointByPath(aps []efstypes.AccessPointDescription, path string) string {
	for _, ap := range aps {
		if ap.RootDirectory != nil && aws.ToString(ap.RootDirectory.Path) == path {
			return aws.ToString(ap.AccessPointId)
		}
	}
	return ""
}
