@echo off
set VERSION=dev

mkdir dist 2>nul

echo Building Windows...
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-X main.version=%VERSION%" -o dist\elf2vkp-go.exe

echo Building Linux...
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-X main.version=%VERSION%" -o dist\elf2vkp-go-linux

echo Done.
