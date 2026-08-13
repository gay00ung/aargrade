package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gay00ung/aargrade/internal/artifact"
	"github.com/gay00ung/aargrade/internal/toolchain"
)

var javaClassNamePattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*(?:/[A-Za-z_$][A-Za-z0-9_$]*)*$`)

func generateConsumer(root string, cell CellConfig, aarPath, probeClass string) error {
	if err := os.MkdirAll(filepath.Join(root, "app", "libs"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "app", "src", "main", "java", "dev", "aargrade", "consumer"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "app", "src", "main", "kotlin", "dev", "aargrade", "consumer"), 0o755); err != nil {
		return err
	}
	if err := copyRegularFile(aarPath, filepath.Join(root, "app", "libs", "sdk.aar")); err != nil {
		return err
	}
	files := map[string]string{
		"settings.gradle":                  "rootProject.name = 'aargrade-consumer'\ninclude ':app'\n",
		"build.gradle":                     rootBuild(cell),
		"app/build.gradle":                 appBuild(cell),
		"app/proguard-rules.pro":           "# Consumer application rules intentionally empty.\n",
		"app/src/main/AndroidManifest.xml": manifest(cell),
	}
	if strings.EqualFold(cell.Language, "kotlin") {
		files["app/src/main/kotlin/dev/aargrade/consumer/MainActivity.kt"] = kotlinProbe(probeClass)
	} else {
		files["app/src/main/java/dev/aargrade/consumer/MainActivity.java"] = javaProbe(probeClass)
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func manifest(cell CellConfig) string {
	packageAttribute := ""
	agp, _ := toolchain.ParseVersion(cell.AGP)
	if agp.Major < 7 {
		packageAttribute = ` package="dev.aargrade.consumer"`
	}
	return fmt.Sprintf(`<manifest xmlns:android="http://schemas.android.com/apk/res/android"%s>
    <application android:label="AARGrade Consumer">
        <activity android:name=".MainActivity" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`, packageAttribute)
}

func rootBuild(cell CellConfig) string {
	var builder strings.Builder
	builder.WriteString("buildscript {\n    repositories { google(); mavenCentral(); gradlePluginPortal() }\n    dependencies {\n")
	fmt.Fprintf(&builder, "        classpath 'com.android.tools.build:gradle:%s'\n", cell.AGP)
	agp, _ := toolchain.ParseVersion(cell.AGP)
	if strings.EqualFold(cell.Language, "kotlin") && agp.Major < 9 {
		fmt.Fprintf(&builder, "        classpath 'org.jetbrains.kotlin:kotlin-gradle-plugin:%s'\n", cell.Kotlin)
	}
	builder.WriteString("    }\n}\n\nallprojects { repositories { google(); mavenCentral() } }\n")
	return builder.String()
}

func appBuild(cell CellConfig) string {
	minSDK := cell.MinSDK
	if minSDK == 0 {
		minSDK = 21
	}
	agp, _ := toolchain.ParseVersion(cell.AGP)
	var builder strings.Builder
	builder.WriteString("apply plugin: 'com.android.application'\n")
	if strings.EqualFold(cell.Language, "kotlin") && agp.Major < 9 {
		builder.WriteString("apply plugin: 'kotlin-android'\n")
	}
	builder.WriteString("\nandroid {\n")
	if agp.Major >= 7 {
		builder.WriteString("    namespace 'dev.aargrade.consumer'\n")
		fmt.Fprintf(&builder, "    compileSdk %d\n", cell.CompileSDK)
	} else {
		fmt.Fprintf(&builder, "    compileSdkVersion %d\n", cell.CompileSDK)
	}
	builder.WriteString("\n    defaultConfig {\n        applicationId 'dev.aargrade.consumer'\n")
	fmt.Fprintf(&builder, "        minSdkVersion %d\n        targetSdkVersion %d\n", minSDK, cell.CompileSDK)
	builder.WriteString("        versionCode 1\n        versionName '1.0'\n    }\n\n")
	builder.WriteString("    buildTypes {\n        release {\n            minifyEnabled true\n            shrinkResources true\n            proguardFiles getDefaultProguardFile('proguard-android-optimize.txt'), 'proguard-rules.pro'\n        }\n    }\n}\n\ndependencies {\n    implementation files('libs/sdk.aar')\n")
	for _, dependency := range cell.Dependencies {
		fmt.Fprintf(&builder, "    implementation %q\n", dependency)
	}
	builder.WriteString("}\n")
	return builder.String()
}

func javaProbe(className string) string {
	reference := "android.app.Activity.class"
	if className != "" {
		reference = strings.ReplaceAll(className, "/", ".") + ".class"
	}
	return fmt.Sprintf(`package dev.aargrade.consumer;

public final class MainActivity extends android.app.Activity {
    private final Class<?> sdkClass = %s;
}
`, reference)
}

func kotlinProbe(className string) string {
	reference := "android.app.Activity::class.java"
	if className != "" {
		reference = strings.ReplaceAll(className, "/", ".") + "::class.java"
	}
	return fmt.Sprintf(`package dev.aargrade.consumer

class MainActivity : android.app.Activity() {
    private val sdkClass = %s
}
`, reference)
}

func chooseProbe(primary, fallback artifact.Snapshot) string {
	for _, snapshot := range []artifact.Snapshot{primary, fallback} {
		classes := append([]artifact.Class(nil), snapshot.Classes...)
		sort.Slice(classes, func(i, j int) bool { return classes[i].Name < classes[j].Name })
		for _, class := range classes {
			if !strings.Contains(class.Name, "$") && javaClassNamePattern.MatchString(class.Name) {
				return class.Name
			}
		}
	}
	return ""
}

func copyRegularFile(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("artifact is not a regular, non-symlink file: %s", source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o644)
}
