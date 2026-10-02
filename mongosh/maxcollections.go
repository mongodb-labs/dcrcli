// Copyright 2023 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongosh

import "fmt"

// DefaultMaxCollections is the collection-walk safelimit when neither
// -max-collections nor config max_collections is set.
const DefaultMaxCollections = 2500

var maxCollections = DefaultMaxCollections

// MaxCollections returns the active safelimit injected as `_maxCollections`
// into getMongoData, mongoWellnessChecker, uniqueIndexes, and the time-series check.
func MaxCollections() int {
	return maxCollections
}

// SetMaxCollections updates the safelimit used for the rest of this process.
func SetMaxCollections(n int) error {
	if n < 1 {
		return fmt.Errorf("max-collections must be at least 1")
	}
	maxCollections = n
	return nil
}

// WithMaxCollections prepends `var _maxCollections = N;` so collection-walk
// scripts honor the active safelimit.
func WithMaxCollections(script string) string {
	return fmt.Sprintf("var _maxCollections = %d;\n", maxCollections) + script
}
