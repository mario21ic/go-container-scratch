#!/bin/bash
set -xe

mkdir -p ubuntu-2404-rootfs/mario21ic_temp
wget https://cloud-images.ubuntu.com/wsl/releases/24.04/current/ubuntu-noble-wsl-amd64-24.04lts.rootfs.tar.gz
tar -xzf ubuntu-noble-wsl-amd64-24.04lts.rootfs.tar.gz -C ubuntu-2404-rootfs
