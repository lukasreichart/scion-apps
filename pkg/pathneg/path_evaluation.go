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

import (
	"github.com/scionproto/scion/go/lib/snet"
	"math"
)

// path_evaluation.go contains the interface and concrete implementation for path evaluation
// used by the path negotiation protocol to calculate weights for snet.Path objects.

/**
# TODO
todo: add an Example for hidden paths?
todo: we need to give some sort of identifier of the end -host on the other side or the negotiation that is currently in process otherwise we can not really react to which client this is.
 */

// PathWeight is the type the path weights are calculated in.
// currently we have selected float64 and normalize to [0,1], but could discuss if this is a little wasteful of data
type PathWeight float64

// PathEvaluator interface needs to be implemented by an object that wants to evaluate paths
// the values from 1 to 255 are path preferences, 0 is paths that should not be used at
// all and will not be sent by the path negotiation protocol.
type PathEvaluator interface {
	WeightsForPaths([]snet.Path) []PathWeight
}

// ConstantPathEvaluator returns a single constant value for all the paths, it implements
// an evaluator that just "does not care"
type ConstantPathEvaluator struct {
	constantWeight PathWeight
}

func (e *ConstantPathEvaluator) WeightsForPaths(paths []snet.Path) []PathWeight {
	weights := make([]PathWeight, len(paths))
	for i, _ := range paths {
		weights[i] = e.constantWeight
	}

	return weights
}

// RandomPathEvaluator: flips a coin with probability use-path to decide whether
// the path should be used at all and then assigns a weight between 1 and 255 to the path
type RandomPathEvaluator struct {

}

func (e *RandomPathEvaluator) WeightsForPaths(paths []snet.Path) []PathWeight {
	//weights[i] = uint8(rand.Intn(255 - 0))
	return nil
}

// ShortestPathEvaluator creates the weights so the shortest path is preferred
// the implementation for this adapts the code from pgk/appnet/path_selection.go
type ShortPathEvaluator struct {
}

// the code in this function is taken from pkg/appnet/path_selection.go and adapter
func (e *ShortPathEvaluator) WeightsForPaths(paths []snet.Path) []PathWeight {
	weights := make([]PathWeight, len(paths))

	metricFn := func(rawMetric int) PathWeight {
		hopCount := float64(rawMetric)
		midpoint := 7.0
		result := math.Exp(-(hopCount - midpoint)) / (1 + math.Exp(-(hopCount - midpoint)))
		return PathWeight(result)
	}

	for i, path := range paths {
		weights[i] = metricFn(len(path.Metadata().Interfaces))
	}

	return weights
}

// LargestMTU Evaluator
// Use the code from appnet
//type LargestMTUEvaluator struct {}
//
//func (e *LargestMTUEvaluator) WeightsForPaths(paths []snet.Path) []PathWeight {
//
//}

// InteractivePathEvaluator interactively queries the user to specify the path preferences.
// adapted the function from pkg/appnet/path_selection.go
// mainly used for debugging purposes.
type InteractivePathEvaluator struct {

}

func (e *InteractivePathEvaluator) WeightsForPaths(paths []snet.Path) []PathWeight {
	return nil
}

// Policy Language Evaluator
// Use the code from ssh connection



