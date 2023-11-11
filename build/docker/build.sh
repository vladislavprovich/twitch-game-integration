#!/bin/bash

imageTag=go1.21.x-gcc-windows-i386
docker build -t $imageTag -f ./build/docker/Dockerfile .

container=$(docker run \
    -d \
    -it \
    -v "$(pwd)/:/src/" \
    $imageTag \
    /bin/sh)

echo "Building native DLL"
docker exec $container \
    /bin/sh -c 'cd /src && \
        env CGO_ENABLED=1 GOARCH=386 GOOS=windows /usr/local/go/bin/go build -buildvcs=false -buildmode c-shared -ldflags="-s -w -extldflags=-static" -o twitch-integration.dll  ./cmd/twitch-integration/'

echo "Building connector application"
docker exec $container \
    /bin/sh -c 'cd /src && \
        env CGO_ENABLED=0 GOARCH=386 GOOS=windows /usr/local/go/bin/go build -buildvcs=false -ldflags="-s -w" ./cmd/twitch-integration-connector/'

docker rm -f $container
