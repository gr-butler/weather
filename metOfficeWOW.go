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
const z0 = 24.71 // River aAD is 16.61, river height at 4.1m is level with the road and I'm 3m above that
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

const reportFreqMin = 10
const tipToInch = 0.011
const baseUrl = "http://wow.metoffice.gov.uk/automaticreading?"

// MetofficeProcessor called as a go routing will send data to the wow url every reportFreqMin mins
func (w *weatherstation) MetofficeProcessor() {
	for min := range time.Tick(time.Minute) {
		if min.Minute()%reportFreqMin == 0 {
			logger.Info("Sending data to met office")
			data, err := w.prepData(min.Minute())
			if err != nil {
				logger.Errorf("Failed to process data [%v]", err)
				continue
			}
			logger.Infof("Data: [%v]", data.Encode())
			// Metoffice accepts a GET... which is easier so wth
			resp, err := http.Get(baseUrl + data.Encode())
			if err != nil {
				logger.Errorf("Failed to POST data [%v]", err)
				continue
			}
			if resp.StatusCode != 200 {
				logger.Errorf("Failed to POST data HTTP [%v]", resp.Status)
			}
		}
	}
}

// build the map with the required data
func (w *weatherstation) prepData(min int) (url.Values, error) {
	//wowData := make(map[string]string)
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
	wowData.Add("softwaretype", "GRB-Weather-0.1.0")

	// data
	/*
		3. Convert the average temperature to Kelvin by adding 273.1 to the Celsius value.
	*/

	tempK := w.hiResTemp + kelvin

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

	mslp := w.pressureInHg * math.Exp(z0/H)

	wowData.Add("baromin", fmt.Sprintf("%f", mslp))
	wowData.Add("humidity", fmt.Sprintf("%0f", w.humidity))
	// rain inches since last reading
	tips := SumLastRange(min, reportFreqMin, w.count, &w.btips)
	// 1 tip = 0.2794mm = 0.011 inch
	wowData.Add("rainin", fmt.Sprintf("%0.2f", RoundTo(2, tips*tipToInch)))
	wowData.Add("tempf", fmt.Sprintf("%0f", w.tempf))
	wowData.Add("winddir", fmt.Sprintf("%0.2f", w.windDirection))
	wowData.Add("windspeedmph", fmt.Sprintf("%0.2f", w.windSpeedAvg))
	wowData.Add("windgustmph", fmt.Sprintf("%0.2f", w.windGust))
	//Td = T - ((100 - RH)/5.)
	dewf := ((((w.hiResTemp + 273) - ((100 - (w.humidity)) / 5.0)) - 273) * 9 / 5.0) + 32
	wowData.Add("dewptf", fmt.Sprintf("%0f", dewf))
	return wowData, nil
}
