package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Config struct {
	LpacDir       string
	EXEName       string
	APDUInterface string
	HTTPInterface string
	DriverIFID    string
	DebugHTTP     bool
	DebugAPDU     bool
	LogDir        string
	LogFilename   string
	LogFile       *os.File
}

var ConfigInstance Config

func LoadConfig() error {
	const lpacDirName = "lpac"
	const apduInterface = "libapduinterface_pcsc"
	const httpInterface = "libhttpinterface_curl"
	ConfigInstance.DebugAPDU = true
	ConfigInstance.DebugHTTP = true

	switch platform := runtime.GOOS; platform {
	case "windows":
		ConfigInstance.EXEName = "lpac.exe"
		ConfigInstance.APDUInterface = apduInterface + ".dll"
		ConfigInstance.HTTPInterface = httpInterface + ".dll"
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ConfigInstance.LpacDir = filepath.Join(pwd, lpacDirName)
		ConfigInstance.LogDir = filepath.Join(pwd, "log")
	case "darwin":
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.APDUInterface = apduInterface + ".dylib"
		ConfigInstance.HTTPInterface = httpInterface + ".dylib"
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		ConfigInstance.LpacDir = filepath.Join(filepath.Dir(exePath), lpacDirName)
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	default:
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.APDUInterface = apduInterface + ".so"
		ConfigInstance.HTTPInterface = httpInterface + ".so"
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ConfigInstance.LpacDir = filepath.Join(pwd, lpacDirName)
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	}

	ConfigInstance.LogFilename = fmt.Sprintf("lpac-%s.txt", time.Now().Format("20060102-150405"))
	return nil
}
