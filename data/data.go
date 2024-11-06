package data

import "github.com/gr-butler/weather/buffer"

// holder and processor for all the data being produced but the sensors

type WeatherData struct {
	buffers map[string]*buffer.SampleBuffer
}

func CreateWeatherData() *WeatherData {
	wd := WeatherData{}

	wd.buffers = make(map[string]*buffer.SampleBuffer)

	return &wd
}

func (wd *WeatherData) AddBuffer(name string, b *buffer.SampleBuffer) {
	wd.buffers[name] = b
}

func (wd *WeatherData) GetBuffer(name string) *buffer.SampleBuffer {
	return wd.buffers[name]
}
