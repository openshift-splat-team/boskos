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

# Entrypoint script for aws-janitor container
# Enables --dry-run by default for safety, but can be disabled via ENABLE_DRY_RUN=false

set -e

# Default to dry-run mode for safety
DRY_RUN_ARG="--dry-run"

# Check if dry-run is explicitly disabled
if [ "${ENABLE_DRY_RUN:-true}" = "false" ]; then
    echo "⚠️  DRY RUN DISABLED - aws-janitor will DELETE resources!" >&2
    DRY_RUN_ARG=""
elif [ "${ENABLE_DRY_RUN:-true}" = "true" ]; then
    echo "ℹ️  Running in DRY RUN mode (set ENABLE_DRY_RUN=false to disable)" >&2
fi

# Check if --dry-run is already in the arguments
DRY_RUN_IN_ARGS=false
for arg in "$@"; do
    if [ "$arg" = "--dry-run" ] || [ "$arg" = "-dry-run" ]; then
        DRY_RUN_IN_ARGS=true
        break
    fi
done

# Build final command
if [ -n "$DRY_RUN_ARG" ] && [ "$DRY_RUN_IN_ARGS" = "false" ]; then
    # Add --dry-run if not already present and not disabled
    exec /bin/aws-janitor "$DRY_RUN_ARG" "$@"
else
    # Run as-is if --dry-run already in args or explicitly disabled
    exec /bin/aws-janitor "$@"
fi
