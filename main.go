package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"database/sql"

	_ "github.com/lib/pq"

	"github.com/pointer2null/weather/data"
	"github.com/pointer2null/weather/db/postgres"
	"github.com/pointer2null/weather/env"
	"github.com/pointer2null/weather/led"
	"github.com/pointer2null/weather/sensors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	logger "github.com/sirupsen/logrus"
)

const version = "GRB-Weather-2.1.0"

const (
	host     = "192.168.1.212"
	port     = 5432
	user     = "weather"
	password = "weather01."
	dbname   = "weather"
)

type weatherstation struct {
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

var Prom_rainDayTotal = prometheus.NewGauge(
	prometheus.GaugeOpts{
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

func main() {
	logger.Infof("Starting weather station [%v]", version)
	w := weatherstation{}
	w.args = &env.Args{}

	w.args.Test = flag.Bool("test", false, "runs in test mode")
	w.args.NoWow = flag.Bool("nowow", false, "does not send met office data")
	w.args.Verbose = flag.Bool("v", false, "verbose logging")
	w.args.Imuon = flag.Bool("imu", false, "activates the IMU output")
	w.args.Speedon = flag.Bool("speed", false, "show wind speed info")
	w.args.Diron = flag.Bool("dir", false, "show wind direction")
	w.args.Rainon = flag.Bool("rain", false, "show rain tip info")
	w.args.WindEnabled = flag.Bool("windOn", true, "disables the anemometer")
	w.args.AtmosphericEnabled = flag.Bool("atmOn", true, "disables atmospheric sensor")
	w.args.RainEnabled = flag.Bool("rainOn", true, "disables rain sensor")
	flag.Parse()

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

	// start web service
	logger.Info("Starting webservice...")
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
		RainRate:  w.s.Rain.GetMinuteRate().Float64(),
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
