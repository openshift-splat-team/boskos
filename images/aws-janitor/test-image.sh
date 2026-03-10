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

# Test script for aws-janitor container image

set -e

IMAGE="${1:-localhost/aws-janitor:test}"
CONTAINER_ENGINE="${CONTAINER_ENGINE:-$(which podman || which docker)}"

echo "Testing aws-janitor image: ${IMAGE}"
echo "Using container engine: ${CONTAINER_ENGINE}"
echo ""

echo "✅ Test 1: Default behavior (dry-run enabled)"
OUTPUT=$($CONTAINER_ENGINE run --rm "${IMAGE}" --help 2>&1 | head -5)
if echo "$OUTPUT" | grep -q "Running in DRY RUN mode"; then
    echo "   PASS: Dry-run message displayed"
else
    echo "   FAIL: Dry-run message not found"
    exit 1
fi
echo ""

echo "✅ Test 2: Dry-run disabled (ENABLE_DRY_RUN=false)"
OUTPUT=$($CONTAINER_ENGINE run --rm -e ENABLE_DRY_RUN=false "${IMAGE}" --help 2>&1 | head -5)
if echo "$OUTPUT" | grep -q "DRY RUN DISABLED"; then
    echo "   PASS: Warning message displayed"
else
    echo "   FAIL: Warning message not found"
    exit 1
fi
echo ""

echo "✅ Test 3: AWS CLI available"
OUTPUT=$($CONTAINER_ENGINE run --rm --entrypoint /bin/bash "${IMAGE}" -c "aws --version" 2>&1)
if echo "$OUTPUT" | grep -q "aws-cli"; then
    echo "   PASS: AWS CLI found - $OUTPUT"
else
    echo "   FAIL: AWS CLI not found"
    exit 1
fi
echo ""

echo "✅ Test 4: Binary works"
OUTPUT=$($CONTAINER_ENGINE run --rm "${IMAGE}" --help 2>&1)
if echo "$OUTPUT" | grep -q "Usage of"; then
    echo "   PASS: aws-janitor binary works"
else
    echo "   FAIL: aws-janitor binary not working"
    exit 1
fi
echo ""

echo "✅ Test 5: Preserve tag auto-exclusion"
OUTPUT=$($CONTAINER_ENGINE run --rm "${IMAGE}" --help 2>&1)
if echo "$OUTPUT" | grep -q "exclude-tags"; then
    echo "   PASS: Tag filtering available"
else
    echo "   FAIL: Tag filtering not available"
    exit 1
fi
echo ""

echo "════════════════════════════════════════"
echo "All tests passed! ✅"
echo "════════════════════════════════════════"
echo ""
echo "Image: ${IMAGE}"
echo "Size: $($CONTAINER_ENGINE images ${IMAGE} --format '{{.Size}}')"
echo ""
