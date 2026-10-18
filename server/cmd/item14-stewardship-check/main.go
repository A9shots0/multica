package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func main() {
	manifestPath := flag.String("manifest", "internal/agentqualification/item14_stewardship_manifest_v3.json", "Item 14 stewardship manifest")
	sourceRoot := flag.String("source-root", "..", "repository root containing bound Item 14 sources")
	flag.Parse()

	raw, err := os.ReadFile(*manifestPath)
	if err != nil {
		fatal(err)
	}
	var identity struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil {
		fatal(err)
	}
	manifestHash := sha256.Sum256(raw)
	actualHash := hex.EncodeToString(manifestHash[:])
	var result any
	switch identity.SchemaVersion {
	case agentqualification.Item14StewardshipSchemaV1:
		if actualHash != agentqualification.Item14StewardshipManifestV1SHA256() {
			fatal(fmt.Errorf("on-disk Item 14 stewardship v1 manifest differs from the compiled manifest"))
		}
		if _, err := agentqualification.DecodeItem14StewardshipManifestV1(raw); err != nil {
			fatal(err)
		}
		result, err = agentqualification.VerifyItem14StewardshipV1(os.DirFS(*sourceRoot))
	case agentqualification.Item14StewardshipSchemaV2:
		if actualHash != agentqualification.Item14StewardshipManifestV2SHA256() {
			fatal(fmt.Errorf("on-disk Item 14 stewardship v2 manifest differs from the compiled manifest"))
		}
		if _, err := agentqualification.DecodeItem14StewardshipManifestV2(raw); err != nil {
			fatal(err)
		}
		result, err = agentqualification.VerifyItem14StewardshipV2(os.DirFS(*sourceRoot))
	case agentqualification.Item14StewardshipSchemaV3:
		if actualHash != agentqualification.Item14StewardshipManifestV3SHA256() {
			fatal(fmt.Errorf("on-disk Item 14 stewardship v3 manifest differs from the compiled manifest"))
		}
		if _, err := agentqualification.DecodeItem14StewardshipManifestV3(raw); err != nil {
			fatal(err)
		}
		result, err = agentqualification.VerifyItem14StewardshipV3(os.DirFS(*sourceRoot))
	default:
		fatal(fmt.Errorf("unsupported Item 14 stewardship schema %q", identity.SchemaVersion))
	}
	if err != nil {
		fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
