package skill

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

func InspectAndQuarantine(r io.Reader, manifest Manifest, destination string, policy ArchivePolicy) (ArchiveReport, error) {
	data, err := io.ReadAll(io.LimitReader(r, policy.MaxArchiveBytes+1))
	if err != nil {
		return ArchiveReport{}, err
	}
	if int64(len(data)) > policy.MaxArchiveBytes {
		return ArchiveReport{}, errors.New("archive exceeds size cap")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != manifest.ArtifactSHA256 {
		return ArchiveReport{}, errors.New("artifact sha256 mismatch")
	}
	return Quarantine(bytes.NewReader(data), destination, policy)
}
