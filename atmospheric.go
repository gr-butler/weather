package main

import (
	"math"
	"time"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/physic"
)

const (
	hgToPa float64 = 133.322387415
)

func (w *weatherstation) readAtmosphericSensors() {
	w.doAtmosphere()
	for range time.Tick(time.Second * 10) {
		w.doAtmosphere()
	}
}

func (w *weatherstation) doAtmosphere() {
	hiT := physic.Env{}
	if w.s.hiResT != nil {
		// if the sensor failed to read we'd get 
		if err := w.s.hiResT.Sense(&hiT); err != nil {
			logger.Errorf("MCP9808 read failed [%v]", err)
			_ = hiT.Temperature.Set("0")
		}
	}
	logger.Debugf("MCP: %8s \n", hiT.Temperature)
	
	em := physic.Env{}
	if w.s.bme != nil {
		if err := w.s.bme.Sense(&em); err != nil {
			// default values
			logger.Errorf("BME280 read failed [%v]", err)
			_ = em.Humidity.Set("0")
			_ = em.Pressure.Set("740")
		}
	}
	
	logger.Infof("BME280: Temp [%8s], Pressure [%10s] Hum [%9s]", em.Temperature, em.Pressure, em.Humidity)
	w.humidity = math.Round(float64(em.Humidity) / float64(physic.PercentRH))
	w.pressure = math.Round(float64(em.Pressure)/float64(100*physic.Pascal)*100) / 100
	w.pressureHg = math.Round(float64(em.Pressure) / (float64(physic.Pascal) * hgToPa))
	w.temp = em.Temperature.Celsius()
	w.hiResTemp = hiT.Temperature.Celsius()

	// prometheus data
	atmPresure.Set(w.pressure)
	rh.Set(w.humidity)
	temperature.Set(w.hiResTemp)
	altTemp.Set(w.temp)
}
