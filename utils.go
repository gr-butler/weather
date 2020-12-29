package main

// SumLastRange sums the last <size> values in the given array starting at <start>
// <current> provides a starting count if needed
func SumLastRange(start int, size int, current float64, data *[]float64) float64 {
	offset := start
	index := 0
	count := current // value for the current minute
	for i := 1; i < hourRateMin; i++ {
		index = offset - i
		if index < 0 {
			offset = len(*data) + i - 1
			index = offset - i
		}
		count += (*data)[index]
	}

	return count
}

// func RoundTo(places int, value float64) float64 {
// 	val := math.Pow(float64(places), 10)
// 	return math.Round(float64(value)*val) / val
// }
