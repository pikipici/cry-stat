package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifestJSON = `{
  "schema_version": "skill.artifact.v1",
  "name": "meme-token-research",
  "version": "1.1.0",
  "artifact_sha256": "07e506260e11ad0d4f9346f39cd0dbce47f6f56e87953c92d6a5d8dcccd10bd8",
  "strategy_contract_version": "meme-research.v1",
  "capabilities": {
    "research_only": true,
    "offline_simulation": true,
    "live_read_only_connectors": true
  },
  "prohibitions": {
    "executes_transactions": false,
    "is_buy_signal": false,
    "stores_private_keys": false
  }
}`

func TestParseManifestAcceptsResearchOnlyArtifact(t *testing.T) {
	manifest, err := ParseManifest(strings.NewReader(validManifestJSON))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if manifest.Name != "meme-token-research" || manifest.Version != "1.1.0" {
		t.Fatalf("identity = %q@%q", manifest.Name, manifest.Version)
	}
	if !manifest.Capabilities.ResearchOnly || manifest.Capabilities.OfflineSimulation == nil || !*manifest.Capabilities.OfflineSimulation {
		t.Fatalf("capabilities tidak dipertahankan: %+v", manifest.Capabilities)
	}
	if *manifest.Prohibitions.ExecutesTransactions || *manifest.Prohibitions.IsBuySignal || *manifest.Prohibitions.StoresPrivateKeys {
		t.Fatalf("prohibitions tidak fail-closed: %+v", manifest.Prohibitions)
	}
}

func TestParseManifestRejectsTrailingJSON(t *testing.T) {
	_, err := ParseManifest(strings.NewReader(validManifestJSON + ` {"extra":true}`))
	if err == nil || !strings.Contains(err.Error(), "single JSON document") {
		t.Fatalf("error = %v, mau single JSON document", err)
	}
}

func TestParseManifestRejectsDuplicateKeysAtAnyDepth(t *testing.T) {
	cases := []string{
		strings.Replace(validManifestJSON, `"name": "meme-token-research"`, `"name": "safe", "name": "meme-token-research"`, 1),
		strings.Replace(validManifestJSON, `"research_only": true`, `"research_only": false, "research_only": true`, 1),
		strings.Replace(validManifestJSON, `"executes_transactions": false`, `"executes_transactions": true, "executes_transactions": false`, 1),
	}
	for _, input := range cases {
		_, err := ParseManifest(strings.NewReader(input))
		if err == nil || !strings.Contains(err.Error(), "duplicate JSON key") {
			t.Fatalf("error = %v, mau duplicate JSON key", err)
		}
	}
}

func TestParseManifestRejectsCaseVariantKeys(t *testing.T) {
	input := strings.Replace(validManifestJSON, `"research_only": true`, `"research_only": false, "Research_Only": true`, 1)
	_, err := ParseManifest(strings.NewReader(input))
	if err == nil || (!strings.Contains(err.Error(), "unknown JSON key") && !strings.Contains(err.Error(), "duplicate JSON key")) {
		t.Fatalf("error = %v, mau reject case-variant key", err)
	}
}

func TestParseManifestRejectsOversizedTrailingJSON(t *testing.T) {
	padding := strings.Repeat(" ", maxManifestBytes-len(validManifestJSON))
	_, err := ParseManifest(strings.NewReader(validManifestJSON + padding + ` {"extra":true}`))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, mau size exceeds", err)
	}
}

func TestParseManifestRequiresAllCapabilities(t *testing.T) {
	for _, field := range []string{
		`    "offline_simulation": true,` + "\n",
		`,` + "\n" + `    "live_read_only_connectors": true`,
	} {
		input := strings.Replace(validManifestJSON, field, "", 1)
		_, err := ParseManifest(strings.NewReader(input))
		if err == nil || !strings.Contains(err.Error(), "is required") {
			t.Fatalf("field %q error = %v, mau is required", field, err)
		}
	}
}

func TestParseManifestRejectsUnsupportedContract(t *testing.T) {
	input := strings.Replace(validManifestJSON, "meme-research.v1", "meme-research.v99", 1)
	_, err := ParseManifest(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "unsupported strategy contract") {
		t.Fatalf("error = %v, mau unsupported strategy contract", err)
	}
}

func TestParseManifestRejectsMissingRequiredProhibition(t *testing.T) {
	input := strings.Replace(validManifestJSON, `    "executes_transactions": false,
`, "", 1)
	_, err := ParseManifest(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "executes_transactions") {
		t.Fatalf("error = %v, mau missing executes_transactions", err)
	}
}

func TestParseManifestRejectsUnsafeTrueProhibition(t *testing.T) {
	input := strings.Replace(validManifestJSON, `"executes_transactions": false`, `"executes_transactions": true`, 1)
	_, err := ParseManifest(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "must be false") {
		t.Fatalf("error = %v, mau must be false", err)
	}
}

func TestBaselineManifestParses(t *testing.T) {
	path := filepath.Join("..", "..", "skills", "meme-token", "v1.1.0", "manifest.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	manifest, err := ParseManifest(file)
	if err != nil {
		t.Fatalf("ParseManifest(baseline) error = %v", err)
	}
	if manifest.ArtifactSHA256 != "07e506260e11ad0d4f9346f39cd0dbce47f6f56e87953c92d6a5d8dcccd10bd8" {
		t.Fatalf("baseline sha256 = %q", manifest.ArtifactSHA256)
	}
}

func TestVerifyArtifactSHA256(t *testing.T) {
	artifact := []byte("immutable artifact bytes")
	sum := sha256.Sum256(artifact)
	manifestJSON := strings.Replace(validManifestJSON,
		"07e506260e11ad0d4f9346f39cd0dbce47f6f56e87953c92d6a5d8dcccd10bd8",
		hex.EncodeToString(sum[:]), 1)
	manifest, err := ParseManifest(strings.NewReader(manifestJSON))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "artifact.zip")
	if err := os.WriteFile(path, artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manifest.VerifyArtifact(path); err != nil {
		t.Fatalf("VerifyArtifact() error = %v", err)
	}

	if err := os.WriteFile(path, append(artifact, '!'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manifest.VerifyArtifact(path); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("error = %v, mau sha256 mismatch", err)
	}
}
