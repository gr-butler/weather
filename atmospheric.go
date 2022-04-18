package main

import (
	"time"

	"github.com/pointer2null/weather/utils"
	logger "github.com/sirupsen/logrus"
)

func (w *weatherstation) StartAtmosphericMonitor() {
	logger.Info("Starting atmosphere monitors")
	// set the the required buffers
	w.setupTemperatureBuffers()
	w.setupHumidityBuffers()
	w.SetupPressurehPaBuffers()

	// sample and store sensor data
	for range time.Tick(time.Second) {
		t, err := w.s.GetTemperature()
		if err != nil {
			logger.Warn("No temp data at %v", time.Now().Format(time.ANSIC))
			w.data.GetBuffer("temperature").AddItem(0.0)
		} else {
			w.data.GetBuffer("temperature").AddItem(float64(t))
		}
		hPa, rh, err := w.s.GetHumidityAndPressure()
		if err != nil {
			logger.Warn("No pressure and humidity data at %v", time.Now().Format(time.ANSIC))
			w.data.GetBuffer("humidity").AddItem(0.0)
			w.data.GetBuffer("pressurehPa").AddItem(0.0)
		} else {
			w.data.GetBuffer("humidity").AddItem(float64(rh))
			w.data.GetBuffer("pressurehPa").AddItem(float64(hPa))
		}
	}
}

func (w *weatherstation) setupTemperatureBuffers() {

	temperatureSecondBuffer := utils.NewBuffer(60)
	tempAvgMinuteBuffer := utils.NewBuffer(60)
	temperatureSecondBuffer.SetAutoAverage(tempAvgMinuteBuffer)
	tempAvgHourBuffer := utils.NewBuffer(24)
	tempAvgMinuteBuffer.SetAutoAverage(tempAvgHourBuffer)

	tempMinMinuteBuffer := utils.NewBuffer(60)
	temperatureSecondBuffer.SetAutoMinimum(tempMinMinuteBuffer)
	tempMinHourBuffer := utils.NewBuffer(24)
	tempMinMinuteBuffer.SetAutoMinimum(tempMinHourBuffer)

	tempMaxMinuteBuffer := utils.NewBuffer(60)
	temperatureSecondBuffer.SetAutoMaximum(tempMaxMinuteBuffer)
	tempMaxHourBuffer := utils.NewBuffer(24)
	tempMaxMinuteBuffer.SetAutoMaximum(tempMaxHourBuffer)

	w.data.AddBuffer("temperature", temperatureSecondBuffer)
}

func (w *weatherstation) setupHumidityBuffers() {

	humiditySecondBuffer := utils.NewBuffer(60)
	humidityAvgMinuteBuffer := utils.NewBuffer(60)
	humiditySecondBuffer.SetAutoAverage(humidityAvgMinuteBuffer)
	humidityAvgHourBuffer := utils.NewBuffer(24)
	humidityAvgMinuteBuffer.SetAutoAverage(humidityAvgHourBuffer)

	humidityMinMinuteBuffer := utils.NewBuffer(60)
	humiditySecondBuffer.SetAutoMinimum(humidityMinMinuteBuffer)
	humidityMinHourBuffer := utils.NewBuffer(24)
	humidityMinMinuteBuffer.SetAutoMinimum(humidityMinHourBuffer)

	humidityMaxMinuteBuffer := utils.NewBuffer(60)
	humiditySecondBuffer.SetAutoMaximum(humidityMaxMinuteBuffer)
	humidityMaxHourBuffer := utils.NewBuffer(24)
	humidityMaxMinuteBuffer.SetAutoMaximum(humidityMaxHourBuffer)

	w.data.AddBuffer("humidity", humiditySecondBuffer)
}

func (w *weatherstation) SetupPressurehPaBuffers() {

	pressurehPaSecondBuffer := utils.NewBuffer(60)
	pressurehPaAvgMinuteBuffer := utils.NewBuffer(60)
	pressurehPaSecondBuffer.SetAutoAverage(pressurehPaAvgMinuteBuffer)
	pressurehPaAvgHourBuffer := utils.NewBuffer(24)
	pressurehPaAvgMinuteBuffer.SetAutoAverage(pressurehPaAvgHourBuffer)

	pressurehPaMinMinuteBuffer := utils.NewBuffer(60)
	pressurehPaSecondBuffer.SetAutoMinimum(pressurehPaMinMinuteBuffer)
	pressurehPaMinHourBuffer := utils.NewBuffer(24)
	pressurehPaMinMinuteBuffer.SetAutoMinimum(pressurehPaMinHourBuffer)

	pressurehPaMaxMinuteBuffer := utils.NewBuffer(60)
	pressurehPaSecondBuffer.SetAutoMaximum(pressurehPaMaxMinuteBuffer)
	pressurehPaMaxHourBuffer := utils.NewBuffer(24)
	pressurehPaMaxMinuteBuffer.SetAutoMaximum(pressurehPaMaxHourBuffer)

	w.data.AddBuffer("pressurehPa", pressurehPaSecondBuffer)
}
