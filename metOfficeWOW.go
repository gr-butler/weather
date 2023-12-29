package main

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"time"

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
const hPaToInHg = 0.02953
const mmToInch = 25.4
const reportFreqMin = 10
const baseUrl = "http://wow.metoffice.gov.uk/automaticreading?"

// MetofficeProcessor called as a go routine will send data to the wow url every reportFreqMin mins
func (w *weatherstation) MetofficeProcessor() {
	/*
	   Safety net for 'too many open files' issue on legacy code.
	   Set a sane timeout duration for the http.DefaultClient, to ensure idle connections are terminated.
	   Reference: https://stackoverflow.com/questions/37454236/net-http-server-too-many-open-files-error
	*/
	http.DefaultClient.Timeout = time.Minute * 2
	client := http.Client{Timeout: time.Second * 2}
	for t := range time.Tick(time.Minute * reportFreqMin) {
		func() {
			logger.Info("Sending data to met office")
			data, err := w.prepData(t.Minute())
			if err != nil {
				logger.Errorf("Failed to process data [%v]", err)
				return
			}
			logger.Infof("Data: [%v]", data)
			sendData, ok := os.LookupEnv("SENDWOWDATA")
			if ok && sendData == "true" {
				// Metoffice accepts a GET... which is easier so wtf
				resp, err := client.Get(baseUrl + data.Encode())
				if err != nil {
					logger.Errorf("Failed to POST data [%v]", err)
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					logger.Errorf("Failed to POST data HTTP [%v]", resp.Status)
				}
			} else {
				logger.Warn("SENDWOWDATA is false.")
			}
		}()
	}
}

// build the map with the required data
func (w *weatherstation) prepData(min int) (url.Values, error) {
	wowData := url.Values{}

	wowsiteid, idok := os.LookupEnv("WOWSITEID")
	wowpin, pinok := os.LookupEnv("WOWPIN")

	if !(idok && pinok) {
		logger.Error("SiteId and or pin not set! WOWSITEID and WOWPIN must be set.")
		return nil, errors.New("SiteId and or pin not set! WOWSITEID and WOWPIN must be set.")
	}

	// user info
	wowData.Add("siteid", wowsiteid)
	wowData.Add("siteAuthenticationKey", wowpin)

	// Timestamp
	// go magic date is Mon Jan 2 15:04:05 MST 2006
	// "The date must be in the following format: YYYY-mm-DD HH:mm:ss"
	wowData.Add("dateutc", time.Now().UTC().Format("2006-01-02+15:04:05"))
	// system info
	wowData.Add("softwaretype", version)

	tempC := float64(w.data.GetBuffer(TempBuffer).AverageLast(reportFreqMin))
	pressureInHg := float64(w.data.GetBuffer(PressureBuffer).AverageLast(reportFreqMin)) * hPaToInHg
	humidity := float64(w.data.GetBuffer(HumidityBuffer).AverageLast(reportFreqMin))
	tempf := ctof(tempC)
	rainInch := mmToIn(float64(w.data.GetBuffer(RainBuffer).AverageLast(reportFreqMin)))

	windDirection := float64(w.data.GetBuffer(AverageWindDirectionBuffer).AverageLast(reportFreqMin))

	windSpeed := float64(w.data.GetBuffer(WindSpeedBuffer).AverageLast(reportFreqMin))
	windGust := float64(w.data.GetBuffer(WindGustBuffer).AverageLast(reportFreqMin))
	_, _, _, s := w.data.GetBuffer(RainBuffer).GetAutoSum().GetAverageMinMaxSum()
	rainDayInch := mmToIn(float64(s) * mmPerBucketTip)

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

	mslp := pressureInHg * math.Exp(z0/H)

	wowData.Add("baromin", fmt.Sprintf("%f", mslp))
	wowData.Add("humidity", fmt.Sprintf("%0f", humidity))

	wowData.Add("tempf", fmt.Sprintf("%0f", tempf))
	//Td = T - ((100 - RH)/5.)
	dewPoint_f := ((((tempC + 273) - ((100 - (humidity)) / 5.0)) - 273) * 9 / 5.0) + 32
	wowData.Add("dewptf", fmt.Sprintf("%0f", dewPoint_f))

	wowData.Add("rainin", fmt.Sprintf("%f", rainInch))
	wowData.Add("winddir", fmt.Sprintf("%0.2f", windDirection))
	wowData.Add("windspeedmph", fmt.Sprintf("%f", windSpeed))
	wowData.Add("windgustmph", fmt.Sprintf("%f", windGust))
	wowData.Add("dailyrainin", fmt.Sprintf("%0.2f", rainDayInch))
	return wowData, nil
}

func ctof(c float64) float64 {
	//(0°C × 9/5) + 32 = 32°F
	return ((c * 9 / 5) + 32)
}

func mmToIn(mm float64) float64 {
	return mm / mmToInch
}
