package format

// VersionFormat defines an interface for version validation and comparison strategies.
// IsValid checks if the provided version is valid based on specific format rules.
// Compare compares two versions and returns 0 if equal, a negative value if version1 is less, or positive if greater.
type VersionFormat interface {

	// IsValid check if the provided version is valid or not
	IsValid(version string) bool

	// Compare two versions, return 0 is version are equals, 0 < if version1 is lower than version 2
	Compare(version1, version2 string) (int, error)
}
