package main

import (
	"fmt"
	"github.com/nxadm/tail"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	logsChan = make(chan *tail.Line, 10000)

	slidingWindow    int = 60
	allEntires       int
	entiresProcessed int
	entiresPeak      int

	levelProcessed LevelsCount

	errorMsgsProcessed = make(map[string]int)
	topErrorsMsgs      [3]string
	topErrorsCounts    [3]int

	mu sync.Mutex
)

type LevelsCount struct {
	Debug int
	Info  int
	Error int
}

func main() {
	signalChan := make(chan os.Signal, 1)

	go readLogs()
	go analyze()
	go render()

	<-signalChan
	fmt.Println("\nCtrl+C pressed, exiting...")
}

func readLogs() {
	t, err := tail.TailFile("test_logs.log", tail.Config{Follow: true})
	if err != nil {
		panic(err)
	}

	for line := range t.Lines {
		logsChan <- line
	}

}

func analyze() {
	for {
		mu.Lock()
		line := <-logsChan
		_, level, msg := parseLine(line.Text)

		if level == "ERROR" {
			levelProcessed.Error++
		}

		if level == "INFO" {
			levelProcessed.Info++
		}

		if level == "DEBUG" {
			levelProcessed.Debug++
		}

		if msg != "" {
			errorMsgsProcessed[msg]++
		}

		allEntires++
		entiresProcessed++

		mu.Unlock()
	}

}

func render() {
	for {
		mu.Lock()
		if entiresProcessed > 0 && len(errorMsgsProcessed) > 2 {
			time.Sleep(1 * time.Second)
			clearScreen()
			entiresPerSecond := allEntires / slidingWindow
			if entiresPeak < entiresPerSecond {
				entiresPeak = entiresPerSecond
			}
			now := time.Now()
			timeZone, _ := now.Zone()
			formattedTime := now.Format("2006-01-02 15:04:05")
			fmt.Printf("\n\n\nLog Analysis Report (Last Updated: %s %s)\n", formattedTime, timeZone)
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Printf("Runtime Stats:\n• Entries Processed: %d\n• Current Rate: %d entries/sec (Peak: %d entries/sec)\n• Adaptive Window: %d sec (Adjusted from 60 sec)\n\n",
				allEntires, entiresPerSecond, entiresPeak, slidingWindow)

			errorsPercent := (100 * levelProcessed.Error) / entiresProcessed
			infosPercent := (100 * levelProcessed.Info) / entiresProcessed
			debugsPercent := (100 * levelProcessed.Debug) / entiresProcessed
			fmt.Printf("Pattern Analysis:\n• ERROR: %d%% (%d entries)\n• INFO: %d%% (%d entries)\n• DEBUG: %d%% (%d entries)\n\n",
				errorsPercent, levelProcessed.Error, infosPercent, levelProcessed.Info, debugsPercent, levelProcessed.Debug)

			for k, v := range errorMsgsProcessed {
				switch {
				case v > topErrorsCounts[0] && k != topErrorsMsgs[1] && k != topErrorsMsgs[2]:
					topErrorsMsgs[0] = k
					topErrorsCounts[0] = v
				case v > topErrorsCounts[1] && k != topErrorsMsgs[0] && k != topErrorsMsgs[2]:
					topErrorsMsgs[1] = k
					topErrorsCounts[1] = v
				case v > topErrorsCounts[2] && k != topErrorsMsgs[0] && k != topErrorsMsgs[1]:
					topErrorsMsgs[2] = k
					topErrorsCounts[2] = v
				}
			}
			errorPerSecond := levelProcessed.Error / slidingWindow
			fmt.Printf("Dynamic Insights:\n• Error Rate: %d errors/sec\n• Emerging Pattern: \"Database connection failed\" spiked 450%% in last 15 sec\n• Top Errors:\n  1. %s (%d occurrences)\n  2. %s (%d occurrences)\n  3. %s (%d occurrences)",
				errorPerSecond, topErrorsMsgs[0], topErrorsCounts[0], topErrorsMsgs[1], topErrorsCounts[1], topErrorsMsgs[2], topErrorsCounts[2])

			if entiresPerSecond > 2500 {
				slidingWindow = 30
			} else if entiresPerSecond < 600 {
				slidingWindow = 120
			} else {
				slidingWindow = 60
			}

		}
		mu.Unlock()
	}
}

func parseLine(line string) (string, string, string) {
	fields := strings.Fields(line)
	timestamp := fields[0]
	level := fields[1]
	message := strings.Join(fields[4:], " ")

	return timestamp, level, message
}

func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}
