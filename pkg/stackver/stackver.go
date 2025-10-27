package stackver

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/robertlestak/stackver/pkg/extractor"
	"github.com/robertlestak/stackver/pkg/selector"
	"github.com/robertlestak/stackver/pkg/tracker"
	"github.com/rodaine/table"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type ObjectMeta struct {
	Name        string            `json:"name" yaml:"name"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

type Source struct {
	File     string `json:"file" yaml:"file"`
	Selector string `json:"selector" yaml:"selector"`
}

type Service struct {
	Name        string                     `json:"name" yaml:"name"`
	Description string                     `json:"description,omitempty" yaml:"description,omitempty"`
	Sources     []Source                   `json:"sources" yaml:"sources"`
	Tracker     tracker.ServiceTrackerMeta `json:"tracker" yaml:"tracker"`
	Status      tracker.ServiceStatus      `json:"status" yaml:"status"`
	Offset      *int                       `json:"offset,omitempty" yaml:"offset,omitempty"`

	// Internal field populated from sources
	version string
}

type StackSpec struct {
	Dependencies      []Service `json:"dependencies" yaml:"dependencies"`
	IgnoreLatest      bool      `json:"ignoreLatest,omitempty" yaml:"ignoreLatest,omitempty"`
	AcceptPrerelease  bool      `json:"acceptPrerelease,omitempty" yaml:"acceptPrerelease,omitempty"`
	Offset            int       `json:"offset,omitempty" yaml:"offset,omitempty"`
}

type Stack struct {
	ObjectMeta *ObjectMeta `json:"metadata" yaml:"metadata"`
	Spec       StackSpec   `json:"spec" yaml:"spec"`
}

func LoadFile(f string) (*Stack, error) {
	fd, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	var s Stack
	err = yaml.Unmarshal(fd, &s)
	if err != nil {
		// try json
		err = json.Unmarshal(fd, &s)
		if err != nil {
			return nil, err
		}
	}
	// for each service, ensure the name is set and unique
	var names []string
	for _, v := range s.Spec.Dependencies {
		if v.Name == "" {
			return nil, errors.New("service name is required")
		}
		if slices.Contains(names, v.Name) {
			return nil, errors.New("service names must be unique")
		}
		names = append(names, v.Name)
	}
	return &s, nil
}

type ServiceStatusJob struct {
	Service     Service
	StackConfig StackSpec
	Error       error
}

func versionCheckWorker(jobs chan *ServiceStatusJob, res chan *ServiceStatusJob) {
	for j := range jobs {
		// Use service offset if explicitly set, otherwise use global offset
		offset := j.StackConfig.Offset
		if j.Service.Offset != nil {
			offset = *j.Service.Offset
		}
		
		stat, err := j.Service.Tracker.TrackerWithConfig(j.StackConfig.AcceptPrerelease).GetStatusWithOffset(j.Service.Version(), offset)
		if err != nil {
			log.WithError(err).WithField("service", j.Service.Name).Error("error getting status")
			j.Error = err
		}
		j.Service.Status = stat
		res <- j
	}
}

// Version returns the current version read from sources
func (s *Service) Version() string {
	return s.version
}

// ReadVersionFromSources reads the current version from file sources
func (s *Service) ReadVersionFromSources() error {
	l := log.WithFields(log.Fields{
		"service": s.Name,
	})

	if len(s.Sources) == 0 {
		return errors.New("sources must be defined")
	}

	// Read from first available source
	for _, source := range s.Sources {
		l = l.WithFields(log.Fields{
			"file":     source.File,
			"selector": source.Selector,
		})
		l.Debug("reading version from source")

		value, err := selector.ReadValue(source.File, source.Selector)
		if err != nil {
			l.WithError(err).Warn("failed to read from source, trying next")
			continue
		}

		version := extractor.ExtractVersion(value)
		if version != "" {
			s.version = version
			l.Debugf("extracted version: %s", version)
			return nil
		}
	}

	return errors.New("could not read version from any source")
}

func (s *Stack) CheckVersions() error {
	l := log.WithFields(log.Fields{
		"app": "stackver",
	})
	l.Debug("checking versions")
	workers := 10
	jobs := make(chan *ServiceStatusJob, len(s.Spec.Dependencies))
	res := make(chan *ServiceStatusJob, len(s.Spec.Dependencies))
	for w := 1; w <= workers; w++ {
		go versionCheckWorker(jobs, res)
	}
	var origNameOrder []string
	for i, d := range s.Spec.Dependencies {
		origNameOrder = append(origNameOrder, d.Name)

		// Read version from sources if needed
		if err := d.ReadVersionFromSources(); err != nil {
			return fmt.Errorf("failed to read version for %s: %w", d.Name, err)
		}

		// Update the dependency in the slice with the read version
		s.Spec.Dependencies[i] = d

		if d.Tracker.URI == "" {
			d.Tracker.URI = d.Name
		}
		jobs <- &ServiceStatusJob{
			Service:     d,
			StackConfig: s.Spec,
		}
	}
	close(jobs)
	var newDeps []Service
	for a := 1; a <= len(s.Spec.Dependencies); a++ {
		j := <-res
		if j.Error != nil {
			return j.Error
		}
		newDeps = append(newDeps, j.Service)
	}
	// reorder the dependencies to match the original order
	var reorderedDeps []Service
	for _, n := range origNameOrder {
		for _, d := range newDeps {
			if d.Name == n {
				reorderedDeps = append(reorderedDeps, d)
			}
		}
	}
	s.Spec.Dependencies = reorderedDeps
	return nil
}

func (s *Stack) PrintStatus() {
	tbl := table.New("Name", "Version", "Latest", "Status")
	for _, d := range s.Spec.Dependencies {
		tbl.AddRow(d.Name, d.Version(), d.Status.LatestVersion, d.Status.Status)
	}
	tbl.Print()
}
