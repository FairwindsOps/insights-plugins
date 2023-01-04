package data

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

func isLessThan24hrs(t time.Time) bool {
	return time.Now().Sub(t) < 24*time.Hour
}

func deleteOlderFile(filePath string) (err error) {
	err = os.Remove(filePath)
	if err != nil {
		return

	}
	return
}

func readDataFromFile(fileName string) (payload FalcoOutput, err error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &payload)
	if err != nil {
		return
	}
	return
}

// Aggregate24hrsData return aggregated report for the past 24 hours
func Aggregate24hrsData(dir string) (aggregatedData []FalcoOutput, err error) {
	tmpfiles, err := ioutil.ReadDir(dir)
	if err != nil {
		return
	}

	for _, file := range tmpfiles {
		if file.Mode().IsRegular() {
			filename := filepath.Join(dir, file.Name())
			if isLessThan24hrs(file.ModTime()) {
				var output FalcoOutput
				output, err = readDataFromFile(filename)
				if err != nil {
					return
				}
				aggregatedData = append(aggregatedData, output)
			} else {
				err = deleteOlderFile(filename)
				if err != nil {
					return
				}
			}
		}
	}
	return
}
