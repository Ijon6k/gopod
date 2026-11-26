package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopod/internal/audit"
	"gopod/internal/notifications"
	"gopod/internal/podman"
)

var allowedUpdateHosts = map[string]bool{
	"api.github.com":                        true,
	"github.com":                            true,
	"objects.githubusercontent.com":         true,
	"release-assets.githubusercontent.com":  true,
	"ghcr.io":                               true,
	"pkg-containers.githubusercontent.com":  true,
}

func newSafeUpdateHTTPClient(allowHTTPForTests bool) *http.Client {
	return &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 6 {
				return errors.New("too many redirects")
			}
			host := strings.ToLower(req.URL.Hostname())
			if allowHTTPForTests && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
				return nil
			}
			if !strings.EqualFold(req.URL.Scheme, "https") {
				return fmt.Errorf("blocked insecure redirect scheme %q", req.URL.Scheme)
			}
			if !allowedUpdateHosts[host] {
				return fmt.Errorf("blocked redirect to untrusted host %q", host)
			}
			return nil
		},
	}
}

// Service inspects GOPOD runtime state, checks upstream releases/digests, and
// safely orchestrates mode-specific updates with health verification and rollback.
type Service struct {
	podmanClient *podman.Client
	dataDir      string
	audit        *audit.Service
	notifier     *notifications.Service

	httpClient        *http.Client
	githubAPIBase     string
	githubDownloadURL string
	ghcrBaseURL       string
	allowLocalHTTP    bool

	cacheMu   sync.Mutex
	cachedAt  time.Time
	cachedCh  string
	cachedRes *UpdateCheckResponse

	updateMu   sync.Mutex
	isUpdating bool
}

// NewService constructs an updater service locked to github.com/ijon6k/gopod and ghcr.io/ijon6k/gopod.
func NewService(pc *podman.Client, dataDir string, auditSvc *audit.Service, notifSvc *notifications.Service) *Service {
	return &Service{
		podmanClient:      pc,
		dataDir:           dataDir,
		audit:             auditSvc,
		notifier:          notifSvc,
		httpClient:        newSafeUpdateHTTPClient(false),
		githubAPIBase:     "https://api.github.com",
		githubDownloadURL: "https://github.com",
		ghcrBaseURL:       "https://ghcr.io",
	}
}

func (s *Service) validateOutboundURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	host := strings.ToLower(u.Hostname())
	if s.allowLocalHTTP && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
		return nil
	}
	if !strings.EqualFold(u.Scheme, "https") || !allowedUpdateHosts[host] {
		return fmt.Errorf("outbound update URL rejected (%s)", host)
	}
	return nil
}

// CheckForUpdates compares installed version / OCI image digest against GitHub Releases and GHCR.
func (s *Service) CheckForUpdates(ctx context.Context, channel string, force bool) (*UpdateCheckResponse, error) {
	ch := ValidateChannel(channel)
	if channel == "" {
		ch = ValidateChannel(Channel)
	}

	if !force {
		s.cacheMu.Lock()
		if s.cachedRes != nil && s.cachedCh == ch && time.Since(s.cachedAt) < 10*time.Minute {
			copyRes := *s.cachedRes
			copyRes.LastStatus = s.ReadStatus()
			s.cacheMu.Unlock()
			return &copyRes, nil
		}
		s.cacheMu.Unlock()
	}

	curVer, commit, buildDate := resolveBuildMetadata()
	self := s.InspectSelf(ctx)
	var sysInfo *podman.SystemInfo
	if s.podmanClient != nil {
		sysInfo, _ = s.podmanClient.GetSystemInfo(ctx)
	}
	podmanCompat := EvaluatePodmanCompatibility(sysInfo)

	res := &UpdateCheckResponse{
		Repository:     GitHubOwner + "/" + GitHubRepo,
		ImageRepo:      GHCRImageBase,
		CurrentVersion: curVer,
		BuildCommit:    commit,
		BuildDate:      buildDate,
		Channel:        ch,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		Self:           self,
		Podman:         podmanCompat,
		LastStatus:     s.ReadStatus(),
	}

	var checkErrs []string
	// 1. Query latest GitHub Release (or fallback tags if releases haven't been published yet).
	if rel, err := s.fetchLatestRelease(ctx); err == nil && rel != nil {
		res.LatestVersion = rel.TagName
		res.LatestTag = rel.TagName
		res.ReleaseURL = rel.HTMLURL
		res.PublishedAt = rel.PublishedAt
		res.ReleaseNotes = truncateNotes(rel.Body, 1800)
	} else if err != nil {
		checkErrs = append(checkErrs, err.Error())
	}

	targetDigestTag := "latest"
	if ch == "dev" {
		targetDigestTag = "dev"
		res.LatestTag = "dev"
		if res.LatestVersion == "" {
			res.LatestVersion = "dev"
		}
	} else if res.LatestTag != "" {
		targetDigestTag = res.LatestTag
	}

	// 2. Query GHCR remote OCI manifest digest when containerized or checking GHCR image.
	if remoteDigest, err := s.fetchGHCRDigest(ctx, targetDigestTag); err == nil && remoteDigest != "" {
		res.RemoteDigest = remoteDigest
	} else if err != nil {
		// Only surface registry error if GitHub Release check also failed.
		if len(checkErrs) > 0 {
			checkErrs = append(checkErrs, err.Error())
		}
	}

	res.UpdateAvailable = computeUpdateAvailable(curVer, ch, self, res.LatestVersion, res.RemoteDigest)
	if len(checkErrs) > 0 && res.LatestVersion == "" && res.RemoteDigest == "" {
		res.CheckError = strings.Join(checkErrs, "; ")
	}

	s.cacheMu.Lock()
	s.cachedAt = time.Now()
	s.cachedCh = ch
	copyCache := *res
	s.cachedRes = &copyCache
	s.cacheMu.Unlock()

	return res, nil
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
}

func (s *Service) fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/releases/latest", strings.TrimRight(s.githubAPIBase, "/"), GitHubOwner, GitHubRepo)
	if err := s.validateOutboundURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gopod-updater/"+Version)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return s.fetchLatestGitTagFallback(ctx)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub releases returned HTTP %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	if _, err := ValidateTargetTag(rel.TagName); err != nil {
		return nil, fmt.Errorf("unexpected release tag %q", rel.TagName)
	}
	return &rel, nil
}

func (s *Service) fetchLatestGitTagFallback(ctx context.Context) (*githubRelease, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=10", strings.TrimRight(s.githubAPIBase, "/"), GitHubOwner, GitHubRepo)
	if err := s.validateOutboundURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gopod-updater/"+Version)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512<<10)).Decode(&tags); err != nil {
		return nil, nil
	}
	best := ""
	for _, t := range tags {
		if _, err := ValidateTargetTag(t.Name); err != nil {
			continue
		}
		if best == "" || compareSemver(best, t.Name) < 0 {
			best = t.Name
		}
	}
	if best == "" {
		return nil, nil
	}
	return &githubRelease{
		TagName: best,
		HTMLURL: fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", GitHubOwner, GitHubRepo, best),
	}, nil
}

func (s *Service) fetchGHCRDigest(ctx context.Context, tag string) (string, error) {
	cleanTag, err := ValidateTargetTag(tag)
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(s.ghcrBaseURL, "/")
	tokenURL := fmt.Sprintf("%s/token?service=ghcr.io&scope=repository:%s/%s:pull", base, GitHubOwner, GitHubRepo)
	if err := s.validateOutboundURL(tokenURL); err != nil {
		return "", err
	}
	tokReq, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", err
	}
	tokResp, err := s.httpClient.Do(tokReq)
	if err != nil {
		return "", err
	}
	defer tokResp.Body.Close()
	if tokResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GHCR token returned HTTP %d", tokResp.StatusCode)
	}
	var tokBody struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(tokResp.Body, 256<<10)).Decode(&tokBody); err != nil {
		return "", err
	}
	if tokBody.Token == "" {
		return "", errors.New("empty GHCR pull token")
	}

	manifestURL := fmt.Sprintf("%s/v2/%s/%s/manifests/%s", base, GitHubOwner, GitHubRepo, url.PathEscape(cleanTag))
	if err := s.validateOutboundURL(manifestURL); err != nil {
		return "", err
	}
	mReq, err := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
	if err != nil {
		return "", err
	}
	mReq.Header.Set("Authorization", "Bearer "+tokBody.Token)
	mReq.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json")

	mResp, err := s.httpClient.Do(mReq)
	if err != nil {
		return "", err
	}
	defer mResp.Body.Close()
	if mResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GHCR manifest returned HTTP %d", mResp.StatusCode)
	}
	digest := strings.TrimSpace(mResp.Header.Get("Docker-Content-Digest"))
	if !digestSha256.MatchString(digest) {
		return "", nil
	}
	return digest, nil
}

func computeUpdateAvailable(curVer, channel string, self SelfInspection, latestVer, remoteDigest string) bool {
	if self.IsDevBuild {
		if channel == "dev" && self.LocalDigest != "" && remoteDigest != "" {
			return self.LocalDigest != remoteDigest
		}
		return latestVer != "" && latestVer != "dev"
	}
	if latestVer != "" && latestVer != "dev" && compareSemver(curVer, latestVer) < 0 {
		return true
	}
	if self.LocalDigest != "" && remoteDigest != "" && self.LocalDigest != remoteDigest {
		// Same version/tag repushed (e.g., `:latest` or `:dev`)
		if latestVer == "" || compareSemver(curVer, latestVer) == 0 || channel == "dev" {
			return true
		}
	}
	return false
}

// compareSemver compares two vX.Y.Z tags. Returns -1 if a < b, 0 if equal, +1 if a > b.
func compareSemver(a, b string) int {
	pa := parseSemverParts(a)
	pb := parseSemverParts(b)
	if pa == nil || pb == nil {
		return strings.Compare(strings.TrimPrefix(a, "v"), strings.TrimPrefix(b, "v"))
	}
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseSemverParts(raw string) []int {
	clean := strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	if idx := strings.IndexAny(clean, "-+"); idx != -1 {
		clean = clean[:idx]
	}
	segs := strings.Split(clean, ".")
	if len(segs) < 2 {
		return nil
	}
	out := make([]int, 3)
	for i := 0; i < 3 && i < len(segs); i++ {
		v, err := strconv.Atoi(segs[i])
		if err != nil {
			return nil
		}
		out[i] = v
	}
	return out
}

func truncateNotes(raw string, max int) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max] + "…"
}

// EvaluatePodmanCompatibility checks the host Podman version against GOPOD's v5 baseline.
func EvaluatePodmanCompatibility(sysInfo *podman.SystemInfo) PodmanCompatibility {
	const minPodman = "5.0.0"
	compat := PodmanCompatibility{
		RecommendedMinVersion: minPodman,
		Compatible:            true,
		Rootless:              true,
		UpgradeHint:           "sudo dnf upgrade -y podman  # Fedora/RHEL  |  sudo apt-get update && sudo apt-get install -y podman  # Debian/Ubuntu",
	}
	if sysInfo != nil {
		compat.Version = strings.TrimSpace(sysInfo.PodmanVersion)
		compat.Rootless = sysInfo.Rootless
		compat.OS = strings.TrimSpace(sysInfo.OS)
		lowerOS := strings.ToLower(compat.OS)
		switch {
		case strings.Contains(lowerOS, "ubuntu") || strings.Contains(lowerOS, "debian"):
			compat.UpgradeHint = "sudo apt-get update && sudo apt-get install -y --only-upgrade podman podman-compose"
		case strings.Contains(lowerOS, "fedora") || strings.Contains(lowerOS, "ultramarine") || strings.Contains(lowerOS, "rhel") || strings.Contains(lowerOS, "rocky") || strings.Contains(lowerOS, "alma"):
			compat.UpgradeHint = "sudo dnf upgrade -y podman podman-compose"
		case strings.Contains(lowerOS, "arch"):
			compat.UpgradeHint = "sudo pacman -Syu podman podman-compose"
		case strings.Contains(lowerOS, "alpine"):
			compat.UpgradeHint = "apk update && apk upgrade podman"
		}
		if compat.Version != "" && compareSemver(compat.Version, minPodman) < 0 {
			compat.Compatible = false
			compat.Warning = fmt.Sprintf("Host Podman %s is older than the recommended v%s baseline; Quadlet v2/cgroups v2 features may be limited.", compat.Version, minPodman)
		}
	} else {
		compat.Version = readHostOSReleaseFallback()
	}
	return compat
}

func readHostOSReleaseFallback() string {
	if _, err := execLookPodman(); err != nil {
		return "unknown"
	}
	return "unknown"
}

func execLookPodman() (string, error) {
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		return "/run/podman/podman.sock", nil
	}
	return "", errors.New("socket missing")
}

