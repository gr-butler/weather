package main

import (
	"math"
	"time"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/physic"
)

const (
	//hgToPa float64 = 133.322387415
	paToInchHg float64 = 0.0002953
)

func (w *weatherstation) readAtmosphericSensors() {
	w.doAtmosphere()
	for range time.Tick(time.Minute) {
		w.doAtmosphere()
	}
}

func (w *weatherstation) doAtmosphere() {
	hiT := physic.Env{}
	
	if w.s.hiResT != nil {
		// if the sensor failed to read we'd get
		if err := w.s.hiResT.Sense(&hiT); err != nil {
			logger.Errorf("MCP9808 read failed [%v]", err)
			w.tGood = false
		} else {
			w.tGood = true
			w.tempf = hiT.Temperature.Fahrenheit()
			w.hiResTemp = hiT.Temperature.Celsius()
			// prometheus data
			temperature.Set(w.hiResTemp)
		}
	}

	em := physic.Env{}
	if w.s.bme != nil {
		if err := w.s.bme.Sense(&em); err != nil {
			// default values
			logger.Errorf("BME280 read failed [%v]", err)
			w.aGood = false
		} else {
			w.aGood = true
			w.humidity = math.Round(float64(em.Humidity) / float64(physic.PercentRH))
			w.pressure = math.Round(float64(em.Pressure)/float64(100*physic.Pascal)*100) / 100
			w.pressureInHg = (float64(em.Pressure) / (float64(physic.Pascal))) * paToInchHg
			w.temp = em.Temperature.Celsius()
			// prometheus data
			altTemp.Set(w.temp)
			atmPresure.Set(w.pressure)
			rh.Set(w.humidity)
		}
	}

	logger.Infof("HiResTemp [%.2fC], Temp [%.2fC], Pressure [%10s] Hum [%6s]", hiT.Temperature.Celsius(), em.Temperature.Celsius(), em.Pressure, em.Humidity)
}
