plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.android)
}

android {
    compileSdkVersion(35)

    defaultConfig {
        minSdkVersion 21
        buildConfigField("String", "SDK_NAME", "\"AARGrade\"")
    }

    kotlinOptions {
        jvmTarget = "17"
    }
}
