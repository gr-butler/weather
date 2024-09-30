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
	position int
	size     int
	data     []float64
	lock     sync.Mutex
	first    bool
}

func NewBuffer(size int) *SampleBuffer {
	b := SampleBuffer{}
	b.first = true

	b.size = size
	b.data = make([]float64, size)

	return &b
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
	}
	if b.first {
		// fill buffer
		for i := 0; i < b.size; i++ {
			b.data[i] = val
		}
		b.first = false
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

func (b *SampleBuffer) GetLast() float64 {
	index := b.position - 1
	if index < 0 {
		index += b.size
	}
	return b.data[index]
}
