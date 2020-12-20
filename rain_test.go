package main

import (
	"testing"
)

func Test_weatherstation_getHourlyRate(t *testing.T) {
	t.Log("Starting!")
	w := weatherstation{}
	w.btips = make([]float64, 60)
	t.Log("Populating rain array")
	for i := 0; i < 60; i++ {
		w.btips[i] = float64(i)
	}
	t.Log("Checking...")
	// check we don't blow up for all offsets
	for i := 0; i < 60; i++ {
		t.Logf("Rate at [%v] is [%v]", i, w.getHourlyRate(i))
	}
	t.Log("Completed!")
}
