#!/bin/sh

# Build core
garble -literals -tiny build -trimpath -ldflags="-s -w -buildid=" -o core main.go
echo "core builded"

# Compress core
upx --best --lzma core
echo "core compressed"

# Copy core to all nodes
SOURCE_FILE="./core"

for i in {1..5}; do
    TARGET_DIR="/home/rad/Desktop/BLVchain/Deploy/cores/node_$i"
    cp "$SOURCE_FILE" "$TARGET_DIR/"
    echo "Copied core to node_$i"
done