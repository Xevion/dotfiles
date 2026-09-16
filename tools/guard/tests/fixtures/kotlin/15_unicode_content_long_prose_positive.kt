fun pad1() {}
fun pad2() {}
fun pad3() {}
fun pad4() {}
fun pad5() {}
fun pad6() {}
@expect: NUDGE long-prose
// Ceci explique le comportement en détail: café, naïve, façade — 🎉
// über wörter mit mehreren bytes pro zeichen können hier auftauchen
// 日本語のコメントも同じルールで数えられるはずです、行数だけで
// と、この最後の行がしきい値をちょうど超えることを保証します
fun f() {}
