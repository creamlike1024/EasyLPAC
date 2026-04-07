package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2/dialog"
	"github.com/mattn/go-runewidth"
)

// runLpacRaw runs lpac with custom environment, used for driver discovery
func runLpacRaw(env []string, args ...string) (json.RawMessage, string, error) {
	lpacPath := filepath.Join(ConfigInstance.LpacDir, ConfigInstance.EXEName)

	cmd := exec.Command(lpacPath, args...)
	HideCmdWindow(cmd)
	cmd.Dir = ConfigInstance.LpacDir
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil && len(bytes.TrimSpace(stderr.Bytes())) != 0 {
		return nil, "", errors.New(stderr.String())
	}

	// Parse the response - look for "driver" type for driver commands
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		var resp struct {
			Type    string `json:"type"`
			Payload struct {
				Env  string          `json:"env"`
				Data json.RawMessage `json:"data"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}
		if resp.Type == "driver" {
			return resp.Payload.Data, resp.Payload.Env, nil
		}
	}

	return nil, "", errors.New("no driver response found")
}

func runLpac(args ...string) (json.RawMessage, error) {
	StatusChan <- StatusProcess
	LockButtonChan <- true
	defer func() {
		StatusChan <- StatusReady
		LockButtonChan <- false
	}()

	// Save to LogFile
	lpacPath := filepath.Join(ConfigInstance.LpacDir, ConfigInstance.EXEName)
	command := lpacPath
	for _, arg := range args {
		command += fmt.Sprintf(" %s", arg)
	}
	if _, err := fmt.Fprintln(ConfigInstance.LogFile, command); err != nil {
		return nil, err
	}

	cmd := exec.Command(lpacPath, args...)
	HideCmdWindow(cmd)

	cmd.Dir = ConfigInstance.LpacDir

	cmd.Env = buildDriverEnv()

	if ConfigInstance.DebugHTTP {
		cmd.Env = append(cmd.Env, "LIBEUICC_DEBUG_HTTP=1")
	}
	if ConfigInstance.DebugAPDU {
		cmd.Env = append(cmd.Env, "LIBEUICC_DEBUG_APDU=1")
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	writer := io.MultiWriter(&stdout, ConfigInstance.LogFile)
	errWriter := io.MultiWriter(ConfigInstance.LogFile, &stderr)
	cmd.Stdout = writer
	cmd.Stderr = errWriter

	err := cmd.Run()
	if err != nil && len(bytes.TrimSpace(stderr.Bytes())) != 0 {
		// fixme
		// if lpac debug enabled, some lpac debug output will write to stderr if something went wrong
		// It shouldn't return here if it not pcsc error
		if strings.Contains(stderr.String(), "SCard") {
			return nil, errors.New(stderr.String())
		}
	}

	var resp LpacReturnValue

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}
		if resp.Type != "lpa" {
			continue
		}
		if resp.Payload.Code != 0 {
			var dataString string
			// 外层
			var jsonString string
			_ = json.Unmarshal(resp.Payload.Data, &jsonString)
			// 内层
			var result map[string]interface{}
			err = json.Unmarshal([]byte(jsonString), &result)
			if err != nil {
				dataString = jsonString
			} else {
				formattedJSON, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					dataString = jsonString
				} else {
					dataString = string(formattedJSON)
				}
			}
			wrapText := func(text string, maxWidth int) string {
				var wrappedText strings.Builder
				lines := strings.Split(text, "\n")
				for _, line := range lines {
					var currentWidth int
					var currentLine strings.Builder
					for _, runeValue := range line {
						// fixme 现在貌似没有必要了
						// 使用字符宽度而不是长度，让包含 CJK 字符的字符串也能正确限制显示长度
						runeWidth := runewidth.RuneWidth(runeValue)
						if currentWidth+runeWidth > maxWidth {
							wrappedText.WriteString(currentLine.String() + "\n")
							currentLine.Reset()
							currentWidth = 0
						}
						currentLine.WriteRune(runeValue)
						currentWidth += runeWidth
					}
					if currentLine.Len() > 0 {
						wrappedText.WriteString(currentLine.String() + "\n")
					}
				}
				return wrappedText.String()
			}
			return nil, fmt.Errorf("Function: %s\nData: %s", resp.Payload.Message, wrapText(dataString, 90))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return resp.Payload.Data, nil
}

// buildDriverEnv builds environment variables for the current driver
func buildDriverEnv() []string {
	env := []string{
		fmt.Sprintf("LPAC_APDU=%s", ConfigInstance.ApduBackend),
		"LPAC_HTTP=curl",
		fmt.Sprintf("LPAC_CUSTOM_ISD_R_AID=%s", ConfigInstance.LpacAID),
	}

	config := GetCurrentDriverConfig()
	if config == nil {
		return env
	}

	driver := ConfigInstance.ApduBackend

	// Add driver-specific environment variables
	switch driver {
	case "pcsc":
		if config.DriverIFID != "" {
			env = append(env, fmt.Sprintf("LPAC_APDU_PCSC_DRV_IFID=%s", config.DriverIFID))
		}
	case "at":
		if config.DriverIFID != "" {
			env = append(env, fmt.Sprintf("LPAC_APDU_PCSC_DRV_IFID=%s", config.DriverIFID))
		}
		if config.DevicePath != "" {
			env = append(env, fmt.Sprintf("LPAC_APDU_AT_DEVICE=%s", config.DevicePath))
		}
	case "at_csim":
		if config.DevicePath != "" {
			env = append(env, fmt.Sprintf("LPAC_APDU_AT_DEVICE=%s", config.DevicePath))
		}
	case "mbim":
		if config.DevicePath != "" {
			env = append(env, fmt.Sprintf("LPAC_APDU_MBIM_DEVICE=%s", config.DevicePath))
		}
		if config.UimSlot > 0 {
			env = append(env, fmt.Sprintf("LPAC_APDU_MBIM_UIM_SLOT=%d", config.UimSlot))
		}
	case "qmi", "qmi_qrtr", "uqmi":
		if config.DevicePath != "" {
			env = append(env, fmt.Sprintf("LPAC_APDU_QMI_DEVICE=%s", config.DevicePath))
		}
		if config.UimSlot > 0 {
			env = append(env, fmt.Sprintf("LPAC_APDU_QMI_UIM_SLOT=%d", config.UimSlot))
		}
	}
	// gbinder and gbinder_hidl need no additional env vars

	return env
}

// LpacDriverList queries available APDU drivers
func LpacDriverList() ([]string, error) {
	env := []string{
		"LPAC_HTTP=curl",
	}

	data, _, err := runLpacRaw(env, "driver", "list")
	if err != nil {
		return nil, err
	}

	var drivers []string
	if err = json.Unmarshal(data, &drivers); err != nil {
		return nil, err
	}
	return drivers, nil
}

// LpacDriverApduList lists available devices for the current APDU driver
func LpacDriverApduList() ([]*ApduDriver, error) {
	payload, err := runLpac("driver", "apdu", "list")
	if err != nil {
		return nil, err
	}
	var apduDrivers []*ApduDriver
	if err = json.Unmarshal(payload, &apduDrivers); err != nil {
		return nil, err
	}
	return apduDrivers, nil
}

// LpacDriverApduListForDriver lists available devices for a specific APDU driver
func LpacDriverApduListForDriver(driver string) ([]*ApduDriver, error) {
	env := []string{
		fmt.Sprintf("LPAC_APDU=%s", driver),
		"LPAC_HTTP=curl",
	}

	data, _, err := runLpacRaw(env, "driver", "apdu", "list")
	if err != nil {
		return nil, err
	}

	var apduDrivers []*ApduDriver
	if err = json.Unmarshal(data, &apduDrivers); err != nil {
		return nil, err
	}
	return apduDrivers, nil
}

func LpacChipInfo() (*EuiccInfo, error) {
	payload, err := runLpac("chip", "info")
	if err != nil {
		return nil, err
	}
	var chipInfo *EuiccInfo
	if err = json.Unmarshal(payload, &chipInfo); err != nil {
		return nil, err
	}
	return chipInfo, nil
}

func LpacProfileList() ([]*Profile, error) {
	payload, err := runLpac("profile", "list")
	if err != nil {
		return nil, err
	}
	var profiles []*Profile
	if err = json.Unmarshal(payload, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

func LpacProfileEnable(iccid string) error {
	_, err := runLpac("profile", "enable", iccid)
	if err != nil {
		return err
	}
	return nil
}

func LpacProfileDisable(iccid string) error {
	_, err := runLpac("profile", "disable", iccid)
	if err != nil {
		return err
	}
	return nil
}

func LpacProfileDelete(iccid string) error {
	_, err := runLpac("profile", "delete", iccid)
	if err != nil {
		return err
	}
	return nil
}

func LpacProfileDownload(info PullInfo) {
	args := []string{"profile", "download"}
	if info.SMDP != "" {
		args = append(args, "-s", info.SMDP)
	}
	if info.MatchID != "" {
		args = append(args, "-m", info.MatchID)
	}
	if info.ConfirmCode != "" {
		args = append(args, "-c", info.ConfirmCode)
	}
	if info.IMEI != "" {
		args = append(args, "-i", info.IMEI)
	}
	_, err := runLpac(args...)
	if err != nil {
		ShowLpacErrDialog(err)
	} else {
		notificationOrigin := Notifications
		Refresh()
		downloadNotification := findNewNotification(notificationOrigin, Notifications)
		if downloadNotification == nil {
			dialog.ShowError(errors.New("notification not found"), WMain)
			return
		}
		if ConfigInstance.AutoMode {
			var dialogText string
			if err2 := LpacNotificationProcess(downloadNotification.SeqNumber, true); err2 != nil {
				dialogText = "Download successful\nSend install notification failed\n"
			} else {
				dialogText = "Download successful\nSend install notification successful\nRemove install notification successful\n"
			}
			if err2 := RefreshNotification(); err2 != nil {
				ShowLpacErrDialog(err2)
			}
			dialog.ShowInformation("Info", dialogText, WMain)
		} else {
			dialog.ShowConfirm("Send Install Notification",
				"Download successful\nSend the install notification now?\n",
				func(b bool) {
					if b {
						go processNotificationManually(downloadNotification.SeqNumber)
					}
				}, WMain)
		}
	}
}

func LpacProfileNickname(iccid, nickname string) error {
	_, err := runLpac("profile", "nickname", iccid, nickname)
	if err != nil {
		return err
	}
	return nil
}

func LpacNotificationList() ([]*Notification, error) {
	payload, err := runLpac("notification", "list")
	if err != nil {
		return nil, err
	}
	var notifications []*Notification
	if err = json.Unmarshal(payload, &notifications); err != nil {
		return nil, err
	}
	return notifications, nil
}

func LpacNotificationProcess(seq int, remove bool) error {
	args := []string{"notification", "process"}
	if remove {
		args = append(args, "-r")
	}
	args = append(args, strconv.Itoa(seq))
	_, err := runLpac(args...)
	if err != nil {
		return err
	}
	return nil
}

func LpacNotificationRemove(seq int) error {
	_, err := runLpac("notification", "remove", strconv.Itoa(seq))
	if err != nil {
		return err
	}
	return nil
}

func LpacChipDefaultSmdp(smdp string) error {
	_, err := runLpac("chip", "defaultsmdp", smdp)
	if err != nil {
		return err
	}
	return nil
}

func LpacVersion() (string, error) {
	payload, err := runLpac("version")
	if err != nil {
		return "", err
	}
	var version string
	err = json.Unmarshal(payload, &version)
	if err != nil {
		return "", err
	}
	return version, nil
}
