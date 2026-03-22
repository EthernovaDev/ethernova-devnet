package eth

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/params"
)

type semver struct {
	major int
	minor int
	patch int
}

func (v semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

func (v semver) lessThan(other semver) bool {
	if v.major != other.major {
		return v.major < other.major
	}
	if v.minor != other.minor {
		return v.minor < other.minor
	}
	return v.patch < other.patch
}

func minPeerVersion() semver {
	return semver{
		major: params.VersionMajor,
		minor: params.VersionMinor,
		patch: params.VersionPatch,
	}
}

func parseNodeVersion(name string) (semver, bool) {
	for _, part := range strings.Split(name, "/") {
		if len(part) == 0 || part[0] != 'v' {
			continue
		}
		candidate := strings.TrimPrefix(part, "v")
		candidate = strings.SplitN(candidate, "-", 2)[0]
		candidate = strings.SplitN(candidate, "+", 2)[0]
		segs := strings.Split(candidate, ".")
		if len(segs) < 3 {
			continue
		}
		major, err := strconv.Atoi(segs[0])
		if err != nil {
			continue
		}
		minor, err := strconv.Atoi(segs[1])
		if err != nil {
			continue
		}
		patch, err := strconv.Atoi(segs[2])
		if err != nil {
			continue
		}
		return semver{major: major, minor: minor, patch: patch}, true
	}
	return semver{}, false
}

func enforcePeerVersion(name string) error {
	if params.VersionName == "" {
		return nil
	}
	if !strings.Contains(strings.ToLower(name), strings.ToLower(params.VersionName)) {
		return fmt.Errorf("unsupported client name: %q", name)
	}
	peerVersion, ok := parseNodeVersion(name)
	if !ok {
		return fmt.Errorf("unable to parse client version from %q", name)
	}
	min := minPeerVersion()
	if peerVersion.lessThan(min) {
		return fmt.Errorf("peer version %s < required %s", peerVersion, min)
	}
	return nil
}
