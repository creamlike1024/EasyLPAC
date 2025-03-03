package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var aids = [][2]string{
	{"Default", "A0000005591010FFFFFFFF8900000100"},
	{"eSTK.me", "A06573746B6D65FFFFFFFF4953442D52"},
	{"eSIM.me", "A0000005591010000000008900000300"},
	{"5ber.eSIM", "A0000005591010FFFFFFFF8900050500"},
}

type Config struct {
	LpacDir     string
	LpacAID     string
	EXEName     string
	DriverIFID  string
	DebugHTTP   bool
	DebugAPDU   bool
	LogDir      string
	LogFilename string
	LogFile     *os.File
	AutoMode    bool
}

var ConfigInstance Config

func LoadConfig() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}
	exeDir := filepath.Dir(exePath)
	ConfigInstance.LpacDir = exeDir

	switch platform := runtime.GOOS; platform {
	case "windows":
		ConfigInstance.EXEName = "lpac.exe"
		ConfigInstance.LogDir = filepath.Join(exeDir, "log")
	case "linux":
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
		_, err = os.Stat(filepath.Join(ConfigInstance.LpacDir, ConfigInstance.EXEName))
		if err != nil {
			ConfigInstance.LpacDir = "/usr/bin"
		}
	default:
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	}
	ConfigInstance.AutoMode = true
	ConfigInstance.LpacAID = aids[0][1]

	ConfigInstance.LogFilename = fmt.Sprintf("lpac-%s.txt", time.Now().Format("20060102-150405"))
	return nil
}
