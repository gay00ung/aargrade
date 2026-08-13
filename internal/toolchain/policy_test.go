package toolchain

import "testing"

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
