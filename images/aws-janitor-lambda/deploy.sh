#!/bin/bash
# Copyright 2024 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Quick deployment script for AWS Janitor Lambda

set -o errexit
set -o nounset
set -o pipefail

STACK_NAME="${STACK_NAME:-aws-janitor}"
AWS_REGION="${AWS_REGION:-us-east-1}"
STATE_BUCKET="${STATE_BUCKET:-}"
DRY_RUN="${DRY_RUN:-true}"
SKIP_IAM="${SKIP_IAM:-true}"

echo "AWS Janitor Lambda Deployment"
echo "=============================="
echo ""

# Check if state bucket is provided
if [ -z "${STATE_BUCKET}" ]; then
    echo "Error: STATE_BUCKET environment variable is required"
    echo ""
    echo "Usage:"
    echo "  STATE_BUCKET=my-janitor-state ./deploy.sh"
    echo ""
    echo "Optional environment variables:"
    echo "  STACK_NAME    - CloudFormation stack name (default: aws-janitor)"
    echo "  AWS_REGION    - AWS region (default: us-east-1)"
    echo "  DRY_RUN       - Run in dry-run mode (default: true)"
    echo "  SKIP_IAM      - Skip IAM cleanup (default: true)"
    exit 1
fi

AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

echo "Configuration:"
echo "  Stack Name: ${STACK_NAME}"
echo "  AWS Account: ${AWS_ACCOUNT_ID}"
echo "  AWS Region: ${AWS_REGION}"
echo "  State Bucket: ${STATE_BUCKET}"
echo "  Dry Run: ${DRY_RUN}"
echo "  Skip IAM: ${SKIP_IAM}"
echo ""

# Step 1: Build and push image
echo "Step 1: Building and pushing container image..."
echo "================================================"
cd "$(git rev-parse --show-toplevel)"
export AWS_REGION
./images/aws-janitor-lambda/build.sh

IMAGE_URI="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/aws-janitor-lambda:latest"

# Step 2: Deploy CloudFormation stack
echo ""
echo "Step 2: Deploying CloudFormation stack..."
echo "=========================================="

aws cloudformation deploy \
  --stack-name "${STACK_NAME}" \
  --template-file ./images/aws-janitor-lambda/cloudformation.yaml \
  --parameter-overrides \
    ImageUri="${IMAGE_URI}" \
    StateBucket="${STATE_BUCKET}" \
    DryRun="${DRY_RUN}" \
    SkipIAMClean="${SKIP_IAM}" \
  --capabilities CAPABILITY_NAMED_IAM \
  --region "${AWS_REGION}"

echo ""
echo "Deployment complete!"
echo "===================="
echo ""
echo "Lambda Function: ${STACK_NAME}-janitor"
echo "CloudWatch Logs: /aws/lambda/${STACK_NAME}-janitor"
echo ""
echo "To view logs:"
echo "  aws logs tail /aws/lambda/${STACK_NAME}-janitor --follow"
echo ""
echo "To test the function:"
echo "  aws lambda invoke --function-name ${STACK_NAME}-janitor response.json"
echo ""
echo "To update configuration, modify parameters and re-run this script."
