package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const LpacDir = "lpac"
const LogDir = "log"

type Config struct {
	lpacDir       string
	exeName       string
	apduInterface string
	httpInterface string
	logDir        string
	logFilename   string
	LogFile       *os.File
}

var ConfigInstance Config

func LoadConfig() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	ConfigInstance.lpacDir = filepath.Join(pwd, LpacDir)
	switch platform := runtime.GOOS; platform {
	case "windows":
		ConfigInstance.exeName = "lpac.exe"
		ConfigInstance.apduInterface = "libapduinterface_pcsc.dll"
		ConfigInstance.httpInterface = "libapduinterface_curl.dll"
	case "darwin":
		ConfigInstance.exeName = "lpac"
		ConfigInstance.apduInterface = "libapduinterface_pcsc.dylib"
		ConfigInstance.httpInterface = "libapduinterface_curl.dylib"
	default:
		ConfigInstance.exeName = "lpac"
		ConfigInstance.apduInterface = "libapduinterface_pcsc.so"
		ConfigInstance.httpInterface = "libapduinterface_curl.so"
	}

	now := time.Now()
	ConfigInstance.logFilename = fmt.Sprintf("output-%s.txt", now.Format("20060102-150405"))
	ConfigInstance.logDir = filepath.Join(pwd, LogDir)
	return nil
}
