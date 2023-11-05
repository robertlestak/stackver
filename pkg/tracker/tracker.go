package tracker

import (
	"strings"
	"time"

	"github.com/robertlestak/stackver/pkg/utils"
)

type ServiceTrackerName string

const (
	ServiceTrackerEndOfLife ServiceTrackerName = "endoflife"
	ServiceTrackerGitHub    ServiceTrackerName = "github"
)

type ServiceTracker interface {
	GetStatus(currentVersion string) (ServiceStatus, error)
	URI() string
	Link() string
}

type StatusString string

const (
	StatusCurrent         StatusString = "current"
	StatusGood            StatusString = "good"
	StatusUpdateAvailable StatusString = "update-available"
	StatusWarning         StatusString = "warning"
	StatusDanger          StatusString = "danger"
	StatusCritical        StatusString = "critical"
)

var (
	DaysUntilWarning = 60
	DaysUntilDanger  = 30
	StatusCodeMap    = map[StatusString]int{
		StatusCurrent:         0,
		StatusGood:            1,
		StatusUpdateAvailable: 2,
		StatusWarning:         3,
		StatusDanger:          4,
		StatusCritical:        5,
	}
)

type ServiceStatus struct {
	LatestVersion         string       `json:"latestVersion,omitempty" yaml:"latestVersion,omitempty"`
	CurrentVersionEOLDate *time.Time   `json:"currentVersionEOLDate,omitempty" yaml:"currentVersionEOLDate,omitempty"`
	Link                  string       `json:"link,omitempty" yaml:"link,omitempty"`
	Status                StatusString `json:"status,omitempty" yaml:"status,omitempty"`
}

func (s *ServiceStatus) CalculateStatus(currentVersion string) {
	if currentVersion == s.LatestVersion {
		s.Status = StatusCurrent
		return
	}
	// if the eol date is not zero, if the date is within 60 days, set status to warning
	// if the date is within or under 30 days, set status to danger
	if s.CurrentVersionEOLDate != nil && !s.CurrentVersionEOLDate.IsZero() {
		s.Status = StatusGood
		if time.Now().AddDate(0, 0, DaysUntilWarning).After(*s.CurrentVersionEOLDate) {
			s.Status = StatusWarning
		}
		if time.Now().AddDate(0, 0, DaysUntilDanger).After(*s.CurrentVersionEOLDate) {
			s.Status = StatusDanger
		}
		// if the date is in the past, set status to critical
		if time.Now().After(*s.CurrentVersionEOLDate) {
			s.Status = StatusCritical
		}
	}
	// if there is no status, check the semver
	if s.Status == "" {
		if utils.CycleContainsVersion(s.LatestVersion, currentVersion) {
			if strings.Contains(s.LatestVersion, currentVersion) {
				s.Status = StatusCurrent
			} else {
				s.Status = StatusGood
			}
		} else {
			s.Status = StatusUpdateAvailable
		}
	}
}

type ServiceTrackerMeta struct {
	Kind ServiceTrackerName `json:"kind" yaml:"kind"`
	URI  string             `json:"uri" yaml:"uri"`
}

func (t *ServiceTrackerMeta) Tracker() ServiceTracker {
	switch t.Kind {
	case ServiceTrackerEndOfLife:
		return &EndOfLifeDateTracker{uri: t.URI}
	case ServiceTrackerGitHub:
		return &GitHubTracker{uri: t.URI}
	default:
		// default to EOL tracker
		t.Kind = ServiceTrackerEndOfLife
		return &EndOfLifeDateTracker{uri: t.URI}
	}
}
