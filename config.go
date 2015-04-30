package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

type config struct {
	concurrencyLevel int
	binPath          string
	featuresPath     string
}

func init() {
	flag.IntVar(&cfg.concurrencyLevel, "c", runtime.NumCPU(), "Concurrency level, defaults to number of CPUs")
	flag.IntVar(&cfg.concurrencyLevel, "concurrency", runtime.NumCPU(), "Concurrency level, defaults to number of CPUs")
	flag.StringVar(&cfg.binPath, "bin", "bin/behat", "Default path to behat executable")
	flag.StringVar(&cfg.featuresPath, "features", "features", "Default path to behat features")
}

func (c config) Validate() error {
	inf, err := os.Stat(c.featuresPath)
	if err != nil {
		return err
	}
	if !inf.IsDir() {
		return fmt.Errorf("feature path \"%s\" is not a directory.", c.featuresPath)
	}
	inf, err = os.Stat(c.binPath)
	if err != nil {
		return err
	}
	if inf.IsDir() {
		return fmt.Errorf("behat bin \"%s\" is not a file.", c.binPath)
	}
	return nil
}
