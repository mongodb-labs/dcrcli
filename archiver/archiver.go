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

package archiver

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func TarWithPatternMatch(src string, filepattern string, writers ...io.Writer) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	mw := io.MultiWriter(writers...)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			fmt.Println("ERROR: In archiving process", err)
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		regexFileName, err := regexp.Compile(filepattern)
		if err != nil {
			fmt.Println("ERROR: processing ", filepattern, " error ", err)
		}

		matched := regexFileName.MatchString(fi.Name())
		if err != nil {
			fmt.Println("ERROR: regex matching filename", err)
		}
		if !matched {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			fmt.Println("ERROR: In archiving process tar.FileInfoHeader: ", err)
			return err
		}

		header.Name = strings.TrimPrefix(
			strings.Replace(file, src, "", -1),
			string(filepath.Separator),
		)

		if err := tw.WriteHeader(header); err != nil {
			fmt.Println("ERROR: In archiving process tw.WriteHeader: ", err)
			return err
		}

		f, err := os.Open(file)
		if err != nil {
			fmt.Println("ERROR: In archiving process os.Open: ", err)
			return err
		}

		if _, err := io.Copy(tw, f); err != nil {
			fmt.Println("WARNING: In archiving process of file", fi.Name(), " io.Copy: ", err)
			return nil
		}

		f.Close()

		return nil
	})
}
