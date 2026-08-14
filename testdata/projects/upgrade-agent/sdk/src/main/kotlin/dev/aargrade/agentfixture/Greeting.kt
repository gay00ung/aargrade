package dev.aargrade.agentfixture

object Greeting {
    @JvmStatic
    fun message(): String = "Hello from ${BuildConfig.SDK_NAME}"
}
