package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	Debug = false
)

// Keyframe represents the JSON structure for keyframes.
type Keyframe struct {
	Time float64 `json:"time"`
}

// readKeyframes reads the keyframe data from a JSON file.
func readKeyframes(filePath string) ([]Keyframe, error) {
	var keyframes []Keyframe
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(fileBytes, &keyframes)
	if err != nil {
		return nil, err
	}
	return keyframes, nil
}

// checkFFmpegAvailable checks if FFmpeg is installed and available in the PATH.
// It returns the path to the FFmpeg executable if found, or an error if not found.
func checkFFmpegAvailable() (string, error) {
	var cmd *exec.Cmd

	// Use 'where' on Windows, 'which' on Unix-like systems
	if runtime.GOOS == "windows" {
		cmd = exec.Command("where", "ffmpeg")
	} else {
		cmd = exec.Command("which", "ffmpeg")
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("FFmpeg is not available: %v", err)
	}

	// The output will have the path to the ffmpeg binary
	ffmpegPath := strings.TrimSpace(out.String())

	return ffmpegPath, nil
}

func ffmpegAdjustSpeed(bpm float64, originalVideoPath string, audioPath string, outputPath string, keyframes []Keyframe) error {
	ffmpegPath, err := checkFFmpegAvailable()
	if err != nil {
		fmt.Println(err)
		return err
	}

	beatDuration := 60 / bpm
	var filterComplexParts []string
	var concatParts []string // To keep track of the labels for concatenation

	lastTime := 0.0
	for i, kf := range keyframes {
		if i == 0 && kf.Time == 0.0 {
			fmt.Println("Skipping first keyframe at time 0.")
			continue
		}

		beatNumber := round(kf.Time / beatDuration)
		nearestBeatTime := beatNumber * beatDuration

		segmentDuration := kf.Time - lastTime
		// Avoid division by zero by ensuring segmentDuration is not zero
		if segmentDuration == 0 {
			fmt.Printf("Skipping segment with zero duration at keyframe %d.\n", i)
			continue
		}

		adjustedSegmentDuration := nearestBeatTime - lastTime
		// ensure adjustedSegmentDuration is not zero to avoid NaN speed factor
		if adjustedSegmentDuration == 0 {
			fmt.Printf("Adjusted segment duration is zero at keyframe %d, adjusting to avoid NaN.\n", i)
			adjustedSegmentDuration = 0.01 // A small, non-zero value
		}

		speedFactor := segmentDuration / adjustedSegmentDuration
		fmt.Printf("Keyframe %d: Original Time = %f, Nearest Beat Time = %f, Speed Factor = %f\n", i, kf.Time, nearestBeatTime, speedFactor)

		filter := fmt.Sprintf("[0:v]trim=start=%f:end=%f,setpts=PTS-STARTPTS*%f[v%d]; ", lastTime, kf.Time, speedFactor, i)
		filterComplexParts = append(filterComplexParts, filter)
		concatParts = append(concatParts, fmt.Sprintf("[v%d]", i))

		lastTime = kf.Time
	}

	// Ensure we have segments to concatenate
	if len(concatParts) == 0 {
		return fmt.Errorf("no segments to process")
	}

	// Adding the concat filter part correctly
	filterComplexParts = append(filterComplexParts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[outv]", strings.Join(concatParts, ""), len(concatParts)))

	// Join all filter parts to form the complete filter_complex string
	filterComplex := strings.Join(filterComplexParts, "")

	// Assemble the FFmpeg command
	cmdArgs := []string{
		"-y", // Add this line to automatically overwrite files without asking
		"-i", originalVideoPath,
		"-filter_complex", filterComplex,
		"-map", "[outv]",
		"-an", // This line ensures no audio tracks are included
		outputPath,
	}

	// cmdArgs = append(cmdArgs, outputPath)
	if Debug {
		log.Println("Running FFmpeg with arguments:", cmdArgs)
	}

	// Create the FFmpeg command using the found path and assembled arguments
	cmd := exec.Command(ffmpegPath, cmdArgs...)

	if Debug {
		// Pipe the standard output and standard error of the command
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// Execute the FFmpeg command
	if err := cmd.Run(); err != nil {
		log.Printf("Error running FFmpeg with arguments: %s - %v\n", cmdArgs, err)
		return err
	}

	if audioPath != "" {
		cmdArgs = []string{
			"-y",
			"-i", outputPath, // Add the video input
			"-i", audioPath, // Add the audio input
			"-c:v", "copy", // Use the same video codec to avoid re-encoding video
			"-c:a", "copy", //
			"-strict", "experimental", // This may be required for certain audio codecs/formats
			"-map", "0:v:0", // Map the video stream from the first input (the modified video)
			"-map", "1:a:0", // Map the audio stream from the second input (the provided audio file)
		}

		// outputPath isn't just the filename so we need to edit the path to inject a prefix or suffix,
		// for instance we have .\Scene-009.mp4_sync122.mp4. We need to inject audio_ before the filename.
		withAudioOutputPath := outputPath
		dir := filepath.Dir(withAudioOutputPath)
		filename := filepath.Base(withAudioOutputPath)
		filename = strings.TrimSuffix(filename, filepath.Ext(filename))
		withAudioOutputPath = filepath.Join(dir, "audio_"+filename+filepath.Ext(withAudioOutputPath))
		cmdArgs = append(cmdArgs, withAudioOutputPath)

		// Then execute the FFmpeg command as before
		cmd := exec.Command(ffmpegPath, cmdArgs...)
		if Debug {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Run(); err != nil {
			fmt.Printf("Error running FFmpeg (injecting audio): %v\n", err)
			return err
		}
	}

	return nil
}

// Helper function to round float64 numbers
func round(f float64) float64 {
	return math.Round(f)
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: <program> BPM originalVideoPath keyframeJsonPath [audioPath]")
		os.Exit(1)
	}

	bpm, err := strconv.ParseFloat(os.Args[1], 64)
	if err != nil {
		panic(err)
	}

	originalVideoPath := os.Args[2]
	keyframeJsonPath := os.Args[3]
	var audioPath string
	if len(os.Args) >= 5 {
		audioPath = os.Args[4]
	}

	keyframes, err := readKeyframes(keyframeJsonPath)
	if err != nil {
		panic(err)
	}

	// outputPath should be the source video filename with a _sync<bpm> suffix.
	originalVideoFilename := originalVideoPath[strings.LastIndex(originalVideoPath, "/")+1:]
	originalExtension := originalVideoFilename[strings.LastIndex(originalVideoFilename, ".")+1:]
	outputPath := fmt.Sprintf("%s_sync%.0f.%s", originalVideoFilename, bpm, originalExtension)
	err = ffmpegAdjustSpeed(bpm, originalVideoPath, audioPath, outputPath, keyframes)
	if err != nil {
		fmt.Println("Failed to sync to beat:", err)
		log.Fatal(err)
	}
}
