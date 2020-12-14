package main

import (
	"os"
	"time"

	logger "github.com/sirupsen/logrus"
)

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

// MetofficeProcessor called as a go routing will send data to the wow url every 15 mins
// on the hour, then 15, 30 and 45 past
func (w *weatherstation) MetofficeProcessor() {
	for min := range time.Tick(time.Minute) {
		if min.Minute()%15 == 0 {
			logger.Info("Sending data to met office")
			//TODO
		}
	}
}

// build the map with the required data
func (w *weatherstation) prepData() {
	wowData := make(map[string]string)

	wowsiteid, idok := os.LookupEnv("WOWSITEID")
	wowpin, pinok := os.LookupEnv("WOWPIN")

	if !(idok && pinok) {
		logger.Error("SiteId and or pin not set! WOWSITEID and WOWPIN must be set.")
		return
	}

	wowData["siteid"] = wowsiteid
	wowData["siteAuthenticationKey"] = wowpin

	// need the date in their odd format. go magic adte Mon Jan 2 15:04:05 MST 2006
	// 	The date must be in the following format: YYYY-mm-DD HH:mm:ss, where ':' is encoded as %3A, and the space is encoded as either '+' or %20. An example,
	// valid date would be: 2011-02-29+10%3A32%3A55, for the 2nd of Feb, 2011 at 10:32:55. Note that the time is in 24 hour format. Also note that the date must be adjusted to UTC time
	wowData["dateutc"] = time.Now().UTC().Format("2006-01-02+15%3A04%3A05")
	wowData["softwaretype"] = "GRB-Weather-0.1.0"
}
