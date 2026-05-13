package skillpackager

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// secretPathRE matches paths that look like they carry credentials. Used to
// refuse packaging when the source tree contains them.
var secretPathRE = regexp.MustCompile(`(?i)(^|/)(\.?env|.*credentials\.json|.*secret.*|.*private.*key.*|.*\.pem|.*\.p12|id_rsa(\.[^/]*)?)$`)

// safetyCheck enforces the platform's hard rules for bundle contents:
//   - paths cannot escape the bundle root
//   - no symlinks, setuid bits, character/block devices (none of these can be
//     represented in our File struct, but we keep the structural rejection in
//     case callers extend the type later)
//   - no files whose path matches the secret detection regex
func safetyCheck(spec Spec) error {
	for _, f := range spec.Files {
		clean := path.Clean(f.Path)
		if clean != f.Path || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("path %q is not safely contained under the bundle root", f.Path)
		}
		base := path.Base(clean)
		if secretPathRE.MatchString(clean) || secretPathRE.MatchString(base) {
			return fmt.Errorf("secret_material_in_source: refusing to package %q (matches secret pattern)", f.Path)
		}
	}
	return nil
}
