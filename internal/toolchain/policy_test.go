package toolchain

import (
	"encoding/hex"
	"testing"
)

func TestForAGP(t *testing.T) {
	compatibility, err := ForAGP("9.3.1")
	if err != nil {
		t.Fatal(err)
	}
	if compatibility.MinimumGradle != "9.5.0" || compatibility.RecommendedJDK != 17 {
		t.Fatalf("compatibility = %#v", compatibility)
	}
}

func TestForAGPRejectsUnknownLine(t *testing.T) {
	if _, err := ForAGP("10.0.0"); err == nil {
		t.Fatal("ForAGP accepted an unsupported line")
	}
}

func TestGradleDistributionSHA256(t *testing.T) {
	checksum, err := GradleDistributionSHA256("9.4.1", "bin")
	if err != nil {
		t.Fatal(err)
	}
	if checksum != "2ab2958f2a1e51120c326cad6f385153bb11ee93b3c216c5fccebfdfbb7ec6cb" {
		t.Fatalf("checksum = %q", checksum)
	}
	if _, err := GradleDistributionSHA256("9.4.1", "source"); err == nil {
		t.Fatal("unknown distribution flavor should fail closed")
	}
}

func TestEveryCompatibilityMinimumHasValidChecksums(t *testing.T) {
	for line, compatibility := range compatibilityByLine {
		for _, flavor := range []string{"bin", "all"} {
			checksum, err := GradleDistributionSHA256(compatibility.MinimumGradle, flavor)
			if err != nil {
				t.Fatalf("AGP %s %s checksum: %v", line, flavor, err)
			}
			decoded, err := hex.DecodeString(checksum)
			if err != nil || len(decoded) != 32 {
				t.Fatalf("AGP %s Gradle %s %s checksum = %q", line, compatibility.MinimumGradle, flavor, checksum)
			}
		}
	}
}

func TestVersionCompare(t *testing.T) {
	left, err := ParseVersion("8.10.2")
	if err != nil {
		t.Fatal(err)
	}
	right, err := ParseVersion("9.1.0-alpha01")
	if err != nil {
		t.Fatal(err)
	}
	if left.Compare(right) >= 0 {
		t.Fatalf("%s should be lower than %s", left.Raw, right.Raw)
	}
}

func TestParseVersionRejectsBuildScriptInjection(t *testing.T) {
	if _, err := ParseVersion("9.2.0'; println('bad')"); err == nil {
		t.Fatal("ParseVersion accepted trailing build script")
	}
}

func TestPrereleaseIsLowerThanStableVersion(t *testing.T) {
	prerelease, err := ParseVersion("9.4.1-rc-1")
	if err != nil {
		t.Fatal(err)
	}
	stable, err := ParseVersion("9.4.1")
	if err != nil {
		t.Fatal(err)
	}
	if prerelease.Compare(stable) >= 0 || stable.Compare(prerelease) <= 0 {
		t.Fatalf("prerelease and stable comparison is incorrect")
	}
}
