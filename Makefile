.PHONY: build push

build:
	env GOARCH=arm GOARM=5 GOOS=linux go build -o weatherServer.exe

push:
	scp weatherServer.exe pi@weather.internal:/home/pi
