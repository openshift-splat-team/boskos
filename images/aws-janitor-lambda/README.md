# AWS Janitor Lambda Deployment

This directory contains resources for deploying aws-janitor as an AWS Lambda function.

## Architecture

The Lambda function:
- Runs on a schedule (EventBridge/CloudWatch Events)
- Executes aws-janitor cleanup logic
- Stores state in S3
- Uses IAM role for AWS permissions
- Max execution time: 15 minutes (Lambda limit)

## Prerequisites

1. AWS CLI configured with appropriate credentials
2. Docker or Podman for building container images
3. An S3 bucket for storing janitor state
4. Appropriate IAM permissions to create Lambda functions, IAM roles, ECR repositories, and EventBridge rules

## Quick Start

### 1. Build and Push Container Image

```bash
# Make the build script executable
chmod +x images/aws-janitor-lambda/build.sh

# Build and push to ECR
./images/aws-janitor-lambda/build.sh
```

The script will:
- Build the Lambda container image
- Create ECR repository if needed
- Push the image to ECR

### 2. Create IAM Role for Lambda

Create an IAM role with this trust policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

Attach permissions:
- `AWSLambdaBasicExecutionRole` (managed policy for CloudWatch Logs)
- Custom policy for resource cleanup (see IAM Permissions section below)
- S3 read/write access to state bucket

```bash
# Create role
aws iam create-role \
  --role-name aws-janitor-lambda-role \
  --assume-role-policy-document file://trust-policy.json

# Attach basic execution role
aws iam attach-role-policy \
  --role-name aws-janitor-lambda-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

# Attach custom cleanup policy (create it first - see below)
aws iam attach-role-policy \
  --role-name aws-janitor-lambda-role \
  --policy-arn arn:aws:iam::YOUR_ACCOUNT:policy/AWSJanitorCleanupPolicy
```

### 3. Create Lambda Function

```bash
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
AWS_REGION="us-east-1"

aws lambda create-function \
  --function-name aws-janitor \
  --package-type Image \
  --code ImageUri=${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/aws-janitor-lambda:latest \
  --role arn:aws:iam::${AWS_ACCOUNT_ID}:role/aws-janitor-lambda-role \
  --timeout 900 \
  --memory-size 512 \
  --environment Variables="{LOG_LEVEL=info}" \
  --region ${AWS_REGION}
```

### 4. Create EventBridge Rule (Schedule)

```bash
# Create rule to run every 6 hours
aws events put-rule \
  --name aws-janitor-schedule \
  --schedule-expression "rate(6 hours)" \
  --state ENABLED \
  --region ${AWS_REGION}

# Add Lambda as target
aws events put-targets \
  --rule aws-janitor-schedule \
  --targets "Id"="1","Arn"="arn:aws:lambda:${AWS_REGION}:${AWS_ACCOUNT_ID}:function:aws-janitor","Input"='{"path":"s3://my-janitor-state/state.json","ttl":"48h","region":"us-east-1","dryRun":false,"skipIAMClean":true,"includeTags":["temporary=true"],"excludeTags":["permanent"]}'

# Grant EventBridge permission to invoke Lambda
aws lambda add-permission \
  --function-name aws-janitor \
  --statement-id AllowEventBridgeInvoke \
  --action lambda:InvokeFunction \
  --principal events.amazonaws.com \
  --source-arn arn:aws:events:${AWS_REGION}:${AWS_ACCOUNT_ID}:rule/aws-janitor-schedule
```

## Lambda Event Input

The Lambda function accepts JSON input with these fields:

```json
{
  "path": "s3://my-bucket/janitor-state.json",
  "ttl": "48h",
  "region": "us-east-1",
  "cleanAll": false,
  "dryRun": false,
  "includeTags": ["temporary=true", "team=ci"],
  "excludeTags": ["permanent", "environment=production"],
  "ttlTagKey": "janitor-ttl",
  "enableTargetGroupClean": false,
  "enableKeyPairsClean": false,
  "enableVPCEndpointsClean": false,
  "skipRoute53ManagementCheck": false,
  "enableDNSZoneClean": false,
  "enableS3BucketsClean": false,
  "skipIAMClean": true,
  "cleanEcrRepositories": ["repo1", "repo2"],
  "skipResourceRecordSetTypes": ["SOA", "NS"]
}
```

## Lambda Response

```json
{
  "message": "AWS Janitor completed successfully",
  "sweptCount": 42,
  "dryRun": false,
  "executionTime": "2m15s"
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Log level: trace, debug, info, warn, error |

## IAM Permissions

The Lambda function's IAM role needs permissions for:

### Minimum S3 Permissions (for state storage)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject"
      ],
      "Resource": "arn:aws:s3:::my-janitor-state/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket"
      ],
      "Resource": "arn:aws:s3:::my-janitor-state"
    }
  ]
}
```

### Cleanup Permissions (comprehensive example)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:Describe*",
        "ec2:DeleteSnapshot",
        "ec2:DeleteVolume",
        "ec2:TerminateInstances",
        "ec2:DeleteSecurityGroup",
        "ec2:DeleteSubnet",
        "ec2:DeleteVpc",
        "ec2:DeleteInternetGateway",
        "ec2:DetachInternetGateway",
        "ec2:DeleteRouteTable",
        "ec2:DeleteNatGateway",
        "ec2:DeleteNetworkInterface",
        "ec2:ReleaseAddress",
        "ec2:DeleteLaunchTemplate",
        "ec2:DeleteKeyPair",
        "ec2:DeleteVpcEndpoints",
        "elasticloadbalancing:Describe*",
        "elasticloadbalancing:DeleteLoadBalancer",
        "elasticloadbalancing:DeleteTargetGroup",
        "autoscaling:Describe*",
        "autoscaling:DeleteAutoScalingGroup",
        "autoscaling:DeleteLaunchConfiguration",
        "autoscaling:UpdateAutoScalingGroup",
        "eks:Describe*",
        "eks:ListClusters",
        "eks:DeleteCluster",
        "elasticfilesystem:Describe*",
        "elasticfilesystem:DeleteFileSystem",
        "sqs:ListQueues",
        "sqs:GetQueueAttributes",
        "sqs:DeleteQueue",
        "cloudformation:Describe*",
        "cloudformation:DeleteStack",
        "s3:ListAllMyBuckets",
        "s3:ListBucket",
        "s3:GetBucketTagging",
        "s3:DeleteBucket",
        "s3:DeleteObject",
        "ecr:DescribeRepositories",
        "ecr:ListImages",
        "ecr:BatchDeleteImage",
        "route53:ListHostedZones",
        "route53:ListResourceRecordSets",
        "route53:ChangeResourceRecordSets",
        "route53:DeleteHostedZone"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "iam:ListRoles",
        "iam:ListInstanceProfiles",
        "iam:ListOpenIDConnectProviders",
        "iam:GetRole",
        "iam:ListRolePolicies",
        "iam:ListAttachedRolePolicies",
        "iam:DeleteRole",
        "iam:DeleteRolePolicy",
        "iam:DetachRolePolicy",
        "iam:DeleteInstanceProfile",
        "iam:DeleteOpenIDConnectProvider"
      ],
      "Resource": "*",
      "Condition": {
        "StringEquals": {
          "aws:RequestedRegion": "us-east-1"
        }
      }
    }
  ]
}
```

**Note**: Adjust permissions based on what resources you want to clean. Use `skipIAMClean: true` if you don't want to manage IAM resources.

## Testing

### Test with dry-run

```bash
aws lambda invoke \
  --function-name aws-janitor \
  --payload '{
    "path": "s3://my-janitor-state/test.json",
    "ttl": "24h",
    "region": "us-east-1",
    "dryRun": true,
    "skipIAMClean": true
  }' \
  response.json

cat response.json
```

### Test actual cleanup

```bash
aws lambda invoke \
  --function-name aws-janitor \
  --payload '{
    "path": "s3://my-janitor-state/state.json",
    "ttl": "48h",
    "region": "us-east-1",
    "dryRun": false,
    "skipIAMClean": true,
    "includeTags": ["temporary=true"]
  }' \
  response.json
```

### View logs

```bash
aws logs tail /aws/lambda/aws-janitor --follow
```

## Limitations

1. **15-minute timeout**: Lambda has a maximum execution time of 15 minutes. For large cleanups, consider:
   - Running more frequently with shorter TTLs
   - Using Step Functions to orchestrate multiple Lambda invocations
   - Deploying to ECS/Fargate instead for longer runs

2. **Memory**: Adjust `--memory-size` based on the number of resources. Start with 512MB and increase if needed.

3. **Cold starts**: First invocation may be slower. Consider using provisioned concurrency if needed.

## Monitoring

### CloudWatch Metrics

Monitor these Lambda metrics:
- `Invocations`: Number of executions
- `Duration`: Execution time
- `Errors`: Failed executions
- `Throttles`: Rate-limited invocations

### CloudWatch Logs

Logs are automatically sent to: `/aws/lambda/aws-janitor`

Set `LOG_LEVEL=debug` environment variable for verbose logging.

### Custom Metrics

Parse the Lambda response to track:
- `sweptCount`: Number of resources cleaned
- `executionTime`: How long cleanup took

## Updating the Function

### Update code

```bash
# Build new image
./images/aws-janitor-lambda/build.sh

# Update Lambda to use new image
aws lambda update-function-code \
  --function-name aws-janitor \
  --image-uri ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/aws-janitor-lambda:latest
```

### Update configuration

```bash
# Update timeout
aws lambda update-function-configuration \
  --function-name aws-janitor \
  --timeout 900

# Update memory
aws lambda update-function-configuration \
  --function-name aws-janitor \
  --memory-size 1024

# Update environment variables
aws lambda update-function-configuration \
  --function-name aws-janitor \
  --environment Variables="{LOG_LEVEL=debug}"
```

## Troubleshooting

### Lambda times out

- Increase timeout (max 900 seconds / 15 minutes)
- Run more frequently with shorter TTL
- Reduce scope (specific region, fewer resource types)

### Permission errors

- Check IAM role has necessary permissions
- Review CloudWatch Logs for specific permission errors
- Add missing permissions to IAM policy

### State file errors

- Verify S3 bucket exists and Lambda has access
- Check bucket name in `path` parameter
- Ensure state bucket is tagged with exclude tags if using `enableS3BucketsClean`

## Cost Optimization

1. **Execution frequency**: Run only as often as needed (e.g., every 6-12 hours)
2. **Memory allocation**: Start with 512MB, only increase if needed
3. **Region scope**: Limit to specific regions to reduce API calls
4. **Resource filtering**: Use tags to limit scope

## Example Configurations

### Conservative (Dry-run)

```json
{
  "path": "s3://my-janitor-state/state.json",
  "ttl": "72h",
  "region": "us-east-1",
  "dryRun": true,
  "skipIAMClean": true,
  "includeTags": ["temporary=true"],
  "excludeTags": ["permanent", "environment=production"]
}
```

### Aggressive Cleanup

```json
{
  "path": "s3://my-janitor-state/state.json",
  "ttl": "24h",
  "dryRun": false,
  "skipIAMClean": true,
  "includeTags": ["team=ci"],
  "excludeTags": ["permanent"],
  "enableS3BucketsClean": true,
  "enableKeyPairsClean": true,
  "enableVPCEndpointsClean": true
}
```

### Region-Specific

```json
{
  "path": "s3://my-janitor-state/us-west-2.json",
  "ttl": "48h",
  "region": "us-west-2",
  "dryRun": false,
  "skipIAMClean": true,
  "includeTags": ["environment=staging"]
}
```

## See Also

- [AWS Janitor Main Documentation](../../aws-janitor/README.md)
- [AWS Lambda Documentation](https://docs.aws.amazon.com/lambda/)
- [EventBridge Scheduling](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-create-rule-schedule.html)
