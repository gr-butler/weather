# weather
Raspberry Pi weather station


 env GOARCH=arm GOARM=5 GOOS=linux go build -o weatherServer.exe

 scp weatherServer.exe pi@192.168.1.69:/home/pi