package main

import (
	"errors"
	"flag"
	"os"
	"path"

	"github.com/robertlestak/stackver/pkg/stackver"
	"github.com/robertlestak/stackver/pkg/tracker"
	log "github.com/sirupsen/logrus"
)

var (
	Version = "dev"
)

func init() {
	ll, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		ll = log.InfoLevel
	}
	log.SetLevel(ll)
}

func processFile(inFile string, outFile string, format string) error {
	l := log.WithFields(log.Fields{
		"app": "stackver",
		"fn":  "processFile",
	})
	l.Debug("processing file")
	if inFile == "" {
		return errors.New("stack file is required")
	}
	s, err := stackver.LoadFile(inFile)
	if err != nil {
		l.Fatal(err)
	}
	l.Debugf("stack: %+v", s)
	if err := s.CheckVersions(); err != nil {
		l.Fatal(err)
	}
	if err := s.Output(format, outFile); err != nil {
		l.Fatal(err)
	}
	return nil
}

func processDir(stackDir string, outDir string, format string) error {
	l := log.WithFields(log.Fields{
		"app": "stackver",
		"fn":  "processDir",
	})
	l.Debug("processing directory")
	if stackDir == "" {
		return errors.New("stack directory is required")
	}
	if outDir == "" {
		return errors.New("output directory is required")
	}
	// if outdir is not stdout and it doesnt exist, create it
	if outDir != "-" {
		if _, err := os.Stat(outDir); os.IsNotExist(err) {
			if err := os.MkdirAll(outDir, 0755); err != nil {
				return err
			}
		}
	}
	// for each file in the directory, process the file and write to the output directory
	// if the output directory is not specified, write to stdout
	files, err := os.ReadDir(stackDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		// if file does not have a yaml or json extension, skip it
		if path.Ext(f.Name()) != ".yaml" && path.Ext(f.Name()) != ".yml" && path.Ext(f.Name()) != ".json" {
			continue
		}
		// remote extension from the file name and replace it with the new one for this format
		fext := format
		if format == "text" {
			fext = "txt"
		}
		fileNameBase := f.Name()[0 : len(f.Name())-len(path.Ext(f.Name()))]
		filename := fileNameBase + "." + fext
		outPath := path.Join(outDir, filename)
		if outDir == "-" {
			outPath = "-"
		}
		if err := processFile(path.Join(stackDir, f.Name()), outPath, format); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	l := log.WithFields(log.Fields{
		"app": "stackver",
	})
	l.Debug("starting stackver")
	stackFile := flag.String("f", "", "stack file")
	format := flag.String("o", "text", "output format")
	daysUntilWarning := flag.Int("w", 60, "days until warning")
	daysUntilDanger := flag.Int("d", 30, "days until danger")
	printVersion := flag.Bool("v", false, "print version")
	flag.Parse()
	if *printVersion {
		log.Info(Version)
		return
	}
	tracker.DaysUntilWarning = *daysUntilWarning
	tracker.DaysUntilDanger = *daysUntilDanger
	outFile := "-"
	// if stackFile is a directory, process all files in the directory
	if fi, err := os.Stat(*stackFile); err == nil && fi.IsDir() {
		l.Debug("stack file is a directory")
		if len(flag.Args()) > 0 {
			outFile = flag.Args()[0]
		}
		if err := processDir(*stackFile, outFile, *format); err != nil {
			l.Fatal(err)
		}
		return
	}
	if err := processFile(*stackFile, outFile, *format); err != nil {
		l.Fatal(err)
	}
}
