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
		return err
	}

	fa.DiagnosticDirPath = fa.Mongo.Getparsedjsonoutput.String()
	fa.DiagnosticDirPath = trimQuote(fa.DiagnosticDirPath)
	fmt.Println(fa.DiagnosticDirPath)

	return nil
}

func (fa *RemoteFTDCarchive) createFTDCTarArchiveFile() error {
	var err error
	fa.FTDCArchiveFile, err = os.Create(fa.Outputdir.Path() + "/ftdcarchive.tar.gz")
	if err != nil {
		fmt.Println("Error: error creating archive file in outputs folder", err)
	}
	return err
}

func (fa *RemoteFTDCarchive) archiveMetricsFiles() error {
	metricsFileSearchPatternString := `^metrics.*`
	err := archiver.TarWithPatternMatch(
		fa.TempOutputdir.Path(),
		metricsFileSearchPatternString,
		fa.FTDCArchiveFile,
	)
	if err != nil {
		fmt.Println(err)
	}
	return err
}

func (fa *RemoteFTDCarchive) remoteCopyFTDCfilesToTemp() error {
	// we need to setup remote copy job and then put it in motion
	fa.RemoteCopyJob.Src.Path = []byte(fa.DiagnosticDirPath)
	err := fa.RemoteCopyJob.StartCopy()
	if err != nil {
		return err
	}
	return nil
}

func (fa *RemoteFTDCarchive) Start() error {
	err := fa.createFTDCTarArchiveFile()
	if err != nil {
		return err
	}
	err = fa.getDiagnosticDataDirPath()
	if err != nil {
		return err
	}

	err = fa.remoteCopyFTDCfilesToTemp()
	if err != nil {
		return err
	}

	err = fa.archiveMetricsFiles()
	if err != nil {
		return err
	}
	return err
}
