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

// Below code from Stever Domino blog posti https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07

// Tar takes a source and variable writers and walks 'source' writing each file
// found to the tar writer; the purpose for accepting multiple writers is to allow
// for multiple outputs (for example a file, or md5 hash)
func Tar(src string, writers ...io.Writer) error {
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	mw := io.MultiWriter(writers...)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// walk path
	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		// return on any error
		if err != nil {
			fmt.Println("ERROR: In archiving process", err)
			return err
		}

		// return on non-regular files (thanks to [kumo](https://medium.com/@komuw/just-like-you-did-fbdd7df829d3) for this suggested update)
		if !fi.Mode().IsRegular() {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			fmt.Println("ERROR: In archiving process", err)
			return err
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = strings.TrimPrefix(
			strings.Replace(file, src, "", -1),
			string(filepath.Separator),
		)

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			fmt.Println("ERROR: In archiving process", err)
			return err
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			fmt.Println("ERROR: In archiving process", err)
			return err
		}

		// copy file data into tar writer
		if _, err := io.Copy(tw, f); err != nil {
			fmt.Println("ERROR: In archiving process", err)
			return err
		}

		// manually close here after each file operation; defering would cause each file close
		// to wait until all operations have completed.
		f.Close()

		return nil
	})
}

// Tar based on file pattern
func TarWithPatternMatch(src string, filepattern string, writers ...io.Writer) error {
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	mw := io.MultiWriter(writers...)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// walk path
	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		// return on any error
		if err != nil {
			fmt.Println("ERROR: In archiving process", err)
			return err
		}

		// return on non-regular files (thanks to [kumo](https://medium.com/@komuw/just-like-you-did-fbdd7df829d3) for this suggested update)
		if !fi.Mode().IsRegular() {
			return nil
		}

		regexFileName, err := regexp.Compile(filepattern)
		if err != nil {
			fmt.Println("Error: processing ", filepattern, " error ", err)
		}

		matched := regexFileName.MatchString(fi.Name())
		if err != nil {
			fmt.Println("Error: regex matching filename", err)
		}
		if !matched {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			fmt.Println("ERROR: In archiving process tar.FileInfoHeader: ", err)
			return err
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = strings.TrimPrefix(
			strings.Replace(file, src, "", -1),
			string(filepath.Separator),
		)

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			fmt.Println("ERROR: In archiving process tw.WriteHeader: ", err)
			return err
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			fmt.Println("ERROR: In archiving process os.Open: ", err)
			return err
		}

		// copy file data into tar writer
		// Files like mongod.log and metrics.interim could be open for writing by mongod
		if _, err := io.Copy(tw, f); err != nil {
			fmt.Println("WARNING: In archiving process of file", fi.Name(), " io.Copy: ", err)
			return nil
		}

		// manually close here after each file operation; defering would cause each file close
		// to wait until all operations have completed.
		f.Close()

		return nil
	})
}
