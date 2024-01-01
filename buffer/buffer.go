package buffer

import (
	"math"
	"sync"
)

type Average float64
type Minimum float64
type Maximum float64
type Sum float64
type Size int
type Position int

type SampleBuffer struct {
	position    int
	size        int
	data        []float64
	lock        sync.Mutex
	autoAverage *SampleBuffer
	autoMin     *SampleBuffer
	autoMax     *SampleBuffer
	autoSum     *SampleBuffer
	first       bool
}

func NewBuffer(size int) *SampleBuffer {
	b := SampleBuffer{}
	b.first = true

	b.size = size
	b.data = make([]float64, size)
	b.autoAverage = nil
	b.autoMin = nil
	b.autoMax = nil
	b.autoSum = nil

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

func (b *SampleBuffer) GetAutoAverage() *SampleBuffer {
	return b.autoAverage
}

func (b *SampleBuffer) GetAutoMinimum() *SampleBuffer {
	return b.autoMin
}

func (b *SampleBuffer) GetAutoMaximum() *SampleBuffer {
	return b.autoMax
}

func (b *SampleBuffer) GetAutoSum() *SampleBuffer {
	return b.autoSum
}

func (b *SampleBuffer) AddItem(val float64) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.addItemNoLock(val)
}

func (b *SampleBuffer) addItemNoLock(val float64) {
	b.data[b.position] = val
	b.position += 1
	if b.position == b.size {
		b.position = 0
		b.checkAutoFill()
	}
	if b.first {
		// fill buffer
		for i := 0; i < b.size; i++ {
			b.data[i] = val
		}
		b.first = false
	}
}

func (b *SampleBuffer) checkAutoFill() {
	a, mn, mx, sm := b.getAverageMinMaxSum()
	if b.autoAverage != nil {
		b.autoAverage.addItemNoLock(float64(a))
	}
	if b.autoMin != nil {
		b.autoMin.addItemNoLock(float64(mn))
	}
	if b.autoMax != nil {
		b.autoMax.addItemNoLock(float64(mx))
	}
	if b.autoSum != nil {
		b.autoSum.addItemNoLock(float64(sm))
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
	return b.getAverageMinMaxSum()
}

func (b *SampleBuffer) SumMinMaxLast(numberOfItems int) (Sum, Minimum, Maximum) {
	b.lock.Lock()
	defer b.lock.Unlock()
	index := b.position - numberOfItems
	if index < 0 {
		// we are at the start of the array, so need to reverse wrap
		index += b.size
	}
	min := math.MaxFloat64
	max := 0.0
	sum := 0.0
	for numberOfItems > 0 {
		x := b.data[index]
		sum += x
		if x > max {
			max = x
		}
		if x < min {
			min = x
		}
		index += 1
		if index == b.size {
			index = 0
		}
		numberOfItems -= 1
	}
	return Sum(sum), Minimum(min), Maximum(max)
}

func (b *SampleBuffer) AverageLast(numberOfItems int) Average {
	b.lock.Lock()
	defer b.lock.Unlock()
	index := b.position - numberOfItems
	if index < 0 {
		// we are at the start of the array, so need to reverse wrap
		index += b.size
	}
	items := numberOfItems
	sum := 0.0
	for numberOfItems > 0 {
		sum += b.data[index]
		index += 1
		if index == b.size {
			index = 0
		}
		numberOfItems -= 1
	}
	return Average(sum / float64(items))
}

func (b *SampleBuffer) AverageLastFrom(numberOfItems int, index int) Average {
	b.lock.Lock()
	defer b.lock.Unlock()
	items := numberOfItems
	sum := 0.0
	for numberOfItems > 0 {
		sum += b.data[index]
		index += 1
		if index == b.size {
			index = 0
		}
		numberOfItems -= 1
	}
	return Average(sum / float64(items))
}

func (b *SampleBuffer) getAverageMinMaxSum() (Average, Minimum, Maximum, Sum) {
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

func (b *SampleBuffer) GetRawData() ([]float64, Size, Position) {
	b.lock.Lock()
	defer b.lock.Unlock()
	copy := b.data
	return copy, Size(b.size), Position(b.position)
}

func (b *SampleBuffer) GetSize() int {
	return b.size
}
