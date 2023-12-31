#!/bin/bash

# https://github.com/firecracker-microvm/firecracker-containerd/blob/main/docs/getting-started.md

# get core count
CORE_COUNT=$(nproc)
# get current script path
current_script_path="$(dirname "$0" | xargs realpath)"

INSTALL_FROM_SOURCE=$1
if [ -z "$INSTALL_FROM_SOURCE" ]; then
  INSTALL_FROM_SOURCE="build_from_bin"
fi

echo Installing Docker, Golang, etc.
sudo apt-get update >/dev/null 2>&1
sudo apt-get install -y make \
  docker-ce \
  golang-go \
  git \
  curl \
  e2fsprogs \
  util-linux \
  bc \
  gnupg >/dev/null 2>&1
sudo usermod -aG docker "$(whoami)"
sudo systemctl restart docker
echo Done.
echo

echo Installing dmsetup...
sudo apt-get install -y dmsetup >/dev/null 2>&1
echo Done.
echo

echo Installing firecracker-containerd...
if [ "$INSTALL_FROM_SOURCE" == "build_from_bin" ]; then
  echo - "Downloading && Installing firecracker-containerd and rootfs.img"
  https_proxy=http://ip:port wget -q --show-progress -O /tmp/firecracker-containerd-puffer-linux-amd64.tar.gz https://github.com/Kingdo777/firecracker-containerd-puffer/releases/download/v1.0.0/firecracker-containerd-puffer-linux-amd64.tar.gz
  sudo tar -C /usr/local/bin/ -xzf /tmp/firecracker-containerd-puffer-linux-amd64.tar.gz
  sudo mv /usr/local/bin/rootfs.img /var/lib/firecracker-containerd/runtime/default-rootfs.img
  echo - "Downloading && Installing firecracker"
  https_proxy=http://ip:port wget -q --show-progress -O /tmp/firecracker-x86_64-unknown-linux-musl.tar.gz https://github.com/Kingdo777/firecracker-faascale/releases/download/v1.0.0/firecracker-x86_64-unknown-linux-musl.tar.gz
  sudo tar -C /usr/local/bin/ -xzf /tmp/firecracker-x86_64-unknown-linux-musl.tar.gz
  echo -n - Checking Install...
  for bin in firecracker-containerd firecracker-ctr containerd-shim-aws-firecracker firecracker jailer; do
    if [ ! -f "/usr/local/bin/$bin" ]; then
      echo -e "\e[31mFailed: $bin is not installed.\e[0m"
      exit
    fi
  done
  if [ ! -f /var/lib/firecracker-containerd/runtime/default-rootfs.img ]; then
    echo -e "\e[31mFailed: /var/lib/firecracker-containerd/runtime/default-rootfs.img is not installed.\e[0m"
    exit
  fi
  echo -e "\e[34mOK.\e[0m"
elif [ "$INSTALL_FROM_SOURCE" == "build_from_source" ]; then
  echo - Clone firecracker-containerd-puffer
  pushd >/dev/null "${HOME}" || exit
  rm -rf firecracker-containerd-puffer
  git clone --recurse-submodules https://github.com/Kingdo777/firecracker-containerd-puffer >/dev/null 2>&1
  # 我们必须先编译`image`, 因为image默认是通过容器环境编译的，如果我们先编译image，那么将同时在容器中编译agent
  # 这样的话生成的rootfs将和agent匹配，否则agent将无法启动
  # 否则如果先编译`all`，那么agent就是基于当前环境生产的，此时需要基于当前环境编译image，而之后再编译image时它默认依然是从容器中编译
  # 由于此时agent已经生成，就会跳过编译agent，从而导致agent和rootfs不匹配，如何在本地编译rootfs：https://github.com/firecracker-microvm/firecracker-containerd/tree/main/tools/image-builder
  pushd >/dev/null firecracker-containerd-puffer || exit
  echo - Build rootfs-image
  sg docker -c "https_proxy=http://ip:port make image" >/dev/null 2>&1
  echo - Build firecracker-containerd, firecracker
  sg docker -c "https_proxy=http://ip:port make all firecracker" >/dev/null 2>&1
  echo -n - Checking Build...
  for bin in runtime/containerd-shim-aws-firecracker \
    firecracker-control/cmd/containerd/firecracker-containerd \
    firecracker-control/cmd/containerd/firecracker-ctr \
    bin/firecracker; do
    if [ ! -f "$bin" ]; then
      echo -e "\e[31mFailed: $bin is not build.\e[0m"
      exit
    fi
  done
  echo -e "\e[34mOK.\e[0m"

  echo - Installing all components
  sudo make install install-firecracker >/dev/null 2>&1
  popd >/dev/null || exit
  popd >/dev/null || exit
  echo Done.
  echo
else
  echo -e "\e[31mFailed: INSTALL_FROM_SOURCE must be build_from_bin or build_from_source.\e[0m"
  exit
fi

echo Downloading kernel from github.com/Kingdo777/linux-5.10-faascale ...
if [ ! -f /tmp/hello-vmlinux.bin ]; then
  https_proxy=http://ip:port wget -q --show-progress -O /tmp/hello-vmlinux.bin /tmp/hello-vmlinux.bin https://github.com/Kingdo777/linux-5.10-faascale/releases/download/v1.0.0/vmlinux
  echo -n - Checking Download...
  if [ ! -f /tmp/hello-vmlinux.bin ]; then
    echo -e "\e[31mFailed: /tmp/hello-vmlinux.bin is not downloaded.\e[0m"
    exit
  else
    echo "8346d69256f41cd2aaa683db65d100b0f04abe16aff73f385faba9d9746fa1b7 /tmp/hello-vmlinux.bin" >/tmp/hello-vmlinux.sha256sum
    if ! sha256sum -c /tmp/hello-vmlinux.sha256sum >/dev/null 2>&1; then
      echo -e "\e[31mFailed: /tmp/hello-vmlinux.bin is not downloaded correctly.\e[0m"
      exit
    fi
  fi
  echo -e "\e[34mOK.\e[0m"
fi
echo Done.
echo

echo Copying rootfs and kernel to /var/lib/firecracker-containerd/runtime
sudo mkdir -p /var/lib/firecracker-containerd/runtime
if [ "$INSTALL_FROM_SOURCE" == "build_from_source" ]; then
  sudo cp ~/firecracker-containerd-puffer/tools/image-builder/rootfs.img /var/lib/firecracker-containerd/runtime/default-rootfs.img
fi
sudo cp /tmp/hello-vmlinux.bin /var/lib/firecracker-containerd/runtime/hello-vmlinux.bin
echo Done.
echo

echo Adding firecracker-containerd config-file and runtime-files
sudo mkdir -p /etc/firecracker-containerd
echo - Adding /etc/firecracker-containerd/config.toml
sudo tee /etc/firecracker-containerd/config.toml >/dev/null 2>&1 <<EOF
version = 2
disabled_plugins = ["io.containerd.grpc.v1.cri"]
root = "/var/lib/firecracker-containerd/containerd"
state = "/run/firecracker-containerd"
[grpc]
  address = "/run/firecracker-containerd/containerd.sock"
[plugins]
  [plugins."io.containerd.snapshotter.v1.devmapper"]
    pool_name = "fc-dev-thinpool"
    base_image_size = "10GB"
    root_path = "/var/lib/firecracker-containerd/snapshotter/devmapper"

[debug]
  level = "debug"
EOF
echo - Adding /etc/containerd/firecracker-runtime.json
sudo tee /etc/containerd/firecracker-runtime.json >/dev/null 2>&1 <<EOF
{
  "firecracker_binary_path": "/usr/local/bin/firecracker",
  "kernel_image_path": "/var/lib/firecracker-containerd/runtime/hello-vmlinux.bin",
  "kernel_args": "console=ttyS0 noapic reboot=k panic=1 pci=off nomodules ro systemd.unified_cgroup_hierarchy=0 systemd.journald.forward_to_console systemd.unit=firecracker.target init=/sbin/overlay-init",
  "root_drive": "/var/lib/firecracker-containerd/runtime/default-rootfs.img",
  "cpu_template": "T2",
  "log_fifo": "fc-logs.fifo",
  "log_levels": ["debug"],
  "metrics_fifo": "fc-metrics.fifo"
}
EOF
echo Done.
echo

echo Creating thinpool for devmapper
"$current_script_path"/create_devmapper.sh
echo Done.
echo

# https://github.com/Kingdo777/firecracker-containerd-puffer/blob/main/docs/scaling.md#scaling-the-number-of-firecracker-microvms-per-host

echo Configuring firecracker-containerd for Scaling the number of Firecracker microVMs per host...
# Lines to be added
lines_to_add=(
  "* soft nofile 1000000"
  "* hard nofile 1000000"
  "root soft nofile 1000000"
  "root hard nofile 1000000"
  "* soft nproc 4000000"
  "* hard nproc 4000000"
  "root soft nproc 4000000"
  "root hard nproc 4000000"
  "* soft stack 65536"
  "* hard stack 65536"
  "root soft stack 65536"
  "root hard stack 65536"
)
# Path to the limits.conf file
limits_conf="/etc/security/limits.conf"
# Check if each line already exists in the file
for line in "${lines_to_add[@]}"; do
  if ! grep -qF "$line" "$limits_conf"; then
    sudo sh -c "echo $line >> $limits_conf"
  fi
done

# provision the ARP cache to avoid garbage collection
sudo sysctl -q -w net.ipv4.neigh.default.gc_thresh1=1024
sudo sysctl -q -w net.ipv4.neigh.default.gc_thresh2=2048
sudo sysctl -q -w net.ipv4.neigh.default.gc_thresh3=4096
sudo sysctl -q -w net.ipv4.ip_local_port_range="32769 65535"

# configure the maximum number of processes and threads in the system.
sudo sysctl -q -w kernel.pid_max=4194303
sudo sysctl -q -w kernel.threads-max=999999999

echo Done.
