package utils

import (
	"math"
	"sync"
)

type CircularBuffer struct {
	position int
	size     int
	data     []int
	lock     sync.Mutex
}

func NewBuffer(size int) *CircularBuffer {
	b := CircularBuffer{
		position: 0,
		size:     size,
		data:     make([]int, size),
	}
	return &b
}

func (b *CircularBuffer) addItem(val int) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.data[b.position] = val
	b.position += 1
	if b.position == b.size {
		b.position = 0
	}
}

func (b *CircularBuffer) getAverageMinMaxSum() (float64, int, int, int) {
	b.lock.Lock()
	defer b.lock.Unlock()
	min := math.MaxInt
	max := 0
	sum := 0

	for _, x := range b.data {
		if x > max {
			max = x
		}
		if x < min {
			min = x
		}
		sum += x
	}

	return (float64(sum) / float64(b.size)), min, max, sum
}
