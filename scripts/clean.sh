#!/bin/bash

# get current script path
current_script_path="$(dirname "$0" | xargs realpath)"
# get project root path
project_root_path="$(dirname "$current_script_path")"
# get scripts path
scripts_path="$project_root_path/scripts"

LOG_DIR=~/tmp/puffer-logs/

echo -e "\e[31mCleaning logs...\e[0m"
sudo rm -rf ${LOG_DIR}
echo "sudo rm -rf ${LOG_DIR}"

echo -e "\e[31mCleaning Knative...\e[0m"
"$scripts_path"/knative/clean.sh

echo -e "\e[31mCleaning Kubernetes...\e[0m"
"$scripts_path"/k8s/clean.sh

echo -e "\e[31mCleaning Puffer...\e[0m"
sudo pkill -9 puffer
ifconfig -a | grep _tap | cut -f1 -d":" | while read line; do sudo ip link delete "$line"; done
ifconfig -a | grep tap_ | cut -f1 -d":" | while read line; do sudo ip link delete "$line"; done
bridge -j vlan | jq -r '.[].ifname' | while read line; do sudo ip link delete "$line"; done
sudo rm -rf /run/puffer/*
sudo rm -rf /var/lib/puffer/*

echo -e "\e[31mCleaning Firecracker...\e[0m"
"$scripts_path"/firecracker/clean.sh
