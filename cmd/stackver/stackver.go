package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/robertlestak/stackver/pkg/selector"
	"github.com/robertlestak/stackver/pkg/stackver"
	"github.com/robertlestak/stackver/pkg/tracker"
	"github.com/robertlestak/stackver/pkg/utils"
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

func showStatus(inFile string) error {
	if inFile == "" {
		return errors.New("stack file is required")
	}
	
	s, err := stackver.LoadFile(inFile)
	if err != nil {
		return err
	}
	
	if err := s.CheckVersions(); err != nil {
		return err
	}
	
	s.PrintStatus()
	return nil
}

func processFileWithUpdate(inFile string, dryRun bool) error {
	l := log.WithFields(log.Fields{
		"app":    "stackver",
		"fn":     "processFileWithUpdate",
		"dryRun": dryRun,
	})
	l.Debug("processing file for updates")
	
	if inFile == "" {
		return errors.New("stack file is required")
	}
	
	s, err := stackver.LoadFile(inFile)
	if err != nil {
		return err
	}
	
	if err := s.CheckVersions(); err != nil {
		return err
	}
	
	// Show current status
	s.PrintStatus()
	
	// Process updates
	updatesAvailable := false
	for _, dep := range s.Spec.Dependencies {
		if dep.Status.LatestVersion != dep.Version() {
			// Check if we should ignore "latest" tags
			if s.Spec.IgnoreLatest && dep.Version() == "latest" {
				l.Warnf("Ignoring update for %s: current version is 'latest' (ignoreLatest=true)", dep.Name)
				continue
			}
			
			// Check for potential downgrade
			if dep.Status.Status == tracker.StatusWarning {
				// Check if it's actually a downgrade
				if utils.IsDowngrade(dep.Version(), dep.Status.LatestVersion) {
					l.Warnf("WARNING: Potential downgrade detected for %s: %s -> %s (check repository mapping)", 
						dep.Name, dep.Version(), dep.Status.LatestVersion)
				}
			}
			
			updatesAvailable = true
			
			if !dryRun {
				// Update each source file
				for _, source := range dep.Sources {
					l.Infof("Updating %s in %s", dep.Name, source.File)
					
					// Get current full value (e.g., "nginx:1.21.0")
					currentValue, err := selector.ReadValue(source.File, source.Selector)
					if err != nil {
						return fmt.Errorf("failed to read current value: %w", err)
					}
					
					// Replace version in the value
					newValue := strings.Replace(currentValue, dep.Version(), dep.Status.LatestVersion, 1)
					
					if err := selector.UpdateValue(source.File, source.Selector, newValue); err != nil {
						return fmt.Errorf("failed to update %s: %w", source.File, err)
					}
					
					l.Infof("Successfully updated %s: %s -> %s", source.File, currentValue, newValue)
				}
			}
		}
	}
	
	if !updatesAvailable && !dryRun {
		l.Info("No updates available")
	}
	
	return nil
}

func main() {
	l := log.WithFields(log.Fields{
		"app": "stackver",
	})
	l.Debug("starting stackver")
	stackFile := flag.String("f", "", "stack file")
	daysUntilWarning := flag.Int("w", 60, "days until warning")
	daysUntilDanger := flag.Int("d", 30, "days until danger")
	printVersion := flag.Bool("v", false, "print version")
	updateMode := flag.Bool("update", false, "update files with new versions")
	dryRun := flag.Bool("dry-run", false, "show what would be updated without making changes")
	flag.Parse()
	if *printVersion {
		log.Info(Version)
		return
	}
	tracker.DaysUntilWarning = *daysUntilWarning
	tracker.DaysUntilDanger = *daysUntilDanger
	
	if *updateMode || *dryRun {
		if err := processFileWithUpdate(*stackFile, *dryRun); err != nil {
			l.Fatal(err)
		}
		return
	}
	
	// Default: show status
	if err := showStatus(*stackFile); err != nil {
		l.Fatal(err)
	}
}
