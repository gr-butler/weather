# weather

Raspberry Pi weather station

A simple weather station based on a raspberry pi. It used a BME280 sensor for pressure and humidity and some generic weather station parts for the rain guage, wind speed and wind direction sensors. I'm using a MCP9808 for temp as I found the BMEwas off by 0.5C

MCP9806 has a typical accuracy of 0.25C and a max error or 0.5C
<https://ww1.microchip.com/downloads/en/DeviceDoc/25095A.pdf>

It's written in go which is each to cross compile for the pi.

I use a prometheus time series database to scrape and record the values. These are displayed on a grafana dashboard. It's served from my home server using nginx reverse proxy.

Below are just notes and common commands (easier to cut and paste than to type out each time!)

## MetOffice

The UK Met Office run an observation site for users to submit thier own data.

<https://wow.metoffice.gov.uk>

We send data to the Met Office if the two relevent env variables are set:

WOWSITEID The site ID
WOWPIN The site PIN
SENDWOWDATA=true
SENDPROMDATA=true

## Pi setup

Use raspi-config to enable ssh and i2c

## compilation

env GOARCH=arm GOARM=5 GOOS=linux go build -o weatherServer.exe

## transfer and logs

scp weatherServer.exe <pi@192.168.1.xxx>:/home/pi

journalctl -f -e -u weather.service

## service file

```service
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
Environment=SENDWOWDATA=true
Environment=SENDPROMDATA=true
Environment=WOWSITEID=aaa-bbb-ccc-ddd-eee-fff
Environment=WOWPIN=0123456789
ExecStart=/usr/local/bin/weatherServer.exe
ExecStartPre=/bin/sh -c "cp -f /home/pi/weatherServer.exe /usr/local/bin"

[Install]
WantedBy=multi-user.target
```

## prometeus

chgrp -R nogroup /prometheus

docker run -d --name prometheus_weather -p 9090:9090 -v /prometheus/config/prometheus.yml:/etc/prometheus/prometheus.yml -v /prometheus/data:/prometheus prom/prometheus --config.file=/etc/prometheus/prometheus.yml

```yaml

global:
  scrape_interval:     15s # Set the scrape interval to every 15 seconds. Default is every 1 minute.
  evaluation_interval: 15s # Evaluate rules every 15 seconds. The default is every 1 minute.
  # scrape_timeout is set to the global default (10s).

# Alertmanager configuration
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      # - alertmanager:9093

# Load rules once and periodically evaluate them according to the global 'evaluation_interval'.
rule_files:
  # - "first_rules.yml"
  # - "second_rules.yml"

# A scrape configuration containing exactly one endpoint to scrape:
# Here it's Prometheus itself.
scrape_configs:
  # The job name is added as a label `job=<job_name>` to any timeseries scraped from this config.
  # - job_name: 'prometheus'
    # metrics_path defaults to '/metrics'
    # scheme defaults to 'http'.
  #  static_configs:
  #  - targets: ['localhost:9090']
  - job_name: weather
    scrape_interval: 30s
    static_configs:
    - targets:
       # change to match weather station ip
      - 192.168.1.202:80
  - job_name: river
    scrape_interval: 30s
    static_configs:
    - targets:
      - localhost:50000
```
