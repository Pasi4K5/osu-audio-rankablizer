package main

import (
	"encoding/json"
	"github.com/alecthomas/kong"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"log"
	"os"
	"strconv"
)

const tmpDir = "./.oar_tmp"
const tmpFile = tmpDir + "/tmp.ogg"
const minInputBitrateBps = 192000

var CLI struct {
	MaxBitrate           int     `short:"b" help:"Maximum allowed bitrate in bps." default:"208000"`
	MaxQualityDifference float64 `short:"q" help:"Maximum allowed quality difference between target bitrate and actual bitrate." default:"0.000001"`
	OutputPath           string  `short:"o" help:"Output audio file path." default:"output.ogg"`

	InputPath string `arg:"" help:"Input audio file path."`
}

func main() {
	defer clearWorkspace()

	kong.Parse(&CLI)

	inputBitrate := getBitrate(CLI.InputPath, getDurationSeconds(CLI.InputPath))

	if inputBitrate <= minInputBitrateBps {
		log.Panicf(
			"Mininum allowed bitrate is %d kbps. '%s' has %f kbps",
			minInputBitrateBps/1000,
			CLI.InputPath,
			inputBitrate/1000,
		)
	}

	var qLow, qHigh = 1.0, 10.0
	maxBitrate := float64(CLI.MaxBitrate)

	initWorkspace()

	durationSec := getDurationSeconds(CLI.InputPath)

	convert(CLI.InputPath, qLow)
	bitrateLow := getBitrate(tmpFile, durationSec)

	convert(CLI.InputPath, qHigh)
	bitrateHigh := getBitrate(tmpFile, durationSec)

	for qHigh-qLow > CLI.MaxQualityDifference {
		m := (qHigh - qLow) / (bitrateHigh - bitrateLow)
		b := qLow - m*bitrateLow
		qEst := m*maxBitrate + b

		convert(CLI.InputPath, qEst)
		bitrate := getBitrate(tmpFile, durationSec)

		log.Printf("Bitrate at quality '%f': %.0f kbps", qEst, bitrate)

		if bitrate > maxBitrate {
			qHigh = qEst
		} else {
			qLow = qEst
		}
	}

	err := os.Rename(tmpFile, CLI.OutputPath)
	if err != nil {
		log.Panicf("Error moving output file '%s' to '%s'", tmpFile, "output.ogg")
	}

	println("Done!")
}

func convert(inputPath string, quality float64) {
	err := ffmpeg.Input(inputPath).
		Output(tmpFile, ffmpeg.KwArgs{
			"q:a": quality,
		}).
		OverWriteOutput().
		Run()

	if err != nil {
		log.Panicf("Error converting input file '%s'", inputPath)
	}
}

func initWorkspace() {
	err := os.MkdirAll(tmpDir, os.ModePerm)

	if err != nil {
		log.Panicf("Error creating directory '%s'", tmpDir)
	}
}

func clearWorkspace() {
	_ = os.RemoveAll(tmpDir)
}

func getBitrate(filePath string, durationSec float64) float64 {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Panicf("Error getting file info '%s'", filePath)
	}

	fileSize := fileInfo.Size()

	return float64(fileSize*8) / durationSec
}

func getDurationSeconds(filePath string) float64 {
	s, err := ffmpeg.Probe(filePath)
	if err != nil {
		log.Panicf("Error probing file '%s'", filePath)
	}

	var jsonData map[string]interface{}
	err = json.Unmarshal([]byte(s), &jsonData)

	val, _ := strconv.ParseFloat(jsonData["format"].(map[string]interface{})["duration"].(string), 64)

	return val
}
