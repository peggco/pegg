package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxArchiveBytes = 250 << 20
	maxMemberBytes  = 200 << 20
)

type ExtractOptions struct {
	BinaryPath  string
	InstallName string
	Repo        string
}

type archiveCandidate struct {
	name       string
	base       string
	data       []byte
	executable bool
}

func ExtractBinary(assetName string, data []byte, opts ExtractOptions) ([]byte, error) {
	lower := strings.ToLower(assetName)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("open gzip archive: %w", err)
		}
		defer reader.Close()
		return extractTar(reader, opts)
	case strings.HasSuffix(lower, ".tar"):
		return extractTar(bytes.NewReader(data), opts)
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(data, opts)
	case strings.HasSuffix(lower, ".gz"):
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("open gzip file: %w", err)
		}
		defer reader.Close()
		out, err := io.ReadAll(io.LimitReader(reader, maxMemberBytes+1))
		if err != nil {
			return nil, err
		}
		if len(out) > maxMemberBytes {
			return nil, fmt.Errorf("decompressed binary exceeds %d bytes", maxMemberBytes)
		}
		return out, nil
	default:
		return data, nil
	}
}

func extractTar(reader io.Reader, opts ExtractOptions) ([]byte, error) {
	tarReader := tar.NewReader(reader)
	var candidates []archiveCandidate
	var total int64
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if !safeArchiveName(header.Name) {
			return nil, fmt.Errorf("unsafe path in archive: %q", header.Name)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if header.Size < 0 || header.Size > maxMemberBytes {
			return nil, fmt.Errorf("archive member %q is too large", header.Name)
		}
		total += header.Size
		if total > maxArchiveBytes {
			return nil, fmt.Errorf("archive expands beyond %d bytes", maxArchiveBytes)
		}
		data, err := io.ReadAll(io.LimitReader(tarReader, header.Size))
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, archiveCandidate{
			name:       header.Name,
			base:       path.Base(header.Name),
			data:       data,
			executable: header.FileInfo().Mode()&0o111 != 0,
		})
	}
	return selectArchiveCandidate(candidates, opts)
}

func extractZip(data []byte, opts ExtractOptions) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}
	var candidates []archiveCandidate
	var total int64
	for _, file := range reader.File {
		if !safeArchiveName(file.Name) {
			return nil, fmt.Errorf("unsafe path in archive: %q", file.Name)
		}
		if file.FileInfo().IsDir() || file.FileInfo().Mode()&os.ModeSymlink != 0 {
			continue
		}
		if file.UncompressedSize64 > maxMemberBytes {
			return nil, fmt.Errorf("archive member %q is too large", file.Name)
		}
		total += int64(file.UncompressedSize64)
		if total > maxArchiveBytes {
			return nil, fmt.Errorf("archive expands beyond %d bytes", maxArchiveBytes)
		}
		handle, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(handle, maxMemberBytes+1))
		_ = handle.Close()
		if err != nil {
			return nil, err
		}
		if len(content) > maxMemberBytes {
			return nil, fmt.Errorf("archive member %q is too large", file.Name)
		}
		candidates = append(candidates, archiveCandidate{
			name:       file.Name,
			base:       path.Base(file.Name),
			data:       content,
			executable: file.FileInfo().Mode()&0o111 != 0,
		})
	}
	return selectArchiveCandidate(candidates, opts)
}

func selectArchiveCandidate(candidates []archiveCandidate, opts ExtractOptions) ([]byte, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("archive contains no regular files")
	}
	if binaryPath := strings.TrimSpace(opts.BinaryPath); binaryPath != "" {
		clean := path.Clean(binaryPath)
		if !safeArchiveName(clean) {
			return nil, fmt.Errorf("unsafe binary path %q", binaryPath)
		}
		for _, candidate := range candidates {
			if path.Clean(candidate.name) == clean || candidate.base == clean {
				return candidate.data, nil
			}
		}
		return nil, fmt.Errorf("binary %q not found in archive (available: %s)", binaryPath, candidateNames(candidates))
	}

	preferred := []string{opts.InstallName, opts.Repo, "pegg-" + opts.Repo}
	for _, name := range preferred {
		if name == "" {
			continue
		}
		for _, candidate := range candidates {
			if candidate.base == name || candidate.base == name+".exe" {
				return candidate.data, nil
			}
		}
	}

	var executable []archiveCandidate
	for _, candidate := range candidates {
		if candidate.executable {
			executable = append(executable, candidate)
		}
	}
	if len(executable) == 1 {
		return executable[0].data, nil
	}
	if len(candidates) == 1 {
		return candidates[0].data, nil
	}
	return nil, fmt.Errorf("multiple files in archive; use --binary (available: %s)", candidateNames(candidates))
}

func candidateNames(candidates []archiveCandidate) string {
	names := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		names = append(names, candidate.name)
	}
	return strings.Join(names, ", ")
}

func safeArchiveName(name string) bool {
	if name == "" || strings.ContainsRune(name, 0) {
		return false
	}
	if path.IsAbs(name) || filepath.IsAbs(name) {
		return false
	}
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	return true
}
