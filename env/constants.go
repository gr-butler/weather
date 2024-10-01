package env

import "time"

const (
	GPIO01 = "GPIO01"
	GPIO02 = "GPIO02" // SDA
	GPIO03 = "GPIO03" // SDC
	GPIO04 = "GPIO04"
	GPIO05 = "GPIO05"
	GPIO06 = "GPIO06"
	GPIO07 = "GPIO07"
	GPIO08 = "GPIO08" // CS   enc28j60
	GPIO09 = "GPIO09" // MISO enc28j60
	GPIO10 = "GPIO10" // MOSI enc28j60
	GPIO11 = "GPIO11" // SCK  enc28j60
	GPIO12 = "GPIO12" // rain pin
	GPIO13 = "GPIO13"
	GPIO14 = "GPIO14"
	GPIO15 = "GPIO15"
	GPIO16 = "GPIO16"
	GPIO17 = "GPIO17"
	GPIO18 = "GPIO18"
	GPIO19 = "GPIO19" // heartbeat LED
	GPIO20 = "GPIO20" // rain tip LED
	GPIO21 = "GPIO21"
	GPIO22 = "GPIO22"
	GPIO23 = "GPIO23"
	GPIO24 = "GPIO24"
	GPIO25 = "GPIO25" // INT  enc28j60
	GPIO26 = "GPIO26"
	GPIO27 = "GPIO27" // wind pin
	GPIO28 = "GPIO28"
	GPIO29 = "GPIO29"

	RainSensorIn = GPIO12
	WindSensorIn = GPIO27

	HeartbeatLed = GPIO19
	RainTipLed   = GPIO20

	MphPerTick     = 1.429
	MMPerBucketTip = 0.2794

	HPaToInHg     = 0.02953
	MmToInch      = 25.4
	ReportFreqMin = 10

	LEDFlashDuration = time.Millisecond * 50
)
