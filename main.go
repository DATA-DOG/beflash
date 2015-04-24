package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type testRunner struct {
	wg sync.WaitGroup
	w  io.Writer
	r  io.Reader

	sync.Mutex
	errors          []error
	stepsInLine     int
	scenarios       int
	scenariosPassed int
	steps           int
	stepsPassed     int
}

func NewTestRunner() *testRunner {
	var e []error
	reader, writer := io.Pipe()
	var wg sync.WaitGroup
	return &testRunner{
		wg:              wg,
		w:               writer,
		r:               reader,
		stepsInLine:     0,
		scenarios:       0,
		scenariosPassed: 0,
		steps:           0,
		stepsPassed:     0,
		errors:          e,
	}
}

func main() {
	t := NewTestRunner()
	start := time.Now()

	features := features()
	pos := 0
	for n := len(features); n > 0; {
		size := 2
		if n < size {
			size = n
		}
		n -= size
		chunk := features[pos : size+pos]
		pos += size
		for _, feature := range chunk {
			t.wg.Add(1)
			go t.executeTest(feature)
		}
		t.wg.Wait()
	}
	//for _, test := range features {
	//t.wg.Add(1)
	//go t.executeTest(test)
	//}
	//t.wg.Wait()
	fmt.Println()
	for _, e := range t.errors {
		fmt.Println(e)
	}
	fmt.Printf("%d scenarios (\033[32m%d passed\033[0m)\n", t.scenarios, t.scenariosPassed)
	fmt.Printf("%d steps (\033[32m%d passed\033[0m)\n", t.steps, t.stepsPassed)
	fmt.Printf("Tests ran in: %s\n", time.Since(start))
}

func (t *testRunner) executeTest(test string) {
	behat := exec.Command("./bin/behat", "-f", "progress", test)
	stdout, err := behat.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	go t.proccessOutput(stdout)
	err = behat.Start()
	if err != nil {
		log.Fatal(err)
	}
	err = behat.Wait()
	if err != nil {
		t.errors = append(t.errors, fmt.Errorf("TODO: handle std err output from behat: %s", err))
	}
	t.wg.Done()
}

func (t *testRunner) proccessOutput(out io.Reader) {
	reader := bufio.NewReader(out)
	for {
		c, err := reader.ReadByte()
		switch {
		case c == '\n':
			// if we encounted two new lines in a row - steps have finished
			// and we try to parse information about runned scenarios and steps
			nextByte, err := reader.Peek(1)
			if err != nil {
				break
			}
			if nextByte[0] == '\n' {
				_, err = reader.ReadByte()
				for {
					line, err := reader.ReadBytes('\n')
					if err != nil {
						break
					}
					// TODO:
					// parse scenarios and steps info and store somewhere.
					// parse failed, skipped and undefined scenarios/steps
					scenariosRe := regexp.MustCompile("([0-9]+) scenario")
					match := scenariosRe.FindString(string(line))
					if match != "" {
						scenarios := strings.Split(match, " ")
						t.Lock()
						i, _ := strconv.Atoi(scenarios[0])
						t.scenarios += i
						t.Unlock()

						//scenarios passed
						scenariosPassedRe := regexp.MustCompile("([0-9]+) passed")
						match = scenariosPassedRe.FindString(string(line))
						if match != "" {
							passed := strings.Split(match, " ")
							t.Lock()
							i, _ := strconv.Atoi(passed[0])
							t.scenariosPassed += i
							t.Unlock()
						}
					}

					stepsRe := regexp.MustCompile("([0-9]+) step")
					match = stepsRe.FindString(string(line))
					if match != "" {
						steps := strings.Split(match, " ")
						t.Lock()
						i, _ := strconv.Atoi(steps[0])
						t.steps += i
						t.Unlock()

						//steps passed
						stepsPassedRe := regexp.MustCompile("([0-9]+) passed")
						match = stepsPassedRe.FindString(string(line))
						if match != "" {
							passed := strings.Split(match, " ")
							t.Lock()
							i, _ := strconv.Atoi(passed[0])
							t.stepsPassed += i
							t.Unlock()
						}
					}
				}
			}
			break
		case c == '.':
			if t.stepsInLine > 0 && t.stepsInLine%70 == 0 {
				fmt.Printf(" %d\n", t.stepsInLine)
			}
			fmt.Printf("\033[32m%s\033[0m", string(c))
			t.Lock()
			t.stepsInLine += 1
			t.Unlock()
			break
		case c == 'F':
			if t.stepsInLine > 0 && t.stepsInLine%70 == 0 {
				fmt.Printf(" %d\n", t.stepsInLine)
			}
			fmt.Printf("\033[31m%s\033[0m", string(c))
			t.Lock()
			t.stepsInLine += 1
			t.Unlock()
			break
		case c == '-':
			if t.stepsInLine > 0 && t.stepsInLine%70 == 0 {
				fmt.Printf(" %d\n", t.stepsInLine)
			}
			fmt.Printf("\033[36m%s\033[0m", string(c))
			t.Lock()
			t.stepsInLine += 1
			t.Unlock()
			break
		case c == 'U':
			if t.stepsInLine > 0 && t.stepsInLine%70 == 0 {
				fmt.Printf(" %d\n", t.stepsInLine)
			}
			fmt.Printf("\033[33m%s\033[0m", string(c))
			t.Lock()
			t.stepsInLine += 1
			t.Unlock()
			break
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			break
			log.Printf("Unknown error while proccessing output: %s", err)
		}
	}
}

func features() []string {
	var features []string
	err := filepath.Walk("features", func(path string, file os.FileInfo, err error) error {
		if err == nil && !file.IsDir() {
			features = append(features, path)
		}
		return err
	})
	if err != nil {
		panic("failed to walk directory: " + err.Error())
	}
	return features
}
