package main

import (
	"math"
	"time"

	logger "github.com/sirupsen/logrus"
	"periph.io/x/periph/conn/physic"
)

const (
	hgToPa      float64 = 133.322387415
)

func (s *weatherstation) readAtmosphericSensors() {
	s.doAtmosphere()
	for range time.Tick(time.Minute) {
		s.doAtmosphere()
	}
}

func (s *weatherstation) doAtmosphere() {
	em := physic.Env{}
		if s.bme != nil {
			s.bme.Sense(&em)
		}
		logger.Debugf("BME: %8s %10s %9s\n", em.Temperature, em.Pressure, em.Humidity)
		s.humidity = math.Round(float64(em.Humidity) / float64(physic.PercentRH))
		s.pressure = math.Round(float64(em.Pressure) / float64(100*physic.Pascal) * 100 ) / 100
		s.pressureHg = math.Round(float64(em.Pressure) / (float64(physic.Pascal) * hgToPa )) 
		s.temp = em.Temperature.Celsius()

		// prometheus data
		atmPresure.Set(s.pressure)
		rh.Set(s.humidity)
		temperature.Set(s.temp)
} 
