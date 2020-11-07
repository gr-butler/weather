# weather
Raspberry Pi weather station


 env GOARCH=arm GOARM=5 GOOS=linux go build -o weatherServer.exe

 scp weatherServer.exe pi@192.168.1.69:/home/pi

 journalctl -e -u weather.service




# service file
bash```
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
'''

# prometeus

chgrp -R nogroup  /prometheus

docker run -d --name prometheus_weather -p 9090:9090 -v /prometheus/config/prometheus.yml:/etc/prometheus/prometheus.yml -v /prometheus/data:/prometheus prom/prometheus --config.file=/etc/prometheus/prometheus.yml

bash```

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
      - 192.168.1.69:80  
  - job_name: river
    scrape_interval: 30s
    static_configs:
    - targets:
      - localhost:50000
'''

