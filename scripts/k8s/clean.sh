#!/bin/bash

set -x

sudo kubeadm reset -f >/dev/null 2>&1

sudo rm -rf /etc/cni/net.d >/dev/null 2>&1

sudo sh -c "iptables -F && iptables -t nat -F && iptables -t mangle -F && iptables -X"

rm -rf "${HOME}"/.kube
sudo rm -rf /root/.kube
