package toolchain

import "fmt"

const compatibilitySource = "https://developer.android.com/build/releases/about-agp"

type Compatibility struct {
	AGPLine        string `json:"agpLine"`
	MinimumGradle  string `json:"minimumGradle"`
	RecommendedJDK int    `json:"recommendedJdk"`
	Source         string `json:"source"`
}

var compatibilityByLine = map[string]Compatibility{
	"4.2":  {AGPLine: "4.2", MinimumGradle: "6.7.1", RecommendedJDK: 11, Source: "https://developer.android.com/build/releases/agp-4-2-0-release-notes"},
	"7.0":  {AGPLine: "7.0", MinimumGradle: "7.0.2", RecommendedJDK: 11, Source: "https://developer.android.com/build/releases/agp-7-0-0-release-notes"},
	"7.1":  {AGPLine: "7.1", MinimumGradle: "7.2", RecommendedJDK: 11, Source: compatibilitySource},
	"7.2":  {AGPLine: "7.2", MinimumGradle: "7.3.3", RecommendedJDK: 11, Source: compatibilitySource},
	"7.3":  {AGPLine: "7.3", MinimumGradle: "7.4", RecommendedJDK: 11, Source: compatibilitySource},
	"7.4":  {AGPLine: "7.4", MinimumGradle: "7.5", RecommendedJDK: 11, Source: compatibilitySource},
	"8.0":  {AGPLine: "8.0", MinimumGradle: "8.0", RecommendedJDK: 17, Source: compatibilitySource},
	"8.1":  {AGPLine: "8.1", MinimumGradle: "8.0", RecommendedJDK: 17, Source: compatibilitySource},
	"8.2":  {AGPLine: "8.2", MinimumGradle: "8.2", RecommendedJDK: 17, Source: compatibilitySource},
	"8.3":  {AGPLine: "8.3", MinimumGradle: "8.4", RecommendedJDK: 17, Source: compatibilitySource},
	"8.4":  {AGPLine: "8.4", MinimumGradle: "8.6", RecommendedJDK: 17, Source: compatibilitySource},
	"8.5":  {AGPLine: "8.5", MinimumGradle: "8.7", RecommendedJDK: 17, Source: compatibilitySource},
	"8.6":  {AGPLine: "8.6", MinimumGradle: "8.7", RecommendedJDK: 17, Source: compatibilitySource},
	"8.7":  {AGPLine: "8.7", MinimumGradle: "8.9", RecommendedJDK: 17, Source: compatibilitySource},
	"8.8":  {AGPLine: "8.8", MinimumGradle: "8.10.2", RecommendedJDK: 17, Source: compatibilitySource},
	"8.9":  {AGPLine: "8.9", MinimumGradle: "8.11.1", RecommendedJDK: 17, Source: compatibilitySource},
	"8.10": {AGPLine: "8.10", MinimumGradle: "8.11.1", RecommendedJDK: 17, Source: compatibilitySource},
	"8.11": {AGPLine: "8.11", MinimumGradle: "8.13", RecommendedJDK: 17, Source: compatibilitySource},
	"8.12": {AGPLine: "8.12", MinimumGradle: "8.13", RecommendedJDK: 17, Source: compatibilitySource},
	"8.13": {AGPLine: "8.13", MinimumGradle: "8.13", RecommendedJDK: 17, Source: compatibilitySource},
	"9.0":  {AGPLine: "9.0", MinimumGradle: "9.1.0", RecommendedJDK: 17, Source: compatibilitySource},
	"9.1":  {AGPLine: "9.1", MinimumGradle: "9.3.1", RecommendedJDK: 17, Source: compatibilitySource},
	"9.2":  {AGPLine: "9.2", MinimumGradle: "9.4.1", RecommendedJDK: 17, Source: compatibilitySource},
	"9.3":  {AGPLine: "9.3", MinimumGradle: "9.5.0", RecommendedJDK: 17, Source: compatibilitySource},
}

func ForAGP(value string) (Compatibility, error) {
	version, err := ParseVersion(value)
	if err != nil {
		return Compatibility{}, err
	}
	compatibility, ok := compatibilityByLine[version.Line()]
	if !ok {
		return Compatibility{}, fmt.Errorf("AGP %s is outside AARGrade's versioned compatibility policy", value)
	}
	return compatibility, nil
}
