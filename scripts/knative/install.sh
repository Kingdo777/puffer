#!/bin/bash

current_script_path="$(dirname "$0" | xargs realpath)"

function wait_for_pods_running() {
  local namespace="$1"
  echo -n -e "\e[32mWaiting All pods in namespace '$namespace' are in Running state...\e[0m"
  while true; do
    running_count=$(kubectl get pods -n "$namespace" | grep -v "NAME" | awk '{print $3}' | grep -c Running)
    total_count=$(kubectl get pods -n "$namespace" | grep -v "NAME" | wc -l)
    if [ "$running_count" -eq "$total_count" ]; then
      echo -e "\e[32mDone\e[0m"
      break
    else
      sleep 1
      echo -n -e "\e[32m.\e[0m"
    fi
  done
}

function wait_for_all_pods_ready() {
  local namespace="$1"
  echo -n -e "\e[32mWaiting All pods in namespace '$namespace' are ready...\e[0m"
  while true; do
    pod_statuses=$(kubectl get pods -n "$namespace" -o=jsonpath='{range .items[*]}{.metadata.name}:{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}')
    all_ready=$(echo "$pod_statuses" | awk -F: '$2 != "True" {print "false"; exit} END {print "true"}')
    if [ "$all_ready" = "true" ]; then
      echo -e "\e[32mDone\e[0m"
      break
    else
      sleep 1
      echo -n -e "\e[32m.\e[0m"
    fi
  done
}

# Install the Knative CLI https://knative.dev/docs/client/install-kn/#install-the-knative-cli
if ! command -v kn >/dev/null 2>&1; then
  https_proxy=http://ip:port wget -q --show-progress https://github.com/knative/client/releases/download/knative-v1.11.0/kn-linux-amd64
  chmod +x kn-linux-amd64
  sudo mv kn-linux-amd64 /usr/local/bin/kn
fi

# Install the Knative Serving component https://knative.dev/docs/install/yaml-install/serving/install-serving-with-yaml/#installing-knative-serving-using-yaml-files
kubectl apply -f "${current_script_path}"/config/serving-crds.yaml >/dev/null 2>&1
kubectl apply -f "${current_script_path}"/config/serving-core.yaml >/dev/null 2>&1
sleep 5
wait_for_pods_running "knative-serving"
wait_for_all_pods_ready "knative-serving"

# Install a networking layer - Kourier https://knative.dev/docs/install/yaml-install/serving/install-serving-with-yaml/#install-a-networking-layer
kubectl apply -f "${current_script_path}"/config/kourier.yaml >/dev/null 2>&1
wait_for_pods_running "kourier-system"
wait_for_all_pods_ready "kourier-system"
kubectl patch configmap/config-network \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}' >/dev/null 2>&1

# Configure DNS - Magic DNS https://knative.dev/docs/install/yaml-install/serving/install-serving-with-yaml/#configure-dns
kubectl apply -f "${current_script_path}"/config/serving-default-domain.yaml >/dev/null 2>&1

# Print the External IP address
kubectl --namespace kourier-system get service kourier
