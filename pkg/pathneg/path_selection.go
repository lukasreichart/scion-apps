// Copyright 2021 ETH Zurich
// Author: Lukas Reichart <lukasre@ethz.ch>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pathneg

import "github.com/scionproto/scion/go/lib/serrors"

// path_selection.go contains the path selection algorithm that is used by the path negotiation
// to select paths based on the path preferences calculated by the PathEvaluator

// PathSelector implements the selection of paths based on the path weights
type PathSelector interface {
	// invariant len(weights1) = len(weights)2
	// num: is the number of paths that should be selected.
	SelectPaths(weights1 []PathWeight, weights2 []PathWeight, num uint) ([]int, error)
}

// currently uses heighest average voting
type pathSelector struct {}

func (p* pathSelector) SelectPaths(weights1 []PathWeight, weights2 []PathWeight, num uint) ([]int, error) {
	if len(weights1) != len(weights2) {
		return nil, serrors.New("PathNeg: the passed weights array need to have the same length")
	}

	highestAverage := PathWeight(0.0)
	index := 0

	for i, w1 := range weights1 {
		w2 := weights2[i]

		avg := (w1 + w2) / 2
		if avg > highestAverage {
			highestAverage = avg
			index = i
		}
	}
	return []int{index}, nil
}
