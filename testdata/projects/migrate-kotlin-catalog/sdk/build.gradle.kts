plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.android)
}

android {
    namespace = "dev.aargrade.migratefixture"
    compileSdk = 35

    defaultConfig {
        minSdk = 21
    }
}
