plugins {
    id("com.android.library")
}

android {
    namespace = "dev.aargrade.example"
    compileSdk = 35

    defaultConfig {
        minSdk = 21
        aarMetadata {
            // This example uses no platform API or resource newer than API 30.
            minCompileSdk = 30
        }
    }
}
