
@ECHO off

REM Requires msys2 + mingw32 and "i686-w64-mingw32-gcc"

set CGO_ENABLED=1
set GOARCH=386
set "CC=i686-w64-mingw32-gcc"
set "CGO_CFLAGS=-m32 %CGO_CFLAGS%"
set "PATH=C:\msys64\mingw32\bin;%PATH%"

go build -buildmode c-shared -ldflags="-s -w -extldflags=-static" -o twitch-integration.dll  .\cmd\twitch-integration\

set CGO_ENABLED=0
REM set GOARCH=amd64
go build -ldflags="-s -w" .\cmd\twitch-integration-connector\
