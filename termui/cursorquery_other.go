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

//go:build !unix

package termui

import "os"

// queryCursorRow is unsupported outside unix; collection keeps the SSH hint and
// password prompt in the scrollback instead of redrawing them in place.
func queryCursorRow(_ *os.File, _ int) (int, bool) {
	return 0, false
}
