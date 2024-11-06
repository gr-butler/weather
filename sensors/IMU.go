package sensors

import (
	"math"

	"github.com/gr-butler/weather/env"
	logger "github.com/sirupsen/logrus"

	"time"

	"periph.io/x/periph/conn/i2c"
)

const (
	MPU6050_ADDRESS = 0x68
	ACCEL_XOUT_H    = 0x3B
	alpha           = 0.5
)

var (
	filteredAccelX float64
	filteredAccelY float64
	filteredAccelZ float64

	accelOffsetX int16
	accelOffsetY int16
	accelOffsetZ int16
)

type IMU struct {
	Sensor *i2c.Dev
	Ok     bool
}

type xG float64
type yG float64
type zG float64

func (x xG) Float64() float64 {
	return float64(x)
}

func (y yG) Float64() float64 {
	return float64(y)
}

func (z zG) Float64() float64 {
	return float64(z)
}

func NewIMU(bus *i2c.Bus, args env.Args) *IMU {
	i := IMU{}
	// Create a connection to the MPU6050.
	i.Sensor = &i2c.Dev{Addr: MPU6050_ADDRESS, Bus: *bus}
	i.Ok = true
	// Calibrate the accelerometer.
	logger.Info("Calibrating IMU...")
	i.calibrateAccel(1000)
	if i.Ok {
		logger.Info("...      IMU Ready")
	} else {
		logger.Info("...      IMU Failed")
		return nil
	}

	return &i
}

func (imu *IMU) calibrateAccel(samples int) {
	var totalX, totalY, totalZ int64

	for i := 0; i < samples; i++ {
		accelX, accelY, accelZ := imu.readRawAccel(false)

		totalX += int64(accelX)
		totalY += int64(accelY)
		totalZ += int64(accelZ)

		time.Sleep(10 * time.Millisecond)
		if !imu.Ok {
			return
		}
	}

	accelOffsetX = int16(totalX / int64(samples))
	accelOffsetY = int16(totalY / int64(samples)) //- 16384 // Assumes sensor is vertical, y should be -1g
	accelOffsetZ = int16(totalZ / int64(samples)) //- 16384 // Assumes sensor is flat, Z should be -1g
}

func (imu *IMU) readRawAccel(verbose bool) (int16, int16, int16) {
	write := []byte{ACCEL_XOUT_H}
	read := make([]byte, 8)

	if err := imu.Sensor.Tx(write, read); err != nil {
		logger.Errorf("IMU read failed [%v]", err)
		imu.Ok = false
		return -1, -1, -1
	}

	accelX := int16(read[0])<<8 | int16(read[1])
	accelY := int16(read[2])<<8 | int16(read[3])
	accelZ := int16(read[4])<<8 | int16(read[5])
	if verbose {
		logger.Infof("IMU raw [%v] [%v] [%v]", accelX, accelY, accelZ)
	}

	return accelX, accelY, accelZ
}

func (imu *IMU) ReadAccel(verbose bool) (xG, yG, zG) {
	accelX, accelY, accelZ := imu.readRawAccel(verbose)

	accelX -= accelOffsetX
	accelY -= accelOffsetY
	accelZ -= accelOffsetZ

	// Apply the low-pass filter.
	filteredAccelX = math.Round((alpha*float64(accelX)+(1.0-alpha)*filteredAccelX)*100) / 100
	filteredAccelY = math.Round((alpha*float64(accelY)+(1.0-alpha)*filteredAccelY)*100) / 100
	filteredAccelZ = math.Round((alpha*float64(accelZ)+(1.0-alpha)*filteredAccelZ)*100) / 100

	return xG(filteredAccelX), yG(filteredAccelY), zG(filteredAccelZ)
}
