package main

import (
	"fmt"
	"github.com/nxadm/tail"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Log struct {
	Type string
	Msg  string
	Time time.Time
}

type Pair struct {
	key   string
	value int
}

type TopError struct {
	Msg   string
	Count int
}

type PairList []Pair

type RenderData struct {
	AllEntriesProcessed int
	CurrentRate         int
	PeakRate            int
	SlidingWindow       int

	ErrorPerSecond int
	ErrorPercent   float64
	ErrorCount     int
	InfoPercent    float64
	InfoCount      int
	DebugPercent   float64
	DebugCount     int

	TopErrors []*TopError
}

var (
	logData      chan *Log
	renderResult chan RenderData
	start        time.Time
)

const (
	minWindowSize = 30
	medWindowSize = 60
	maxWindowSize = 120

	ErrorType = "ERROR"
	DebugType = "DEBUG"
	InfoType  = "INFO"
)

func main() {
	signalChan := make(chan os.Signal, 1)
	logData = make(chan *Log, 10000)
	renderResult = make(chan RenderData)

	//for i := 0; i < 2; i++ {
	go readLogs()
	//}
	go analyze()
	go render()

	<-signalChan
}

func readLogs() {
	t, err := tail.TailFile("test_logs.log", tail.Config{Follow: true})
	if err != nil {
		panic(err)
	}

	for line := range t.Lines {
		_, level, msg := parseLine(line.Text)
		logData <- &Log{
			Type: level,
			Msg:  msg,
			Time: line.Time,
		}
	}

}

func analyze() {
	timeSecondTicker := time.After(1 * time.Second)
	start = time.Now()
	slidingWindow := medWindowSize
	mapToAnalyze := make(map[time.Time][]Log, 120)
	for {
		select {
		case <-timeSecondTicker:
			renderData := RenderData{}
			topErrors := make(map[string]int, 100)
			if time.Now().Sub(start).Seconds() <= medWindowSize {
				slidingWindow = int(time.Now().Sub(start).Seconds())
			}
			for logTime, logArray := range mapToAnalyze {
				if int(time.Now().Sub(logTime).Seconds()) > slidingWindow {
					delete(mapToAnalyze, logTime)
					continue
				}
				for _, log := range logArray {
					renderData.AllEntriesProcessed++
					renderData.CurrentRate = renderData.AllEntriesProcessed / slidingWindow
					if renderData.PeakRate < renderData.CurrentRate {
						renderData.PeakRate = renderData.CurrentRate
					}
					renderData.SlidingWindow = slidingWindow
					switch log.Type {
					case ErrorType:
						renderData.ErrorCount++
						renderData.ErrorPerSecond = renderData.ErrorCount / slidingWindow
						renderData.ErrorPercent = (float64(renderData.ErrorCount) / float64(renderData.AllEntriesProcessed)) * 100
						topErrors[log.Msg]++
					case InfoType:
						renderData.InfoCount++
						renderData.InfoPercent = (float64(renderData.InfoCount) / float64(renderData.AllEntriesProcessed)) * 100
					case DebugType:
						renderData.DebugCount++
						renderData.DebugPercent = (float64(renderData.DebugCount) / float64(renderData.AllEntriesProcessed)) * 100
					}
				}
				renderData.TopErrors = topThree(topErrors)
			}
			if renderData.CurrentRate > 2500 {
				slidingWindow = minWindowSize
			} else if renderData.CurrentRate < 600 {
				slidingWindow = maxWindowSize
			} else {
				slidingWindow = medWindowSize
			}
			//fmt.Printf("\r %d - %d - %d", len(logData), allEntriesProcessed, len(mapToAnalyze))
			renderResult <- renderData
			timeSecondTicker = time.After(1 * time.Second)
		case log := <-logData:
			mapToAnalyze[log.Time] = append(mapToAnalyze[log.Time], Log{
				Type: log.Type,
				Msg:  log.Msg,
				Time: log.Time,
			})
		}
	}
}

func render() {
	for {
		renderData := <-renderResult
		clearScreen()
		now := time.Now()
		timeZone, _ := now.Zone()
		formattedTime := now.Format("2006-01-02 15:04:05")
		fmt.Printf("\n\n\nLog Analysis Report (Last Updated: %s %s)\n", formattedTime, timeZone)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Runtime Stats:\n• Entries Processed: %d\n• Current Rate: %d entries/sec (Peak: %d entries/sec)\n• Adaptive Window: %d sec (Adjusted from 60 sec)\n\n",
			renderData.AllEntriesProcessed, renderData.CurrentRate, renderData.PeakRate, renderData.SlidingWindow)

		fmt.Printf("Pattern Analysis:\n• ERROR: %.2f%% (%d entries)\n• INFO: %.2f%% (%d entries)\n• DEBUG: %.2f%% (%d entries)\n\n",
			renderData.ErrorPercent, renderData.ErrorCount, renderData.InfoPercent, renderData.InfoCount, renderData.DebugPercent, renderData.DebugCount)

		fmt.Printf("Dynamic Insights:\n• Error Rate: %d errors/sec\n• Top Errors:\n  1. %s (%d occurrences)\n  2. %s (%d occurrences)\n  3. %s (%d occurrences)",
			renderData.ErrorPerSecond, renderData.TopErrors[0].Msg, renderData.TopErrors[0].Count, renderData.TopErrors[1].Msg, renderData.TopErrors[1].Count, renderData.TopErrors[2].Msg, renderData.TopErrors[2].Count)
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

func (p PairList) Len() int {
	return len(p)
}
func (p PairList) Less(i, j int) bool {
	return p[i].value < p[j].value
}
func (p PairList) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func topThree(m map[string]int) []*TopError {
	pl := make(PairList, len(m))
	i := 0
	for k, v := range m {
		pl[i] = Pair{k, v}
		i++
	}

	sort.Sort(sort.Reverse(pl))
	var topKeys []*TopError
	n := len(m)
	if n > 3 {
		n = 3
	}
	for i := 0; i < n; i++ {
		topKeys = append(topKeys, &TopError{
			Msg:   pl[i].key,
			Count: pl[i].value,
		})
	}
	return topKeys
}
