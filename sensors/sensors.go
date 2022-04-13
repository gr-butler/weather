package sensors

import (
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/devices/bmxx80"
	"periph.io/x/periph/experimental/devices/ads1x15"
	"periph.io/x/periph/experimental/devices/mcp9808"
)

/*
* Sensors is responsible for reading the sensors only.
* No processing save a simple count is done here
 */

type sensors struct {
	iic  IIC  // I2C bus sensors
	gpio GPIO // Direct GPIO sensor inputs
}

type GPIO struct {
	rainpin *gpio.PinIO // Rain bucket tip pin
	windpin *gpio.PinIO // Wind speed pulse
}

type IIC struct {
	atm     *bmxx80.Dev     // BME280 Pressure & humidity
	temp    *mcp9808.Dev    // MCP9808 temperature sensor
	windDir *ads1x15.PinADC // wind dir ADC output
	bus     *i2c.BusCloser
}

func (s *sensors) GetWindCount() int {
	return 0
}

func (s *sensors) GetRainCount() int {
	return 0
}
