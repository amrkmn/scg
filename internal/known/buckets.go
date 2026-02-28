package known

// KnownBuckets maps official Scoop bucket names to their source URLs.
var KnownBuckets = map[string]string{
	"main":         "https://github.com/ScoopInstaller/Main",
	"extras":       "https://github.com/ScoopInstaller/Extras",
	"versions":     "https://github.com/ScoopInstaller/Versions",
	"nirsoft":      "https://github.com/kodybrown/scoop-nirsoft",
	"sysinternals": "https://github.com/niheaven/scoop-sysinternals",
	"php":          "https://github.com/ScoopInstaller/PHP",
	"nerd-fonts":   "https://github.com/matthewjberger/scoop-nerd-fonts",
	"nonportable":  "https://github.com/ScoopInstaller/Nonportable",
	"java":         "https://github.com/ScoopInstaller/Java",
	"games":        "https://github.com/Calinou/scoop-games",
}

// KnownBucket holds a bucket name and its source URL.
type KnownBucket struct {
	Name   string
	Source string
}

// GetKnownBucket returns the source URL for a known bucket name, or empty string if not found.
func GetKnownBucket(name string) string {
	return KnownBuckets[name]
}

// GetAllKnownBuckets returns all known buckets as a sorted slice.
func GetAllKnownBuckets() []KnownBucket {
	// Return in a stable order
	order := []string{"main", "extras", "versions", "nirsoft", "sysinternals", "php", "nerd-fonts", "nonportable", "java", "games"}
	result := make([]KnownBucket, 0, len(order))
	for _, name := range order {
		if url, ok := KnownBuckets[name]; ok {
			result = append(result, KnownBucket{Name: name, Source: url})
		}
	}
	return result
}

// IsKnownBucket reports whether name is an officially known Scoop bucket.
func IsKnownBucket(name string) bool {
	_, ok := KnownBuckets[name]
	return ok
}
