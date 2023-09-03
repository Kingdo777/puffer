#!/bin/bash

# get current script path
current_script_path="$(dirname "$0" | xargs realpath)"

echo Killing firecracker-containerd and firecracker
sudo pkill -9 firecracker-containerd
sudo pkill -9 firecracker

echo Removing devmapper devices
for de in $(sudo dmsetup ls| cut -f1|grep "fc-dev-thinpool-snap"); do sudo dmsetup remove "$de" && echo - Removed "$de"; done
sudo dmsetup remove fc-dev-thinpool &&  echo - Removed fc-dev-thinpool
sudo rm -rf /var/lib/firecracker-containerd/snapshotter/devmapper/*

echo "Cleaning /run/firecracker-containerd /var/lib/firecracker-containerd"
sudo rm -rf /var/lib/firecracker-containerd/containerd/
sudo rm -rf /var/lib/firecracker-containerd/shim-base
sudo rm -rf /var/lib/firecracker-containerd/snapshotter
sudo rm -rf /run/firecracker-containerd

echo Cleaning /run/containerd
sudo systemctl stop containerd
# this is very mont, otherwise will get a Bug: `panic: protobuf tag not enough fields in Status.state`
# I break a point at `/home/kingdo/go/pkg/mod/github.com/containerd/ttrpc@v1.1.2/client.go:378` to debug, and then find it!
sudo rm -rf /run/containerd

echo Recreating devmapper devices
"$current_script_path"/create_devmapper.sh > /dev/null 2>&1

