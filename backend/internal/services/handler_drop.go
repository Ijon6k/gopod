package services

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopod/pkg/httputil"
)

// handleDropUpload handles multipart .zip file upload for static sites.
func (h *Handler) handleDropUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc, err := h.service.GetService(r.Context(), id)
	if err != nil || svc == nil {
		httputil.WriteError(w, http.StatusNotFound, "Service not found")
		return
	}

	// Cap the request body at the configured upload limit: ParseMultipartForm
	// only bounds in-memory buffering, not the body size.
	maxUploadMB := 100
	if raw := strings.TrimSpace(os.Getenv("GOPOD_MAX_UPLOAD_MB")); raw != "" {
		if v, convErr := strconv.Atoi(raw); convErr == nil && v > 0 && v <= 4096 {
			maxUploadMB = v
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxUploadMB)<<20+(1<<20))
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httputil.WriteError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Upload exceeds the %d MB limit", maxUploadMB))
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, "Invalid multipart upload: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "No zip file uploaded: "+err.Error())
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		httputil.WriteError(w, http.StatusBadRequest, "Only .zip archive files are supported")
		return
	}

	// Target directory on disk
	dataDir := "./data"
	if h.service.deployer != nil && h.service.deployer.dataDir != "" {
		dataDir = h.service.deployer.dataDir
	}
	distDir := filepath.Join(dataDir, "static", svc.ProjectID, svc.ID, "dist")

	// Save zip to temporary file first for safe reading
	tempFile, err := os.CreateTemp("", "gopod-upload-*.zip")
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to create temp buffer: "+err.Error())
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, file); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to save upload: "+err.Error())
		return
	}

	zipReader, err := zip.OpenReader(tempFile.Name())
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Corrupted or invalid zip archive: "+err.Error())
		return
	}
	defer zipReader.Close()

	// Clean out previous files in distDir
	_ = os.RemoveAll(distDir)
	if err := os.MkdirAll(distDir, 0755); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "Failed to create static destination dir: "+err.Error())
		return
	}

	// Filter valid entries (skip macOS __MACOSX metadata)
	var validEntries []*zip.File
	for _, f := range zipReader.File {
		if !strings.HasPrefix(f.Name, "__MACOSX/") && !strings.Contains(f.Name, "/__MACOSX/") {
			validEntries = append(validEntries, f)
		}
	}

	if len(validEntries) == 0 {
		_ = os.RemoveAll(distDir)
		httputil.WriteError(w, http.StatusBadRequest, "Zip archive contains no valid files")
		return
	}

	cleanDist := filepath.Clean(distDir)

	// Pre-scan: reject zip-slip path traversal early before any extraction or root analysis
	for _, f := range validEntries {
		rawTarget := filepath.Clean(filepath.Join(distDir, f.Name))
		if !strings.HasPrefix(rawTarget, cleanDist+string(os.PathSeparator)) && rawTarget != cleanDist {
			_ = os.RemoveAll(distDir)
			httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Illegal zip path detected: %s", f.Name))
			return
		}
	}

	// Check if all files are wrapped in a single root folder (e.g. "dist/" or "build/")
	singleRoot := ""
	allInRoot := len(validEntries) > 0
	for _, f := range validEntries {
		name := strings.Trim(f.Name, "/")
		parts := strings.Split(name, "/")
		if len(parts) == 0 || parts[0] == "" || parts[0] == "." || parts[0] == ".." {
			allInRoot = false
			break
		}
		// If an entry is at root level and is not a directory, the archive has root-level files
		if len(parts) == 1 && !strings.HasSuffix(f.Name, "/") && !f.FileInfo().IsDir() {
			allInRoot = false
			break
		}
		if singleRoot == "" {
			singleRoot = parts[0]
		} else if singleRoot != parts[0] {
			allInRoot = false
			break
		}
	}
	if !allInRoot {
		singleRoot = ""
	}

	// Zip bomb guard: compressed uploads are cheap to make enormous when
	// decompressed (a 10 MB zip can expand to tens of GB). Cap the total
	// uncompressed payload at 10× the upload cap and the entry count, then
	// refuse — and leave no partially extracted tree on disk — if either
	// bound is crossed.
	maxTotalUncompressed := (int64(maxUploadMB) * 10) << 20 // 10× the upload cap, e.g. 100 MB → 1 GB uncompressed
	const maxZipEntries = 50000
	if len(validEntries) > maxZipEntries {
		_ = os.RemoveAll(distDir)
		httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Zip archive contains too many entries (%d files, limit is %d)", len(validEntries), maxZipEntries))
		return
	}

	var totalUncompressed int64
	exceeded := false

	extractedCount := 0

	for _, f := range validEntries {
		relPath := f.Name
		if singleRoot != "" && strings.HasPrefix(relPath, singleRoot+"/") {
			relPath = strings.TrimPrefix(relPath, singleRoot+"/")
		}
		if relPath == "" || relPath == singleRoot || relPath == singleRoot+"/" {
			continue
		}

		targetPath := filepath.Join(distDir, relPath)
		cleanTarget := filepath.Clean(targetPath)

		// Zip Slip security guard: ensure target is inside distDir
		if !strings.HasPrefix(cleanTarget, cleanDist+string(os.PathSeparator)) && cleanTarget != cleanDist {
			_ = os.RemoveAll(distDir)
			httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Illegal zip path detected: %s", f.Name))
			return
		}

		// Reject symlinks to prevent symlink traversal
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(cleanTarget, 0755)
			_ = os.Chmod(cleanTarget, 0755)
			continue
		}

		_ = os.MkdirAll(filepath.Dir(cleanTarget), 0755)
		outFile, err := os.OpenFile(cleanTarget, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			continue
		}

		// Bounded copy: read at most one byte beyond the remaining budget so
		// a breach is detected without extracting it.
		remaining := maxTotalUncompressed - totalUncompressed
		if remaining <= 0 {
			rc.Close()
			exceeded = true
			break
		}
		written, _ := io.Copy(outFile, io.LimitReader(rc, remaining+1))
		outFile.Close()
		rc.Close()
		_ = os.Chmod(cleanTarget, 0644)
		extractedCount++

		totalUncompressed += written
		if totalUncompressed > maxTotalUncompressed {
			exceeded = true
			break
		}
	}

	if exceeded {
		// A zip bomb was streaming through: drop the half-extracted tree so
		// nothing partial is served, then report the real cause.
		_ = os.RemoveAll(distDir)
		httputil.WriteError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Decompressed upload exceeds the %d MB budget (zip bomb protection)", maxTotalUncompressed>>20))
		return
	}

	// Update service state to static drop
	absDist, _ := filepath.Abs(distDir)
	svc.SourceType = "drop"
	svc.Source = absDist
	if svc.Port <= 0 {
		svc.Port = 80
	}
	if svc.Image == "" {
		svc.Image = "docker.io/library/nginx:alpine"
	}

	_ = h.service.repo.Update(r.Context(), *svc)

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "success",
		"message":    fmt.Sprintf("Successfully extracted %d static files", extractedCount),
		"filesCount": extractedCount,
		"filename":   header.Filename,
		"directory":  absDist,
	})
}
