package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type testRunner struct {
	wg        sync.WaitGroup
	semaphore chan int

	sync.Mutex
	errors      []error
	stepsInLine int
	summaryInfo *summary
}

type summary struct {
	sync.Mutex
	scenarios        int
	scenariosPassed  int
	scenariosFailed  int
	scenariosSkipped int
	steps            int
	stepsPassed      int
	stepsFailed      int
	stepsSkipped     int
}

var flagConcurrencyLevel int

func init() {
	flag.IntVar(&flagConcurrencyLevel, "c", runtime.NumCPU(), "Concurrency level, defaults to number of CPUs")
	flag.IntVar(&flagConcurrencyLevel, "concurrency", runtime.NumCPU(), "Concurrency level, defaults to number of CPUs")
}

func main() {
	flag.Parse()
	t := NewTestRunner(flagConcurrencyLevel)
	start := time.Now()
	t.run()
	t.summary()
	fmt.Printf("Tests ran in: %s\n", time.Since(start))
}

func NewTestRunner(concurrencyLevel int) *testRunner {
	return &testRunner{
		wg:          sync.WaitGroup{},
		stepsInLine: 0,
		errors:      make([]error, 0),
		semaphore:   make(chan int, concurrencyLevel),
		summaryInfo: &summary{
			scenarios:        0,
			scenariosPassed:  0,
			scenariosFailed:  0,
			scenariosSkipped: 0,
			steps:            0,
			stepsPassed:      0,
			stepsFailed:      0,
			stepsSkipped:     0,
		},
	}
}

func (t *testRunner) summary() {
	fmt.Println()
	for _, e := range t.errors {
		fmt.Println(e)
	}
	fmt.Println(t.summaryInfo)
}

func (t *testRunner) run() {
	features := features()
	for _, feature := range features {
		t.wg.Add(1)
		go t.executeTest(feature)
	}
	t.wg.Wait()
}

func (t *testRunner) executeTest(test string) {
	t.semaphore <- 1
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
	<-t.semaphore
	t.wg.Done()
}

func (t *testRunner) proccessOutput(out io.Reader) {
	colorMap := map[byte]func(string) string{
		'.': green,
		'-': cyan,
		'F': red,
		'U': yellow,
	}
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
					t.summaryInfo.parseTestSummary(line)
				}
			}
			break
		case c == '.' || c == '-' || c == 'F' || c == 'U':
			t.Lock()
			if t.stepsInLine > 0 && t.stepsInLine%70 == 0 {
				fmt.Printf(" %d\n", t.stepsInLine)
			}
			fmt.Print(colorMap[c](string(c)))
			t.stepsInLine += 1
			t.Unlock()
			break
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Unknown error while proccessing output: %s", err)
			break
		}
	}
}

// TODO: add undefined steps
func (s *summary) parseTestSummary(line []byte) {
	if n, matched := parseSuiteInfo("scenario", line); matched {
		s.Lock()
		s.scenarios += n
		s.Unlock()
		if n, matched = parseSuiteInfo("passed", line); matched {
			s.Lock()
			s.scenariosPassed += n
			s.Unlock()
		}
		if n, matched = parseSuiteInfo("failed", line); matched {
			s.Lock()
			s.scenariosFailed += n
			s.Unlock()
		}
		if n, matched = parseSuiteInfo("skipped", line); matched {
			s.Lock()
			s.scenariosSkipped += n
			s.Unlock()
		}
	}

	if n, matched := parseSuiteInfo("step", line); matched {
		s.Lock()
		s.steps += n
		s.Unlock()
		if n, matched = parseSuiteInfo("passed", line); matched {
			s.Lock()
			s.stepsPassed += n
			s.Unlock()
		}
		if n, matched = parseSuiteInfo("failed", line); matched {
			s.Lock()
			s.stepsFailed += n
			s.Unlock()
		}
		if n, matched = parseSuiteInfo("skipped", line); matched {
			s.Lock()
			s.stepsSkipped += n
			s.Unlock()
		}
	}
}

func parseSuiteInfo(s string, buf []byte) (n int, matched bool) {
	re := regexp.MustCompile("([0-9]+) " + s)
	match := re.FindString(string(buf))
	if match != "" {
		splitted := strings.Split(match, " ")
		n, _ := strconv.Atoi(splitted[0])
		return n, true
	}
	return 0, false
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

func (s *summary) String() string {
	res := fmt.Sprintf("%d scenarios (%s", s.scenarios, green(fmt.Sprintf("%d passed", s.scenariosPassed)))
	if s.scenariosFailed > 0 {
		res += fmt.Sprintf(", %s", red(fmt.Sprintf("%d failed", s.scenariosFailed)))
	}
	if s.scenariosSkipped > 0 {
		res += fmt.Sprintf(", %s", cyan(fmt.Sprintf("%d skipped", s.scenariosSkipped)))
	}
	res += fmt.Sprintf(")\n")
	res += fmt.Sprintf("%d steps (%s", s.steps, green(fmt.Sprintf("%d passed", s.stepsPassed)))
	if s.stepsFailed > 0 {
		res += fmt.Sprintf(", %s", red(fmt.Sprintf("%d failed", s.stepsFailed)))
	}
	if s.stepsSkipped > 0 {
		res += fmt.Sprintf(", %s", cyan(fmt.Sprintf("%d skipped", s.stepsSkipped)))
	}
	res += fmt.Sprintf(")\n")
	return res
}
