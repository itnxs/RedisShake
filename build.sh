#!/bin/bash

set -e

echo "[ BUILD RELEASE ]"
BIN_DIR=$(pwd)/bin/
rm -rf "$BIN_DIR"
mkdir -p "$BIN_DIR"

cp shake.toml "$BIN_DIR"

dist() {
    echo "try build GOOS=$1 GOARCH=$2"
    export GOOS=$g
    export GOARCH=$a
    export CGO_ENABLED=0
    go build -v -trimpath -o "$BIN_DIR/redis-shake" "./cmd/redis-shake"
    unset GOOS
    unset GOARCH
    echo "build success GOOS=$1 GOARCH=$2"

    cd "$BIN_DIR"
    tar -czvf ./redis-shake-"$1"-"$2".tar.gz ./redis-shake ./shake.toml
    cd ..
}

if [ "$1" == "dist" ]; then
    echo "[ DIST ]"
    for g in "linux" "darwin" "windows"; do
        for a in "amd64" "arm64"; do
            dist "$g" "$a"
        done
    done
fi

# build the current platform
echo "try build for current platform"
go build -v -trimpath -o "$BIN_DIR/redis-shake" "./cmd/redis-shake"
go build -v -trimpath -o "$BIN_DIR/rdb-reader" "./cmd/rdb-reader"
echo "build success"
