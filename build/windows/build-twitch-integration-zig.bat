
@ECHO off

REM Requires zig installed - https://ziglang.org/

set CGO_ENABLED=1
set GOARCH=386
set "CC=zig.exe cc -target x86-windows-gnu -march=baseline"

go build -buildmode c-shared -ldflags="-s -w -extldflags='-static'" -o twitch-integration.dll  .\cmd\twitch-integration\

set CGO_ENABLED=0
REM set GOARCH=amd64
go build -ldflags="-s -w" .\cmd\twitch-integration-connector\
