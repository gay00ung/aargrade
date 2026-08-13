dependencyResolutionManagement {
    versionCatalogs {
        create("tools") {
            from(files("gradle/tools.versions.toml"))
        }
    }
}

rootProject.name = "convention-unknown-fixture"
include(":sdk")
