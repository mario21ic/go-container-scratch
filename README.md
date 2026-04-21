Based on https://baconyao.notion.site/Containers-From-Scratch-by-Golang-feat-Liz-Rice-2938a3a7d9d480dc9598e8efd86cfd4b#2938a3a7d9d480fcba55e8692eabd2dc

wget https://cloud-images.ubuntu.com/wsl/releases/24.04/current/ubuntu-noble-wsl-amd64-24.04lts.rootfs.tar.gz
mkdir ubuntu-2404-rootfs
sudo tar -xzf ubuntu-noble-wsl-amd64-24.04lts.rootfs.tar.gz -C ubuntu-2404-rootfs
mkdir ubuntu-2404-rootfs/mario21ic_temp

sudo go run main.go run /bin/bash
