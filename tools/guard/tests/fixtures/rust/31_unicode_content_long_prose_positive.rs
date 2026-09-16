fn pad1() {}
fn pad2() {}
fn pad3() {}
fn pad4() {}
fn pad5() {}
fn pad6() {}
@expect: NUDGE long-prose
// Ceci explique le comportement en détail: café, naïve, façade — 🎉
// über wörter mit mehreren bytes pro zeichen können hier auftauchen
// 日本語のコメントも同じルールで数えられるはずです、行数だけで
// と、この最後の行がしきい値をちょうど超えることを保証します
fn f() {}
