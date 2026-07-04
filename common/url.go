package common

import (
	"net/url"
	"path"
	"strings"
)

// JoinURLPath appends URL path elements to a configured base URL without
// depending on whether the base ends with a slash.
func JoinURLPath(base string, elems ...string) string {
	if len(elems) == 0 {
		return strings.TrimRight(base, "/")
	}

	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return joinURLPathFallback(base, elems...)
	}

	parts := make([]string, 0, len(elems)+1)
	if basePath := strings.Trim(u.Path, "/"); basePath != "" {
		parts = append(parts, basePath)
	}
	for _, elem := range elems {
		if segment := strings.Trim(elem, "/"); segment != "" {
			parts = append(parts, segment)
		}
	}
	if len(parts) == 0 {
		u.Path = "/"
	} else {
		u.Path = "/" + path.Join(parts...)
	}
	return u.String()
}

// JoinURLPathWithVersion appends an API version and endpoint to base, but does
// not duplicate the version if the configured base already ends with it.
func JoinURLPathWithVersion(base, version string, elems ...string) string {
	if pathEndsWithSegments(base, elems...) {
		return JoinURLPath(base)
	}
	if version == "" || pathEndsWithSegment(base, version) {
		return JoinURLPath(base, elems...)
	}
	parts := make([]string, 0, len(elems)+1)
	parts = append(parts, version)
	parts = append(parts, elems...)
	return JoinURLPath(base, parts...)
}

func joinURLPathFallback(base string, elems ...string) string {
	out := strings.TrimRight(base, "/")
	for _, elem := range elems {
		if segment := strings.Trim(elem, "/"); segment != "" {
			out += "/" + segment
		}
	}
	return out
}

func pathEndsWithSegment(rawURL, segment string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return lastPathSegment(rawURL) == segment
	}
	return lastPathSegment(u.Path) == segment
}

func lastPathSegment(p string) string {
	parts := pathSegments(p)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func pathEndsWithSegments(rawURL string, suffix ...string) bool {
	want := make([]string, 0, len(suffix))
	for _, elem := range suffix {
		want = append(want, pathSegments(elem)...)
	}
	if len(want) == 0 {
		return false
	}

	p := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Scheme != "" && u.Host != "" {
		p = u.Path
	}
	got := pathSegments(p)
	if len(got) < len(want) {
		return false
	}
	got = got[len(got)-len(want):]
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func pathSegments(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	parts := strings.Split(p, "/")
	out := parts[:0]
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
