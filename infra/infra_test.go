package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

func TestAwsJanitorStack(t *testing.T) {
	t.Run("creates stack with default configuration", func(t *testing.T) {
		app := awscdk.NewApp(nil)

		config := &JanitorConfig{
			ImageUri:        DefaultImageUri,
			Schedule:        DefaultSchedule,
			TTL:             DefaultTTL,
			CleanRegion:     DefaultCleanRegion,
			ExcludeTags:     DefaultExcludeTags,
			DryRun:          true,
			StateBucketName: DefaultStateBucket,
		}

		stack := NewAwsJanitorStack(app, "TestStack", &AwsJanitorStackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("us-east-1"),
				},
			},
			Config: config,
		})

		template := assertions.Template_FromStack(stack, nil)

		// Verify S3 bucket is created with correct properties
		template.HasResourceProperties(jsii.String("AWS::S3::Bucket"), map[string]interface{}{
			"BucketName": DefaultStateBucket,
			"BucketEncryption": map[string]interface{}{
				"ServerSideEncryptionConfiguration": []interface{}{
					map[string]interface{}{
						"ServerSideEncryptionByDefault": map[string]interface{}{
							"SSEAlgorithm": "AES256",
						},
					},
				},
			},
			"PublicAccessBlockConfiguration": map[string]interface{}{
				"BlockPublicAcls":       true,
				"BlockPublicPolicy":     true,
				"IgnorePublicAcls":      true,
				"RestrictPublicBuckets": true,
			},
		})

		// Verify VPC is created
		template.ResourceCountIs(jsii.String("AWS::EC2::VPC"), jsii.Number(1))

		// Verify Security Group blocks inbound
		template.HasResourceProperties(jsii.String("AWS::EC2::SecurityGroup"), map[string]interface{}{
			"GroupDescription": "Security group for aws-janitor Fargate tasks",
		})

		// Verify ECS cluster is created
		template.HasResourceProperties(jsii.String("AWS::ECS::Cluster"), map[string]interface{}{
			"ClusterName": "aws-janitor",
		})

		// Verify Fargate task definition
		template.HasResourceProperties(jsii.String("AWS::ECS::TaskDefinition"), map[string]interface{}{
			"RequiresCompatibilities": []interface{}{"FARGATE"},
			"Cpu":                     "1024",
			"Memory":                  "2048",
			"NetworkMode":             "awsvpc",
		})

		// Verify EventBridge rule
		template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
			"ScheduleExpression": DefaultSchedule,
			"State":              "ENABLED",
		})

		// Verify CloudWatch log group
		template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
			"RetentionInDays": 7,
		})
	})

	t.Run("security group blocks all inbound traffic", func(t *testing.T) {
		app := awscdk.NewApp(nil)

		config := &JanitorConfig{
			ImageUri:        DefaultImageUri,
			Schedule:        DefaultSchedule,
			TTL:             DefaultTTL,
			CleanRegion:     DefaultCleanRegion,
			ExcludeTags:     DefaultExcludeTags,
			DryRun:          true,
			StateBucketName: DefaultStateBucket,
		}

		stack := NewAwsJanitorStack(app, "TestStack", &AwsJanitorStackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("us-east-1"),
				},
			},
			Config: config,
		})

		template := assertions.Template_FromStack(stack, nil)

		// Security group should allow all outbound
		template.HasResourceProperties(jsii.String("AWS::EC2::SecurityGroup"), map[string]interface{}{
			"GroupDescription": "Security group for aws-janitor Fargate tasks",
			"SecurityGroupEgress": []interface{}{
				map[string]interface{}{
					"CidrIp":      "0.0.0.0/0",
					"IpProtocol":  "-1",
					"Description": "Allow all outbound traffic by default",
				},
			},
		})

		// Should not have any ingress rules
		template.HasResourceProperties(jsii.String("AWS::EC2::SecurityGroup"), map[string]interface{}{
			"SecurityGroupIngress": assertions.Match_Absent(),
		})
	})

	t.Run("creates IAM roles with correct permissions", func(t *testing.T) {
		app := awscdk.NewApp(nil)

		config := &JanitorConfig{
			ImageUri:        DefaultImageUri,
			Schedule:        DefaultSchedule,
			TTL:             DefaultTTL,
			CleanRegion:     DefaultCleanRegion,
			ExcludeTags:     DefaultExcludeTags,
			DryRun:          true,
			StateBucketName: "test-bucket",
		}

		stack := NewAwsJanitorStack(app, "TestStack", &AwsJanitorStackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("us-east-1"),
				},
			},
			Config: config,
		})

		template := assertions.Template_FromStack(stack, nil)

		// Task execution role should exist
		template.HasResourceProperties(jsii.String("AWS::IAM::Role"), map[string]interface{}{
			"AssumeRolePolicyDocument": map[string]interface{}{
				"Statement": []interface{}{
					map[string]interface{}{
						"Action": "sts:AssumeRole",
						"Effect": "Allow",
						"Principal": map[string]interface{}{
							"Service": "ecs-tasks.amazonaws.com",
						},
					},
				},
			},
		})

		// Task role should have broad permissions for cleaning
		template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]interface{}{
			"PolicyDocument": map[string]interface{}{
				"Statement": assertions.Match_ArrayWith(&[]interface{}{
					map[string]interface{}{
						"Action": assertions.Match_ArrayWith(&[]interface{}{
							"ec2:Describe*",
							"ec2:DeleteSecurityGroup",
							"ec2:TerminateInstances",
						}),
						"Effect":   "Allow",
						"Resource": "*",
					},
				}),
			},
		})
	})

	t.Run("configures container with correct command arguments", func(t *testing.T) {
		app := awscdk.NewApp(nil)

		config := &JanitorConfig{
			ImageUri:                "quay.io/test/janitor:v1",
			Schedule:                "rate(1 hour)",
			TTL:                     "12h",
			CleanRegion:             "us-west-2",
			ExcludeTags:             "preserve,important",
			DryRun:                  true,
			EnableS3Clean:           true,
			EnableDNSZoneClean:      true,
			SkipIAMClean:            true,
			EnableVPCEndpointsClean: true,
			EnableKeyPairsClean:     true,
			EnableTargetGroupClean:  true,
			StateBucketName:         "my-state-bucket",
		}

		stack := NewAwsJanitorStack(app, "TestStack", &AwsJanitorStackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("us-east-1"),
				},
			},
			Config: config,
		})

		template := assertions.Template_FromStack(stack, nil)

		// Verify task definition has correct container configuration
		template.HasResourceProperties(jsii.String("AWS::ECS::TaskDefinition"), map[string]interface{}{
			"ContainerDefinitions": []interface{}{
				map[string]interface{}{
					"Image": "quay.io/test/janitor:v1",
					"Command": assertions.Match_ArrayWith(&[]interface{}{
						"--path",
						"s3://my-state-bucket/janitor-state.json",
						"--ttl",
						"12h",
						"--exclude-tags",
						"preserve,important",
						"--log-level",
						"info",
						"--region",
						"us-west-2",
						"--enable-s3-buckets-clean",
						"--enable-dns-zone-clean",
						"--skip-iam-clean",
						"--enable-vpc-endpoints-clean",
						"--enable-key-pairs-clean",
						"--enable-target-group-clean",
						"--dry-run",
					}),
				},
			},
		})
	})

	t.Run("dry-run flag is optional in command", func(t *testing.T) {
		app := awscdk.NewApp(nil)

		config := &JanitorConfig{
			ImageUri:        DefaultImageUri,
			Schedule:        DefaultSchedule,
			TTL:             DefaultTTL,
			CleanRegion:     DefaultCleanRegion,
			ExcludeTags:     DefaultExcludeTags,
			DryRun:          false, // Disabled
			StateBucketName: DefaultStateBucket,
		}

		stack := NewAwsJanitorStack(app, "TestStack", &AwsJanitorStackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("us-east-1"),
				},
			},
			Config: config,
		})

		template := assertions.Template_FromStack(stack, nil)

		// Command should contain required flags but NOT --dry-run
		template.HasResourceProperties(jsii.String("AWS::ECS::TaskDefinition"), map[string]interface{}{
			"ContainerDefinitions": []interface{}{
				map[string]interface{}{
					"Command": assertions.Match_ArrayWith(&[]interface{}{
						"--path",
						"--ttl",
						"--exclude-tags",
						"--log-level",
						"--region",
					}),
				},
			},
		})
	})

	t.Run("region flag is omitted when set to 'all'", func(t *testing.T) {
		app := awscdk.NewApp(nil)

		config := &JanitorConfig{
			ImageUri:        DefaultImageUri,
			Schedule:        DefaultSchedule,
			TTL:             DefaultTTL,
			CleanRegion:     "all", // Clean all regions
			ExcludeTags:     DefaultExcludeTags,
			DryRun:          true,
			StateBucketName: DefaultStateBucket,
		}

		stack := NewAwsJanitorStack(app, "TestStack", &AwsJanitorStackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("us-east-1"),
				},
			},
			Config: config,
		})

		template := assertions.Template_FromStack(stack, nil)

		// Command should NOT contain --region flag
		template.HasResourceProperties(jsii.String("AWS::ECS::TaskDefinition"), map[string]interface{}{
			"ContainerDefinitions": []interface{}{
				map[string]interface{}{
					"Command": assertions.Match_ArrayWith(&[]interface{}{
						"--path",
						"--ttl",
						"--exclude-tags",
						"--log-level",
						"--dry-run",
					}),
				},
			},
		})
	})

	t.Run("creates correct number of resources", func(t *testing.T) {
		app := awscdk.NewApp(nil)

		config := &JanitorConfig{
			ImageUri:        DefaultImageUri,
			Schedule:        DefaultSchedule,
			TTL:             DefaultTTL,
			CleanRegion:     DefaultCleanRegion,
			ExcludeTags:     DefaultExcludeTags,
			DryRun:          true,
			StateBucketName: DefaultStateBucket,
		}

		stack := NewAwsJanitorStack(app, "TestStack", &AwsJanitorStackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("us-east-1"),
				},
			},
			Config: config,
		})

		template := assertions.Template_FromStack(stack, nil)

		// Count key resources
		template.ResourceCountIs(jsii.String("AWS::S3::Bucket"), jsii.Number(1))
		template.ResourceCountIs(jsii.String("AWS::EC2::VPC"), jsii.Number(1))
		template.ResourceCountIs(jsii.String("AWS::EC2::SecurityGroup"), jsii.Number(1))
		template.ResourceCountIs(jsii.String("AWS::ECS::Cluster"), jsii.Number(1))
		template.ResourceCountIs(jsii.String("AWS::ECS::TaskDefinition"), jsii.Number(1))
		template.ResourceCountIs(jsii.String("AWS::Events::Rule"), jsii.Number(1))
		template.ResourceCountIs(jsii.String("AWS::Logs::LogGroup"), jsii.Number(1))
		template.ResourceCountIs(jsii.String("AWS::IAM::Role"), jsii.Number(3)) // Execution + Task + EventBridge role
	})

	t.Run("defaults are applied correctly from constants", func(t *testing.T) {
		if DefaultCleanRegion != "us-east-2" {
			t.Errorf("Expected DefaultCleanRegion to be us-east-2, got %s", DefaultCleanRegion)
		}
		if DefaultSchedule != "rate(6 hours)" {
			t.Errorf("Expected DefaultSchedule to be rate(6 hours), got %s", DefaultSchedule)
		}
		if DefaultDryRun != "true" {
			t.Errorf("Expected DefaultDryRun to be true, got %s", DefaultDryRun)
		}
		if DefaultSkipIAMClean != "true" {
			t.Errorf("Expected DefaultSkipIAMClean to be true, got %s", DefaultSkipIAMClean)
		}
		if DefaultStateBucket != "splat-team-janitor-state" {
			t.Errorf("Expected DefaultStateBucket to be splat-team-janitor-state, got %s", DefaultStateBucket)
		}
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("returns default when env var not set", func(t *testing.T) {
		result := getEnv("NONEXISTENT_VAR_12345", "default-value")
		if result != "default-value" {
			t.Errorf("Expected 'default-value', got '%s'", result)
		}
	})

	t.Run("returns env var when set", func(t *testing.T) {
		t.Setenv("TEST_VAR", "test-value")
		result := getEnv("TEST_VAR", "default-value")
		if result != "test-value" {
			t.Errorf("Expected 'test-value', got '%s'", result)
		}
	})
}
