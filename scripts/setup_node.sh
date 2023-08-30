#!/bin/bash

set -e

# get current script path
current_script_path="$(dirname "$0" | xargs realpath)"
# get project root path
project_root_path="$(dirname "$current_script_path")"
# get scripts path
scripts_path="$project_root_path/scripts"

# function to check if a command exists
command_exists() {
    if ! command -v "$1" &> /dev/null
    then
      echo
      echo -e "\e[31mFailed: $1 could not be found.\e[0m"
      exit
    fi
}

echo -n -e "\e[34mInstalling utils ...\e[0m"
sudo apt-get update > /dev/null 2>&1
sudo apt-get install -y bridge-utils jq net-tools > /dev/null 2>&1
echo -e "\e[34mDone.\e[0m"

echo -e "\e[34mSetting up Kubernetes Environment...\e[0m"
"$scripts_path"/k8s/setup_node.sh

# Checking is kubelet, kubeadm, kubectl installed...
echo -n -e "\e[34mChecking is kubelet, kubeadm, kubectl installed...\e[0m"
command_exists kubelet
command_exists kubeadm
command_exists kubectl
echo -e "\e[34mOK.\e[0m"
echo

echo -e "\e[34mSetting up Firecracker Environment...\e[0m"
"$scripts_path"/firecracker/setup_node.sh

# Checking is firecracker-containerd installed...
echo -n -e "\e[34mChecking is firecracker-containerd, firecracker installed...\e[0m"
command_exists firecracker-containerd
command_exists firecracker
echo -e "\e[34mOK.\e[0m"

echo -n -e "\e[34mSetup Puffer Environment...\e[0m"
sudo mkdir -p /etc/puffer-cri
echo -e "\e[34mDone.\e[0m"

