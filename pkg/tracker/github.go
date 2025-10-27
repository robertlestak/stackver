package tracker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/robertlestak/stackver/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type GitHubTracker struct {
	uri              string
	hasReleases      bool
	acceptPrerelease bool
}

type GitHubAPIError struct {
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url"`
}

type GitHubRelease struct {
	URL       string `json:"url"`
	AssetsURL string `json:"assets_url"`
	UploadURL string `json:"upload_url"`
	HTMLURL   string `json:"html_url"`
	ID        int    `json:"id"`
	Author    struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"author"`
	NodeID          string    `json:"node_id"`
	TagName         string    `json:"tag_name"`
	TargetCommitish string    `json:"target_commitish"`
	Name            string    `json:"name"`
	Draft           bool      `json:"draft"`
	Prerelease      bool      `json:"prerelease"`
	CreatedAt       time.Time `json:"created_at"`
	PublishedAt     time.Time `json:"published_at"`
	Assets          []struct {
		URL      string `json:"url"`
		ID       int    `json:"id"`
		NodeID   string `json:"node_id"`
		Name     string `json:"name"`
		Label    string `json:"label"`
		Uploader struct {
			Login             string `json:"login"`
			ID                int    `json:"id"`
			NodeID            string `json:"node_id"`
			AvatarURL         string `json:"avatar_url"`
			GravatarID        string `json:"gravatar_id"`
			URL               string `json:"url"`
			HTMLURL           string `json:"html_url"`
			FollowersURL      string `json:"followers_url"`
			FollowingURL      string `json:"following_url"`
			GistsURL          string `json:"gists_url"`
			StarredURL        string `json:"starred_url"`
			SubscriptionsURL  string `json:"subscriptions_url"`
			OrganizationsURL  string `json:"organizations_url"`
			ReposURL          string `json:"repos_url"`
			EventsURL         string `json:"events_url"`
			ReceivedEventsURL string `json:"received_events_url"`
			Type              string `json:"type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"uploader"`
		ContentType        string    `json:"content_type"`
		State              string    `json:"state"`
		Size               int       `json:"size"`
		DownloadCount      int       `json:"download_count"`
		CreatedAt          time.Time `json:"created_at"`
		UpdatedAt          time.Time `json:"updated_at"`
		BrowserDownloadURL string    `json:"browser_download_url"`
	} `json:"assets"`
	TarballURL string `json:"tarball_url"`
	ZipballURL string `json:"zipball_url"`
	Body       string `json:"body"`
	Reactions  struct {
		URL        string `json:"url"`
		TotalCount int    `json:"total_count"`
		Num1       int    `json:"+1"`
		Num10      int    `json:"-1"`
		Laugh      int    `json:"laugh"`
		Hooray     int    `json:"hooray"`
		Confused   int    `json:"confused"`
		Heart      int    `json:"heart"`
		Rocket     int    `json:"rocket"`
		Eyes       int    `json:"eyes"`
	} `json:"reactions,omitempty"`
	MentionsCount int `json:"mentions_count,omitempty"`
}

func (t *GitHubTracker) Link() string {
	if t.hasReleases {
		return fmt.Sprintf("https://github.com/%s/releases", t.URI())
	} else {
		return fmt.Sprintf("https://github.com/%s", t.URI())
	}
}

type GitHubCommit struct {
	Sha    string `json:"sha"`
	NodeID string `json:"node_id"`
	Commit struct {
		Author struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"committer"`
		Message string `json:"message"`
		Tree    struct {
			Sha string `json:"sha"`
			URL string `json:"url"`
		} `json:"tree"`
		URL          string `json:"url"`
		CommentCount int    `json:"comment_count"`
		Verification struct {
			Verified  bool   `json:"verified"`
			Reason    string `json:"reason"`
			Signature string `json:"signature"`
			Payload   string `json:"payload"`
		} `json:"verification"`
	} `json:"commit"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	CommentsURL string `json:"comments_url"`
	Author      struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"author"`
	Committer struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"committer"`
	Parents []struct {
		Sha     string `json:"sha"`
		URL     string `json:"url"`
		HTMLURL string `json:"html_url"`
	} `json:"parents"`
}

func (t *GitHubTracker) getCommitStatus(currentVersion string) (ServiceStatus, error) {
	l := log.WithFields(log.Fields{
		"tracker": "github.commit",
		"uri":     t.uri,
	})
	l.Debug("getting status")
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/commits", t.URI())
	l = l.WithField("endpoint", endpoint)
	l.Debug("getting endpoint")
	c := &http.Client{}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		l.WithError(err).Error("error creating request")
		return ServiceStatus{}, err
	}
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	if os.Getenv("GITHUB_TOKEN") != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_TOKEN")))
	}
	resp, err := c.Do(req)
	if err != nil {
		l.WithError(err).Error("error getting endpoint")
		return ServiceStatus{}, err
	}
	defer resp.Body.Close()
	
	// Check for API errors first
	if resp.StatusCode != 200 {
		var apiError GitHubAPIError
		if err := json.NewDecoder(resp.Body).Decode(&apiError); err == nil {
			return ServiceStatus{}, fmt.Errorf("GitHub API error: %s", apiError.Message)
		}
		return ServiceStatus{}, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	
	var releases []GitHubCommit
	err = json.NewDecoder(resp.Body).Decode(&releases)
	if err != nil {
		l.WithError(err).Error("error decoding response")
		return ServiceStatus{}, err
	}
	if len(releases) == 0 {
		return ServiceStatus{}, fmt.Errorf("no releases found")
	}
	stat := ServiceStatus{
		LatestVersion: utils.TrimVersionPrefix(releases[0].Sha[:7]),
		Link:          t.Link(),
	}
	stat.CalculateStatus(currentVersion)
	l = l.WithField("stat", stat)
	l.Debug("got status")
	return stat, nil
}

func (t *GitHubTracker) getReleaseStatus(currentVersion string) (ServiceStatus, error) {
	return t.getReleaseStatusWithOffset(currentVersion, 0)
}

func (t *GitHubTracker) getReleaseStatusWithOffset(currentVersion string, offset int) (ServiceStatus, error) {
	l := log.WithFields(log.Fields{
		"tracker": "github.date",
		"uri":     t.uri,
	})
	l.Debug("getting status")
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/releases", t.URI())
	l = l.WithField("endpoint", endpoint)
	l.Debug("getting endpoint")
	c := &http.Client{}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		l.WithError(err).Error("error creating request")
		return ServiceStatus{}, err
	}
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	if os.Getenv("GITHUB_TOKEN") != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_TOKEN")))
	}
	resp, err := c.Do(req)
	if err != nil {
		l.WithError(err).Error("error getting endpoint")
		return ServiceStatus{}, err
	}
	defer resp.Body.Close()
	
	// Check for API errors first
	if resp.StatusCode != 200 {
		var apiError GitHubAPIError
		if err := json.NewDecoder(resp.Body).Decode(&apiError); err == nil {
			return ServiceStatus{}, fmt.Errorf("GitHub API error: %s", apiError.Message)
		}
		return ServiceStatus{}, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	
	var releases []GitHubRelease
	err = json.NewDecoder(resp.Body).Decode(&releases)
	if err != nil {
		l.WithError(err).Error("error decoding response")
		return ServiceStatus{}, err
	}
	if len(releases) == 0 {
		return ServiceStatus{}, fmt.Errorf("no releases found")
	}
	
	// Extract all version strings
	var versions []string
	for _, release := range releases {
		versions = append(versions, release.TagName)
	}
	
	// Get version at offset
	targetVersion := utils.GetVersionAtOffset(versions, offset, t.acceptPrerelease)
	if targetVersion == "" {
		return ServiceStatus{}, fmt.Errorf("no suitable version found")
	}
	
	stat := ServiceStatus{
		LatestVersion: utils.TrimVersionPrefix(targetVersion),
		Link:          t.Link(),
	}
	stat.CalculateStatus(currentVersion)
	l = l.WithField("stat", stat)
	l.Debug("got status")
	return stat, nil
}

func (t *GitHubTracker) GetStatus(currentVersion string) (ServiceStatus, error) {
	return t.GetStatusWithOffset(currentVersion, 0)
}

func (t *GitHubTracker) GetStatusWithOffset(currentVersion string, offset int) (ServiceStatus, error) {
	l := log.WithFields(log.Fields{
		"tracker": "github.date",
		"uri":     t.uri,
	})
	l.Debug("getting status")
	t.hasReleases = true
	stat, err := t.getReleaseStatusWithOffset(currentVersion, offset)
	if err != nil {
		t.hasReleases = false
		stat, err = t.getCommitStatus(currentVersion)
		if err != nil {
			return ServiceStatus{}, err
		}
	}
	return stat, nil
}

func (t *GitHubTracker) SetAcceptPrerelease(accept bool) {
	t.acceptPrerelease = accept
}

func (t *GitHubTracker) URI() string {
	return t.uri
}
