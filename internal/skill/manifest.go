package skill

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	ManifestSchemaVersion     = "skill.artifact.v1"
	SupportedStrategyContract = "meme-research.v1"
	maxManifestBytes          = 1 << 20
)

type Manifest struct {
	SchemaVersion           string       `json:"schema_version"`
	Name                    string       `json:"name"`
	Version                 string       `json:"version"`
	ArtifactSHA256          string       `json:"artifact_sha256"`
	StrategyContractVersion string       `json:"strategy_contract_version"`
	Capabilities            Capabilities `json:"capabilities"`
	Prohibitions            Prohibitions `json:"prohibitions"`
}

type Capabilities struct {
	ResearchOnly           bool  `json:"research_only"`
	OfflineSimulation      *bool `json:"offline_simulation"`
	LiveReadOnlyConnectors *bool `json:"live_read_only_connectors"`
}

type Prohibitions struct {
	ExecutesTransactions *bool `json:"executes_transactions"`
	IsBuySignal          *bool `json:"is_buy_signal"`
	StoresPrivateKeys    *bool `json:"stores_private_keys"`
}

func ParseManifest(r io.Reader) (Manifest, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxManifestBytes+1))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	if len(data) > maxManifestBytes {
		return Manifest{}, fmt.Errorf("manifest exceeds %d bytes", maxManifestBytes)
	}
	if err := validateJSONDocument(data); err != nil {
		return Manifest{}, err
	}

	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func validateJSONDocument(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("decode manifest: %w", err)
		}
		return errors.New("manifest must contain a single JSON document")
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}

	switch delimiter {
	case '{':
		keys := make(map[string]string)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("decode manifest: %w", err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("decode manifest: object key must be a string")
			}
			foldedKey := strings.ToLower(key)
			if prior, exists := keys[foldedKey]; exists {
				return fmt.Errorf("duplicate JSON key %q conflicts with %q", key, prior)
			}
			keys[foldedKey] = key
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return errors.New("decode manifest: unexpected closing delimiter")
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	return nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("unsupported manifest schema %q", m.SchemaVersion)
	}
	if m.Name == "" || m.Version == "" {
		return errors.New("manifest name and version are required")
	}
	if _, err := decodeSHA256(m.ArtifactSHA256); err != nil {
		return err
	}
	if m.StrategyContractVersion != SupportedStrategyContract {
		return fmt.Errorf("unsupported strategy contract %q", m.StrategyContractVersion)
	}
	if !m.Capabilities.ResearchOnly {
		return errors.New("capabilities.research_only must be true")
	}
	for name, value := range map[string]*bool{
		"capabilities.offline_simulation":        m.Capabilities.OfflineSimulation,
		"capabilities.live_read_only_connectors": m.Capabilities.LiveReadOnlyConnectors,
	} {
		if value == nil {
			return fmt.Errorf("%s is required", name)
		}
	}
	for name, value := range map[string]*bool{
		"prohibitions.executes_transactions": m.Prohibitions.ExecutesTransactions,
		"prohibitions.is_buy_signal":         m.Prohibitions.IsBuySignal,
		"prohibitions.stores_private_keys":   m.Prohibitions.StoresPrivateKeys,
	} {
		if value == nil {
			return fmt.Errorf("%s is required", name)
		}
		if *value {
			return fmt.Errorf("%s must be false", name)
		}
	}
	return nil
}

func (m Manifest) VerifyArtifact(path string) error {
	want, err := decodeSHA256(m.ArtifactSHA256)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open artifact: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash artifact: %w", err)
	}
	got := hash.Sum(nil)
	if !equalBytes(got, want) {
		return fmt.Errorf("artifact sha256 mismatch: got %s", hex.EncodeToString(got))
	}
	return nil
}

func decodeSHA256(value string) ([]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return nil, errors.New("artifact_sha256 must be 64 lowercase hexadecimal characters")
	}
	if value != hex.EncodeToString(decoded) {
		return nil, errors.New("artifact_sha256 must be 64 lowercase hexadecimal characters")
	}
	return decoded, nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var difference byte
	for i := range a {
		difference |= a[i] ^ b[i]
	}
	return difference == 0
}
