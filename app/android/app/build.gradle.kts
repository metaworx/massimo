// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

import groovy.json.JsonSlurper
import javax.inject.Inject

plugins {
    alias(libs.plugins.android.application)
}

// The version is the server's: app/version.json, which make release writes
// with the tag's version and its versionCode (packaging/app-version.sh);
// a checkout without the file is 0.0.0-dev. A nightly build gives its own,
// git describe's, as -Pmassimo.version and -Pmassimo.versionCode
// (.github/workflows/build.yml).
val versionFile = rootProject.file("../version.json")
val fileVersion: Pair<String, Int> =
    if (versionFile.isFile) {
        val v = JsonSlurper().parse(versionFile) as Map<*, *>
        (v["version"] as String) to (v["versionCode"] as Number).toInt()
    } else {
        "0.0.0-dev" to 1
    }
val appVersion: Pair<String, Int> =
    (providers.gradleProperty("massimo.version").orNull ?: fileVersion.first) to
        (providers.gradleProperty("massimo.versionCode").orNull?.toInt() ?: fileVersion.second)

android {
    namespace = "ch.metaworx.massimo"
    compileSdk = 37
    buildToolsVersion = "36.0.0"

    defaultConfig {
        applicationId = "ch.metaworx.massimo"
        minSdk = 26
        targetSdk = 36
        versionName = appVersion.first
        versionCode = appVersion.second
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        // Each instrumented test starts as the app installed anew: the
        // orchestrator (testOptions) clears the app's data, its Keystore
        // keys among them, before every test, so that a test of the
        // emulator suite (tools/app-emulator/README.md) finds nothing of
        // the one before.
        testInstrumentationRunnerArguments["clearPackageData"] = "true"
    }

    // Each instrumented test in an instrumentation of its own, so that a
    // crash or a hung prompt ends that test alone.
    testOptions {
        execution = "ANDROIDX_TEST_ORCHESTRATOR"
    }

    // foss is the F-Droid build, play the Google Play build; they differ
    // from Block 8 on, where play carries the FCM distributor. nightly is
    // foss as the nightly build publishes it on GitHub alone: an app of
    // its own beside the release, "Massimo nightly", with its own icon
    // (src/nightly/res) and key (AP MobileApp, D18-A).
    flavorDimensions += "store"
    productFlavors {
        create("foss") { dimension = "store" }
        create("play") { dimension = "store" }
        create("nightly") {
            dimension = "store"
            applicationIdSuffix = ".nightly"
        }
    }

    buildTypes {
        // A developer's build never passes for the release it follows.
        debug {
            versionNameSuffix = "-dev"
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            // No checkout path or commit in the APK: F-Droid rebuilds it
            // from the tag and compares.
            vcsInfo.include = false
        }
    }

    buildFeatures {
        buildConfig = true
    }

    // The icon's layers, written from Massimo's mark by make ui-brand
    // (tools/brand/README.md), beside the resources written by hand.
    sourceSets {
        getByName("main") {
            res.directories.add("src/main/res-brand")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    // Reproducible builds: no dependency metadata signed by Google's key.
    dependenciesInfo {
        includeInApk = false
        includeInBundle = false
    }

    lint {
        abortOnError = true
        warningsAsErrors = true
        // Versions move by a decision, not by the day lint runs.
        disable += setOf("GradleDependency", "NewerVersionAvailable", "AndroidGradlePluginVersion", "OldTargetApi")
    }
}

// BundleUi copies the web UI's files into the app's assets under ui/,
// so the connect page is served as the server serves it (docs/app.md).
// The server's own page (index.html with its partials), its parts (the
// TypeScript under src/ and parts.js, which make ui bundles from it) and
// the OPAQUE client are the server's to serve, not the app's.
abstract class BundleUi : DefaultTask() {
    @get:InputDirectory
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val uiDir: DirectoryProperty

    @get:OutputDirectory
    abstract val outputDir: DirectoryProperty

    @get:Inject
    abstract val fs: FileSystemOperations

    @TaskAction
    fun copy() {
        fs.sync {
            from(uiDir) {
                exclude("index.html", "partials/**", "src/**", "parts.js", "vendor/**")
            }
            into(outputDir.dir("ui"))
        }
    }
}

// -Pmassimo.ui=<dir> takes the UI from another checkout, for a page not
// merged yet.
val uiSource = file(providers.gradleProperty("massimo.ui").getOrElse("../../../internal/server/ui"))

androidComponents {
    onVariants { variant ->
        val task = tasks.register<BundleUi>("bundleUi${variant.name.replaceFirstChar { it.uppercase() }}") {
            uiDir.set(uiSource)
        }
        variant.sources.assets?.addGeneratedSourceDirectory(task, BundleUi::outputDir)
    }
}

dependencies {
    implementation(libs.androidx.activity)
    implementation(libs.androidx.appcompat)
    implementation(libs.androidx.biometric)
    implementation(libs.androidx.core)
    implementation(libs.androidx.fragment)
    implementation(libs.androidx.swiperefreshlayout)
    implementation(libs.androidx.webkit)

    testImplementation(libs.junit)
    testImplementation(libs.json)

    androidTestImplementation(libs.androidx.test.core)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.uiautomator)
    androidTestUtil(libs.androidx.test.orchestrator)
    androidTestUtil(libs.androidx.test.services)
}
