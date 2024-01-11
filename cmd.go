package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2/dialog"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
)

func runLpac(args []string) (json.RawMessage, error) {
	StatusChan <- StatusProcess
	LockButtonChan <- true
	defer func() {
		StatusChan <- StatusReady
		LockButtonChan <- false
	}()

	// Save to logFile
	lpacPath := filepath.Join(ConfigInstance.lpacDir, ConfigInstance.exeName)
	command := lpacPath
	for _, arg := range args {
		command += fmt.Sprintf(" %s", arg)
	}
	if _, err := fmt.Fprintln(ConfigInstance.LogFile, command); err != nil {
		return nil, err
	}

	cmd := exec.Command(lpacPath, args...)
	HideCmdWindow(cmd)

	cmd.Dir = ConfigInstance.lpacDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	writer := io.MultiWriter(&stdout, ConfigInstance.LogFile)
	errWriter := io.MultiWriter(ConfigInstance.LogFile, &stderr)
	cmd.Stdout = writer
	cmd.Stderr = errWriter

	err := cmd.Run()
	if err != nil {
		if len(bytes.TrimSpace(stderr.Bytes())) != 0 {
			return nil, fmt.Errorf("lpac error:\n%s", stderr.String())
		}
	}

	var resp LpacReturnValue

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			return nil, err
		}
		if resp.Payload.Code != 0 {
			return nil, fmt.Errorf("lpac error: %s", resp.Payload.Message)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return resp.Payload.Data, nil
}

func LpacChipInfo() (EuiccInfo, error) {
	args := []string{"chip", "info"}
	payload, err := runLpac(args)
	if err != nil {
		return EuiccInfo{}, err
	}
	var chipInfo EuiccInfo
	if err = json.Unmarshal(payload, &chipInfo); err != nil {
		return EuiccInfo{}, err
	}
	return chipInfo, nil
}

func LpacProfileList() ([]Profile, error) {
	args := []string{"profile", "list"}
	payload, err := runLpac(args)
	if err != nil {
		return nil, err
	}
	var profiles []Profile
	if err = json.Unmarshal(payload, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

func LpacProfileEnable(iccid string) error {
	args := []string{"profile", "enable", iccid}
	_, err := runLpac(args)
	if err != nil {
		return err
	}
	return nil
}

func LpacProfileDelete(iccid string) error {
	args := []string{"profile", "delete", iccid}
	_, err := runLpac(args)
	if err != nil {
		return err
	}
	return nil
}

func LpacProfileDownload(info PullInfo) {
	args := []string{"profile", "download"}
	if info.smdp != "" {
		args = append(args, "-s", info.smdp)
	}
	if info.matchID != "" {
		args = append(args, "-m", info.matchID)
	}
	if info.confirmCode != "" {
		args = append(args, "-c", info.confirmCode)
	}
	if info.imei != "" {
		args = append(args, "-i", info.imei)
	}
	_, err := runLpac(args)
	if err != nil {
		ErrDialog(err)
	} else {
		d := dialog.NewInformation("Info", "Downloaded successfully", WMain)
		d.Show()
	}
}

func LpacProfileDiscovery() error {
	// args := []string{"profile", "discovery"}
	// payload,err := runLpac(args)
	// if err != nil {
	// 	ErrDialog(err)
	// }else {
	//
	// }
	return nil
}

func LpacProfileNickname(iccid, nickname string) error {
	args := []string{"profile", "nickname", iccid, nickname}
	_, err := runLpac(args)
	if err != nil {
		return err
	}
	return nil
}

func LpacNotificationList() ([]Notification, error) {
	args := []string{"notification", "list"}
	payload, err := runLpac(args)
	if err != nil {
		return nil, err
	}
	var notifications []Notification
	if err = json.Unmarshal(payload, &notifications); err != nil {
		return nil, err
	}
	return notifications, nil
}

func LpacNotificationProcess(seq int) error {
	args := []string{"notification", "process", strconv.Itoa(seq)}
	_, err := runLpac(args)
	if err != nil {
		return err
	}
	return nil
}

func LpacNotificationRemove(seq int) error {
	args := []string{"notification", "remove", strconv.Itoa(seq)}
	_, err := runLpac(args)
	if err != nil {
		return err
	}
	return nil
}
