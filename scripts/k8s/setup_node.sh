#!/bin/bash

echo Checking Memory, CPU...
total_memory_gb=$(free -g | awk '/Mem:/{print $2}')
core_count=$(nproc)
if [ "$total_memory_gb" -ge 2 ] && [ "$core_count" -ge 2 ]; then
  echo "Memory and CPU are sufficient"
else
  echo "Memory and CPU are insufficient"
  exit
fi
echo

# Disable swap
echo Disabling swap...
sudo swapoff -a
sudo sh -c "sed -i '/\sswap\s/s/^/#/' /etc/fstab"
echo Done
echo

# Install Container Runtime
echo Installing Container Runtime...

echo - Forwarding IPv4 and letting iptables see bridged traffic
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf > /dev/null 2>&1
overlay
br_netfilter
EOF
sudo modprobe overlay
sudo modprobe br_netfilter
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf > /dev/null 2>&1
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF
sudo sysctl --system > /dev/null 2>&1

echo - Installing Containerd
sudo apt-get update > /dev/null 2>&1
sudo apt-get install ca-certificates curl gnupg > /dev/null 2>&1
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null 2>&1
sudo apt-get update > /dev/null 2>&1
sudo apt-get install containerd.io > /dev/null 2>&1

echo - Installing CNI
wget https://github.com/containernetworking/plugins/releases/download/v1.3.0/cni-plugins-linux-amd64-v1.3.0.tgz > /dev/null 2>&1
sudo mkdir -p /opt/cni/bin
sudo tar Cxzvf /opt/cni/bin cni-plugins-linux-amd64-v1.3.0.tgz > /dev/null 2>&1
rm cni-plugins-linux-amd64-v1.3.0.tgz

echo - Making default containerd config
sudo sh -c "containerd config default > /etc/containerd/config.toml"

echo - Configuring systemd cgroup driver
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
sudo systemctl restart containerd > /dev/null 2>&1

echo "- Overriding the sandbox (pause) image"
sudo sed -i 's#registry\.k8s\.io/pause:[^"]*#registry.aliyuncs.com/google_containers/pause:3.9#' /etc/containerd/config.toml
sudo systemctl restart containerd > /dev/null 2>&1
echo Done
echo

# Install kubeadm, kubelet and kubectl
echo Installing kubeadm, kubelet and kubectl...
K8S_VERSION=1.28.1-00
sudo apt-get update > /dev/null 2>&1
sudo apt-get install -y apt-transport-https ca-certificates curl > /dev/null 2>&1
curl -fsSL https://dl.k8s.io/apt/doc/apt-key.gpg | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-archive-keyring.gpg > /dev/null 2>&1
echo \
  "deb [signed-by=/etc/apt/keyrings/kubernetes-archive-keyring.gpg] \
  https://apt.kubernetes.io/ kubernetes-xenial main" | \
  sudo tee /etc/apt/sources.list.d/kubernetes.list > /dev/null 2>&1
sudo apt-get update > /dev/null 2>&1
sudo apt-get install -y kubeadm=$K8S_VERSION kubectl=$K8S_VERSION kubelet=$K8S_VERSION > /dev/null 2>&1
sudo apt-mark hold kubelet kubeadm kubectl > /dev/null 2>&1
echo Done
echo