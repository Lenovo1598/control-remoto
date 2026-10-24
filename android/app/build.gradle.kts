plugins {
    id("com.android.application")
}

android {
    namespace = "com.josii.controlremoto"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.josii.controlremoto"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0"
    }

    // Firma propia para uso personal: permite instalar actualizaciones encima
    // sin desinstalar. Mantené el repositorio privado.
    signingConfigs {
        create("release") {
            storeFile = file("controlremoto.jks")
            storePassword = "controlremoto"
            keyAlias = "controlremoto"
            keyPassword = "controlremoto"
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.getByName("release")
        }
        debug {
            signingConfig = signingConfigs.getByName("release")
        }
    }

    // La interfaz es la misma que sirve el programa de Windows.
    sourceSets {
        getByName("main") {
            assets.srcDirs("../../pc/web")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}
