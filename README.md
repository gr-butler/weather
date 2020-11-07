# weather
Raspberry Pi weather station


 env GOARCH=arm GOARM=5 GOOS=linux go build -o weatherServer.exe

 scp weatherServer.exe pi@192.168.1.69:/home/pi

 journalctl -e -u weather.service




# service file
 [Unit]
Description=Weather monitor service
After=network.target
StartLimitIntervalSec=0
StartLimitBurst=5

[Service]
Type=simple
Restart=always
RestartSec=1
User=root
ExecStart=/usr/local/bin/weatherServer.exe
ExecStartPre=/bin/sh -c "cp -f /home/pi/weatherServer.exe /usr/local/bin"

[Install]
WantedBy=multi-user.target