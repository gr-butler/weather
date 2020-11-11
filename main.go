package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
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
	"periph.io/x/periph/experimental/devices/ads1x15"
	"periph.io/x/periph/host"

	logger "github.com/sirupsen/logrus"
)

const (
	mmPerBucket float64 = 0.2794
	hgToPa      float64 = 133.322387415
	// 1 tick/second = 1.492MPH wind
	mphPerTick float64 = 1.429 / 2 // i seem to get 2 ticks per rev on my sensor 
)

type sensors struct {
	bme              *bmxx80.Dev
	btips            []int
	count            int       // GPIO bucket tip counter
	lastTip          time.Time // Last bucket tip
	bus              *i2c.BusCloser
	rainpin          *gpio.PinIO
	windpin          *gpio.PinIO
	windDir          *ads1x15.PinADC
	rainHr           float64
	pressure         float64
	pressureHg       float64
	humidity         float64
	temp             float64
	instantWindSpeed float64
	windSpeedAvg     float64
	windDirection    float64
	windVolts        float64
	rain24           []float64
	pressure24       []float64
	humidity24       []float64
	temp24           []float64
	windhist         []time.Time
	pHist            int
}

type webdata struct {
	TimeNow      string  `json:"time"`
	Temp         float64 `json:"temp_C"`
	Humidity     float64 `json:"humidity_RH"`
	Pressure     float64 `json:"pressure_hPa"`
	PressureHg   float64 `json:"pressure_mmHg"`
	RainHr       float64 `json:"rain_mm_hr"`
	LastTip      string  `json:"last_tip"`
	WindDir      float64 `json:"wind_dir"`
	WindVolts    float64 `json:"wind_volt"`
	WindSpeed    float64 `json:"wind_speed"`
	WindSpeedAvg float64 `json:"wind_speed_avg"`
}

type webHistory struct {
	Temp24H     []float64 `json:"temp_24hr"`
	Rain24H     []float64 `json:"rain_mm_24hr"`
	Pressure24H []float64 `json:"pressure_24hr"`
	Humidity24H []float64 `json:"humidity_24hr"`
	TipHistory  []int     `json:"tip_last_hour"`
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

var mmRainPerMin = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "mm_rain_last_min",
		Help: "mm of Rain in the last min",
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

var windspeed = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "windspeed",
		Help: "Average Wind Speed mph",
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
	prometheus.MustRegister(atmPresure)
	prometheus.MustRegister(mmRainPerHour)
	prometheus.MustRegister(rh)
	prometheus.MustRegister(temperature)
	prometheus.MustRegister(mmRainPerMin)
	prometheus.MustRegister(windspeed)
}

func main() {
	logger.Infof("%v: Initialize sensors...", time.Now().Format(time.RFC822))
	s := sensors{}
	s.initSensors()
	defer (*s.bus).Close()

	// get initial values
	s.measureSensors()

	// start go routines
	go s.recordHistory()
	go s.monitorRainGPIO()
	go s.monitorWindGPIO()
	go s.processWindSpeed()

	// start web service
	http.HandleFunc("/", s.handler)
	http.HandleFunc("/hist", s.history)
	http.Handle("/metrics", promhttp.Handler())
	logger.Info("Starting webservice...")
	defer logger.Info("Exiting...")
	logger.Fatal(http.ListenAndServe(":80", nil))
}

func (s *sensors) history(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	h := webHistory{
		Temp24H:     s.temp24,
		Rain24H:     s.rain24,
		TipHistory:  s.btips,
		Pressure24H: s.pressure24,
		Humidity24H: s.humidity24,
	}

	js, err := json.Marshal(h)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

func (s *sensors) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.measureSensors()
	wd := webdata{
		Temp:         s.temp,
		Humidity:     s.humidity,
		Pressure:     s.pressure,
		PressureHg:   s.pressureHg,
		RainHr:       s.getMMLastHour(),
		LastTip:      s.lastTip.Format(time.RFC822),
		TimeNow:      time.Now().Format(time.RFC822),
		WindDir:      s.windDirection,
		WindVolts:    s.windVolts,
		WindSpeed:    s.instantWindSpeed,
		WindSpeedAvg: s.windSpeedAvg,
	}

	js, err := json.Marshal(wd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

func (s *sensors) initSensors() {
	if _, err := host.Init(); err != nil {
		logger.Error("Failed to init i2c bus")
		logger.Fatal(err)
	}
	i2cbus := flag.String("bus", "", "I²C bus (/dev/i2c-1)")

	flag.Parse()

	// Open default I²C bus.
	bus, err := i2creg.Open(*i2cbus)
	if err != nil {
		logger.Fatalf("failed to open I²C: %v", err)
	}

	logger.Info("Starting BMP280 reader...")
	bme, err := bmxx80.NewI2C(bus, 0x76, &bmxx80.DefaultOpts)
	if err != nil {
		logger.Errorf("failed to initialize bme280: %v", err)
	}

	logger.Info("Starting MCP9808 Temperature Sensor")

	// Lookup a rainpin by its number:
	rainpin := gpioreg.ByName("GPIO17")
	if rainpin == nil {
		logger.Error("Failed to find GPIO17")
	}

	logger.Infof("%s: %s", rainpin, rainpin.Function())

	if err = rainpin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		log.Fatal(err)
	}
	// Lookup a rainpin by its number:
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
	defer dirPin.Halt()

	s.bme = bme
	//s.mcp = sensor
	s.btips = make([]int, 60)
	s.count = 0
	s.rain24 = make([]float64, 24)
	s.pressure24 = make([]float64, 24)
	s.humidity24 = make([]float64, 24)
	s.temp24 = make([]float64, 24)
	s.bus = &bus
	s.rainpin = &rainpin
	s.windpin = &windpin
	s.windDir = &dirPin
	s.windhist = make([]time.Time, 10)
	s.pHist = 0
}

func (s *sensors) monitorRainGPIO() {
	logger.Info("Starting tip bucket")
	for {
		(*s.rainpin).WaitForEdge(-1)
		logger.Info("Bucket tip")
		s.count++
		s.lastTip = time.Now()
	}
}

/*
	// 1 tick/second = 1.492MPH wind
	mphPerTick  float64 = 1.429
*/
func (s *sensors) monitorWindGPIO() {
	logger.Info("Starting wind sensor")
	lasttick := time.Now()
	var edge time.Time
	for {
		(*s.windpin).WaitForEdge(-1)

		edge = time.Now()
		f := 1000 / float64( edge.Sub(lasttick).Milliseconds())
		s.instantWindSpeed = math.Round(f*mphPerTick*100) / 100
		//logger.Infof("Duration [%v], freq [%v], ws [%v]", edge.Sub(lasttick), f, s.instantWindSpeed)
		lasttick = edge
		s.windhist[s.pHist] = lasttick
		s.pHist++
		if s.pHist == len(s.windhist) {
			s.pHist = 0
		}
	}
}

func (s *sensors) processWindSpeed() {
	for range time.Tick(time.Second * 10) {
		// iterate over array, cal period on any value less that 5 seconds old
		// determine average
		now := time.Now()
		max := len(s.windhist) - 1
		var duration time.Duration
		var last time.Time
		var total time.Duration
		var count int64 = 0
		for i := 0; i <= max; i++ {
			tick := s.windhist[i]
			if i == 0 {
				last = s.windhist[max]
			} else {
				last = s.windhist[i-1]
			}
			duration = tick.Sub(last)
			if now.Sub(tick) > (5*time.Second) || duration > time.Second || duration < (10*time.Millisecond) {
				continue
			}
			//logger.Infof("[%v]: Last [%v] this [%v] Duration [%v]",i, last, tick, duration)
			total += duration
			count++
		}
		logger.Info("")
		if count > 0 {
			// average tick interval
			avg := (total.Milliseconds() / count)
			// f := 1 / period
			f := 1000 / float64(avg)
			s.windSpeedAvg = math.Round(f*mphPerTick*100) / 100
			//logger.Infof("Average duration [%v], freq [%v], ws [%v]", avg, f, s.windSpeedAvg)
		} else {
			s.windSpeedAvg = 0.0
		}

		windspeed.Set(s.instantWindSpeed)
	}
}

func (s *sensors) measureSensors() {
	em := physic.Env{}
	if s.bme != nil {
		s.bme.Sense(&em)
	}
	logger.Debugf("BME: %8s %10s %9s\n", em.Temperature, em.Pressure, em.Humidity)
	s.humidity = math.Round(float64(em.Humidity) / float64(physic.PercentRH))
	s.pressure = math.Round(float64(em.Pressure) / float64(100*physic.Pascal))
	s.pressureHg = math.Round(float64(em.Pressure) / (float64(physic.Pascal) * hgToPa))
	s.temp = em.Temperature.Celsius()
	sample, err := (*s.windDir).Read()
	if err != nil {
		logger.Errorf("Error reading wind direction value [%v]", err)
		sample.Raw = 0
	}
	s.windVolts = float64(sample.V) / float64(physic.Volt)
	s.windDirection = voltToDegrees(s.windVolts)
	logger.Debugf("Volt [%v], Dir [%v]", s.windVolts, s.windDirection)

	// prometheus data
	mmRainPerHour.Set(s.getMMLastHour())
	atmPresure.Set(s.pressure)
	rh.Set(s.humidity)
	temperature.Set(s.temp)
	mmRainPerMin.Set(s.getMMLastMin())
	windDirection.Set(s.windDirection)
	windspeed.Set(s.instantWindSpeed)
}

func voltToDegrees(v float64) float64 {
	// this is based on the sensor datasheet that gives a list of voltages for each direction when set up according
	// to the circuit given. Have noticed the output isn't that accurate relative to the sensor direction...
	switch {
	case v < 0.365:
		return 112.5
	case v < 0.430:
		return 67.5
	case v < 0.535:
		return 90.0
	case v < 0.760:
		return 157.5
	case v < 1.045:
		return 135.0
	case v < 1.295:
		return 202.5
	case v < 1.690:
		return 180.0
	case v < 2.115:
		return 22.5
	case v < 2.590:
		return 45.0
	case v < 3.005:
		return 247.5
	case v < 3.225:
		return 225.0
	case v < 3.635:
		return 337.5
	case v < 3.940:
		return 0
	case v < 4.185:
		return 292.5
	case v < 4.475:
		return 315.0
	default:
		return 270.0
	}
}

func (s *sensors) recordHistory() {
	for x := range time.Tick(time.Minute) {
		s.measureSensors()
		min := x.Minute()
		// store the bucket tip count for the last minute
		s.btips[min] = s.count
		// reset the bucket tip counter
		s.count = 0

		// local history - this will ultimately dissappear when prometheus and grafana are fully working
		if min == 0 {
			h := x.Hour()
			s.rain24[h] = s.getMMLastHour()
			s.humidity24[h] = s.humidity
			s.pressure24[h] = s.pressure
			s.temp24[h] = s.temp
		}
	}
}

func (s *sensors) getMMLastHour() float64 {
	total := s.count
	for _, x := range s.btips {
		total += x
	}
	return math.Round(float64(total)*mmPerBucket*100) / 100
}

func (s *sensors) getMMLastMin() float64 {
	min := time.Now().Minute()
	count := 0
	if min >= 2 {
		count = s.count + s.btips[min-1] + s.btips[min-2]
	} else if min == 1 {
		count = s.count + s.btips[0] + s.btips[59]
	} else if min == 0 {
		count = s.count + s.btips[59] + s.btips[58]
	}
	return (float64(count) / 3 * mmPerBucket)
}

/*
Measuring gusts and wind intensity

Because wind is an element that varies rapidly over very short periods of
time it is sampled at high frequency (every 0.25 sec) to capture the intensity
of gusts, or short-lived peaks in speed, which inflict greatest damage in
storms. The gust speed and direction are defined by the maximum three second
average wind speed occurring in any period.

A better measure of the overall wind intensity is defined by the average speed
and direction over the ten minute period leading up to the reporting time.
Mean wind over other averaging periods may also be calculated. A gale is
defined as a surface wind of mean speed of 34-40 knots, averaged over a period
of ten minutes. Terms such as 'severe gale', 'storm', etc are also used to
describe winds of 41 knots or greater.
*/
