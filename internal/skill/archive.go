package skill

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ArchivePolicy struct {
	MaxArchiveBytes, MaxFileBytes, MaxTotalBytes int64
	MaxFiles                                     int
}
type ArchiveReport struct {
	FileCount  int
	TotalBytes int64
	HasSkillMD bool
	Files      []string
}

func DefaultArchivePolicy() ArchivePolicy {
	return ArchivePolicy{MaxArchiveBytes: 32 << 20, MaxFileBytes: 8 << 20, MaxTotalBytes: 64 << 20, MaxFiles: 1000}
}

func Quarantine(r io.Reader, destination string, policy ArchivePolicy) (ArchiveReport, error) {
	data, err := io.ReadAll(io.LimitReader(r, policy.MaxArchiveBytes+1))
	if err != nil {
		return ArchiveReport{}, err
	}
	if int64(len(data)) > policy.MaxArchiveBytes {
		return ArchiveReport{}, errors.New("archive exceeds size cap")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ArchiveReport{}, fmt.Errorf("open zip: %w", err)
	}
	report := ArchiveReport{}
	seen := map[string]struct{}{}
	for _, f := range zr.File {
		clean, err := safeArchivePath(f.Name)
		if err != nil {
			return ArchiveReport{}, err
		}
		mode := f.Mode()
		isDirectory := strings.HasSuffix(f.Name, "/") || mode.IsDir()
		if mode&os.ModeSymlink != 0 || (!isDirectory && !mode.IsRegular()) || (isDirectory && !mode.IsDir()) {
			return ArchiveReport{}, errors.New("non-regular archive entry rejected")
		}
		if clean == "" || isDirectory {
			continue
		}
		if _, ok := seen[strings.ToLower(clean)]; ok {
			return ArchiveReport{}, errors.New("duplicate normalized path")
		}
		seen[strings.ToLower(clean)] = struct{}{}
		if nestedArchiveName(clean) {
			return ArchiveReport{}, errors.New("nested archive rejected")
		}
		if int64(f.UncompressedSize64) > policy.MaxFileBytes {
			return ArchiveReport{}, errors.New("file exceeds size cap")
		}
		archiveLike, err := archiveMagic(f)
		if err != nil {
			return ArchiveReport{}, err
		}
		if archiveLike {
			return ArchiveReport{}, errors.New("nested archive content rejected")
		}
		report.FileCount++
		report.TotalBytes += int64(f.UncompressedSize64)
		if report.FileCount > policy.MaxFiles || report.TotalBytes > policy.MaxTotalBytes {
			return ArchiveReport{}, errors.New("archive limits exceeded")
		}
		if strings.EqualFold(filepath.Base(clean), "SKILL.md") {
			report.HasSkillMD = true
		}
		report.Files = append(report.Files, clean)
	}
	if !report.HasSkillMD {
		return ArchiveReport{}, errors.New("SKILL.md is required")
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return ArchiveReport{}, err
	}
	lock, err := os.OpenFile(destination+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ArchiveReport{}, errors.New("quarantine destination is locked or already publishing")
	}
	lock.Close()
	defer os.Remove(destination + ".lock")
	temp, err := os.MkdirTemp(parent, ".quarantine-")
	if err != nil {
		return ArchiveReport{}, err
	}
	defer os.RemoveAll(temp)
	for _, f := range zr.File {
		clean, err := safeArchivePath(f.Name)
		if err != nil {
			return ArchiveReport{}, err
		}
		if clean == "" || strings.HasSuffix(f.Name, "/") {
			continue
		}
		target := filepath.Join(temp, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return ArchiveReport{}, err
		}
		src, err := f.Open()
		if err != nil {
			return ArchiveReport{}, err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			src.Close()
			return ArchiveReport{}, err
		}
		written, copyErr := io.Copy(dst, io.LimitReader(src, policy.MaxFileBytes+1))
		closeErr := dst.Close()
		src.Close()
		if copyErr != nil || closeErr != nil || written > policy.MaxFileBytes || written != int64(f.UncompressedSize64) {
			return ArchiveReport{}, errors.New("archive extraction size mismatch")
		}
	}
	if _, err := os.Stat(destination); err == nil {
		return ArchiveReport{}, errors.New("quarantine destination exists")
	}
	if err := os.Rename(temp, destination); err != nil {
		return ArchiveReport{}, err
	}
	return report, nil
}
func nestedArchiveName(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{".zip", ".jar", ".tar", ".tar.gz", ".tgz", ".7z", ".rar", ".gz", ".bz2", ".xz"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func archiveMagic(file *zip.File) (bool, error) {
	reader, err := file.Open()
	if err != nil {
		return false, err
	}
	defer reader.Close()
	header, err := io.ReadAll(io.LimitReader(reader, int64(file.UncompressedSize64)+1))
	if err != nil {
		return false, err
	}
	if uint64(len(header)) != file.UncompressedSize64 {
		return false, errors.New("archive entry size mismatch during inspection")
	}
	for _, signature := range [][]byte{
		[]byte("PK\x03\x04"), []byte("PK\x05\x06"), []byte("PK\x07\x08"),
		[]byte("7z\xbc\xaf\x27\x1c"), []byte("Rar!\x1a\x07"), []byte("\x1f\x8b"),
		[]byte("BZh"), []byte("\xfd7zXZ\x00"),
	} {
		if bytes.HasPrefix(header, signature) {
			return true, nil
		}
	}
	// Self-extracting archives allow arbitrary prepended stubs.
	for _, signature := range [][]byte{
		[]byte("PK\x03\x04"), []byte("PK\x05\x06"), []byte("PK\x07\x08"),
		[]byte("7z\xbc\xaf\x27\x1c"), []byte("Rar!\x1a\x07"),
	} {
		if bytes.Contains(header, signature) {
			return true, nil
		}
	}
	return len(header) >= 262 && string(header[257:262]) == "ustar", nil
}

func safeArchivePath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.HasPrefix(name, "/") || filepath.IsAbs(name) {
		return "", errors.New("absolute archive path rejected")
	}
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(name, "/../") {
		return "", errors.New("archive path traversal rejected")
	}
	if strings.ContainsRune(clean, 0) || strings.Contains(clean, ":") {
		return "", errors.New("invalid archive path")
	}
	return clean, nil
}
