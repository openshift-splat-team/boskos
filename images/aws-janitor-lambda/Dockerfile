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

# Build stage
ARG go_version=1.23.4

FROM golang:${go_version} as build
WORKDIR /go/src/app

# Cache module downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

ARG DOCKER_TAG
# Build the Lambda handler
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -o /go/src/app/_output/bin/lambda-handler \
    -ldflags "-X 'k8s.io/test-infra/prow/version.Name=aws-janitor-lambda' -X 'k8s.io/test-infra/prow/version.Version=${DOCKER_TAG}'" \
    ./cmd/aws-janitor-lambda

# Lambda runtime stage
FROM public.ecr.aws/lambda/provided:al2023

# Copy the Lambda handler
COPY --from=build /go/src/app/_output/bin/lambda-handler ${LAMBDA_TASK_ROOT}/bootstrap

# Set the CMD to the Lambda handler
CMD [ "bootstrap" ]
