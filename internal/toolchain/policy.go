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

// Official Gradle distribution checksums for the minimum versions selected by
// compatibilityByLine. Keeping these values in versioned policy lets a
// migration update an already checksum-pinned Wrapper without weakening it.
var gradleDistributionSHA256 = map[string]map[string]string{
	"6.7.1":  {"bin": "3239b5ed86c3838a37d983ac100573f64c1f3fd8e1eb6c89fa5f9529b5ec091d", "all": "22449f5231796abd892c98b2a07c9ceebe4688d192cd2d6763f8e3bf8acbedeb"},
	"7.0.2":  {"bin": "0e46229820205440b48a5501122002842b82886e76af35f0f3a069243dca4b3c", "all": "13bf8d3cf8eeeb5770d19741a59bde9bd966dd78d17f1bbad787a05ef19d1c2d"},
	"7.2":    {"bin": "f581709a9c35e9cb92e16f585d2c4bc99b2b1a5f85d2badbd3dc6bff59e1e6dd", "all": "a8da5b02437a60819cad23e10fc7e9cf32bcb57029d9cb277e26eeff76ce014b"},
	"7.3.3":  {"bin": "b586e04868a22fd817c8971330fec37e298f3242eb85c374181b12d637f80302", "all": "c9490e938b221daf0094982288e4038deed954a3f12fb54cbf270ddf4e37d879"},
	"7.4":    {"bin": "8cc27038d5dbd815759851ba53e70cf62e481b87494cc97cfd97982ada5ba634", "all": "cd5c2958a107ee7f0722004a12d0f8559b4564c34daad7df06cffd4d12a426d0"},
	"7.5":    {"bin": "cb87f222c5585bd46838ad4db78463a5c5f3d336e5e2b98dc7c0c586527351c2", "all": "97a52d145762adc241bad7fd18289bf7f6801e08ece6badf80402fe2b9f250b1"},
	"8.0":    {"bin": "4159b938ec734a8388ce03f52aa8f3c7ed0d31f5438622545de4f83a89b79788", "all": "f30b29580fe11719087d698da23f3b0f0d04031d8995f7dd8275a31f7674dc01"},
	"8.2":    {"bin": "38f66cd6eef217b4c35855bb11ea4e9fbc53594ccccb5fb82dfd317ef8c2c5a3", "all": "5022b0b25fe182b0e50867e77f484501dba44feeea88f5c1f13b6b4660463640"},
	"8.4":    {"bin": "3e1af3ae886920c3ac87f7a91f816c0c7c436f276a6eefdb3da152100fef72ae", "all": "f2b9ed0faf8472cbe469255ae6c86eddb77076c75191741b4a462f33128dd419"},
	"8.6":    {"bin": "9631d53cf3e74bfa726893aee1f8994fee4e060c401335946dba2156f440f24c", "all": "85719317abd2112f021d4f41f09ec370534ba288432065f4b477b6a3b652910d"},
	"8.7":    {"bin": "544c35d6bd849ae8a5ed0bcea39ba677dc40f49df7d1835561582da2009b961d", "all": "194717442575a6f96e1c1befa2c30e9a4fc90f701d7aee33eb879b79e7ff05c0"},
	"8.9":    {"bin": "d725d707bfabd4dfdc958c624003b3c80accc03f7037b5122c4b1d0ef15cecab", "all": "258e722ec21e955201e31447b0aed14201765a3bfbae296a46cf60b70e66db70"},
	"8.10.2": {"bin": "31c55713e40233a8303827ceb42ca48a47267a0ad4bab9177123121e71524c26", "all": "2ab88d6de2c23e6adae7363ae6e29cbdd2a709e992929b48b6530fd0c7133bd6"},
	"8.11.1": {"bin": "f397b287023acdba1e9f6fc5ea72d22dd63669d59ed4a289a29b1a76eee151c6", "all": "89d4e70e4e84e2d2dfbb63e4daa53e21b25017cc70c37e4eea31ee51fb15098a"},
	"8.13":   {"bin": "20f1b1176237254a6fc204d8434196fa11a4cfb387567519c61556e8710aed78", "all": "fba8464465835e74f7270bbf43d6d8a8d7709ab0a43ce1aa3323f73e9aa0c612"},
	"9.1.0":  {"bin": "a17ddd85a26b6a7f5ddb71ff8b05fc5104c0202c6e64782429790c933686c806", "all": "b84e04fa845fecba48551f425957641074fcc00a88a84d2aae5808743b35fc85"},
	"9.3.1":  {"bin": "b266d5ff6b90eada6dc3b20cb090e3731302e553a27c5d3e4df1f0d76beaff06", "all": "17f277867f6914d61b1aa02efab1ba7bb439ad652ca485cd8ca6842fccec6e43"},
	"9.4.1":  {"bin": "2ab2958f2a1e51120c326cad6f385153bb11ee93b3c216c5fccebfdfbb7ec6cb", "all": "708d2c6ecc97ca9a11838ef64a6c2301151b8dd10387e22dc1a12c30557cab5b"},
	"9.5.0":  {"bin": "553c78f50dafcd54d65b9a444649057857469edf836431389695608536d6b746", "all": "a3c4ba4aca8f0075688b9c5b18939fd28e8cb4357c227da5c1d9f38343791439"},
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

// GradleDistributionSHA256 returns the pinned official checksum for a Gradle
// distribution flavor ("bin" or "all").
func GradleDistributionSHA256(version, flavor string) (string, error) {
	flavors, ok := gradleDistributionSHA256[version]
	if !ok {
		return "", fmt.Errorf("Gradle %s has no pinned distribution checksum", version)
	}
	checksum, ok := flavors[flavor]
	if !ok {
		return "", fmt.Errorf("unsupported Gradle distribution flavor %q", flavor)
	}
	return checksum, nil
}
