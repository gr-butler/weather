package main

import (
	"testing"
	"time"
)

func Test_weatherstation_prepData(t *testing.T) {
	w := &weatherstation{
		btips: make([]float64, 60),
	}
	wowdata, err := w.prepData(time.Now().Minute())
	if err != nil {
		t.Errorf("Failed to set data [%v]", err)
	}
	t.Logf("URL: [%v]", wowdata.Encode())
}
