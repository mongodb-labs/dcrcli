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

package webui

import (
	"os"
	"path/filepath"
	"strings"
)

// productName is the preferred user-facing binary name (Data Capture Request tool).
// Releases may also ship an identical dcrcli alias for compatibility.
const productNamePreferred = "dcr"

func productName() string {
	base := filepath.Base(os.Args[0])
	base = strings.TrimSuffix(base, ".exe")
	if base == "" || base == "." {
		return productNamePreferred
	}
	return base
}
