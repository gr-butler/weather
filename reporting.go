package main

import (
	"context"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/pointer2null/weather/constants"
	"github.com/pointer2null/weather/db/postgres"

	logger "github.com/sirupsen/logrus"
)

const Rd = 287.1
const g = 9.807  // gravity
const z0 = 24.71 // River aOD is 16.61, river height at 4.1m is level with the road and I'm 3m above that
const kelvin = 273.1

/*

https://wow.metoffice.gov.uk/support/dataformats

Key points:

 WOW expects an HTTP request, in the form of either GET or POST, to the following URL. When received, WOW will interpret and validate the information supplied and respond as below.

The URL to send your request to is: http://wow.metoffice.gov.uk/automaticreading? followed by a set of key/value pairs indicating pieces of data.


 All uploads must contain 4 pieces of mandatory information plus at least 1 piece of weather data.

    Site ID - siteid:
    The unique numeric id of the site
    Authentication Key - siteAuthenticationKey:
    A pin number, chosen by the user to authenticate with WOW.
    Date - dateutc:
    Each observation must have a date, in the date encoding specified below.
    Software Type - softwaretype
    The name of the software, to identify which piece of software and which version is uploading data

The date must be in the following format: YYYY-mm-DD HH:mm:ss, where ':' is encoded as %3A, and the space is encoded as either '+' or %20. An example,
valid date would be: 2011-02-29+10%3A32%3A55, for the 2nd of Feb, 2011 at 10:32:55. Note that the time is in 24 hour format. Also note that the date must be adjusted to UTC time

KEY				Description															UNIT

baromin 		Barometric Pressure (see note) 										Inch of Mercury
dailyrainin 	Accumulated rainfall so far today 									Inches
dewptf 			Outdoor Dewpoint 													Fahrenheit
humidity 		Outdoor Humidity 													0-100 %
rainin 			Accumulated rainfall since the previous observation 				Inches
soilmoisture 	% Moisture 															0-100 %
soiltempf 		Soil Temperature (10cm) 											Fahrenheit
tempf 			Outdoor Temperature 												Fahrenheit
visibility 		Visibility 															Kilometres
winddir 		Instantaneous Wind Direction 										Degrees (0-360)
windspeedmph 	Instantaneous Wind Speed 											Miles per Hour
windgustdir 	Current Wind Gust Direction (using software specific time period) 	0-360 degrees
windgustmph 	Current Wind Gust (using software specific time period) 			Miles per Hour

*/
//PressureinHg = 29.92 * ( Pressurehpa / 1013.2) = 0.02953 * Pressurehpa

const baseUrl = "http://wow.metoffice.gov.uk/automaticreading?"

type weatherData struct {
	SiteId       string `url:"siteid,omitempty"`
	AuthKey      string `url:"siteAuthenticationKey,omitempty"`
	DateString   string `url:"dateutc,omitempty"`
	SoftwareType string `url:"softwaretype,omitempty"`
	PressureHpa  float64
	TempC        float64
	RainMM       float64
	PressureIn   float64 `url:"baromin,omitempty"`
	Humidity     float64 `url:"humidity,omitempty"`
	TempF        float64 `url:"tempf,omitempty"`
	DewPointF    float64 `url:"dewptf,omitempty"`
	RainIn       float64 `url:"rainin,omitempty"`
	WindDir      float64 `url:"winddir,omitempty"`
	WindSpeedMph float64 `url:"windspeedmph,omitempty"`
	WindGustMph  float64 `url:"windgustmph,omitempty"`
}

// Reporting called as a go routine:
// * send data to the wow url every reportFreqMin mins
// * update grafana endpoints
// * update db
func (w *weatherstation) Reporting(testMode bool) {
	/*
	   Safety net for 'too many open files' issue on legacy code.
	   Set a sane timeout duration for the http.DefaultClient, to ensure idle connections are terminated.
	   Reference: https://stackoverflow.com/questions/37454236/net-http-server-too-many-open-files-error
	*/

	for t := range time.Tick(time.Minute) {
		func() {
			logger.Info("Recording data")
			data := w.prepData()

			wowsiteid, idok := os.LookupEnv("WOWSITEID")
			wowpin, pinok := os.LookupEnv("WOWPIN")

			if !(idok && pinok) {
				logger.Error("SiteId and or pin not set! WOWSITEID and WOWPIN must be set.")
			}

			// user info
			data.SiteId = wowsiteid
			data.AuthKey = wowpin

			vals, _ := query.Values(data)
			logger.Infof("Data: [%v]", vals)

			// met office
			if !testMode &&
				(t.Minute()%constants.ReportFreqMin == 0) {
				// write data to db
				logger.Info("Saving record to db")
				err := w.Db.WriteRecord(context.Background(), postgres.WriteRecordParams{
					Temperature:   data.TempC,
					Pressure:      data.PressureHpa,
					RainMm:        data.RainMM,
					WindSpeed:     data.WindSpeedMph,
					WindGust:      data.WindGustMph,
					WindDirection: data.WindDir,
				})
				if err != nil {
					logger.Errorf("Failed to write to db [%v]", err)
				}

				logger.Info("Sending data to met office")

				// Metoffice accepts a GET... which is easier so wtf
				http.DefaultClient.Timeout = time.Minute * 2
				client := http.Client{Timeout: time.Second * 30}
				resp, err := client.Get(baseUrl + vals.Encode())
				if err != nil {
					logger.Errorf("Failed to POST data [%v] \n [%v]", err, vals.Encode())
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					logger.Errorf("Failed to POST data HTTP [%v]", resp.Status)
				}
			}
		}()
	}
}

// build the map with the required data
func (w *weatherstation) prepData() *weatherData {
	wd := weatherData{}

	// Timestamp
	// go magic date is Mon Jan 2 15:04:05 MST 2006
	// "The date must be in the following format: YYYY-mm-DD HH:mm:ss"
	wd.DateString = time.Now().UTC().Format("2006-01-02+15:04:05")
	// system info
	wd.SoftwareType = version

	tempC := w.s.Atm.GetTemperature().Float64()
	wd.TempC = tempC
	tempf := ctof(tempC)

	Prom_temperature.Set(float64(tempC))

	pressure, humidity := w.s.Atm.GetHumidityAndPressure()
	wd.PressureHpa = pressure.Float64()

	Prom_humidity.Set(humidity.Float64())

	pressureInHg := pressure * constants.HPaToInHg

	rainInch := mmToIn(w.s.Rain.GetAccumulation().Float64())
	w.s.Rain.ResetAccumulation()

	windDirection := w.s.Wind.GetDirection()
	Prom_windDirection.Set(windDirection)

	windSpeed := w.s.Wind.GetSpeed()
	windGust := w.s.Wind.GetGust()

	Prom_windspeed.Set(windSpeed)
	Prom_windgust.Set(windGust)

	// rain day total would need to be added or calculated from db entries
	//rainDayInch := mmToIn(float64(s) * constants.MMPerBucketTip)

	// data
	/*
		3. Convert the average temperature to Kelvin by adding 273.1 to the Celsius value.
	*/

	tempK := tempC + kelvin

	/*
		4. Compute the scale height H = RdT/g, where Rd = 287.1 J/(kg K) and g = 9.807 m/s2.
		Be sure to record H to at least 4 significant figures.
	*/

	H := (Rd * tempK) / g

	/*
		5. Compute the sea level pressure psl from
		psl = p0 exp(z0/H)
		where p0 is the observed pressure and z0 is the altitude above sea level where you
		made your pressure observation.
	*/

	mslp := pressureInHg.Float64() * math.Exp(z0/H)
	Prom_atmPresure.Set(pressureInHg.Float64())

	wd.PressureIn = mslp
	wd.Humidity = humidity.Float64()
	wd.TempF = tempf

	//Td = T - ((100 - RH)/5.)
	dewPoint_f := ((((tempC + 273) - ((100 - (humidity.Float64())) / 5.0)) - 273) * 9 / 5.0) + 32
	wd.DewPointF = dewPoint_f

	wd.RainIn = rainInch
	wd.WindDir = windDirection
	wd.WindSpeedMph = windSpeed
	wd.WindGustMph = windGust
	return &wd
}

func ctof(c float64) float64 {
	//(0°C × 9/5) + 32 = 32°F
	return ((c * 9 / 5) + 32)
}

func mmToIn(mm float64) float64 {
	return mm / constants.MmToInch
}
