/*
Package logging implements a logger.
*/
package logging

import (
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// log file paths for go server components
const (
	LogDir        = "/var/log/go/"
	ServerLogPath = LogDir + "server.log"
	LTMLogDir     = LogDir + "ltm_logs/"
	KCSLogDir     = LogDir + "kcs_logs/"
	KCSCachedDir  = "/cache/log/"
)

const (
	// DEBUG redirects log to stdout and disable auto shutdown for KCS.
	DEBUG = false
	// MOCK uses mock modules to skip actual kernel build and test.
	MOCK = false
)

// InitLogger initializes a logrus logger and writes to logfile.
// It writes to stdout if cannot open logfile.
func InitLogger(logfile string) *logrus.Entry {
	log := logrus.New()
	file, err := os.OpenFile(logfile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		log.Out = file
	} else {
		log.Out = os.Stdout
		log.WithError(err).Warn("Failed to open log file, use os.Stdout instead")
	}
	log.SetLevel(logrus.DebugLevel)
	log.SetFormatter(&logrus.TextFormatter{})
	log.SetReportCaller(true)

	if DEBUG {
		log.Out = os.Stdout
		log.SetReportCaller(false)
	}

	return logrus.NewEntry(log)
}

// CloseLog closes the log file handler. It does nothing if log writes
// to os.Stdout or os.Stderr.
func CloseLog(log *logrus.Entry) {
	if file, ok := log.Logger.Out.(*os.File); ok {
		if !strings.HasPrefix(file.Name(), "/dev") {
			file.Close()
		}
	} else if handler, ok := log.Logger.Out.(io.Closer); ok {
		handler.Close()
	}
}

// Sync flushes the log to disk file.
func Sync(log *logrus.Entry) {
	if file, ok := log.Logger.Out.(*os.File); ok {
		if !strings.HasPrefix(file.Name(), "/dev") {
			file.Sync()
		}
	}
}

// GetFile returns the log file descripter if it exists,
// and returns nil otherwise.
func GetFile(log *logrus.Entry) *os.File {
	if file, ok := log.Logger.Out.(*os.File); ok {
		return file
	}
	return nil
}
