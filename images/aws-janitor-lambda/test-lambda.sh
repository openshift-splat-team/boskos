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

# Test AWS Janitor Lambda function

set -o errexit
set -o nounset
set -o pipefail

FUNCTION_NAME="${1:-aws-janitor-janitor}"
STATE_BUCKET="${STATE_BUCKET:-my-janitor-state}"
AWS_REGION="${AWS_REGION:-us-east-1}"

echo "Testing Lambda function: ${FUNCTION_NAME}"
echo "State bucket: ${STATE_BUCKET}"
echo ""

# Test event
PAYLOAD=$(cat <<EOF
{
  "path": "s3://${STATE_BUCKET}/test.json",
  "ttl": "24h",
  "region": "${AWS_REGION}",
  "dryRun": true,
  "skipIAMClean": true,
  "includeTags": [],
  "excludeTags": ["permanent"]
}
EOF
)

echo "Invoking Lambda with dry-run=true..."
echo ""

aws lambda invoke \
  --function-name "${FUNCTION_NAME}" \
  --payload "${PAYLOAD}" \
  --cli-binary-format raw-in-base64-out \
  response.json

echo ""
echo "Response:"
cat response.json | jq .

echo ""
echo "Recent logs:"
aws logs tail "/aws/lambda/${FUNCTION_NAME}" --since 5m --format short

rm -f response.json
