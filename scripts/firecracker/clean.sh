#!/bin/bash

# get current script path
current_script_path="$(dirname "$0" | xargs realpath)"

echo Killing firecracker-containerd and firecracker
sudo pkill -9 firecracker-containerd
sudo pkill -9 firecracker

echo Removing devmapper devices
for de in $(sudo dmsetup ls| cut -f1|grep thinpool); do sudo dmsetup remove "$de" && echo - Removed "$de"; done
sudo rm -rf /var/lib/firecracker-containerd/snapshotter/devmapper/*

echo "Cleaning /run/firecracker-containerd /var/lib/firecracker-containerd"
sudo rm -rf /var/lib/firecracker-containerd/containerd/
sudo rm -rf /var/lib/firecracker-containerd/shim-base
sudo rm -rf /var/lib/firecracker-containerd/snapshotter
sudo rm -rf /run/firecracker-containerd

echo Recreating devmapper devices
"$current_script_path"/create_devmapper.sh > /dev/null 2>&1

