package buffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddItem(t *testing.T) {
	buf := NewBuffer(10)

	buf.AddItem(1)
	buf.AddItem(1)
	buf.AddItem(1)
	buf.AddItem(1)
	buf.AddItem(1)
	buf.AddItem(1)
	buf.AddItem(1)
	buf.AddItem(1)
	buf.AddItem(1)
	buf.AddItem(1)

	a, mn, mx, s := buf.GetAverageMinMaxSum()

	assert.Equal(t, Average(1), a)
	assert.Equal(t, Minimum(1), mn)
	assert.Equal(t, Maximum(1), mx)
	assert.Equal(t, Sum(10), s)

	buf.AddItem(10)

	a, mn, mx, s = buf.GetAverageMinMaxSum()
	assert.Equal(t, Average(1.9), a)
	assert.Equal(t, Minimum(1), mn)
	assert.Equal(t, Maximum(10), mx)
	assert.Equal(t, Sum(19), s)
	buf.AddItem(5)

	a, mn, mx, s = buf.GetAverageMinMaxSum()
	assert.Equal(t, Average(2.3), a)
	assert.Equal(t, Minimum(1), mn)
	assert.Equal(t, Maximum(10), mx)
	assert.Equal(t, Sum(23), s)

	buf.AddItem(30)
	buf.AddItem(8)
	buf.AddItem(5)
	buf.AddItem(9)
	buf.AddItem(4.1)
	buf.AddItem(5)
	buf.AddItem(155)
	buf.AddItem(88)
	buf.AddItem(17)
	buf.AddItem(9)

	_, mn, _, _ = buf.GetAverageMinMaxSum()
	assert.Equal(t, Minimum(4.1), mn)

	s, mn, mx = buf.SumMinMaxLast(2)
	assert.Equal(t, Minimum(9), mn)
	assert.Equal(t, Maximum(17), mx)
	assert.Equal(t, Sum(26), s)

}

func TestAverageLast(t *testing.T) {
	buf := NewBuffer(10)

	buf.AddItem(4)
	buf.AddItem(4)
	buf.AddItem(4)
	buf.AddItem(4)
	buf.AddItem(4)
	buf.AddItem(2)
	buf.AddItem(2)
	buf.AddItem(2)
	buf.AddItem(2)
	buf.AddItem(2)

	a := buf.AverageLast(2)
	assert.Equal(t, Average(2), a)
	a = buf.AverageLast(6)
	assert.Equal(t, Average(2.3333333333333335), a)

	buf.AddItem(2)
	buf.AddItem(2)
	buf.AddItem(2)
	buf.AddItem(2)

	a = buf.AverageLast(9)
	assert.Equal(t, Average(2), a)

	a = buf.AverageLast(10)
	assert.Equal(t, Average(2.2), a)
}
