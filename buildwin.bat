@echo off
go-winres simply --icon logo.png --manifest gui
go build -ldflags -H=windowsgui -o MonarchUpload.exe