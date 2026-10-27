package infra

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

// AWSClients groups the AWS SDK service clients shared across infrastructure operations.
type AWSClients struct {
	ECS   *ecs.Client
	EC2   *ec2.Client
	ELBV2 *elasticloadbalancingv2.Client
	EFS   *efs.Client
}
