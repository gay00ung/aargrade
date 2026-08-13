package host

import (
	"fmt"
	"path/filepath"
	"strings"
)

func settingsBlock(kotlin bool, hostModule string) string {
	if kotlin {
		return fmt.Sprintf("%s\ninclude(%q)\nproject(%q).projectDir = file(%q)\n%s\n", markerStart, hostModule, hostModule, hostRelativeDir, markerEnd)
	}
	return fmt.Sprintf("%s\ninclude '%s'\nproject('%s').projectDir = file('%s')\n%s\n", markerStart, hostModule, hostModule, hostRelativeDir, markerEnd)
}

func settingsInsertion(current, block string) string {
	if strings.HasSuffix(current, "\n") {
		return "\n" + block
	}
	return "\n\n" + block
}

func hostBuildFile(kotlin bool, version agpVersion, libraryModule string, compileSDK, minSDK int) string {
	if kotlin {
		return kotlinHostBuild(version, libraryModule, compileSDK, minSDK)
	}
	return groovyHostBuild(version, libraryModule, compileSDK, minSDK)
}

func kotlinHostBuild(version agpVersion, libraryModule string, compileSDK, minSDK int) string {
	var builder strings.Builder
	builder.WriteString("// Generated and owned by AARGrade. Remove with `aargrade host remove --apply`.\n")
	if version.usesModernDSL() {
		builder.WriteString("plugins {\n    id(\"com.android.application\")\n}\n\n")
	} else {
		builder.WriteString("apply(plugin = \"com.android.application\")\n\n")
	}
	builder.WriteString("android {\n")
	if version.supportsNamespace() {
		builder.WriteString("    namespace = \"dev.aargrade.upgradehost\"\n")
	}
	if version.usesModernDSL() {
		fmt.Fprintf(&builder, "    compileSdk = %d\n\n    defaultConfig {\n        minSdk = %d\n    }\n", compileSDK, minSDK)
	} else {
		fmt.Fprintf(&builder, "    compileSdkVersion(%d)\n\n    defaultConfig {\n        minSdkVersion(%d)\n    }\n", compileSDK, minSDK)
	}
	builder.WriteString("}\n\ndependencies {\n")
	fmt.Fprintf(&builder, "    implementation(project(%q))\n", libraryModule)
	builder.WriteString("}\n")
	return builder.String()
}

func groovyHostBuild(version agpVersion, libraryModule string, compileSDK, minSDK int) string {
	var builder strings.Builder
	builder.WriteString("// Generated and owned by AARGrade. Remove with `aargrade host remove --apply`.\n")
	if version.usesModernDSL() {
		builder.WriteString("plugins {\n    id 'com.android.application'\n}\n\n")
	} else {
		builder.WriteString("apply plugin: 'com.android.application'\n\n")
	}
	builder.WriteString("android {\n")
	if version.supportsNamespace() {
		builder.WriteString("    namespace 'dev.aargrade.upgradehost'\n")
	}
	if version.usesModernDSL() {
		fmt.Fprintf(&builder, "    compileSdk %d\n\n    defaultConfig {\n        minSdk %d\n    }\n", compileSDK, minSDK)
	} else {
		fmt.Fprintf(&builder, "    compileSdkVersion %d\n\n    defaultConfig {\n        minSdkVersion %d\n    }\n", compileSDK, minSDK)
	}
	builder.WriteString("}\n\ndependencies {\n")
	fmt.Fprintf(&builder, "    implementation project('%s')\n", libraryModule)
	builder.WriteString("}\n")
	return builder.String()
}

func hostManifest(version agpVersion) string {
	if version.supportsNamespace() {
		return "<!-- Generated and owned by AARGrade. -->\n<manifest />\n"
	}
	return "<!-- Generated and owned by AARGrade. -->\n<manifest package=\"dev.aargrade.upgradehost\" />\n"
}

func hostBuildRelativePath(kotlin bool) string {
	if kotlin {
		return filepath.ToSlash(filepath.Join(hostRelativeDir, "build.gradle.kts"))
	}
	return filepath.ToSlash(filepath.Join(hostRelativeDir, "build.gradle"))
}

func hostManifestRelativePath() string {
	return filepath.ToSlash(filepath.Join(hostRelativeDir, "src", "main", "AndroidManifest.xml"))
}
