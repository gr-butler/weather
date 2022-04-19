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
	for range time.Tick(time.Minute) {
		t := w.s.GetTemperature()

		w.data.GetBuffer("temperature").AddItem(float64(t))
		Prom_temperature.Set(float64(t))

		hPa, rh := w.s.GetHumidityAndPressure()

		w.data.GetBuffer("humidity").AddItem(float64(rh))
		w.data.GetBuffer("pressurehPa").AddItem(float64(hPa))
		Prom_atmPresure.Set(float64(hPa))
		Prom_humidity.Set(float64(rh))

	}
}

func (w *weatherstation) setupTemperatureBuffers() {

	temperatureMinuteBuffer := utils.NewBuffer(60)

	tempAvgHourBuffer := utils.NewBuffer(24)
	temperatureMinuteBuffer.SetAutoAverage(tempAvgHourBuffer)

	tempMinHourBuffer := utils.NewBuffer(24)
	temperatureMinuteBuffer.SetAutoMinimum(tempMinHourBuffer)

	tempMaxHourBuffer := utils.NewBuffer(24)
	temperatureMinuteBuffer.SetAutoMaximum(tempMaxHourBuffer)

	w.data.AddBuffer("temperature", temperatureMinuteBuffer)
}

func (w *weatherstation) setupHumidityBuffers() {

	humidityMinuteBuffer := utils.NewBuffer(60)

	humidityAvgHourBuffer := utils.NewBuffer(24)
	humidityMinuteBuffer.SetAutoAverage(humidityAvgHourBuffer)

	humidityMinHourBuffer := utils.NewBuffer(24)
	humidityMinuteBuffer.SetAutoMinimum(humidityMinHourBuffer)

	humidityMaxHourBuffer := utils.NewBuffer(24)
	humidityMinuteBuffer.SetAutoMaximum(humidityMaxHourBuffer)

	w.data.AddBuffer("humidity", humidityMinuteBuffer)
}

func (w *weatherstation) SetupPressurehPaBuffers() {

	pressurehPaMinuteBuffer := utils.NewBuffer(60)

	pressurehPaAvgHourBuffer := utils.NewBuffer(24)
	pressurehPaMinuteBuffer.SetAutoAverage(pressurehPaAvgHourBuffer)

	pressurehPaMinHourBuffer := utils.NewBuffer(24)
	pressurehPaMinuteBuffer.SetAutoMinimum(pressurehPaMinHourBuffer)

	pressurehPaMaxHourBuffer := utils.NewBuffer(24)
	pressurehPaMinuteBuffer.SetAutoMaximum(pressurehPaMaxHourBuffer)

	w.data.AddBuffer("pressurehPa", pressurehPaMinuteBuffer)
}
