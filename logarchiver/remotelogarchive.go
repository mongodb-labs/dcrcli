package logarchiver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dcrcli/archiver"
	"dcrcli/dcroutdir"
	"dcrcli/fscopy"
	"dcrcli/mongosh"
)

type RemoteMongoDLogarchive struct {
	Mongo              mongosh.CaptureGetMongoData
	LogPath            string
	LogArchiveFile     *os.File
	LogDir             string
	CurrentLogFileName string
	LogDestination     string
	Outputdir          *dcroutdir.DCROutputDir
	TempOutputdir      *dcroutdir.DCROutputDir
	RemoteCopyJob      *fscopy.FSCopyJobWithPattern
}

func (rla *RemoteMongoDLogarchive) getLogPathAndSetCurrentLogFileName() error {
	err := rla.Mongo.RunGetMongoDLogDetails()
	if err != nil {
		return err
	}

	// need to handle case where logAppend: boolean is there
	var systemLogOutput map[string]interface{}
	err = json.Unmarshal(rla.Mongo.Getparsedjsonoutput.Bytes(), &systemLogOutput)
	if err != nil {
		return err
	}
	if systemLogOutput["destination"].(string) == "file" {
		rla.LogDestination = "file"
		rla.LogPath = trimQuote(systemLogOutput["path"].(string))
		rla.LogDir = filepath.Dir(rla.LogPath)
		rla.CurrentLogFileName = filepath.Base(rla.LogPath)
		fmt.Println("The mongod log file path is: ", rla.LogDir)
	}

	return nil
}

func (rla *RemoteMongoDLogarchive) createMongodTarArchiveFile() error {
	var err error
	rla.LogArchiveFile, err = os.Create(rla.Outputdir.Path() + "/logarchive.tar.gz")
	fmt.Println("Estimating log path will then archive to:", rla.LogArchiveFile.Name())
	if err != nil {
		fmt.Println("Error: error creating archive file in outputs folder", err)
	}
	return err
}

func (rla *RemoteMongoDLogarchive) archiveLogFiles() error {
	var err error
	// Define search pattern based on latest mongod log file name
	fileSearchPatterString := `^` + rla.CurrentLogFileName + `.*`

	// Debug messages
	// fmt.Println("la.Logdir", la.LogDir)
	// fmt.Println("fileSearchPatterString", fileSearchPatterString)

	err = archiver.TarWithPatternMatch(
		rla.TempOutputdir.Path(),
		fileSearchPatterString,
		rla.LogArchiveFile,
	)
	if err != nil {
		fmt.Println(err)
	}
	return err
}

func (rla *RemoteMongoDLogarchive) remoteCopyLogFilesToTemp() error {
	rla.RemoteCopyJob.CopyJobDetails.Src.Path = []byte(rla.LogDir)
	rla.RemoteCopyJob.CurrentFileName = rla.CurrentLogFileName

	err := rla.RemoteCopyJob.StartCopyWithPattern()
	if err != nil {
		return err
	}
	return nil
}

func (rla *RemoteMongoDLogarchive) Start() error {
	var err error

	err = rla.getLogPathAndSetCurrentLogFileName()
	if err != nil {
		return err
	}
	// return early if the mongod log destination is not file
	if rla.LogDestination != "file" {
		fmt.Println("WARNING: MongoDLogArchive only works for systemLog:file")
		return nil
	}

	err = rla.createMongodTarArchiveFile()
	if err != nil {
		return err
	}

	err = rla.remoteCopyLogFilesToTemp()
	if err != nil {
		return err
	}

	err = rla.archiveLogFiles()
	if err != nil {
		return err
	}
	return err
}
