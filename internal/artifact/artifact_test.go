package artifact

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectAAR(t *testing.T) {
	aarPath := filepath.Join(t.TempDir(), "fixture.aar")
	jar := zipBytes(t, map[string][]byte{
		"dev/aargrade/Example.class": minimalClass(t, "dev/aargrade/Example", "VALUE"),
	})
	writeZip(t, aarPath, map[string][]byte{
		"AndroidManifest.xml": []byte("<manifest />"),
		"classes.jar":         jar,
		"META-INF/com/android/build/gradle/aar-metadata.properties": []byte("minCompileSdk=35\nminAndroidGradlePluginVersion=8.8.0\n"),
		"proguard.txt":                []byte("-dontoptimize\n-keep class dev.aargrade.** { *; }\n"),
		"jni/arm64-v8a/libexample.so": []byte("native"),
		"prefab/modules/example/libs/android.x86_64/libexample.so": []byte("prefab-native"),
	})

	snapshot, err := Inspect(aarPath)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.HasManifest || !snapshot.HasClassesJar || len(snapshot.Classes) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if snapshot.Metadata["minCompileSdk"] != "35" || len(snapshot.Native) != 2 {
		t.Fatalf("metadata/native = %#v %#v", snapshot.Metadata, snapshot.Native)
	}
	if len(snapshot.RuleIssues) != 2 || snapshot.RuleIssues[0].ID != "r8.consumer-global-option" {
		t.Fatalf("rule issues = %#v", snapshot.RuleIssues)
	}
}

func TestCompareABIDetectsSealedHierarchyChanges(t *testing.T) {
	baseline := Snapshot{Classes: []Class{{Name: "dev/Example", Access: accPublic}}}
	candidate := Snapshot{Classes: []Class{{Name: "dev/Example", Access: accPublic, PermittedSubclasses: []string{"dev/Child"}}}}
	comparison := CompareABI(baseline, candidate)
	if comparison.Compatible || len(comparison.IncompatibleChanges) == 0 {
		t.Fatalf("comparison = %#v", comparison)
	}

	baseline = candidate
	candidate = Snapshot{Classes: []Class{{Name: "dev/Example", Access: accPublic, PermittedSubclasses: []string{"dev/Other"}}}}
	comparison = CompareABI(baseline, candidate)
	if comparison.Compatible {
		t.Fatalf("removed permitted subclass was accepted: %#v", comparison)
	}

	baseline = Snapshot{Classes: []Class{{Name: "dev/Example", Access: accPublic, PermittedSubclasses: []string{"dev/Child"}}}}
	candidate = Snapshot{Classes: []Class{{Name: "dev/Example", Access: accPublic}}}
	comparison = CompareABI(baseline, candidate)
	if !comparison.Compatible {
		t.Fatalf("changing sealed to non-sealed should be compatible: %#v", comparison)
	}
}

func TestCompareABIDetectsRemovedMember(t *testing.T) {
	baseline := Snapshot{Classes: []Class{{
		Name: "dev/Example", Access: accPublic,
		Members: []Member{{Kind: "field", Name: "VALUE", Descriptor: "I", Access: accPublic | accStatic}},
	}}}
	candidate := Snapshot{Classes: []Class{{Name: "dev/Example", Access: accPublic}}}
	comparison := CompareABI(baseline, candidate)
	if comparison.Compatible || len(comparison.RemovedMembers) != 1 {
		t.Fatalf("comparison = %#v", comparison)
	}
}

func TestCompareABIDetectsFinalAndAbstractChanges(t *testing.T) {
	tests := []struct {
		name      string
		baseline  Class
		candidate Class
		wantOK    bool
	}{
		{
			name:      "class became final",
			baseline:  Class{Name: "dev/Example", Access: accPublic},
			candidate: Class{Name: "dev/Example", Access: accPublic | accFinal},
		},
		{
			name:      "class became abstract",
			baseline:  Class{Name: "dev/Example", Access: accPublic},
			candidate: Class{Name: "dev/Example", Access: accPublic | accAbstract},
		},
		{
			name: "field became final",
			baseline: Class{Name: "dev/Example", Access: accPublic, Members: []Member{
				{Kind: "field", Name: "value", Descriptor: "I", Access: accPublic},
			}},
			candidate: Class{Name: "dev/Example", Access: accPublic, Members: []Member{
				{Kind: "field", Name: "value", Descriptor: "I", Access: accPublic | accFinal},
			}},
		},
		{
			name: "instance method became final",
			baseline: Class{Name: "dev/Example", Access: accPublic, Members: []Member{
				{Kind: "method", Name: "call", Descriptor: "()V", Access: accPublic},
			}},
			candidate: Class{Name: "dev/Example", Access: accPublic, Members: []Member{
				{Kind: "method", Name: "call", Descriptor: "()V", Access: accPublic | accFinal},
			}},
		},
		{
			name: "static method became final is binary compatible",
			baseline: Class{Name: "dev/Example", Access: accPublic, Members: []Member{
				{Kind: "method", Name: "call", Descriptor: "()V", Access: accPublic | accStatic},
			}},
			candidate: Class{Name: "dev/Example", Access: accPublic, Members: []Member{
				{Kind: "method", Name: "call", Descriptor: "()V", Access: accPublic | accStatic | accFinal},
			}},
			wantOK: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			comparison := CompareABI(Snapshot{Classes: []Class{test.baseline}}, Snapshot{Classes: []Class{test.candidate}})
			if comparison.Compatible != test.wantOK {
				t.Fatalf("compatible = %t, want %t: %#v", comparison.Compatible, test.wantOK, comparison)
			}
		})
	}
}

func TestCompareABIWarnsOnDeclaredExceptionChange(t *testing.T) {
	baseline := Snapshot{Classes: []Class{{Name: "dev/Example", Access: accPublic, Members: []Member{
		{Kind: "method", Name: "call", Descriptor: "()V", Access: accPublic, Exceptions: []string{"java/io/IOException"}},
	}}}}
	candidate := Snapshot{Classes: []Class{{Name: "dev/Example", Access: accPublic, Members: []Member{
		{Kind: "method", Name: "call", Descriptor: "()V", Access: accPublic, Exceptions: []string{"java/lang/Exception"}},
	}}}}
	comparison := CompareABI(baseline, candidate)
	if !comparison.Compatible || len(comparison.Warnings) != 1 {
		t.Fatalf("comparison = %#v", comparison)
	}
}

func TestInspectRejectsUnsafeEntry(t *testing.T) {
	aarPath := filepath.Join(t.TempDir(), "unsafe.aar")
	writeZip(t, aarPath, map[string][]byte{"../escape": []byte("bad")})
	if _, err := Inspect(aarPath); err == nil {
		t.Fatal("Inspect accepted an unsafe entry")
	}
}

func TestArchiveBounds(t *testing.T) {
	if err := validateEntryCount("AAR", maxArchiveEntries+1, maxArchiveEntries); err == nil {
		t.Fatal("entry count above the limit was accepted")
	}
	files := []*zip.File{
		{FileHeader: zip.FileHeader{UncompressedSize64: maxArtifactExtractedSize}},
		{FileHeader: zip.FileHeader{UncompressedSize64: 1}},
	}
	if err := validateExtractedSize("AAR", files, maxArtifactExtractedSize); err == nil {
		t.Fatal("extracted size above the limit was accepted")
	}
}

func minimalClass(t *testing.T, className, fieldName string) []byte {
	t.Helper()
	var output bytes.Buffer
	write := func(value any) {
		if err := binary.Write(&output, binary.BigEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	write(uint32(0xcafebabe))
	write(uint16(0))
	write(uint16(52))
	write(uint16(7))
	writeUTF8 := func(value string) {
		write(uint8(1))
		write(uint16(len(value)))
		if _, err := output.WriteString(value); err != nil {
			t.Fatal(err)
		}
	}
	writeUTF8(className) // #1
	write(uint8(7))
	write(uint16(1))              // #2 Class
	writeUTF8("java/lang/Object") // #3
	write(uint8(7))
	write(uint16(3))      // #4 Class
	writeUTF8(fieldName)  // #5
	writeUTF8("I")        // #6
	write(uint16(0x0021)) // public, super
	write(uint16(2))      // this
	write(uint16(4))      // super
	write(uint16(0))      // interfaces
	write(uint16(1))      // fields
	write(uint16(accPublic | accStatic | accFinal))
	write(uint16(5))
	write(uint16(6))
	write(uint16(0)) // field attributes
	write(uint16(0)) // methods
	write(uint16(0)) // class attributes
	return output.Bytes()
}

func zipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func writeZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	if err := os.WriteFile(path, zipBytes(t, files), 0o644); err != nil {
		t.Fatal(err)
	}
}
