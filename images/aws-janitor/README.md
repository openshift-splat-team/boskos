# AWS Janitor Container Image

This directory contains the Dockerfile for building the AWS Janitor container image.

## Pre-Built Images

Pre-built images are available at:

```
quay.io/ocp-splat/boskos:latest          # Latest from main/master branch
quay.io/ocp-splat/boskos:v<YYYYMMDD>-<sha>  # Dated releases
quay.io/ocp-splat/boskos:pr-<number>     # Pull request builds
quay.io/ocp-splat/boskos:sha-<commit>    # Specific commits
```

## Features

- Built on Debian Bookworm Slim for minimal size while maintaining compatibility
- Includes AWS CLI v2 for debugging and potential future enhancements
- Multi-architecture support (linux/amd64)
- Dry-run mode enabled by default for safety

## Building

### Using Make (recommended)

Build the image using the project's standard build system:

```bash
# Set required environment variables
export DOCKER_REPO=gcr.io/your-project
export DOCKER_TAG=v$(date -u '+%Y%m%d')-$(git describe --tags --always --dirty)

# Build the image (requires docker with buildx for multi-arch)
make aws-janitor-image
```

### Local build with convenience script

For local testing:

```bash
# Builds localhost/aws-janitor:latest by default
./images/aws-janitor/build-local.sh

# Or specify custom repository
DOCKER_REPO=quay.io/myorg/boskos ./images/aws-janitor/build-local.sh
```

### Manual build with Podman/Docker

```bash
# Using podman (single arch)
podman build \
  --build-arg "DOCKER_TAG=test" \
  --build-arg "go_version=1.23.4" \
  --build-arg "cmd=aws-janitor" \
  -t localhost/aws-janitor:test \
  -f ./images/aws-janitor/Dockerfile .

# Using docker (single arch)
docker build \
  --build-arg "DOCKER_TAG=test" \
  --build-arg "go_version=1.23.4" \
  --build-arg "cmd=aws-janitor" \
  -t localhost/aws-janitor:test \
  -f ./images/aws-janitor/Dockerfile .
```

## Usage

### Safety Feature: Dry-Run by Default

**The container runs in `--dry-run` mode by default** to prevent accidental resource deletion. This means it will only log what would be deleted without actually deleting anything.

To **disable dry-run** and actually delete resources, set `ENABLE_DRY_RUN=false`:

```bash
docker run -it --rm \
  -e ENABLE_DRY_RUN=false \
  -e AWS_ACCESS_KEY_ID=your-key \
  -e AWS_SECRET_ACCESS_KEY=your-secret \
  quay.io/ocp-splat/boskos:latest \
  --path s3://your-bucket/janitor-state.json \
  --region us-east-1 \
  --ttl=24h \
  --skip-iam-clean
```

### Basic Usage (Dry-Run Mode)

```bash
# Default: runs in dry-run mode (safe)
docker run -it --rm \
  -e AWS_ACCESS_KEY_ID=your-key \
  -e AWS_SECRET_ACCESS_KEY=your-secret \
  quay.io/ocp-splat/boskos:latest \
  --path s3://your-bucket/janitor-state.json \
  --region us-east-1 \
  --ttl=24h
```

### With AWS Credentials File

```bash
docker run -it --rm \
  -v ~/.aws:/root/.aws:ro \
  quay.io/ocp-splat/boskos:latest \
  --dry-run \
  --path s3://your-bucket/janitor-state.json \
  --region us-east-1
```

### Kubernetes CronJob Example

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: aws-janitor
  namespace: janitor
spec:
  schedule: "0 */6 * * *"  # Run every 6 hours
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: aws-janitor
          restartPolicy: OnFailure
          containers:
          - name: aws-janitor
            image: quay.io/ocp-splat/boskos:latest
            args:
            - --path=s3://cleanup-state/janitor.json
            - --region=us-east-1
            - --ttl=72h
            - --skip-iam-clean
            - --include-tags=temporary=true
            - --exclude-tags=permanent
            - --log-level=info
            env:
            - name: AWS_REGION
              value: us-east-1
            # IMPORTANT: Set to false to actually delete resources
            # Remove or set to true for dry-run mode
            - name: ENABLE_DRY_RUN
              value: "false"
            # AWS credentials should be provided via IAM roles for service accounts (IRSA)
            # or by mounting a secret
            resources:
              requests:
                memory: "256Mi"
                cpu: "100m"
              limits:
                memory: "512Mi"
                cpu: "500m"
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ENABLE_DRY_RUN` | `true` | When `true`, runs in dry-run mode (safe). Set to `false` to actually delete resources. |
| `AWS_REGION` | - | AWS region to use (can also be set via `--region` flag) |
| `AWS_ACCESS_KEY_ID` | - | AWS access key (prefer IAM roles instead) |
| `AWS_SECRET_ACCESS_KEY` | - | AWS secret key (prefer IAM roles instead) |

## Security Considerations

1. **Dry-Run by Default**: The container runs in `--dry-run` mode by default. You must explicitly set `ENABLE_DRY_RUN=false` to delete resources
2. **Preserve Tag**: Resources tagged with `preserve` are automatically excluded from cleanup as a safety mechanism
3. **Skip IAM by Default**: Consider using `--skip-iam-clean` in shared AWS accounts to prevent accidental deletion of IAM roles/policies
4. **Credentials**: Use IAM roles when running in AWS (EKS, EC2) rather than static credentials
5. **Least Privilege**: Grant only necessary IAM permissions for the resources you want to clean

## Image Size

The image is approximately 200-300MB due to:
- Go binary: ~90MB
- AWS CLI v2: ~100MB
- Debian base: minimal

## Debugging

The AWS CLI is available in the container for debugging:

```bash
# Check AWS credentials
docker run -it --rm \
  -v ~/.aws:/root/.aws:ro \
  quay.io/ocp-splat/boskos:latest \
  /bin/bash -c "aws sts get-caller-identity"

# List EC2 instances
docker run -it --rm \
  -v ~/.aws:/root/.aws:ro \
  quay.io/ocp-splat/boskos:latest \
  /bin/bash -c "aws ec2 describe-instances --region us-east-1"

# Interactive shell
docker run -it --rm \
  -v ~/.aws:/root/.aws:ro \
  --entrypoint /bin/bash \
  quay.io/ocp-splat/boskos:latest
```
