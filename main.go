package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/devices/bmxx80"
	"periph.io/x/periph/experimental/conn/gpio/gpioutil"
	"periph.io/x/periph/experimental/devices/ads1x15"
	"periph.io/x/periph/experimental/devices/mcp9808"
	"periph.io/x/periph/host"

	logger "github.com/sirupsen/logrus"
)

const version = "GRB-Weather-0.1.1"

type sensors struct {
	bme     *bmxx80.Dev
	hiResT  *mcp9808.Dev
	bus     *i2c.BusCloser
	rainpin *gpio.PinIO
	windpin *gpio.PinIO
	windDir *ads1x15.PinADC
}
type weatherstation struct {
	s             *sensors
	btips         []float64
	count         float64   // GPIO bucket tip counter
	lastTip       time.Time // Last bucket tip
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

var mmRainPerHour = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "mm_rain_last_hour",
		Help: "mm of Rain in the last hour",
	},
)

var rainRatePerHour = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "rain_hour_rate",
		Help: "The rain rate based on the last 5 minuntes",
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
	prometheus.MustRegister(atmPresure, mmRainPerHour, rh, temperature, altTemp, rainRatePerHour, windspeed, windgust, windDirection)
}

func main() {
	logger.Infof("Starting weather station [%v]", version)
	logger.Infof("%v: Initialize sensors...", time.Now().Format(time.RFC822))
	w := weatherstation{}
	w.s = &sensors{}
	w.initSensors()
	defer (*w.s.bus).Close()

	// start go routines
	go w.readAtmosphericSensors()
	go w.readRainData()
	go w.readWindData()
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
		Temp:         s.temp,
		TempHiRes:    s.hiResTemp,
		Humidity:     s.humidity,
		Pressure:     s.pressure,
		PressureHg:   s.pressureInHg,
		RainHr:       s.getMMLastHour(),
		RainRate:     s.getHourlyRate(time.Now().Minute()),
		LastTip:      s.lastTip.Format(time.RFC822),
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

	logger.Infof("Web read: \n[%v]", js)
	_, _ = w.Write(js) // not much we can do if this fails
}

func (w *weatherstation) initSensors() {
	if _, err := host.Init(); err != nil {
		logger.Error("Failed to init i2c bus")
		logger.Fatal(err)
	}
	i2cbus := flag.String("bus", "", "I²C bus (/dev/i2c-1)")
	temperatureAddr := flag.Int("address", 0x18, "I²C address")

	// Open default I²C bus.
	bus, err := i2creg.Open(*i2cbus)
	if err != nil {
		logger.Fatalf("failed to open I²C: %v", err)
	}

	// Create a new temperature sensor a sense with default options.
	tempSensor, err := mcp9808.New(bus, &mcp9808.Opts{Addr: *temperatureAddr})
	if err != nil {
		logger.Errorf("failed to open MCP9808 sensor: %v", err)
	}

	logger.Info("Starting BMP280 reader...")
	bme, err := bmxx80.NewI2C(bus, 0x76, &bmxx80.DefaultOpts)
	if err != nil {
		logger.Errorf("failed to initialize bme280: %v", err)
	}

	logger.Info("Starting MCP9808 Temperature Sensor")

	// Lookup a rainpin by its number:
	rp := gpioreg.ByName("GPIO17")
	if rp == nil {
		logger.Error("Failed to find GPIO17")
	}

	logger.Infof("%s: %s", rp, rp.Function())

	// if err = rainpin.In(gpio.PullUp, gpio.BothEdges); err != nil {
	// 	log.Fatal(err)
	// }

	// Set up debounced pin
	// Ignore glitches lasting less than 3ms, and ignore repeated edges within
	// 300ms.
	rainpin, err := gpioutil.Debounce(rp, 3*time.Millisecond, 300*time.Millisecond, gpio.FallingEdge)
	if err != nil {
		log.Fatal(err)
	}

	// Lookup a windpin by its number:
	windpin := gpioreg.ByName("GPIO27")
	if windpin == nil {
		logger.Error("Failed to find GPIO27")
	}

	logger.Infof("%s: %s", windpin, windpin.Function())

	if err = windpin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		logger.Error(err)
	}

	logger.Info("Starting Wind direction ADC")
	// Create a new ADS1115 ADC.
	adc, err := ads1x15.NewADS1115(bus, &ads1x15.DefaultOpts)
	if err != nil {
		logger.Error(err)
	}

	// Obtain an analog pin from the ADC.
	dirPin, err := adc.PinForChannel(ads1x15.Channel0, 5*physic.Volt, 1*physic.Hertz, ads1x15.SaveEnergy)
	if err != nil {
		logger.Error(err)
	}
	defer dirPin.Halt() //nolint

	w.s.bme = bme
	w.s.hiResT = tempSensor
	w.btips = make([]float64, 60)
	w.count = 0
	w.s.bus = &bus
	w.s.rainpin = &rainpin
	w.s.windpin = &windpin
	w.s.windDir = &dirPin
	w.windhist = make([]time.Time, 300)
	w.pHist = 0
}
