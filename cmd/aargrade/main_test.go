package main

import "testing"

func TestSelectVersion(t *testing.T) {
	tests := []struct {
		name          string
		linkedVersion string
		moduleVersion string
		want          string
	}{
		{name: "release build wins", linkedVersion: "0.2.0", moduleVersion: "v0.1.0", want: "0.2.0"},
		{name: "go install version", linkedVersion: "dev", moduleVersion: "v0.1.0", want: "v0.1.0"},
		{name: "go install pseudo version", linkedVersion: "dev", moduleVersion: "v0.0.0-20260813065516-dc040df53fd0", want: "v0.0.0-20260813065516-dc040df53fd0"},
		{name: "local build", linkedVersion: "dev", moduleVersion: "(devel)", want: "dev"},
		{name: "empty values", want: "dev"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selectVersion(test.linkedVersion, test.moduleVersion); got != test.want {
				t.Fatalf("selectVersion(%q, %q) = %q, want %q", test.linkedVersion, test.moduleVersion, got, test.want)
			}
		})
	}
}
