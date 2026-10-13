package services

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

	// Limit zip upload size to 100 MB
	_ = r.ParseMultipartForm(100 << 20)

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

	// Check if all files are wrapped in a single root folder (e.g. "dist/" or "build/")
	singleRoot := ""
	for _, f := range validEntries {
		parts := strings.Split(strings.Trim(f.Name, "/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			if singleRoot == "" {
				singleRoot = parts[0]
			} else if singleRoot != parts[0] {
				singleRoot = ""
				break
			}
		}
	}

	extractedCount := 0
	cleanDist := filepath.Clean(distDir)

	for _, f := range validEntries {
		relPath := f.Name
		if singleRoot != "" && strings.HasPrefix(relPath, singleRoot+"/") {
			relPath = strings.TrimPrefix(relPath, singleRoot+"/")
		}
		if relPath == "" || relPath == singleRoot {
			continue
		}

		targetPath := filepath.Join(distDir, relPath)
		cleanTarget := filepath.Clean(targetPath)

		// Zip Slip security guard: ensure target is inside distDir
		if !strings.HasPrefix(cleanTarget, cleanDist+string(os.PathSeparator)) && cleanTarget != cleanDist {
			httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("Illegal zip path detected: %s", f.Name))
			return
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

		_, _ = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		_ = os.Chmod(cleanTarget, 0644)
		extractedCount++
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
