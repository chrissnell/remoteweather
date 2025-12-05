package snow

import (
	"math"
	"sort"
)

// PeltDetector implements the PELT algorithm exactly as in ruptures
type PeltDetector struct {
	cost    *RBFCost
	minSize int
	jump    int
	signal  []float64
}

// NewPeltDetector creates a new PELT detector
func NewPeltDetector(minSize, jump int) *PeltDetector {
	return &PeltDetector{
		cost:    NewRBFCost(minSize),
		minSize: minSize,
		jump:    jump,
	}
}

// Fit sets the signal for segmentation
func (p *PeltDetector) Fit(signal []float64) {
	p.signal = signal
	p.cost.Fit(signal)
}

// segmentInternal implements the core PELT algorithm
// This is a direct port of ruptures' _seg method
func (p *PeltDetector) segmentInternal(penalty float64) map[[2]int]float64 {
	nSamples := len(p.signal)

	// Initialize partitions
	// partitions[t] contains the optimal partition of signal[0:t]
	partitions := make(map[int]map[[2]int]float64)
	partitions[0] = map[[2]int]float64{{0, 0}: 0}

	admissible := []int{}

	// Build candidate breakpoint indices
	ind := []int{}
	for k := 0; k < nSamples; k += p.jump {
		if k >= p.minSize {
			ind = append(ind, k)
		}
	}
	ind = append(ind, nSamples)

	// Main PELT recursion
	for _, bkp := range ind {
		// Add new admissible point from previous loop
		newAdmPt := int(math.Floor(float64(bkp-p.minSize)/float64(p.jump))) * p.jump
		admissible = append(admissible, newAdmPt)

		subproblems := []map[[2]int]float64{}
		validAdmissible := []int{}

		for _, t := range admissible {
			// Get left partition
			leftPartition, exists := partitions[t]
			if !exists {
				continue // no partition of 0:t exists
			}

			// Copy partition and add right segment
			tmpPartition := make(map[[2]int]float64)
			for k, v := range leftPartition {
				tmpPartition[k] = v
			}
			tmpPartition[[2]int{t, bkp}] = p.cost.Error(t, bkp) + penalty

			subproblems = append(subproblems, tmpPartition)
			validAdmissible = append(validAdmissible, t)
		}

		if len(subproblems) == 0 {
			continue
		}

		// Find optimal partition (minimum total cost)
		minIdx := 0
		minCost := sumPartition(subproblems[0])
		for i := 1; i < len(subproblems); i++ {
			cost := sumPartition(subproblems[i])
			if cost < minCost {
				minCost = cost
				minIdx = i
			}
		}
		partitions[bkp] = subproblems[minIdx]

		// Prune admissible set
		// Keep only points whose cost is within penalty of optimal
		prunedAdmissible := []int{}
		for i, t := range validAdmissible {
			if sumPartition(subproblems[i]) <= minCost+penalty {
				prunedAdmissible = append(prunedAdmissible, t)
			}
		}
		admissible = prunedAdmissible
	}

	bestPartition := partitions[nSamples]
	delete(bestPartition, [2]int{0, 0})
	return bestPartition
}

// Predict returns optimal breakpoints for given penalty
func (p *PeltDetector) Predict(penalty float64) []int {
	partition := p.segmentInternal(penalty)

	// Extract breakpoints and sort
	bkps := []int{}
	for segment := range partition {
		bkps = append(bkps, segment[1])
	}
	sort.Ints(bkps)

	return bkps
}

// sumPartition calculates total cost across all segments
func sumPartition(partition map[[2]int]float64) float64 {
	sum := 0.0
	for _, cost := range partition {
		sum += cost
	}
	return sum
}
