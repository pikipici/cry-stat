package skill

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zipEntry struct {
	name, body string
	mode       os.FileMode
}

func makeZIP(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func validArchive(t *testing.T) []byte {
	return makeZIP(t, zipEntry{name: "meme-token-skill/SKILL.md", body: "---\nname: test\n---\n"}, zipEntry{name: "meme-token-skill/README.md", body: "safe"})
}

func TestQuarantineExtractsSafeArchive(t *testing.T) {
	out := filepath.Join(t.TempDir(), "q")
	report, err := Quarantine(bytes.NewReader(validArchive(t)), out, DefaultArchivePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if report.FileCount != 2 || report.TotalBytes == 0 || !report.HasSkillMD {
		t.Fatalf("report=%+v", report)
	}
	if _, err := os.Stat(filepath.Join(out, "meme-token-skill", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}
func TestQuarantineRejectsDangerousEntries(t *testing.T) {
	cases := map[string][]byte{
		"zip-slip":              makeZIP(t, zipEntry{name: "../escape", body: "x"}, zipEntry{name: "SKILL.md", body: "x"}),
		"absolute":              makeZIP(t, zipEntry{name: "/evil", body: "x"}, zipEntry{name: "SKILL.md", body: "x"}),
		"symlink":               makeZIP(t, zipEntry{name: "link", body: "target", mode: os.ModeSymlink | 0777}, zipEntry{name: "SKILL.md", body: "x"}),
		"nested":                makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.zip", body: "PK"}),
		"nested-magic":          makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.txt", body: "PK\x03\x04hidden"}),
		"nested-empty-zip":      makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.txt", body: "PK\x05\x06hidden"}),
		"nested-prepended":      makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.txt", body: "stub-data-PK\x03\x04hidden"}),
		"nested-long-prepended": makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.txt", body: strings.Repeat("s", 5000) + "PK\x03\x04hidden"}),
		"nested-prepended-rar":  makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.txt", body: strings.Repeat("s", 5000) + "Rar!\x1a\x07hidden"}),
		"nested-prepended-7z":   makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.txt", body: strings.Repeat("s", 5000) + "7z\xbc\xaf\x27\x1chidden"}),
		"nested-bzip":           makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.txt", body: "BZhhidden"}),
		"nested-xz":             makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.txt", body: "\xfd7zXZ\x00hidden"}),
		"nested-tar":            makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "payload.tar", body: "hidden"}),
		"dir-symlink":           makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "link/", body: "", mode: os.ModeSymlink | 0777}),
		"duplicate":             makeZIP(t, zipEntry{name: "SKILL.md", body: "x"}, zipEntry{name: "a/../SKILL.md", body: "y"}),
		"missing-skill":         makeZIP(t, zipEntry{name: "README.md", body: "x"}),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "q")
			if _, err := Quarantine(bytes.NewReader(data), out, DefaultArchivePolicy()); err == nil {
				t.Fatal("dangerous archive accepted")
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("partial quarantine left: %v", err)
			}
		})
	}
}
func TestQuarantineEnforcesCaps(t *testing.T) {
	p := DefaultArchivePolicy()
	p.MaxFiles = 1
	if _, err := Quarantine(bytes.NewReader(validArchive(t)), filepath.Join(t.TempDir(), "q"), p); err == nil {
		t.Fatal("file cap bypass")
	}
	p = DefaultArchivePolicy()
	p.MaxFileBytes = 2
	if _, err := Quarantine(bytes.NewReader(validArchive(t)), filepath.Join(t.TempDir(), "q"), p); err == nil {
		t.Fatal("size cap bypass")
	}
}
func TestRegistryVerifyHashBeforeQuarantine(t *testing.T) {
	data := validArchive(t)
	manifest := Manifest{ArtifactSHA256: strings.Repeat("0", 64)}
	_, err := InspectAndQuarantine(bytes.NewReader(data), manifest, filepath.Join(t.TempDir(), "q"), DefaultArchivePolicy())
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("err=%v", err)
	}
}
