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

# Build AWS Janitor Lambda container image

set -o errexit
set -o nounset
set -o pipefail

AWS_ACCOUNT_ID="${AWS_ACCOUNT_ID:-$(aws sts get-caller-identity --query Account --output text)}"
AWS_REGION="${AWS_REGION:-us-east-1}"
DOCKER_TAG="${DOCKER_TAG:-$(date -u '+%Y%m%d')-$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')}"
GO_VERSION="${GO_VERSION:-1.23.4}"
CONTAINER_ENGINE="${CONTAINER_ENGINE:-$(which docker || which podman)}"
ECR_REPO="${ECR_REPO:-aws-janitor-lambda}"

echo "Building aws-janitor Lambda image..."
echo "  AWS Account: ${AWS_ACCOUNT_ID}"
echo "  AWS Region: ${AWS_REGION}"
echo "  ECR Repository: ${ECR_REPO}"
echo "  Tag: ${DOCKER_TAG}"
echo "  Go Version: ${GO_VERSION}"
echo "  Engine: ${CONTAINER_ENGINE}"
echo ""

cd "$(git rev-parse --show-toplevel)"

# Build the image
${CONTAINER_ENGINE} build \
  --build-arg "DOCKER_TAG=${DOCKER_TAG}" \
  --build-arg "go_version=${GO_VERSION}" \
  -t "${ECR_REPO}:${DOCKER_TAG}" \
  -t "${ECR_REPO}:latest" \
  -f "./images/aws-janitor-lambda/Dockerfile" .

echo ""
echo "Successfully built:"
echo "  ${ECR_REPO}:${DOCKER_TAG}"
echo "  ${ECR_REPO}:latest"
echo ""

# Ask to push to ECR
read -p "Push to ECR ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO}? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Logging in to ECR..."
    aws ecr get-login-password --region "${AWS_REGION}" | \
        ${CONTAINER_ENGINE} login --username AWS --password-stdin \
        "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"

    # Create ECR repository if it doesn't exist
    aws ecr describe-repositories --repository-names "${ECR_REPO}" --region "${AWS_REGION}" 2>/dev/null || \
        aws ecr create-repository --repository-name "${ECR_REPO}" --region "${AWS_REGION}"

    # Tag for ECR
    ${CONTAINER_ENGINE} tag "${ECR_REPO}:${DOCKER_TAG}" \
        "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO}:${DOCKER_TAG}"
    ${CONTAINER_ENGINE} tag "${ECR_REPO}:latest" \
        "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO}:latest"

    # Push to ECR
    echo "Pushing to ECR..."
    ${CONTAINER_ENGINE} push "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO}:${DOCKER_TAG}"
    ${CONTAINER_ENGINE} push "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO}:latest"

    echo ""
    echo "Successfully pushed to ECR:"
    echo "  ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO}:${DOCKER_TAG}"
    echo "  ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO}:latest"
fi
