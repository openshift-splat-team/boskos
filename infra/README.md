# AWS Janitor Infrastructure

AWS CDK (Go) infrastructure for deploying the aws-janitor as a scheduled Fargate task.

## What It Does

Deploys infrastructure to AWS that runs `aws-janitor` every 6 hours to clean up AWS resources.

**Features:**
- Serverless execution on AWS Fargate
- Stores state in S3 to track resource age
- Uses specific git SHA-tagged images (`sha-abc1234`)
- **Dry-run enabled by default** 
- All infrastructure tagged with `preserve` to prevent self-deletion

## Quick Start

**Important Note:** these instructions are to run the CDK locally. This is not needed normally as the infrastructure is deployed automatically via GitHub Actions.

Install AWS CDK and Golang. Then:

```bash
cd infra

# To Run tests on infrastructure code
go test -v ./...

# To Preview changes
cdk diff

# To Preview the cloudformation template (will be stored in infra/cdk.out)
cdk synth

# To Deploy janitor stack (do not do it unless you know what you are doing! GitHub actions take care of the deployment automatically so there should be no need of executing this manually)
cdk deploy

# To Destroy janitor stack (careful, this tears down the infrastructure, do not do it unless you know what you are doing!)
cdk destroy
```

## Configuration

All settings via environment variables set at CDK build time (see [infra.go](./infra.go) for defaults):

| Variable | Default | Description |
|----------|---------|-------------|
| `JANITOR_IMAGE_URI` | `quay.io/ocp-splat/boskos:latest` | Container image. Note that github actions override this to use the specific built tag for better deployment stability. |
| `JANITOR_CLEAN_REGION` | `us-east-2` | Region to clean (use `all` to clean all regions) |
| `JANITOR_SCHEDULE` | `rate(6 hours)` | How often to run |
| `JANITOR_TTL` | `24h` | Resource age before deletion |
| `JANITOR_DRY_RUN` | `true` | Dry-run mode |
| `JANITOR_EXCLUDE_TAGS` | `preserve` | Tags to exclude |
| `JANITOR_STATE_BUCKET_NAME` | `splat-team-janitor-state` | S3 bucket |

**Feature flags:**
- `JANITOR_ENABLE_S3_CLEAN` - Enable S3 cleanup (default: false)
- `JANITOR_ENABLE_DNS_ZONE_CLEAN` - Enable DNS cleanup (default: false)
- `JANITOR_SKIP_IAM_CLEAN` - Skip IAM cleanup (default: true)
- `JANITOR_ENABLE_VPC_ENDPOINTS_CLEAN` - Enable VPC endpoints (default: false)
- `JANITOR_ENABLE_KEY_PAIRS_CLEAN` - Enable key pairs (default: false)
- `JANITOR_ENABLE_TARGET_GROUP_CLEAN` - Enable target groups (default: false)

## CI/CD

Automatically deploys via GitHub Actions when container image builds.

**Workflow**: [deploy-janitor-infra.yaml](../.github/workflows/deploy-janitor-infra.yaml)

**Process**: Build → Test → Diff → Deploy

**Image tag**: Automatically uses `sha-XXXXXXX` (first 7 chars of git SHA)

## Safety

### Cannot Delete Itself

1. All resources tagged with `preserve`
2. Janitor excludes `preserve` tags by default
3. CloudFormation propagates tags to all resources

### Dry-Run by Default

Identifies and logs resources but no actual deletions

**To enable deletion:** Set the `JANITOR_DRY_RUN` env variable to `false` before running `cdk deploy` inside the `deploy-janitor-infra.yaml` GitHub workflow.

## Monitoring

## Monitoring

Check the Janitor execution logs in Cloudwatch.

**Check schedule:**
```bash
aws events describe-rule --name AwsJanitorStack-JanitorScheduleRule*
```

## Troubleshooting

### Bootstrap Required

First-time setup per account/region, to setup CDK resources:
```bash
cdk bootstrap aws://ACCOUNT-ID/REGION-NAME
```

### Image Not Found

Ensure build workflow completed:
```bash
docker pull quay.io/ocp-splat/boskos:sha-abc1234
```

## Resources Cleaned

- **Compute**: EC2, Auto Scaling, Launch Templates
- **Networking**: VPCs, Subnets, Security Groups, NAT Gateways, Elastic IPs, DHCP Options
- **Storage**: EBS, S3 (optional), EFS
- **Load Balancers**: ALB, NLB, Target Groups
- **Containers**: EKS, ECR
- **DNS**: Route53 (optional)
- **Messaging**: SQS
- **IaC**: CloudFormation
- **Identity**: IAM (unless skipped)
- **Keys**: EC2 Key Pairs (optional), VPC Endpoints (optional)

Resources with `preserve` tag are always excluded.