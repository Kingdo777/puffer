#!/bin/bash

# Sets up a devicemapper thin pool with loop devices in
# /var/lib/firecracker-containerd/snapshotter/devmapper

set -e

DIR=/var/lib/firecracker-containerd/snapshotter/devmapper
POOL=fc-dev-thinpool

sudo mkdir -p "${DIR}"

if [[ ! -f "${DIR}/data" ]]; then
sudo touch "${DIR}/data"
sudo truncate -s 100G "${DIR}/data"
fi

if [[ ! -f "${DIR}/metadata" ]]; then
sudo touch "${DIR}/metadata"
sudo truncate -s 2G "${DIR}/metadata"
fi

DATADEV="$(sudo losetup --output NAME --noheadings --associated ${DIR}/data)"
if [[ -z "${DATADEV}" ]]; then
DATADEV="$(sudo losetup --find --show ${DIR}/data)"
fi

METADEV="$(sudo losetup --output NAME --noheadings --associated ${DIR}/metadata)"
if [[ -z "${METADEV}" ]]; then
METADEV="$(sudo losetup --find --show ${DIR}/metadata)"
fi

SECTORSIZE=512
DATASIZE="$(sudo blockdev --getsize64 -q ${DATADEV})"
LENGTH_SECTORS=$(bc <<< "${DATASIZE}/${SECTORSIZE}")
DATA_BLOCK_SIZE=128 # see https://www.kernel.org/doc/Documentation/device-mapper/thin-provisioning.txt
LOW_WATER_MARK=32768 # picked arbitrarily
THINP_TABLE="0 ${LENGTH_SECTORS} thin-pool ${METADEV} ${DATADEV} ${DATA_BLOCK_SIZE} ${LOW_WATER_MARK} 1 skip_block_zeroing"
echo "${THINP_TABLE}"

if ! $(sudo dmsetup reload "${POOL}" --table "${THINP_TABLE}"); then
sudo dmsetup create "${POOL}" --table "${THINP_TABLE}"
fi