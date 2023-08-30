#!/bin/bash

set -e

# get current script path
current_script_path="$(dirname "$0" | xargs realpath)"

SANDBOX=$1

"$current_script_path"/clean.sh

"$current_script_path"/start.sh "$SANDBOX"
