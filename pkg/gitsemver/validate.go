package gitsemver

import "regexp"

// numID is a semver-compliant numeric identifier: zero, or a positive integer
// with no leading zeros (semver spec Rule 9).
const numID = `(?:0|[1-9][0-9]*)`

// vXYZ matches the optional "v" prefix and three dot-separated numeric
// identifiers, each without leading zeros.
const vXYZ = `v?` + numID + `\.` + numID + `\.` + numID

var validStableRegex = regexp.MustCompile(`^` + vXYZ + `$`)
var validRCRegex = regexp.MustCompile(`^` + vXYZ + `-rc\.` + numID + `$`)

// validDevRegex matches the current dev build schema
// "X.Y.Z-b<8 hex>t<14 digits>c<7 hex>", the only schema generation emits.
var validDevRegex = regexp.MustCompile(`^` + vXYZ + `-b[0-9a-f]{8}t[0-9]{14}c[0-9a-f]{7}$`)

// validLegacyDevRegex matches the superseded dev build schema
// "X.Y.Z-dev.<branch>.<YYYY-MM-DD>.<HH-MM-SS>[.h<7-hex>]". Tags in that format
// are already published, so they must keep validating. The trailing commit-hash
// segment is optional because it was added later than the rest of the schema.
var validLegacyDevRegex = regexp.MustCompile(`^` + vXYZ + `-dev\.[a-zA-Z0-9-]+\.[0-9]{4}-[0-9]{2}-[0-9]{2}\.[0-9]{2}-[0-9]{2}-[0-9]{2}(?:\.h[0-9a-f]{7})?$`)

// IsValidStable reports whether version is a valid stable release version
// (X.Y.Z or vX.Y.Z, no leading zeros in any component).
func IsValidStable(version string) bool {
	return validStableRegex.MatchString(version)
}

// IsValidRC reports whether version is a valid release-candidate version
// (X.Y.Z-rc.N or vX.Y.Z-rc.N, no leading zeros in any numeric component).
func IsValidRC(version string) bool {
	return validRCRegex.MatchString(version)
}

// IsValidDev reports whether version is a valid dev-build version, with or
// without a leading "v" and with no leading zeros in the X.Y.Z components. It
// accepts both the current schema (X.Y.Z-b<8-hex>t<14-digits>c<7-hex>) and the
// superseded one (X.Y.Z-dev.<branch>.<YYYY-MM-DD>.<HH-MM-SS>[.h<7-hex>]),
// because tags in the old format are already published. This checks the shape
// only, not whether the time stamp is a real date.
func IsValidDev(version string) bool {
	return validDevRegex.MatchString(version) || validLegacyDevRegex.MatchString(version)
}

// IsValid reports whether version is a valid version string in any of the
// three recognised formats: stable, RC, or dev build.
func IsValid(version string) bool {
	return IsValidStable(version) || IsValidRC(version) || IsValidDev(version)
}
