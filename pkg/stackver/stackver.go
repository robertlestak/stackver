package stackver

import (
	"encoding/json"
	"errors"
	"os"
	"slices"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/robertlestak/stackver/pkg/tracker"
	"github.com/rodaine/table"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type ObjectMeta struct {
	Name string `json:"name" yaml:"name"`
}

type Service struct {
	Name    string                     `json:"name" yaml:"name"`
	Version string                     `json:"version" yaml:"version"`
	Tracker tracker.ServiceTrackerMeta `json:"tracker" yaml:"tracker"`
	Status  tracker.ServiceStatus      `json:"status" yaml:"status"`
}

type StackSpec struct {
	Dependencies []Service `json:"dependencies" yaml:"dependencies"`
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
	Service Service
	Error   error
}

func versionCheckWorker(jobs chan *ServiceStatusJob, res chan *ServiceStatusJob) {
	for j := range jobs {
		stat, err := j.Service.Tracker.Tracker().GetStatus(j.Service.Version)
		if err != nil {
			log.WithError(err).Error("error getting status")
			j.Error = err
		}
		j.Service.Status = stat
		res <- j
	}
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
	for _, d := range s.Spec.Dependencies {
		origNameOrder = append(origNameOrder, d.Name)
		if d.Tracker.URI == "" {
			d.Tracker.URI = d.Name
		}
		jobs <- &ServiceStatusJob{
			Service: d,
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

func (s *Stack) Output(format string, file string) error {
	l := log.WithFields(log.Fields{
		"app": "stackver",
	})
	l.Debug("outputting")
	var err error
	var f *os.File
	if file == "" || file == "-" {
		f = os.Stdout
	} else {
		f, err = os.Create(file)
		if err != nil {
			return err
		}
		defer f.Close()
	}
	switch format {
	case "json":
		// make it pretty
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		err = enc.Encode(s)
	case "yaml":
		// add a doc separator
		f.WriteString("---\n")
		err = yaml.NewEncoder(f).Encode(s)
	case "text":
		err = s.outputText(f)
	case "prometheus":
		err = s.outputPrometheus(f)
	default:
		err = errors.New("invalid format")
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *Stack) outputText(f *os.File) error {
	tbl := table.New("Name", "Version", "Latest", "EOL Date", "Status", "Link").WithWriter(f)
	for _, d := range s.Spec.Dependencies {
		var eolDate string
		if d.Status.CurrentVersionEOLDate != nil && !d.Status.CurrentVersionEOLDate.IsZero() {
			eolDate = d.Status.CurrentVersionEOLDate.Format("2006-01-02")
		} else {
			eolDate = "unknown"
		}
		tbl.AddRow(d.Name, d.Version, d.Status.LatestVersion, eolDate, d.Status.Status, d.Status.Link)
	}
	tbl.Print()
	return nil
}

func (s *Stack) outputPrometheus(f *os.File) error {
	serviceStatusGauges := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "stackver_service_status",
		Help: "Stackver service status",
	}, []string{"name", "version", "latest", "eol_date", "status", "link"})
	for _, d := range s.Spec.Dependencies {
		var eolDate string
		if !d.Status.CurrentVersionEOLDate.IsZero() {
			eolDate = d.Status.CurrentVersionEOLDate.Format("2006-01-02")
		} else {
			eolDate = "unknown"
		}
		fv := float64(tracker.StatusCodeMap[d.Status.Status])
		serviceStatusGauges.WithLabelValues(d.Name, d.Version, d.Status.LatestVersion, eolDate, string(d.Status.Status), d.Status.Link).Set(fv)
	}
	prometheus.MustRegister(serviceStatusGauges)
	// if f is stdout, write to a temp file, print, and delete
	if f == os.Stdout {
		nf, err := os.CreateTemp("", "stackver.*.prom")
		if err != nil {
			return err
		}
		defer os.Remove(nf.Name())
		err = prometheus.WriteToTextfile(nf.Name(), prometheus.DefaultGatherer)
		if err != nil {
			return err
		}
		// read the file and print it
		fd, err := os.ReadFile(nf.Name())
		if err != nil {
			return err
		}
		_, err = f.Write(fd)
		if err != nil {
			return err
		}
		return nil
	}
	prometheus.WriteToTextfile(f.Name(), prometheus.DefaultGatherer)
	return nil
}
