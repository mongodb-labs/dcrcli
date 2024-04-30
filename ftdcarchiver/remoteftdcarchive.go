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

package ftdcarchiver

import (
	"fmt"
	"os"

	"dcrcli/archiver"
	"dcrcli/dcroutdir"
	"dcrcli/fscopy"
	"dcrcli/mongosh"
)

type RemoteFTDCarchive struct {
	Mongo             mongosh.CaptureGetMongoData
	DiagnosticDirPath string
	FTDCArchiveFile   *os.File
	Outputdir         *dcroutdir.DCROutputDir
	TempOutputdir     *dcroutdir.DCROutputDir
	RemoteCopyJob     *fscopy.FSCopyJob
}

func (fa *RemoteFTDCarchive) getDiagnosticDataDirPath() error {
	err := fa.Mongo.RunGetCommandDiagnosticDataCollectionDirectoryPath()
	if err != nil {
		return fmt.Errorf("Error in getDiagnosticDataDirPath: %w", err)
	}

	fa.DiagnosticDirPath = fa.Mongo.Getparsedjsonoutput.String()
	fa.DiagnosticDirPath = trimQuote(fa.DiagnosticDirPath)

	return nil
}

func (fa *RemoteFTDCarchive) createFTDCTarArchiveFile() error {
	var err error
	fa.FTDCArchiveFile, err = os.Create(fa.Outputdir.Path() + "/ftdcarchive.tar.gz")
	if err != nil {
		return fmt.Errorf("Error in createFTDCTarArchiveFile: %w", err)
	}
	return nil
}

func (fa *RemoteFTDCarchive) archiveMetricsFiles() error {
	metricsFileSearchPatternString := `^metrics.*`
	err := archiver.TarWithPatternMatch(
		fa.TempOutputdir.Path(),
		metricsFileSearchPatternString,
		fa.FTDCArchiveFile,
	)
	if err != nil {
		return fmt.Errorf("Error in archiveMetricsFiles %w", err)
	}
	return nil
}

func (fa *RemoteFTDCarchive) remoteCopyFTDCfilesToTemp() error {
	// we need to setup remote copy job and then put it in motion
	fa.RemoteCopyJob.Src.Path = []byte(fa.DiagnosticDirPath)
	err := fa.RemoteCopyJob.StartCopy()
	if err != nil {
		return fmt.Errorf("Error in remoteCopyFTDCfilesToTemp %w", err)
	}
	return nil
}

func (fa *RemoteFTDCarchive) Start() error {
	err := fa.createFTDCTarArchiveFile()
	if err != nil {
		return fmt.Errorf("Error in RemoteFTDCarchive.Start: %w", err)
	}
	err = fa.getDiagnosticDataDirPath()
	if err != nil {
		return fmt.Errorf("Error in RemoteFTDCarchive.Start: %w", err)
	}

	err = fa.remoteCopyFTDCfilesToTemp()
	if err != nil {
		return fmt.Errorf("Error in RemoteFTDCarchive.Start: %w", err)
	}

	err = fa.archiveMetricsFiles()
	if err != nil {
		return fmt.Errorf("Error in RemoteFTDCarchive.Start: %w", err)
	}

	return nil
}
