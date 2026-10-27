package infra

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ExtractENIID finds the Elastic Network Interface ID from an ECS task's attachments.
func ExtractENIID(task types.Task) (string, error) {
	for _, att := range task.Attachments {
		if aws.ToString(att.Type) != "ElasticNetworkInterface" {
			continue
		}
		if id := findENIDetail(att.Details); id != "" {
			return id, nil
		}
	}
	return "", ErrNoENI
}

func findENIDetail(details []types.KeyValuePair) string {
	for _, d := range details {
		if aws.ToString(d.Name) == "networkInterfaceId" {
			return aws.ToString(d.Value)
		}
	}
	return ""
}

// DescribeENIInput builds an EC2 DescribeNetworkInterfaces request for a single ENI.
func DescribeENIInput(eniID string) *ec2.DescribeNetworkInterfacesInput {
	return &ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: []string{eniID},
	}
}

// ResolvePublicIP extracts the ENI from an ECS task and looks up its public IP address.
func ResolvePublicIP(ctx context.Context, clients AWSClients, task types.Task) (string, error) {
	eniID, err := ExtractENIID(task)
	if err != nil {
		return "", err
	}
	return lookupPublicIP(ctx, clients.EC2, eniID)
}

func lookupPublicIP(ctx context.Context, ec2Client *ec2.Client, eniID string) (string, error) {
	out, err := ec2Client.DescribeNetworkInterfaces(ctx, DescribeENIInput(eniID))
	if err != nil {
		return "", fmt.Errorf("describe ENI: %w", err)
	}
	if len(out.NetworkInterfaces) == 0 || out.NetworkInterfaces[0].Association == nil {
		return "", fmt.Errorf("%w: %s", ErrNoPublicIP, eniID)
	}
	return aws.ToString(out.NetworkInterfaces[0].Association.PublicIp), nil
}

// ResolvePrivateIP extracts the ENI from an ECS task and looks up its private IP address.
func ResolvePrivateIP(ctx context.Context, clients AWSClients, task types.Task) (string, error) {
	eniID, err := ExtractENIID(task)
	if err != nil {
		return "", err
	}
	return lookupPrivateIP(ctx, clients.EC2, eniID)
}

func lookupPrivateIP(ctx context.Context, ec2Client *ec2.Client, eniID string) (string, error) {
	out, err := ec2Client.DescribeNetworkInterfaces(ctx, DescribeENIInput(eniID))
	if err != nil {
		return "", fmt.Errorf("describe ENI: %w", err)
	}
	if len(out.NetworkInterfaces) == 0 {
		return "", fmt.Errorf("%w: %s", ErrNoPrivateIP, eniID)
	}
	return aws.ToString(out.NetworkInterfaces[0].PrivateIpAddress), nil
}
