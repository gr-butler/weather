package utils

import (
	"math"
	"sync"
)

type Average float64
type Minimum float64
type Maximum float64
type Sum float64

type SampleBuffer struct {
	position    int
	size        int
	data        []float64
	lock        sync.Mutex
	autoAverage *SampleBuffer
	autoMin     *SampleBuffer
	autoMax     *SampleBuffer
	autoSum     *SampleBuffer
}

func NewBuffer(size int) *SampleBuffer {
	b := SampleBuffer{
		position:    0,
		size:        size,
		data:        make([]float64, size),
		autoAverage: nil,
		autoMin:     nil,
		autoMax:     nil,
		autoSum:     nil,
	}
	return &b
}

func (b *SampleBuffer) SetAutoAverage(buf *SampleBuffer) {
	b.autoAverage = buf
}

func (b *SampleBuffer) SetAutoMinimum(buf *SampleBuffer) {
	b.autoMin = buf
}

func (b *SampleBuffer) SetAutoMaximum(buf *SampleBuffer) {
	b.autoMax = buf
}

func (b *SampleBuffer) SetAutoSum(buf *SampleBuffer) {
	b.autoSum = buf
}

func (b *SampleBuffer) AddItem(val float64) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.data[b.position] = val
	b.position += 1
	if b.position == b.size {
		b.position = 0
		b.checkAutoFill()
	}
}

func (b *SampleBuffer) checkAutoFill() {
	a, mn, mx, sm := b.GetAverageMinMaxSum()
	if b.autoAverage != nil {
		b.autoAverage.AddItem(float64(a))
	}
	if b.autoMin != nil {
		b.autoMin.AddItem(float64(mn))
	}
	if b.autoMax != nil {
		b.autoMax.AddItem(float64(mx))
	}
	if b.autoSum != nil {
		b.autoSum.AddItem(float64(sm))
	}
}

func (b *SampleBuffer) AddValueToCurrentItem(val float64) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.data[b.position] += val
}

func (b *SampleBuffer) GetAverageMinMaxSum() (Average, Minimum, Maximum, Sum) {
	b.lock.Lock()
	defer b.lock.Unlock()
	min := math.MaxFloat64
	max := 0.0
	sum := 0.0

	for _, x := range b.data {
		if x > max {
			max = x
		}
		if x < min {
			min = x
		}
		sum += x
	}

	return Average((sum / float64(b.size))), Minimum(min), Maximum(max), Sum(sum)
}
