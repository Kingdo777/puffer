#!/bin/bash

# get current script path
current_script_path="$(dirname "$0" | xargs realpath)"
# get project root path
project_root_path="$(dirname "$current_script_path")"
# get scripts path
scripts_path="$project_root_path/scripts"

SANDBOX=$1

if [ -z "$SANDBOX" ]; then
  SANDBOX="firecracker"
fi

if [ "$SANDBOX" != "container" ] && [ "$SANDBOX" != "firecracker" ]; then
  echo Specified sanboxing technique is not supported. Possible are \"firecracker\" and \"gvisor\"
  exit 1
fi

if [ "$SANDBOX" == "container" ]; then
  CRI_SOCK="/run/containerd/containerd.sock"
else
  CRI_SOCK="/run/puffer/puffer.sock"
fi

LOG_DIR=~/tmp/puffer-logs/
sudo mkdir -p -m777 -p ${LOG_DIR}

echo -e "\e[32mRunning the stock containerd daemon...\e[0m"
sudo systemctl restart containerd

if [ "$SANDBOX" == "firecracker" ]; then
  echo -e "\e[32mRunning the firecracker-containerd daemon...\e[0m"
  sudo sh -c "firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>${LOG_DIR}/firecracker-containerd.out 2>${LOG_DIR}/firecracker-containerd.err &"
  echo -e "\e[32mBuilding puffer...\e[0m"
  pushd >/dev/null "$project_root_path" || exit
  https_proxy=http://ip:port go build -o puffer
  popd >/dev/null || exit
  echo -e "\e[32mRunning puffer...\e[0m"
  sudo sh -c "$project_root_path/puffer -dbg 1>${LOG_DIR}/puffer.out 2>${LOG_DIR}/puffer.err &"
fi

echo -e "\e[32mIniting the Kubernetes ...\e[0m"
"$scripts_path"/k8s/init_control_plane.sh "${CRI_SOCK}"

echo -e "\e[32mInstalling the Knative ...\e[0m"
"$scripts_path"/knative/install.sh

echo -e "\e[32mDone.\e[0m"
echo

echo -e "\e[32mTesting the Knative installation...\e[0m"

# Fetch the CLUSTER IP address
LOADBALANCER_IP=$(kubectl --namespace kourier-system get service kourier -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

if [ "$SANDBOX" == "container" ]; then

  # Deploying a Knative Service https://knative.dev/docs/getting-started/first-service/#deploying-a-knative-service
  kn service create hello \
    --image registry.cn-hangzhou.aliyuncs.com/kingdo_knative/helloworld-go:latest \
    --port 8080 \
    --env TARGET=World

  curl http://hello.default."${LOADBALANCER_IP}".sslip.io

elif [ "$SANDBOX" == "firecracker" ]; then
  kn service create hello \
      --image registry.cn-hangzhou.aliyuncs.com/kingdo_puffer/stub-helloworld:latest \
      --port 50051 \
      --env GUEST_PORT=50051 \
      --env GUEST_IMAGE="registry.cn-hangzhou.aliyuncs.com/kingdo_puffer/function-helloworld-python:latest"

  curl http://hello.default."${LOADBALANCER_IP}".sslip.io

fi
