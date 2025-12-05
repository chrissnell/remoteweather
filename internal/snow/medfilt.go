package snow

import "sort"

// MedFilt applies median filter with zero-padding (scipy.signal.medfilt compatible)
// kernelSize must be a positive odd integer
func MedFilt(data []float64, kernelSize int) []float64 {
	if kernelSize < 1 || kernelSize%2 == 0 {
		panic("kernelSize must be positive odd integer")
	}
	n := len(data)
	if n == 0 {
		return nil
	}

	half := kernelSize / 2
	result := make([]float64, n)

	for i := 0; i < n; i++ {
		window := make([]float64, 0, kernelSize)

		for j := -half; j <= half; j++ {
			idx := i + j
			if idx < 0 || idx >= n {
				window = append(window, 0.0) // zero-padding
			} else {
				window = append(window, data[idx])
			}
		}

		// Sort and pick median
		sorted := append([]float64(nil), window...)
		sort.Float64s(sorted)
		result[i] = sorted[kernelSize/2]
	}
	return result
}
