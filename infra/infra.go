package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// Default settings, can be overridden via environment variables at CDK build time
const (
	DefaultImageUri                = "quay.io/ocp-splat/boskos:latest" // Default image to deploy
	DefaultSchedule                = "rate(6 hours)"                   // Run the janitor every 6 hours
	DefaultTTL                     = "24h"                             // Wait 24 hours before deleting resources
	DefaultCleanRegion             = "us-east-2"
	DefaultExcludeRegions          = ""                         // Comma-separated list of regions to exclude from cleanup
	DefaultExcludeTags             = "preserve"                 // Exclude resources with the "preserve" tag
	DefaultDryRun                  = "true"                     // By default run in dry-run mode
	DefaultStateBucket             = "splat-team-janitor-state" // Name of the S3 bucket storing the Janitor state
	DefaultSkipIAMClean            = "true"                     // Skip IAM roles cleanup by default
	DefaultEnableS3Clean           = "false"                    // Disable S3 cleanup by default
	DefaultEnableDNSClean          = "false"                    // Disable DNS Cleanup by default
	DefaultEnableVPCEndpointsClean = "false"                    // Disable VPC Endpoints cleanup by default
	DefaultEnableKeyPairsClean     = "false"                    // Disable KeyParis cleanup by default
	DefaultEnableTargetGroupClean  = "false"                    // Disable Target Groups cleanup by default
)

type AwsJanitorStackProps struct {
	awscdk.StackProps
	Config *JanitorConfig
}

type JanitorConfig struct {
	ImageUri                string
	Schedule                string
	TTL                     string
	CleanRegion             string
	ExcludeRegions          string
	EnableS3Clean           bool
	EnableDNSZoneClean      bool
	SkipIAMClean            bool
	EnableVPCEndpointsClean bool
	EnableKeyPairsClean     bool
	EnableTargetGroupClean  bool
	ExcludeTags             string
	DryRun                  bool
	StateBucketName         string
}

// NewAwsJanitorStack returns a stack containing all the infrastructure needed to run the janitor
// on a schedule
func NewAwsJanitorStack(scope constructs.Construct, id string, props *AwsJanitorStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	config := props.Config

	// S3 bucket for janitor state storage
	stateBucket := awss3.NewBucket(stack, jsii.String("JanitorStateBucket"), &awss3.BucketProps{
		BucketName:        jsii.String(config.StateBucketName),
		RemovalPolicy:     awscdk.RemovalPolicy_RETAIN,
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		EnforceSSL:        jsii.Bool(true),
	})

	// Create minimal VPC with public subnets only
	vpc := awsec2.NewVpc(stack, jsii.String("JanitorVPC"), &awsec2.VpcProps{
		MaxAzs:      jsii.Number(2),
		NatGateways: jsii.Number(0), // No NAT gateways - use public subnets, block inbound via security groups
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("Public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
				CidrMask:   jsii.Number(24),
			},
		},
	})

	// Security group for Fargate tasks - allow all outbound, no inbound
	securityGroup := awsec2.NewSecurityGroup(stack, jsii.String("JanitorTaskSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		Description:      jsii.String("Security group for aws-janitor Fargate tasks"),
		AllowAllOutbound: jsii.Bool(true),
	})

	// ECS Cluster
	cluster := awsecs.NewCluster(stack, jsii.String("JanitorCluster"), &awsecs.ClusterProps{
		ClusterName: jsii.String("aws-janitor"),
		Vpc:         vpc,
	})

	// IAM execution role
	executionRole := awsiam.NewRole(stack, jsii.String("TaskExecutionRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AmazonECSTaskExecutionRolePolicy")),
		},
	})

	// IAM task role
	taskRole := awsiam.NewRole(stack, jsii.String("TaskRole"), &awsiam.RoleProps{
		AssumedBy:   awsiam.NewServicePrincipal(jsii.String("ecs-tasks.amazonaws.com"), nil),
		Description: jsii.String("Role for aws-janitor to clean AWS resources"),
	})

	taskRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			// EC2 permissions
			jsii.String("ec2:Describe*"),
			jsii.String("ec2:DeleteDhcpOptions"),
			jsii.String("ec2:DeleteInternetGateway"),
			jsii.String("ec2:DeleteKeyPair"),
			jsii.String("ec2:DeleteLaunchTemplate"),
			jsii.String("ec2:DeleteNatGateway"),
			jsii.String("ec2:DeleteNetworkInterface"),
			jsii.String("ec2:DeleteRouteTable"),
			jsii.String("ec2:DeleteSecurityGroup"),
			jsii.String("ec2:DeleteSnapshot"),
			jsii.String("ec2:DeleteSubnet"),
			jsii.String("ec2:DeleteVolume"),
			jsii.String("ec2:DeleteVpc*"),
			jsii.String("ec2:DisassociateAddress"),
			jsii.String("ec2:ReleaseAddress"),
			jsii.String("ec2:TerminateInstances"),
			jsii.String("ec2:RevokeSecurityGroupIngress"),
			jsii.String("ec2:RevokeSecurityGroupEgress"),
			jsii.String("ec2:DetachInternetGateway"),
			jsii.String("ec2:AssociateDhcpOptions"),
			jsii.String("ec2:DisassociateRouteTable"),

			// ELB permissions
			jsii.String("elasticloadbalancing:Describe*"),
			jsii.String("elasticloadbalancing:DeleteLoadBalancer"),
			jsii.String("elasticloadbalancing:DeleteTargetGroup"),

			// Auto Scaling permissions
			jsii.String("autoscaling:Describe*"),
			jsii.String("autoscaling:DeleteAutoScalingGroup"),
			jsii.String("autoscaling:DeleteLaunchConfiguration"),
			jsii.String("autoscaling:UpdateAutoScalingGroup"),

			// IAM permissions (read-only until the janitor is tested)
			jsii.String("iam:List*"),
			jsii.String("iam:Get*"),
			// jsii.String("iam:DeleteRole"),
			// jsii.String("iam:DeleteRolePolicy"),
			// jsii.String("iam:DeleteUser"),
			// jsii.String("iam:DeleteUserPolicy"),
			// jsii.String("iam:DeleteInstanceProfile"),
			// jsii.String("iam:RemoveRoleFromInstanceProfile"),
			// jsii.String("iam:DetachRolePolicy"),
			// jsii.String("iam:DetachUserPolicy"),

			// Route53 permissions (read-only until the janitor is tested)
			jsii.String("route53:List*"),
			jsii.String("route53:Get*"),
			// jsii.String("route53:DeleteHostedZone"),
			// jsii.String("route53:ChangeResourceRecordSets"),

			// S3 permissions (read-only until the janitor is tested)
			jsii.String("s3:List*"),
			jsii.String("s3:Get*"),
			// jsii.String("s3:DeleteBucket"),
			// jsii.String("s3:DeleteObject*"),

			// ECR permissions
			jsii.String("ecr:Describe*"),
			jsii.String("ecr:List*"),
			jsii.String("ecr:BatchDeleteImage"),

			// CloudFormation permissions
			jsii.String("cloudformation:DescribeStacks"),
			jsii.String("cloudformation:ListStacks"),
			jsii.String("cloudformation:DeleteStack"),

			// EKS permissions
			jsii.String("eks:DescribeCluster"),
			jsii.String("eks:ListClusters"),
			jsii.String("eks:DeleteCluster"),

			// EFS permissions
			jsii.String("elasticfilesystem:DescribeFileSystems"),
			jsii.String("elasticfilesystem:DescribeMountTargets"),
			jsii.String("elasticfilesystem:DeleteFileSystem"),
			jsii.String("elasticfilesystem:DeleteMountTarget"),

			// SQS permissions
			jsii.String("sqs:ListQueues"),
			jsii.String("sqs:GetQueueAttributes"),
			jsii.String("sqs:DeleteQueue"),
		},
		Resources: &[]*string{jsii.String("*")},
	}))

	// Allow the task to read / write to the task state bucket
	stateBucket.GrantReadWrite(taskRole, nil)

	// Store janitor execution logs in CloudWatch for one week
	logGroup := awslogs.NewLogGroup(stack, jsii.String("JanitorLogGroup"), &awslogs.LogGroupProps{
		Retention:     awslogs.RetentionDays_ONE_WEEK,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	// Build janitor command
	command := []*string{
		jsii.String("--path"),
		jsii.String("s3://" + config.StateBucketName + "/janitor-state.json"),
		jsii.String("--ttl"),
		jsii.String(config.TTL),
		jsii.String("--exclude-tags"),
		jsii.String(config.ExcludeTags),
		jsii.String("--log-level"),
		jsii.String("info"),
	}

	// Only add --region if not "all" (omitting region means clean all regions)
	if config.CleanRegion != "all" {
		command = append(command, jsii.String("--region"), jsii.String(config.CleanRegion))
	}

	// Add --exclude-regions if specified
	if config.ExcludeRegions != "" {
		command = append(command, jsii.String("--exclude-regions"), jsii.String(config.ExcludeRegions))
	}

	if config.EnableS3Clean {
		command = append(command, jsii.String("--enable-s3-buckets-clean"))
	}
	if config.EnableDNSZoneClean {
		command = append(command, jsii.String("--enable-dns-zone-clean"))
	}
	if config.SkipIAMClean {
		command = append(command, jsii.String("--skip-iam-clean"))
	}
	if config.EnableVPCEndpointsClean {
		command = append(command, jsii.String("--enable-vpc-endpoints-clean"))
	}
	if config.EnableKeyPairsClean {
		command = append(command, jsii.String("--enable-key-pairs-clean"))
	}
	if config.EnableTargetGroupClean {
		command = append(command, jsii.String("--enable-target-group-clean"))
	}

	// Fargate Task Definition
	taskDefinition := awsecs.NewFargateTaskDefinition(stack, jsii.String("JanitorTaskDef"), &awsecs.FargateTaskDefinitionProps{
		MemoryLimitMiB: jsii.Number(2048),
		Cpu:            jsii.Number(1024),
		ExecutionRole:  executionRole,
		TaskRole:       taskRole,
	})

	// Set ENABLE_DRY_RUN environment variable for entrypoint.sh
	envVars := &map[string]*string{}
	if config.DryRun {
		(*envVars)["ENABLE_DRY_RUN"] = jsii.String("true")
	} else {
		(*envVars)["ENABLE_DRY_RUN"] = jsii.String("false")
	}

	taskDefinition.AddContainer(jsii.String("janitor"), &awsecs.ContainerDefinitionOptions{
		Image:       awsecs.ContainerImage_FromRegistry(jsii.String(config.ImageUri), nil),
		Command:     &command,
		Environment: envVars,
		Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("janitor"),
			LogGroup:     logGroup,
		}),
	})

	// EventBridge rule to run the task on a schedule
	rule := awsevents.NewRule(stack, jsii.String("JanitorScheduleRule"), &awsevents.RuleProps{
		Schedule:    awsevents.Schedule_Expression(jsii.String(config.Schedule)),
		Description: jsii.String("Trigger aws-janitor on a schedule"),
		Enabled:     jsii.Bool(true),
	})

	rule.AddTarget(awseventstargets.NewEcsTask(&awseventstargets.EcsTaskProps{
		Cluster:        cluster,
		TaskDefinition: taskDefinition,
		SubnetSelection: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PUBLIC,
		},
		SecurityGroups:  &[]awsec2.ISecurityGroup{securityGroup},
		LaunchType:      awsecs.LaunchType_FARGATE,
		PlatformVersion: awsecs.FargatePlatformVersion_LATEST,
	}))

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	config := &JanitorConfig{
		ImageUri:                getEnv("JANITOR_IMAGE_URI", DefaultImageUri),
		Schedule:                getEnv("JANITOR_SCHEDULE", DefaultSchedule),
		TTL:                     getEnv("JANITOR_TTL", DefaultTTL),
		CleanRegion:             getEnv("JANITOR_CLEAN_REGION", DefaultCleanRegion),
		ExcludeRegions:          getEnv("JANITOR_EXCLUDE_REGIONS", DefaultExcludeRegions),
		EnableS3Clean:           getEnv("JANITOR_ENABLE_S3_CLEAN", DefaultEnableS3Clean) == "true",
		EnableDNSZoneClean:      getEnv("JANITOR_ENABLE_DNS_ZONE_CLEAN", DefaultEnableDNSClean) == "true",
		SkipIAMClean:            getEnv("JANITOR_SKIP_IAM_CLEAN", DefaultSkipIAMClean) == "true",
		EnableVPCEndpointsClean: getEnv("JANITOR_ENABLE_VPC_ENDPOINTS_CLEAN", DefaultEnableVPCEndpointsClean) == "true",
		EnableKeyPairsClean:     getEnv("JANITOR_ENABLE_KEY_PAIRS_CLEAN", DefaultEnableKeyPairsClean) == "true",
		EnableTargetGroupClean:  getEnv("JANITOR_ENABLE_TARGET_GROUP_CLEAN", DefaultEnableTargetGroupClean) == "true",
		ExcludeTags:             getEnv("JANITOR_EXCLUDE_TAGS", DefaultExcludeTags),
		DryRun:                  getEnv("JANITOR_DRY_RUN", DefaultDryRun) == "true",
		StateBucketName:         getEnv("JANITOR_STATE_BUCKET_NAME", DefaultStateBucket),
	}

	NewAwsJanitorStack(app, "AwsJanitorStack", &AwsJanitorStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{},
			Tags: &map[string]*string{
				"preserve": jsii.String("janitor"),
			},
			Description: jsii.String("AWS Janitor infrastructure for automated resource cleanup"),
		},
		Config: config,
	})

	app.Synth(nil)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
