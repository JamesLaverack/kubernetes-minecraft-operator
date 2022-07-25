#!/usr/bin/env bash

set -euxo pipefail

script_dir=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
root_dir="${script_dir}/.."
local_bin_dir="${root_dir}/bin"

PATH="${PATH}:${local_bin_dir}" go test ./...
