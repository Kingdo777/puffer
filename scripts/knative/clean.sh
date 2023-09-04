#!/bin/bash

set -x

current_script_path="$(dirname "$0" | xargs realpath)"

# Delete Knative Serving
kn service delete --all > /dev/null 2>&1

# Uninstalling Knative https://knative.dev/docs/install/uninstall/#uninstalling-knative

# Uninstalling a networking layer
# 1. Uninstall the Knative Kourier controller by running:
#kubectl delete -f "$current_script_path"/config/kourier.yaml --ignore-not-found > /dev/null 2>&1

# Uninstalling the Serving component
# 1. Uninstall the Serving core components by running:
#kubectl delete -f "$current_script_path"/config/serving-core.yaml --ignore-not-found > /dev/null 2>&1
# 2.Uninstall the required custom resources by running:
#kubectl delete -f "$current_script_path"/config/serving-crds.yaml --ignore-not-found > /dev/null 2>&1
