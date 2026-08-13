package dev.aargrade.example;

/** A tiny public API used by the buildable AARGrade example. */
public final class Greeting {
    private Greeting() {}

    public static String message() {
        return "Hello from the AARGrade example SDK";
    }
}
