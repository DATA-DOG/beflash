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
	"strconv"
	"strings"
	"sync"
	"time"
)

type testRunner struct {
	wg          sync.WaitGroup
	semaphore   chan int
	summaryInfo *summary
	errors      []error
	extras      []string // failed, undefined step information

	sync.Mutex
	stepsInLine int
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

var cfg config

func main() {
	flag.Parse()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	t := NewTestRunner()
	t.Run()
}

func NewTestRunner() *testRunner {
	return &testRunner{
		wg:          sync.WaitGroup{},
		stepsInLine: 0,
		errors:      make([]error, 0),
		semaphore:   make(chan int, cfg.concurrencyLevel),
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

func (t *testRunner) Run() {
	start := time.Now()
	features := t.features()
	for _, feature := range features {
		t.wg.Add(1)
		go t.executeTest(feature)
	}
	t.wg.Wait()
	t.summary()
	fmt.Printf("Tests ran in: %s\n", time.Since(start))
}

func (t *testRunner) summary() {
	fmt.Println("\n")
	for _, extra := range t.extras {
		fmt.Println(extra)
	}
	// for _, e := range t.errors {
	// 	fmt.Println(e)
	// }
	fmt.Println(t.summaryInfo)
}

func (t *testRunner) executeTest(test string) {
	t.semaphore <- 1
	behat := exec.Command(cfg.binPath, "-f", "progress", test)
	stdout, err := behat.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = behat.Start()
	if err != nil {
		log.Fatal(err)
	}
	t.proccessOutput(stdout)
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
		switch c {
		case '\n':
			// if we encounted two new lines in a row - steps have finished
			// and we try to parse information about runned scenarios and steps
			nextByte, err := reader.Peek(1)
			if err != nil {
				break
			}
			if nextByte[0] == '\n' {
				_, err = reader.ReadByte()
				t.parseExtras(reader)
				for {
					line, err := reader.ReadBytes('\n')
					if err != nil {
						break
					}
					t.summaryInfo.parseTestSummary(line)
				}
			}
			break
		case '.', '-', 'F', 'U':
			t.Lock()
			if t.stepsInLine%70 == 0 && t.stepsInLine > 0 {
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

func (t *testRunner) parseExtras(reader *bufio.Reader) {
	next, err := reader.Peek(1)
	if err != nil {
		return
	}
	if next[0] != '-' {
		return
	}

	var lines []string
	for {
		next, err = reader.Peek(1)
		if err != nil {
			break
		}
		// check if extras
		if next[0] != ' ' && next[0] != '-' {
			break
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}
		lines = append(lines, red(string(line)))
		_, _ = reader.ReadByte()
	}

	if len(lines) > 0 {
		t.Lock()
		t.extras = append(t.extras, strings.Join(lines, "\n"))
		t.Unlock()
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

func (t *testRunner) features() []string {
	var features []string
	err := filepath.Walk(cfg.featuresPath, func(path string, file os.FileInfo, err error) error {
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
