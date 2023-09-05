sudo firecracker-containerd --config /etc/firecracker-containerd/config.toml

sudo firecracker-ctr --address /run/firecracker-containerd/containerd.sock images \
  pull --snapshotter devmapper \
  docker.io/library/busybox:latest

sudo firecracker-ctr --address /run/firecracker-containerd/containerd.sock \
  run \
  --snapshotter devmapper \
  --runtime aws.firecracker \
  --rm --tty --net-host \
  docker.io/library/busybox:latest busybox-test

mkdir -p "$HOME"/.kube111
