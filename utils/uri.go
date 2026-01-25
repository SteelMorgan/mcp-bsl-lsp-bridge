package utils

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// NormalizeURI normalizes input into a valid URI.
//
// Accepts:
// - file URI (file://...)
// - absolute/relative local paths (Windows or POSIX)
// - other schemes (returned as-is)
//
// For file URIs, it also unescapes/normalizes the path part to be usable on the current OS.
func NormalizeURI(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return uri
	}

	// If it already has a file scheme, return as-is
	if strings.HasPrefix(uri, "file://") || strings.HasPrefix(uri, "file:") {
		// IMPORTANT: do not rewrite/percent-encode file URIs here.
		// Some language servers are sensitive to URI string equality for opened documents.
		return uri
	}

	// If it has any other scheme (http://, https://, etc.), return as-is
	if strings.Contains(uri, "://") {
		return uri
	}

	// Treat it as a local path.
	if u, err := PathToFileURI(uri); err == nil {
		return u
	}
	// Fallback: preserve original input with file:// prefix.
	return "file://" + filepath.ToSlash(uri)
}

// URIToFilePath converts an input (file URI or local path) to a local file path.
// If the input is not a file URI, it is returned unchanged.
func URIToFilePath(uri string) string {
	uri = strings.TrimSpace(uri)
	if strings.HasPrefix(uri, "file://") || strings.HasPrefix(uri, "file:") {
		if p, err := FileURIToPath(uri); err == nil {
			return p
		}
		// best-effort fallback
		return strings.TrimPrefix(strings.TrimPrefix(uri, "file://"), "file:")
	}
	return uri
}

// FilePathToURI converts a local file path to a file URI.
// If the input already looks like a URI, it is returned unchanged.
func FilePathToURI(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}

	if strings.Contains(path, "://") {
		return path
	}

	if u, err := PathToFileURI(path); err == nil {
		return u
	}
	return "file://" + filepath.ToSlash(path)
}

// FileURIToPath converts a file:// URI into a local OS path (decoding % escapes).
func FileURIToPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("invalid uri: %w", err)
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("not a file uri: %s", u.Scheme)
	}

	// UNC paths: file://server/share/path
	// Keep as a UNC-like path on the current OS.
	if u.Host != "" {
		p, err := url.PathUnescape(u.Path)
		if err != nil {
			return "", fmt.Errorf("invalid uri path escape: %w", err)
		}
		// For non-Windows OSes this will return //server/share/path (POSIX-style).
		return filepath.FromSlash("//" + u.Host + p), nil
	}

	p, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", fmt.Errorf("invalid uri path escape: %w", err)
	}

	// Windows drive-letter file URI: file:///C:/path -> /C:/path
	// IMPORTANT: handle this regardless of runtime OS (bridge may run in Linux container).
	if strings.HasPrefix(p, "/") && len(p) >= 3 && p[2] == ':' {
		p = p[1:]
	}

	return filepath.FromSlash(p), nil
}

// PathToFileURI converts a local OS path into a file:// URI.
func PathToFileURI(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	// Check if it's already a Windows absolute path (D:/... or D:\...)
	// IMPORTANT: Do this BEFORE filepath.Abs, because on Linux filepath.Abs
	// would prepend cwd to Windows paths like "D:/..." making them invalid.
	isWindowsAbs := IsWindowsAbsPath(path)
	
	if !isWindowsAbs {
		// Only call Abs for non-Windows paths (Unix paths or relative paths)
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}
	}

	// Normalize path separators to forward slashes
	slashPath := strings.ReplaceAll(path, "\\", "/")

	// Clean double slashes but preserve Windows drive letter
	if isWindowsAbs {
		// For Windows: D:/path → D:/path (keep as-is, just clean)
		slashPath = strings.ReplaceAll(slashPath, "//", "/")
	} else {
		slashPath = filepath.ToSlash(filepath.Clean(path))
	}

	// Windows drive-letter paths must have leading "/" in the URI path.
	// IMPORTANT: handle this regardless of runtime OS (bridge may run in Linux container).
	if len(slashPath) >= 2 && slashPath[1] == ':' {
		slashPath = "/" + slashPath
	}

	u := url.URL{
		Scheme: "file",
		Path:   slashPath,
	}
	return u.String(), nil
}
