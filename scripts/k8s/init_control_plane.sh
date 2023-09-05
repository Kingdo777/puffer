#!/bin/bash

current_script_path="$(dirname "$0" | xargs realpath)"

CRI_SOCK=$1

if [ -z "$CRI_SOCK" ]; then
  CRI_SOCK="/run/puffer/puffer.sock"
fi

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

# Waiting all nodes are ready
function wait_for_all_node_ready() {
  echo -n -e "\e[32mWaiting all nodes are ready...\e[0m"
  while true; do
    node_info=$(kubectl get nodes -o=jsonpath='{range .items[*]}{.metadata.name}:{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}')
    all_ready=$(echo "$node_info" | awk -F: '$2 != "True" {print "false"; exit} END {print "true"}')
    if [ "$all_ready" = "true" ]; then
      echo -e "\e[32mDone\e[0m"
      break
    else
      sleep 1
      echo -n -e "\e[32m.\e[0m"
    fi
  done
}

# Initialize the control-plane node https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/#initializing-your-control-plane-node
echo -n Initializing the control-plane node...
sudo kubeadm config images pull --image-repository registry.aliyuncs.com/google_containers >/dev/null
sudo kubeadm init --cri-socket="unix://${CRI_SOCK}" \
  --pod-network-cidr=192.168.0.0/16 \
  --image-repository registry.aliyuncs.com/google_containers \
  >/dev/null
mkdir -p "$HOME"/.kube
sudo cp -i /etc/kubernetes/admin.conf "$HOME"/.kube/config
sudo chown "$(id -u):$(id -g)" "$HOME"/.kube/config
# let root user use kubectl
sudo mkdir -p /root/.kube
sudo cp -i /etc/kubernetes/admin.conf /root/.kube/config
sudo chown "$(id -u):$(id -g)" /root/.kube/config
echo Done

## Install Calico  https://docs.tigera.io/calico/latest/getting-started/kubernetes/quickstart#install-calico
#echo Installing Calico...
#kubectl create -f "$current_script_path"/config/calico/tigera-operator.yaml
#kubectl create -f "$current_script_path"/config/calico/custom-resources.yaml
## https://docs.tigera.io/calico/latest/operations/calicoctl/install#install-calicoctl-as-a-binary-on-a-single-host
#if ! command -v calicoctl >/dev/null 2>&1; then
#    echo Installing Calicoctl...
#    https_proxy=http://ip:port wget -q --show-progress https://github.com/projectcalico/calico/releases/latest/download/calicoctl-linux-amd64 -O calicoctl
#    chmod +x calicoctl
#    sudo mv calicoctl /usr/local/bin
#fi
#echo Done
#echo

# Install Flannel https://github.com/flannel-io/flannel#deploying-flannel-with-kubectl
echo -n Installing Flannel...
kubectl apply -f "$current_script_path"/config/flannel/kube-flannel.yml >/dev/null 2>&1
echo Done

# Control plane node isolation
echo -n Control plane node isolation...
kubectl taint nodes --all node-role.kubernetes.io/control-plane- >/dev/null 2>&1
kubectl taint nodes --all node-role.kubernetes.io/master- >/dev/null 2>&1
echo Done

sleep 5
wait_for_pods_running kube-system
wait_for_all_pods_ready kube-system
sleep 3
wait_for_pods_running kube-flannel
wait_for_all_pods_ready kube-flannel
wait_for_all_node_ready

# Install MetalLB https://metallb.universe.tf/installation/
echo Installing MetalLB
echo -n - Preparing configmap... # https://metallb.universe.tf/installation/#preparation
kubectl get configmap kube-proxy -n kube-system -o yaml |
  sed -e "s/strictARP: false/strictARP: true/" |
  kubectl apply -f - -n kube-system >/dev/null 2>&1
echo Done

echo -n - Installing MetalLB by Manifest... # https://metallb.universe.tf/installation/#installation-by-manifest
kubectl apply -f "$current_script_path"/config/metallb/metallb-native.yaml >/dev/null 2>&1
echo Done

sleep 3
wait_for_pods_running metallb-system
wait_for_all_pods_ready metallb-system
echo -n - Defining The IPs To Assign To The Load Balancer Services... # https://metallb.universe.tf/configuration/#defining-the-ips-to-assign-to-the-load-balancer-services
kubectl apply -f "$current_script_path"/config/metallb/first-pool.yaml >/dev/null 2>&1
echo Done

echo -n - "Announce The Service IPs (Layer 2 Configuration)..." # https://metallb.universe.tf/configuration/#layer-2-configuration
kubectl apply -f "$current_script_path"/config/metallb/layer2-config.yaml >/dev/null 2>&1
echo Done
echo

# Test networking, pod-to-pod communication https://docs.tigera.io/calico/latest/getting-started/kubernetes/hardway/test-networking#pod-to-pod-pings
#kubectl create deployment pingtest --image=busybox --replicas=2 -- sleep infinity
#kubectl get pods --selector=app=pingtest --output=wide
#kubectl exec -ti pingtest-b4b6f8cf-b5z78 -- sh
#ping 192.168.45.193 -c 4
