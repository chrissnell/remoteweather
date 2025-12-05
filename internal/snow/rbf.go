package snow

import (
	"math"
	"sort"
)

// RBFCost implements the Radial Basis Function cost model
// Matches ruptures' RBF kernel implementation exactly
type RBFCost struct {
	signal   []float64
	gram     [][]float64 // Pre-computed Gram (kernel) matrix
	gamma    float64
	minSize  int
	nSamples int
}

// NewRBFCost creates a new RBF cost function
func NewRBFCost(minSize int) *RBFCost {
	return &RBFCost{
		minSize: minSize,
	}
}

// computeMedianGamma calculates gamma using median heuristic
// gamma = 1 / median(pairwise_distances)
func (r *RBFCost) computeMedianGamma() float64 {
	n := len(r.signal)
	if n < 2 {
		return 1.0
	}

	// Compute all pairwise squared distances
	var distances []float64
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			diff := r.signal[i] - r.signal[j]
			dist := diff * diff
			if dist > 0 {
				distances = append(distances, dist)
			}
		}
	}

	if len(distances) == 0 {
		return 1.0
	}

	// Find median
	sort.Float64s(distances)
	median := distances[len(distances)/2]

	if median == 0 {
		return 1.0
	}

	return 1.0 / median
}

// Fit sets the signal and computes the Gram matrix
// This matches ruptures' implementation exactly
func (r *RBFCost) Fit(signal []float64) {
	r.signal = signal
	r.nSamples = len(signal)

	// Compute gamma using median heuristic
	r.gamma = r.computeMedianGamma()

	// Pre-compute full Gram (kernel) matrix
	// gram[i][j] = exp(-gamma * (signal[i] - signal[j])^2)
	r.gram = make([][]float64, r.nSamples)
	for i := 0; i < r.nSamples; i++ {
		r.gram[i] = make([]float64, r.nSamples)
		for j := 0; j < r.nSamples; j++ {
			diff := r.signal[i] - r.signal[j]
			r.gram[i][j] = math.Exp(-r.gamma * diff * diff)
		}
	}
}

// Error computes the RBF kernel-based cost for a segment
// Matches ruptures exactly: sum(diagonal) - sum(all) / length
func (r *RBFCost) Error(start, end int) float64 {
	if start >= end || start < 0 || end > r.nSamples {
		return math.Inf(1)
	}

	length := float64(end - start)

	// Extract sub-Gram matrix for segment [start:end]
	// Calculate: sum(diagonal) - sum(all elements) / length
	diagSum := 0.0
	totalSum := 0.0

	for i := start; i < end; i++ {
		for j := start; j < end; j++ {
			val := r.gram[i][j]
			totalSum += val
			if i == j {
				diagSum += val
			}
		}
	}

	cost := diagSum - totalSum/length

	return cost
}
