package main

import (
	// "context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"

	// "os/signal"
	"time"

	"database/sql"

	_ "github.com/lib/pq"

	"github.com/gr-butler/weather/data"
	"github.com/gr-butler/weather/db/postgres"
	"github.com/gr-butler/weather/env"
	"github.com/gr-butler/weather/led"
	"github.com/gr-butler/weather/sensors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	logger "github.com/sirupsen/logrus"
)

const version = "GRB-Weather-2.2.0"

// 2.2.0 added mqtt

const (
	host     = "server.internal"
	port     = 5432
	user     = "weather"
	password = "weather01."
	dbname   = "weather"
	broker   = "tcp://server.internal:1883"
	clientID = "weather-mqtt-client"
	topic    = "culverhay/weather"
)

type weatherstation struct {
	client       mqtt.Client
	s            *sensors.Sensors
	data         *data.WeatherData
	Db           *postgres.Queries
	HeartbeatLed *led.LED
	args         *env.Args
}

type webdata struct {
	TimeNow   string  `json:"time"`
	TempHiRes float64 `json:"hiResTemp_C"`
	Humidity  float64 `json:"humidity_RH"`
	Pressure  float64 `json:"pressure_hPa"`
	RainHr    float64 `json:"rain_mm_hr"`
	RainRate  float64 `json:"rain_rate"`
	RainDay   float64 `json:"rain_day"`
	WindDir   float64 `json:"wind_dir"`
	WindSpeed float64 `json:"wind_speed"`
	WindGust  float64 `json:"wind_gust"`
}

var Prom_atmPresure = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "atmospheric_pressure",
		Help: "Atmospheric pressure hPa",
	},
)

var Prom_rainRatePerMin = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "rain_min_rate",
		Help: "The rain rate based on the last 1 minutes",
	},
)

var Prom_rainDayTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "rain_day",
		Help: "The rain total today (9.01am - 9am)",
	},
)

var Prom_humidity = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "relative_humidity",
		Help: "Relative Humidity",
	},
)

var Prom_temperature = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "temperature",
		Help: "Temperature C",
	},
)

var Prom_windspeed = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "windspeed",
		Help: "Average Wind Speed mph",
	},
)

var Prom_windgust = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "windgust",
		Help: "Instant wind speed mph",
	},
)

var Prom_windDirection = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "winddirection",
		Help: "Wind Direction Deg",
	},
)

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	logger.Info("Connected to MQTT Broker")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	logger.Errorf("Connection lost: %v", err)

	client.Disconnect(250) // Gracefully disconnect
	// Attempt to reconnect
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("Failed to reconnect to MQTT broker: %v", token.Error())
	} else {
		logger.Info("Reconnected to MQTT Broker")
	}
}

// called by prometheus
func init() {
	logger.Infof("%v: Initialize prometheus...", time.Now().Format(time.RFC822))
	prometheus.MustRegister(
		Prom_atmPresure,
		Prom_humidity,
		Prom_rainRatePerMin,
		Prom_rainDayTotal,
		Prom_temperature,
		Prom_windspeed,
		Prom_windgust,
		Prom_windDirection)
}

// Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		logger.Errorf("Failed to get outbound IP: %v", err)
		return nil // Return nil if unable to determine IP
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func main() {
	logger.Infof("Starting weather station [%v]", version)
	w := weatherstation{}
	w.args = &env.Args{}

	w.args.Test = flag.Bool("test", false, "runs in test mode")
	w.args.NoWow = flag.Bool("nowow", false, "does not send met office data")
	w.args.Verbose = flag.Bool("v", false, "verbose logging")
	w.args.Speedon = flag.Bool("speed", false, "show wind speed info")
	w.args.Diron = flag.Bool("dir", false, "show wind direction")
	w.args.Rainon = flag.Bool("rain", false, "show rain tip info")
	w.args.WindEnabled = flag.Bool("windOn", true, "disables the anemometer")
	w.args.AtmosphericEnabled = flag.Bool("atmOn", true, "disables atmospheric sensor")
	w.args.RainEnabled = flag.Bool("rainOn", true, "disables rain sensor")
	w.args.Humidity = flag.Bool("humOn", false, "Debug log raw humidity")
	flag.Parse()

	wowsiteid, idok := os.LookupEnv("WOWSITEID")
	wowpin, pinok := os.LookupEnv("WOWPIN")
	if !idok || !pinok {
		logger.Warn("Missing WOW details")
		w.args.NoWow = &env.Enabled
	}
	w.args.WowPin = wowpin
	w.args.WowSiteID = wowsiteid

	if *w.args.Test {
		logger.Info("TEST MODE")
	}

	// connect to database
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		logger.Errorf("Failed to initialise database: [%v]", err)
		logger.Exit(1)
	}
	defer db.Close()

	logger.Info("Successfully connected to db.")

	w.Db = postgres.New(db)

	logger.Info("Initializing sensors...")

	w.s = sensors.InitSensors(w.args)
	if w.s == nil {
		logger.Error("Failed to initialise sensors")
		logger.Exit(1)
	}
	defer (*w.s.Closer).Close()

	//setup heartbeat
	w.HeartbeatLed = led.NewLED("Heartbeat LED", env.HeartbeatLed)
	go w.Heartbeat()

	w.data = data.CreateWeatherData()

	go w.Reporting()

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetKeepAlive(30)
	opts.SetPingTimeout(10 * time.Second)
	opts.AutoReconnect = true
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler

	w.client = mqtt.NewClient(opts)
	if token := w.client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("Failed to connect to MQTT broker: %v", token.Error())
	}

	// start web service
	logger.Infof("[%v] Starting webservice...", version)
	http.HandleFunc("/", w.handler)
	http.Handle("/metrics", promhttp.Handler())

	logger.Info(http.ListenAndServe(":80", nil))
	w.HeartbeatLed.Off()
	w.s.Rain.GetLED().Off()
	defer logger.Info("Exiting...")
}

func (w *weatherstation) Heartbeat() {
	logger.Info("Heartbeat started")
	for {
		w.HeartbeatLed.Flash()
		time.Sleep(time.Second * 60)
	}
}

func (w *weatherstation) handler(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	pres, hum := w.s.Atm.GetHumidityAndPressure()
	wd := webdata{
		TempHiRes: w.s.Atm.GetTemperature().Float64(),
		Humidity:  hum.Float64(),
		Pressure:  pres.Float64(),
		RainHr:    w.s.Rain.GetRate().Float64(),
		RainDay:   w.s.Rain.GetDayAccumulation().Float64(),
		TimeNow:   time.Now().Format(time.RFC822),
		WindDir:   w.s.Wind.GetDirection(),
		WindSpeed: w.s.Wind.GetSpeed(),
		WindGust:  w.s.Wind.GetGust(),
	}

	js, err := json.Marshal(wd)
	if err != nil {
		logger.Errorf("JSON error [%v]", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Infof("Web read: \n[%v]", string(js))
	_, _ = rw.Write(js) // not much we can do if this fails
}

// GetInterruptContext gives a context that will call cancel() when an os.Interupt is signalled
// func getInterruptContext() context.Context {
// 	ctx, cancel := context.WithCancel(context.Background())

// 	sigChan := make(chan os.Signal, 1)
// 	signal.Notify(sigChan, os.Interrupt)

// 	go func() {
// 		<-sigChan
// 		logger.Info("SIGTERM received, stopping...")
// 		cancel()
// 	}()

// 	return ctx
// }
