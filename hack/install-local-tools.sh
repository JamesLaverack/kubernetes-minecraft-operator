#!/usr/bin/env bash

set -euxo pipefail

script_dir=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
root_dir="${script_dir}/.."
local_bin_dir="${root_dir}/bin"

mkdir "${local_bin_dir}"

GOBIN="${local_bin_dir}" go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2
GOBIN="${local_bin_dir}" go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
