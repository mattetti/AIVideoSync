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

// getVideoDuration retrieves the duration of the given video file in seconds.
func getVideoDuration(videoPath string) (float64, error) {
	// First, check if ffprobe is available
	ffprobePath, err := checkFFprobeAvailable()
	if err != nil {
		return 0, err // ffprobe is not available
	}

	// Construct the ffprobe command to get the duration of the video
	cmdArgs := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	}

	cmd := exec.Command(ffprobePath, cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("ffprobe error: %v", err)
	}

	// Parse the output to get the duration
	durationStr := strings.TrimSpace(out.String())
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %v", err)
	}

	return duration, nil
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

// checkFFprobeAvailable checks if FFprobe is installed and available in the PATH.
// It returns the path to the FFprobe executable if found, or an error if not found.
func checkFFprobeAvailable() (string, error) {
	var cmd *exec.Cmd

	// Use 'where' on Windows, 'which' on Unix-like systems
	if runtime.GOOS == "windows" {
		cmd = exec.Command("where", "ffprobe")
	} else {
		cmd = exec.Command("which", "ffprobe")
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("FFprobe is not available: %v", err)
	}

	// The output will have the path to the ffprobe binary
	ffprobePath := strings.TrimSpace(out.String())

	return ffprobePath, nil
}

// Adds a white pulse to a video based on the specified BPM and saves the result to a new file.
func addPulseToVideo(inputVideoPath string, bpm float64, outputVideoPath string) error {
	ffmpegPath, err := checkFFmpegAvailable()
	if err != nil {
		return fmt.Errorf("ffmpeg is not available: %v", err)
	}

	totalDuration, err := getVideoDuration(inputVideoPath)
	if err != nil {
		return fmt.Errorf("failed to get video duration: %v", err)
	}

	beatDuration := 60 / bpm
	numBeats := int(totalDuration / beatDuration)

	// Generate the base filter for the white color source
	baseFilter := fmt.Sprintf("color=c=white:s=1920x1080:r=25:d=%f[white];", totalDuration)

	// Initialize the filter complex string with the base filter
	filterComplex := baseFilter

	// Loop to generate overlay filters for each beat
	for i := 0; i <= numBeats; i++ {
		start := float64(i) * beatDuration
		// Create overlay filters for each beat
		filterComplex += fmt.Sprintf("[0:v][white]overlay='if(between(t,%f,%f),1,0)':shortest=1[v%d];", start, start+0.2, i)
	}

	// The final video stream label needs to be adjusted according to the last overlay filter applied
	finalStreamLabel := fmt.Sprintf("[v%d]", numBeats)

	cmdArgs := []string{
		"-y",
		"-i", inputVideoPath,
		"-filter_complex", filterComplex,
		"-map", finalStreamLabel,
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "22",
		outputVideoPath,
	}

	cmd := exec.Command(ffmpegPath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running ffmpeg: %v", err)
	}

	return nil
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

		beatNumber := roundToBeat(kf.Time / beatDuration)
		nearestBeatTime := beatNumber * beatDuration

		targetBeatPosition := roundToBeat(nearestBeatTime / beatDuration)

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
		fmt.Printf("Keyframe %d: %.2fs/%.2f, Nearest Beat: %.2fs/%.2f, Speed Factor = %f\n", i, kf.Time, (kf.Time / beatDuration), nearestBeatTime, targetBeatPosition, speedFactor)

		filter := fmt.Sprintf("[0:v]trim=start=%f:end=%f,setpts=PTS-STARTPTS*%f[v%d]; ", lastTime, kf.Time, speedFactor, i)
		if Debug {
			fmt.Println(filter)
		}
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

func roundToBeat(value float64) float64 {
	return math.Round(value*100) / 100
}

// estimateBPM calculates the estimated BPM from a slice of Keyframe structs, adjusting for potential whole bar durations
func estimateBPM(keyframes []Keyframe) float64 {
	if len(keyframes) < 2 {
		fmt.Println("Need at least two keyframes to estimate BPM.")
		return 0
	}

	// Calculate intervals between consecutive keyframes
	var totalInterval float64
	for i := 1; i < len(keyframes); i++ {
		interval := keyframes[i].Time - keyframes[i-1].Time
		totalInterval += interval
	}

	// Compute average interval
	averageInterval := totalInterval / float64(len(keyframes)-1)

	// Initial BPM estimation (assuming the interval is per beat)
	initialEstimate := 60 / averageInterval

	// Adjust for 4/4 rhythm if necessary (considering common multipliers for beats per bar)
	multipliers := []float64{1, 2, 4} // Represents single beat, 2 beats (half-note), and whole bar (4 beats) in 4/4 time
	closestBPM := initialEstimate
	for _, multiplier := range multipliers {
		adjustedBPM := initialEstimate * multiplier
		if adjustedBPM >= 50 && adjustedBPM <= 200 {
			closestBPM = adjustedBPM
			break
		}
	}

	return closestBPM
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

	estimatedBPM := estimateBPM(keyframes)
	fmt.Printf("Estimated original BPM based on keyframes: %.2f\n", estimatedBPM)

	// outputPath should be the source video filename with a _sync<bpm> suffix.
	originalVideoFilename := originalVideoPath[strings.LastIndex(originalVideoPath, "/")+1:]
	originalExtension := originalVideoFilename[strings.LastIndex(originalVideoFilename, ".")+1:]
	outputPath := fmt.Sprintf("%s_sync%.0f.%s", originalVideoFilename, bpm, originalExtension)
	err = ffmpegAdjustSpeed(bpm, originalVideoPath, audioPath, outputPath, keyframes)
	if err != nil {
		fmt.Println("Failed to sync to beat:", err)
		log.Fatal(err)
	}

	// outputPulsePath := fmt.Sprintf("pulsed_%s_sync%.0f.%s", originalVideoFilename, bpm, originalExtension)
	// if err := addPulseToVideo(outputPath, bpm, outputPulsePath); err != nil {
	// 	log.Fatalf("Failed to add pulse to video: %v", err)
	// }
}
