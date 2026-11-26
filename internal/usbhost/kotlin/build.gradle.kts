// The Kotlin of the native module usbhost: an Android library, which mygo
// build adds to the app's Gradle build. The build sets its SDK levels, Java
// and Kotlin, and its dependency on MyGo's host library (dev.mygo), which
// brings coroutines: add only what the implementation uses.
plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "easylpac.internal.usbhost"
}

dependencies {
}
