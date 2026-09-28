# kotlinx.serialization: generierte Serializer behalten
-keepclassmembers @kotlinx.serialization.Serializable class io.flimmer.app.** {
    *** Companion;
    kotlinx.serialization.KSerializer serializer(...);
}
-keep class io.flimmer.app.**$$serializer { *; }
