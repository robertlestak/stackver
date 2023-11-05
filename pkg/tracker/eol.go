package tracker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/robertlestak/stackver/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type EOLRelease struct {
	Cycle             string `json:"cycle"`
	ReleaseDate       string `json:"releaseDate"`
	Support           any    `json:"support"`
	Eol               any    `json:"eol"`
	Latest            string `json:"latest"`
	LatestReleaseDate string `json:"latestReleaseDate"`
	Lts               bool   `json:"lts"`
}

type EndOfLifeDateTracker struct {
	uri string
}

func (t *EndOfLifeDateTracker) Link() string {
	return fmt.Sprintf("https://endoflife.date/%s", t.URI())
}

func (t *EndOfLifeDateTracker) GetStatus(currentVersion string) (ServiceStatus, error) {
	l := log.WithFields(log.Fields{
		"tracker": "endoflife.date",
		"uri":     t.uri,
	})
	l.Debug("getting status")
	endpoint := fmt.Sprintf("https://endoflife.date/api/%s.json", t.URI())
	l = l.WithField("endpoint", endpoint)
	l.Debug("getting endpoint")
	resp, err := http.Get(endpoint)
	if err != nil {
		l.WithError(err).Error("error getting endpoint")
		return ServiceStatus{}, err
	}
	defer resp.Body.Close()
	bd, err := io.ReadAll(resp.Body)
	if err != nil {
		l.WithError(err).Error("error reading response body")
		return ServiceStatus{}, err
	}
	l.Debugf("response body: %s", string(bd))
	var releases []EOLRelease
	err = json.Unmarshal(bd, &releases)
	if err != nil {
		l.WithError(err).Error("error decoding response")
		return ServiceStatus{}, err
	}
	if len(releases) == 0 {
		return ServiceStatus{}, fmt.Errorf("no releases found")
	}
	stat := ServiceStatus{
		LatestVersion: releases[0].Latest,
		Link:          t.Link(),
	}
	// if the current version is supplied, find it in the releases
	// to set the current version EOL date
	if currentVersion != "" {
		for _, r := range releases {
			if utils.CycleContainsVersion(r.Cycle, currentVersion) {
				l.Debugf("found current version %s in cycle %s", currentVersion, r.Cycle)
				// if r.Eol is bool, set eol to time.Now
				// otherwise parse the date string
				var eol time.Time
				if r.Eol == nil {
					eol = time.Time{}
				}
				if _, ok := r.Eol.(bool); ok {
					if r.Eol.(bool) {
						eol = time.Now()
					} else {
						eol = time.Time{}
					}
				} else {
					eol, err = time.Parse("2006-01-02", r.Eol.(string))
					if err != nil {
						// try bool
						if r.Eol == "true" {
							eol = time.Now()
						} else {
							eol = time.Time{}
						}
					}
				}
				stat.CurrentVersionEOLDate = &eol
				stat.CalculateStatus(currentVersion)
			}
		}
	}
	l = l.WithField("stat", stat)
	l.Debug("got status")
	return stat, nil
}

func (t *EndOfLifeDateTracker) URI() string {
	return t.uri
}
