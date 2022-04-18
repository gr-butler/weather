package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pointer2null/weather/data"
	"github.com/pointer2null/weather/sensors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	logger "github.com/sirupsen/logrus"
)

const version = "GRB-Weather-0.3.0"

type weatherstation struct {
	s    *sensors.Sensors
	data *data.WeatherData

	pressure      float64
	pressureInHg  float64
	humidity      float64
	temp          float64
	tempf         float64
	hiResTemp     float64
	windGust      float64
	windSpeedAvg  float64
	windDirection float64
	windVolts     float64
	windhist      []time.Time
	pHist         int
	tGood         bool // true if temp readings are good
	aGood         bool // true if the atmostperic readings are good
}

type webdata struct {
	TimeNow      string  `json:"time"`
	Temp         float64 `json:"temp_C"`
	TempHiRes    float64 `json:"hiResTemp_C"`
	Humidity     float64 `json:"humidity_RH"`
	Pressure     float64 `json:"pressure_hPa"`
	PressureHg   float64 `json:"pressure_InchHg"`
	RainHr       float64 `json:"rain_mm_hr"`
	RainRate     float64 `json:"rain_rate"`
	LastTip      string  `json:"last_tip"`
	WindDir      float64 `json:"wind_dir"`
	WindVolts    float64 `json:"wind_volt"`
	WindSpeed    float64 `json:"wind_speed"`
	WindSpeedAvg float64 `json:"wind_speed_avg"`
}

var atmPresure = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "atmospheric_pressure",
		Help: "Atmospheric pressure hPa",
	},
)

// var Prom_mmRainPerHour = prometheus.NewGauge(
// 	prometheus.GaugeOpts{
// 		Name: "mm_rain_last_hour",
// 		Help: "mm of Rain in the last hour",
// 	},
// )

var Prom_rainRatePerHour = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "rain_hour_rate",
		Help: "The rain rate based on the last 5 minuntes",
	},
)

var Prom_rainDayTotal = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "rain_day",
		Help: "The rain total today (9.01am - 9am)",
	},
)

var rh = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "relative_humidity",
		Help: "Relative Humidity",
	},
)

var temperature = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "temperature",
		Help: "Temperature C",
	},
)

var altTemp = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "altTemperature",
		Help: "Temperature C",
	},
)

var windspeed = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "windspeed",
		Help: "Average Wind Speed mph",
	},
)

var windgust = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "windgust",
		Help: "Instant wind speed mph",
	},
)

var windDirection = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "winddirection",
		Help: "Wind Direction Deg",
	},
)

// called by prometheus
func init() {
	logger.Infof("%v: Initialize prometheus...", time.Now().Format(time.RFC822))
	prometheus.MustRegister(atmPresure,
		// Prom_mmRainPerHour,
		rh,
		temperature,
		altTemp,
		Prom_rainRatePerHour,
		windspeed,
		windgust,
		windDirection,
		Prom_rainDayTotal)
}

func main() {
	logger.Infof("Starting weather station [%v]", version)
	logger.Infof("%v: Initialize sensors...", time.Now().Format(time.RFC822))
	w := weatherstation{}
	w.s = &sensors.Sensors{}
	err := w.s.InitSensors()
	defer (*w.s.IIC.Bus).Close()
	if err != nil {
		logger.Errorf("Failed to initialise sensors!! [%v]", err)
		logger.Exit(1)
	}

	w.data = data.CreateWeatherData()

	// start go routines
	go w.StartAtmosphericMonitor()
	go w.StartRainMonitor()
	go w.StartWindMonitor()

	go w.MetofficeProcessor()

	// start web service
	http.HandleFunc("/", w.handler)
	http.Handle("/metrics", promhttp.Handler())
	logger.Info("Starting webservice...")
	defer logger.Info("Exiting...")
	logger.Fatal(http.ListenAndServe(":80", nil))
}

func (s *weatherstation) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	wd := webdata{
		Temp:       s.temp,
		TempHiRes:  s.hiResTemp,
		Humidity:   s.humidity,
		Pressure:   s.pressure,
		PressureHg: s.pressureInHg,
		// RainHr:     s.getMMLastHour(),
		//		RainRate:     s.getHourlyRate(time.Now().Minute()),
		//LastTip:      s.lastTip.Format(time.RFC822),
		TimeNow:      time.Now().Format(time.RFC822),
		WindDir:      s.windDirection,
		WindVolts:    s.windVolts,
		WindSpeed:    s.windGust,
		WindSpeedAvg: s.windSpeedAvg,
	}

	js, err := json.Marshal(wd)
	if err != nil {
		logger.Errorf("JSON error [%v]", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Infof("Web read: \n[%v]", string(js))
	_, _ = w.Write(js) // not much we can do if this fails
}
