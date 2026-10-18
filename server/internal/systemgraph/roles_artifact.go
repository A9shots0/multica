package systemgraph

import (
	"crypto/sha256"
	"encoding/hex"
)

// RolesArtifactSHA256 identifies the exact embedded roles.json bytes that
// define mechanical role, action, host, and separation-of-duty boundaries.
func RolesArtifactSHA256() string {
	digest := sha256.Sum256(roleCatalogJSON)
	return hex.EncodeToString(digest[:])
}
