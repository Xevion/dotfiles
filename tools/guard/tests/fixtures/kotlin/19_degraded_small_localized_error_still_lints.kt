@doc: bug 5 counterpart to the Rust degraded-parse fixture - confirmed via
@doc: examples/kotlin_probe.rs that an equivalently-sized single unclosed
@doc: paren, padded on both sides, keeps Kotlin at Degraded rather than
@doc: Untrusted, same as Rust. This pins that comparison down as a fixture
@doc: instead of a one-off probe run.
@fixture: degraded
@expect: BLOCK bare-label
// Handlers
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun pad(): Int { return 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fun broken(
