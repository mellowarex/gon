package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// FileExists reports whether the named file or directory exists.
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// ParseConfigFile opens json file and unmarshals
func ParseConfigFile(filename string, data interface{}) (err error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open config file: %s cause: %v", filename, err)
	}
	defer jsonFile.Close()
	jsonData, err := io.ReadAll(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read json config file: %s cause: %v", filename, err)
	}
	err = json.Unmarshal(jsonData, data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal json file: %s cause: %v", filename, err)
	}
	return
}

func GetFilename() (string, error) {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return "", errors.New("unable to get current filename")
	}
	return filename, nil
}

func GetAppPath() (string, error){
	filename, err := GetFilename()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filename), nil
}
