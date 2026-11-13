package detect

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"screenpick/internal/fakefs"
	"screenpick/internal/resolve"
)

// hostScan renders what Scan finds that is not a path, so these cases stay
// independent of path detection.
func hostScan(in string) []string {
	var got []string
	for _, m := range Scan(in, resolve.New(testEnv())) {
		if m.Kind != KindPath {
			got = append(got, render(m))
		}
	}
	return got
}

func runHostCases(t *testing.T, cases []scanCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, hostScan(tc.in)); diff != "" {
				t.Errorf("Scan(%q) (-want +got):\n%s", tc.in, diff)
			}
		})
	}
}

func TestBareDomains(t *testing.T) {
	runHostCases(t, []scanCase{
		{"subdomain of a real tld", "see borders.xevion.dev now", []string{"host:borders.xevion.dev"}},
		{"apex domain", "visit example.com today", []string{"host:example.com"}},
		{"www prefix", "www.example.com", []string{"host:www.example.com"}},
		{"www prefix on an unlisted tld", "www.example.zzzz", nil},
		{"www prefix on any real tld", "www.foo.bar", []string{"host:www.foo.bar"}},
		{"two-part public suffix", "bbc.co.uk", []string{"host:bbc.co.uk"}},
		{"upper case", "EXAMPLE.COM and Example.com.", []string{"host:EXAMPLE.COM", "host:Example.com"}},
		{"sentence period", "go to example.com.", []string{"host:example.com"}},
		{"period then next sentence", "example.com. Next", []string{"host:example.com"}},
		{"comma and semicolon", "example.com, foo.org; bar.net", []string{"host:example.com", "host:foo.org", "host:bar.net"}},
		{"parens", "(example.com)", []string{"host:example.com"}},
		{"brackets", "[example.com]", []string{"host:example.com"}},
		{"angle brackets", "<example.com>", []string{"host:example.com"}},
		{"double quotes", `"example.com"`, []string{"host:example.com"}},
		{"single quotes", "'example.com'", []string{"host:example.com"}},
		{"backticks", "`example.com`", []string{"host:example.com"}},
		{"smart quotes", "“example.com”", []string{"host:example.com"}},
		{"key value", "host=example.com", []string{"host:example.com"}},
		{"flag value", "--host=example.com", []string{"host:example.com"}},
		{"header", "Host: example.com", []string{"host:example.com"}},
		{"colon without port", "example.com: down", []string{"host:example.com"}},
		{"box border", "│ example.com │", []string{"host:example.com"}},
		{"markdown link text is the link's business", "[example.com](https://example.com)", []string{"url:https://example.com"}},
		{"idn latin", "bücher.de", []string{"host:bücher.de"}},
		{"idn cjk", "例え.jp", []string{"host:例え.jp"}},
		{"idn tld", "пример.рф", []string{"host:пример.рф"}},
		{"punycode label", "xn--bcher-kva.de", []string{"host:xn--bcher-kva.de"}},
		{"punycode tld", "example.xn--p1ai", []string{"host:example.xn--p1ai"}},
		{"hyphenated label", "my-site.example.org", []string{"host:my-site.example.org"}},
		{"digits in labels", "s3.us-east-1.amazonaws.com", []string{"host:s3.us-east-1.amazonaws.com"}},
		{"after cjk text", "访问example.com获取", []string{"host:example.com"}},
		{"between kana", "これはexample.comです", []string{"host:example.com"}},
		{"cjk label is not trimmed", "访问 例え.jp 获取", []string{"host:例え.jp"}},
		{"after emoji", "🎉example.com✓", []string{"host:example.com"}},
		{"fullwidth punctuation", "（example.com）。", []string{"host:example.com"}},
		{"two hosts in prose", "a.example.com and b.example.com", []string{"host:a.example.com", "host:b.example.com"}},
		{"subdomain starting with a tld word", "dev.example.com app.example.com", []string{"host:dev.example.com", "host:app.example.com"}},
	})
}

func TestLocalNames(t *testing.T) {
	runHostCases(t, []scanCase{
		{"localhost", "localhost", []string{"host:localhost"}},
		{"localhost in prose", "listening on localhost.", []string{"host:localhost"}},
		{"localhost with port", "localhost:8080", []string{"host:localhost:8080"}},
		{"localhost url", "http://localhost:3000", []string{"url:http://localhost:3000"}},
		{"mdns", "myhost.local", []string{"host:myhost.local"}},
		{"mdns with port", "printer.local:631", []string{"host:printer.local:631"}},
		{"mdns url", "http://myhost.local/", []string{"url:http://myhost.local/"}},
		{"lan", "nas.lan", []string{"host:nas.lan"}},
		{"home arpa", "router.home.arpa", []string{"host:router.home.arpa"}},
		{"internal", "foo.internal", []string{"host:foo.internal"}},
		{"localhost suffix", "app.localhost", []string{"host:app.localhost"}},
		{"nested localhost suffix", "a.b.localhost:3000", []string{"host:a.b.localhost:3000"}},
		{"reserved test", "foo.test", []string{"host:foo.test"}},
		{"reserved example", "foo.example", []string{"host:foo.example"}},
		{"reserved invalid", "foo.invalid", []string{"host:foo.invalid"}},
		{"single label with scheme", "http://devbox/", []string{"url:http://devbox/"}},
		{"single label with port", "devbox:3000", []string{"host:devbox:3000"}},
		{"single label with low service port", "ssh devbox:22", []string{"host:devbox:22"}},
		{"single label alone is a word", "the devbox is down", nil},
		{"single label with slash is a path", "devbox/foo", nil},
		{"cluster names", "svc.default.svc.cluster.local:80", []string{"host:svc.default.svc.cluster.local:80"}},
		{"leading dot is a dotfile", ".env.local and .env.example", nil},
		{"filename stem", "config.example and settings.local", nil},
		{"struct field named like a local name", "r.local s.test a.lan", nil},
		{"short real domains", "x.com t.co", []string{"host:x.com", "host:t.co"}},
		{"test files", "foo.test.ts foo.test.js", nil},
		{"local host path", "localhost/foo", []string{"url:localhost/foo"}},
		{"hostname then port elsewhere", "devbox then :3000", nil},
	})
}

func TestNotHosts(t *testing.T) {
	runHostCases(t, []scanCase{
		{"file names", "index.html main.rs Cargo.toml foo.js config.json README.md", nil},
		{"versions", "v1.2.3 1.0.0 3.14 0.1.0-rc.1", nil},
		{"go and python idents", "os.File std.fs exec.Cmd self.user.name", nil},
		{"abbreviations", "e.g. i.e. a.k.a. etc.", nil},
		{"attribute named like a tld", "self.name user.id x.is this.in obj.at obj.group x.host log.info s.store", nil},
		{"getter chain", "self.config.get", nil},
		{"calls", "example.com(x) list.io(3) conn.co[0]", nil},
		{"python import", "from foo.app import bar", nil},
		{"import statement", "import foo.dev", nil},
		{"java package", "com.example.app and org.apache.dev", nil},
		{"config file stems", "docker-compose.dev Dockerfile.dev webpack.config.dev config.dev", nil},
		{"extension that is also a tld", "install.sh build.py mod.rs notes.md a.zip clip.mov lib.so script.pl x.cc", nil},
		{"shared library", "libfoo.so.1", nil},
		{"build files", "Makefile.am configure.ac main.kt main.tf", nil},
		{"line suffix on a tld-like extension", "main.rs:12 foo.py:42:5 src.sv:2000", nil},
		{"line suffix on a known extension", "app.ts:3000 notes.md:1500", nil},
		{"key colon number", "error:404 port:8080 id:12345 line:42 uid:1000", nil},
		{"key colon number then more", "foo:12:30:45", nil},
		{"clock", "12:30:45", nil},
		{"drive", `C:\Users\x\file.dev`, nil},
		{"rust and c++ paths", "std::io::Result a::b Foo::bar::baz", nil},
		{"key value", "key:value foo: bar", nil},
		{"email", "user@example.com a@b.co", nil},
		{"email in angle brackets", "Name <name@example.com>", nil},
		{"email in mailto text", "write to a@b.com.", nil},
		{"path component", "/home/u/x.example.com", nil},
		{"relative path component", "src/x.example.com ./y.dev ../z.io", nil},
		{"home path", "~/proj/foo.dev", nil},
		{"windows path component", `C:\a\b.dev`, nil},
		{"hyphen glued", "--foo.bar-baz", nil},
		{"underscore glued", "my_host.example.com", nil},
		{"digit-only tld", "1.2.3.4.5", nil},
		{"mixed case tld", "Foo.Bar Foo.Com", nil},
		{"unlisted tld", "foo.zzzz", nil},
		{"one letter tld", "foo.x:80", nil},
		{"ellipsis", "wait...example", nil},
		{"wildcard", "*.js", nil},
		{"glob", "src/**/*.dev", nil},
		{"word char after", "example.comx", nil},
		{"trailing underscore", "example.com_x", nil},
		{"uuid and hash", "550e8400-e29b-41d4-a716-446655440000", nil},
		{"statusline", "cost $0.64 at 152.3 tok/s", nil},
	})
}

func TestSchemes(t *testing.T) {
	runHostCases(t, []scanCase{
		{"unknown scheme", "foo://bar", []string{"url:foo://bar"}},
		{"scheme with dotted authority and path", "foo://bar.baz/q", []string{"url:foo://bar.baz/q"}},
		{"app scheme", "myapp://open/x", []string{"url:myapp://open/x"}},
		{"vscode", "vscode://file/home/u/x.rs:12:3", []string{"url:vscode://file/home/u/x.rs:12:3"}},
		{"postgres", "postgres://u:p@h:5432/db", []string{"url:postgres://u:p@h:5432/db"}},
		{"redis", "redis://localhost:6379/0", []string{"url:redis://localhost:6379/0"}},
		{"s3", "s3://bucket/key", []string{"url:s3://bucket/key"}},
		{"websocket", "ws://h/x wss://h/y", []string{"url:ws://h/x", "url:wss://h/y"}},
		{"chrome", "chrome://settings", []string{"url:chrome://settings"}},
		{"steam", "steam://run/123", []string{"url:steam://run/123"}},
		{"extended scheme", "x-scheme+sub.ext://a/b", []string{"url:x-scheme+sub.ext://a/b"}},
		{"compound scheme", "git+ssh://git@github.com/x/y.git", []string{"url:git+ssh://git@github.com/x/y.git"}},
		{"upper case scheme", "HTTPS://EXAMPLE.COM/A", []string{"url:HTTPS://EXAMPLE.COM/A"}},
		{"mailto", "mailto:a@b.com", []string{"url:mailto:a@b.com"}},
		{"mailto with query", "mailto:a@b.com?subject=hi.", []string{"url:mailto:a@b.com?subject=hi"}},
		{"tel", "call tel:+15551234567 now", []string{"url:tel:+15551234567"}},
		{"magnet", "magnet:?xt=urn:btih:abc123&dn=x", []string{"url:magnet:?xt=urn:btih:abc123&dn=x"}},
		{"data uri is not a link", "data:text/plain;base64,AAAA", nil},
		{"javascript is not a link", "javascript:alert(1)", nil},
		{"scheme glued to a colon", "see:https://example.com", []string{"url:https://example.com"}},
		{"scheme after ellipsis", "wait...https://example.com", []string{"url:https://example.com"}},
		{"scheme after equals", "url=https://example.com/a&next=https://b.com", []string{"url:https://example.com/a&next=https://b.com"}},
		{"scheme beginning with a digit", "1abc://x", nil},
		{"scheme alone", "foo:// and redis://", nil},
		{"embedded url stays inside", "https://web.archive.org/web/2020/https://example.com/", []string{"url:https://web.archive.org/web/2020/https://example.com/"}},
		{"drive letter", `C:\ and D:/x`, nil},
		{"drive letter with doubled slash", "C://Users/x and a://b", nil},
		{"regex fragments", `https?://  \w+://  {scheme}://host  ${proto}://h`, nil},
		{"double colon", "a::b std::io", nil},
		{"location", "file.rs:12", nil},
		{"key value", "key:value foo: bar", nil},
		{"timestamp", "12:30:45", nil},
		{"unknown word colon", "mailto: nobody", nil},
	})
}

func TestURLBounds(t *testing.T) {
	runHostCases(t, []scanCase{
		{"parens", "(https://example.com/a)", []string{"url:https://example.com/a"}},
		{"parens then period", "(see https://example.com/a).", []string{"url:https://example.com/a"}},
		{"square brackets", "[https://example.com/a]", []string{"url:https://example.com/a"}},
		{"braces", "{https://example.com/a}", []string{"url:https://example.com/a"}},
		{"angle", "<https://example.com/a>", []string{"url:https://example.com/a"}},
		{"quotes", `"https://example.com/a", 'https://example.com/b'`, []string{"url:https://example.com/a", "url:https://example.com/b"}},
		{"backticks", "`https://example.com/a`", []string{"url:https://example.com/a"}},
		{"markdown link", "[docs](https://docs.rs/serde)", []string{"url:https://docs.rs/serde"}},
		{"markdown link with url text", "[https://x.com](https://x.com)", []string{"url:https://x.com", "url:https://x.com"}},
		{"adjacent markdown links", "[a](https://x.com/a)[b](https://y.com/b)", []string{"url:https://x.com/a", "url:https://y.com/b"}},
		{"markdown link with parens in url", "[w](https://en.wikipedia.org/wiki/Foo_(bar))", []string{"url:https://en.wikipedia.org/wiki/Foo_(bar)"}},
		{"wikipedia parens in parens", "(https://en.wikipedia.org/wiki/Foo_(bar))", []string{"url:https://en.wikipedia.org/wiki/Foo_(bar)"}},
		{"closing bracket glued", "https://x.com/a)b", []string{"url:https://x.com/a"}},
		{"markdown bold", "**https://example.com/a**", []string{"url:https://example.com/a"}},
		{"trailing slash", "https://example.com/a/", []string{"url:https://example.com/a/"}},
		{"trailing question mark is prose", "seen https://example.com/a?", []string{"url:https://example.com/a"}},
		{"trailing fragment", "https://example.com/a#frag", []string{"url:https://example.com/a#frag"}},
		{"trailing ampersand", "https://example.com/a?x=1&", []string{"url:https://example.com/a?x=1&"}},
		{"trailing equals", "https://example.com/a?x=", []string{"url:https://example.com/a?x="}},
		{"trailing colon", "see https://example.com: and", []string{"url:https://example.com"}},
		{"two urls with comma", "https://x.com/a, https://y.com/b", []string{"url:https://x.com/a", "url:https://y.com/b"}},
		{"ipv6 literal", "https://[::1]:8080/path", []string{"url:https://[::1]:8080/path"}},
		{"userinfo", "http://user:pw@host:80/x", []string{"url:http://user:pw@host:80/x"}},
		{"idn url", "https://bücher.example/straße", []string{"url:https://bücher.example/straße"}},
		{"punycode url", "https://xn--bcher-kva.example/", []string{"url:https://xn--bcher-kva.example/"}},
		{"cjk path", "https://example.com/日本語", []string{"url:https://example.com/日本語"}},
		{"ellipsis glyph", "https://example.com/a…", []string{"url:https://example.com/a"}},
		{"percent encoding", "https://example.com/a%20b?q=%E2%9C%93", []string{"url:https://example.com/a%20b?q=%E2%9C%93"}},
	})
}

func TestSchemelessURLs(t *testing.T) {
	runHostCases(t, []scanCase{
		{"domain with path and query", "example.com/path/x?y=1", []string{"url:example.com/path/x?y=1"}},
		{"docs.rs", "docs.rs/serde", []string{"url:docs.rs/serde"}},
		{"bun.sh", "curl bun.sh/install | bash", []string{"url:bun.sh/install"}},
		{"trailing period", "see example.com/a.", []string{"url:example.com/a"}},
		{"trailing paren", "(example.com/a)", []string{"url:example.com/a"}},
		{"domain with slash only", "example.com/", []string{"url:example.com/"}},
		{"extension tld with bare slash stays quiet", "main.rs/", nil},
		{"domain with port and path", "example.com:8080/x", []string{"url:example.com:8080/x"}},
		{"localhost port path", "localhost:8080/api/v1", []string{"url:localhost:8080/api/v1"}},
		{"ipv4 port path", "10.0.0.1:8080/api", []string{"url:10.0.0.1:8080/api"}},
		{"ipv4 path", "10.0.0.1/admin", []string{"url:10.0.0.1/admin"}},
		{"ipv4 cidr stays a host", "10.0.0.0/8", []string{"host:10.0.0.0/8"}},
		{"ipv6 port path", "[::1]:8080/x", []string{"url:[::1]:8080/x"}},
		{"image reference", "docker.io/library/nginx:latest", []string{"url:docker.io/library/nginx:latest"}},
		{"registry reference", "pull ghcr.io/owner/img:1.2", []string{"url:ghcr.io/owner/img:1.2"}},
		{"unlisted tld with path", "foo.zzzz/x", nil},
		{"file list joined by a slash", "CLAUDE.md/AGENTS.md and README.md/LICENSE.txt", nil},
		{"extension tld with a plain path", "lib.rs/crates/serde", []string{"url:lib.rs/crates/serde"}},
		{"ordinary relative path", "src/lib.rs", nil},
		{"tld glued before the slash", "a.b.c/d", nil},
	})
}

func TestHostPorts(t *testing.T) {
	runHostCases(t, []scanCase{
		{"domain", "example.com:443", []string{"host:example.com:443"}},
		{"domain in prose", "connect to db.internal:5432, then", []string{"host:db.internal:5432"}},
		{"internal tld with port", "api.corp:8443", []string{"host:api.corp:8443"}},
		{"port zero", "example.com:0", nil},
		{"port out of range", "example.com:70000", nil},
		{"six digit port", "example.com:123456", nil},
		{"port then line", "example.com:80:12", nil},
		{"port glued to letters", "example.com:80x", nil},
		{"after at sign", "user@example.com:22", nil},
		{"single label high port", "devbox:8080", []string{"host:devbox:8080"}},
		{"single label odd low port", "devbox:12", nil},
		{"single label capitalised", "Devbox:8080", []string{"host:Devbox:8080"}},
		{"single label one letter", "x:8080", nil},
		{"single label digits", "123:8080", nil},
		{"two hosts", "a.example.com:80 b.example.com:81", []string{"host:a.example.com:80", "host:b.example.com:81"}},
	})
}

func TestScpAndUser(t *testing.T) {
	runHostCases(t, []scanCase{
		{"scp github", "git@github.com:org/repo.git", []string{"url:git@github.com:org/repo.git"}},
		{"scp with absolute path", "deploy@myhost.local:/srv/app", []string{"url:deploy@myhost.local:/srv/app"}},
		{"scp single label host", "git@host:org/repo.git", []string{"url:git@host:org/repo.git"}},
		{"slice is not a bracketed address", "arr[::1] x[::2]", nil},
		{"scp tilde path", "user@devbox.local:~/x", []string{"url:user@devbox.local:~/x"}},
		{"single label host needs a repo suffix", "xevion@desktop:~/proj and user@devbox:/srv/x", nil},
		{"scp then period", "clone git@github.com:org/repo.git.", []string{"url:git@github.com:org/repo.git"}},
		{"shell prompt with absolute path", "user@host.local:/srv/x$ ls", nil},
		{"shell prompt with tilde", "xevion@desktop:~/proj$", nil},
		{"remote without a user", "To github.com:Xevion/dotfiles.git", []string{"url:github.com:Xevion/dotfiles.git"}},
		{"host and colon without a repo", "To github.com: now", []string{"host:github.com"}},
		{"not scp: version", "foo@1.2.3:bar", nil},
		{"not scp: no path", "git@github.com:", nil},
		{"npm scope", "@types/node@1.0.0", nil},
	})
}

func TestIPs(t *testing.T) {
	runHostCases(t, []scanCase{
		{"v4 with port", "192.168.1.10:8080", []string{"host:192.168.1.10:8080"}},
		{"v4 in parens", "(10.0.0.1)", []string{"host:10.0.0.1"}},
		{"section number", "RFC 5280, Section 4.2.1.6 and version 1.2.3.4", nil},
		{"v4 after a verb", "ping 1.2.3.4", []string{"host:1.2.3.4"}},
		{"v4 sentence end", "ping 10.0.0.1.", []string{"host:10.0.0.1"}},
		{"v4 cidr", "10.0.0.0/8", []string{"host:10.0.0.0/8"}},
		{"v4 with port then comma", "10.0.0.1:22, 10.0.0.2:22", []string{"host:10.0.0.1:22", "host:10.0.0.2:22"}},
		{"v6 loopback", "::1", []string{"host:::1"}},
		{"v6 compressed", "2001:db8::1", []string{"host:2001:db8::1"}},
		{"v6 full", "2001:db8:0:0:0:0:0:1", []string{"host:2001:db8:0:0:0:0:0:1"}},
		{"v6 bracketed", "[2001:db8::1]:443", []string{"host:[2001:db8::1]:443"}},
		{"v6 bracketed no port", "[::1]", []string{"host:[::1]"}},
		{"v6 in prose", "bound to fe80::1, ok", []string{"host:fe80::1"}},
		{"v4-mapped v6", "::ffff:192.0.2.1", []string{"host:::ffff:192.0.2.1"}},
		{"v4-embedded v6", "64:ff9b::192.0.2.33 up", []string{"host:64:ff9b::192.0.2.33"}},
		{"v6 with prefix length", "fe80::1ff:fe23:4567:890a/64", []string{"host:fe80::1ff:fe23:4567:890a"}},
	})
}

func TestSHAHostInteraction(t *testing.T) {
	runHostCases(t, []scanCase{
		{"sha in a url path", "https://github.com/x/y/commit/3f2a9c1", []string{"url:https://github.com/x/y/commit/3f2a9c1"}},
		{"sha next to a host", "3f2a9c1 example.com", []string{"sha:3f2a9c1", "host:example.com"}},
		{"sha named like a domain label", "3f2a9c1.example.com", []string{"host:3f2a9c1.example.com"}},
	})
}

func TestOverlapAndLinks(t *testing.T) {
	runHostCases(t, []scanCase{
		{"url then bare host", "http://example.com/a example.com", []string{"url:http://example.com/a", "host:example.com"}},
		{"host inside url is not repeated", "https://sub.example.com/x", []string{"url:https://sub.example.com/x"}},
		{"scp host not repeated", "git@github.com:org/repo.git", []string{"url:git@github.com:org/repo.git"}},
		{"osc 8 link wins over the host in its text", "\x1b]8;;https://example.com/real\x1b\\example.com\x1b]8;;\x1b\\", []string{"url:example.com"}},
		{"osc 8 link wins over a longer path", "\x1b]8;;https://example.com/real\x1b\\example.com/path\x1b]8;;\x1b\\", []string{"url:example.com/path"}},
		{"hosts across a wrap", "see borders.xev\r\nion.dev now", []string{"host:borders.xevion.dev"}},
	})
}

func TestHostPathPrecedence(t *testing.T) {
	files := fakefs.New(
		"/home/u/proj/config.example",
		"/home/u/proj/app.dev",
		"/home/u/proj/site.com/index.html",
	)
	env := testEnv()
	env.Lstat, env.Readlink, env.ReadDir = files.Lstat, files.Readlink, files.ReadDir
	for _, tc := range []scanCase{
		{"existing file beats a host reading", "config.example app.dev", []string{"path:config.example", "path:app.dev"}},
		{"existing directory path beats a url reading", "site.com/index.html", []string{"path:site.com/index.html"}},
		{"missing sibling is a host", "other.dev", []string{"host:other.dev"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, m := range Scan(tc.in, resolve.New(env)) {
				got = append(got, render(m))
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Scan(%q) (-want +got):\n%s", tc.in, diff)
			}
		})
	}
}

func TestHostBesidePath(t *testing.T) {
	for _, tc := range []scanCase{
		{"path that ends like a domain", "/home/u/x.example.com", []string{"path:/home/u/x.example.com"}},
		{"path and host in prose", "(see /etc/hosts and example.com)", []string{"path:/etc/hosts", "host:example.com"}},
		{"url and path", "https://example.com/a /etc/hosts", []string{"url:https://example.com/a", "path:/etc/hosts"}},
		{"domain-named directory with a file", "~/src/xevion.dev/index.html", []string{"path:~/src/xevion.dev/index.html"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, m := range Scan(tc.in, resolve.New(testEnv())) {
				got = append(got, render(m))
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Scan(%q) (-want +got):\n%s", tc.in, diff)
			}
		})
	}
}

func TestFileURLStillPath(t *testing.T) {
	var got []string
	for _, m := range Scan("file:///home/u/x.html and file://host/etc/hosts", resolve.New(testEnv())) {
		got = append(got, render(m))
	}
	want := []string{"path:file:///home/u/x.html", "path:file://host/etc/hosts"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestReportedScreen(t *testing.T) {
	in := "see borders.xevion.dev and myhost.local nas.lan http://localhost:3000 localhost:8080 localhost http://devbox/ foo://bar.baz/q"
	want := []string{
		"host:borders.xevion.dev", "host:myhost.local", "host:nas.lan", "url:http://localhost:3000",
		"host:localhost:8080", "host:localhost", "url:http://devbox/", "url:foo://bar.baz/q",
	}
	if diff := cmp.Diff(want, hostScan(in)); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// Offsets must point at exactly the visible text, as in the main scan cases.
func TestHostOffsets(t *testing.T) {
	in := "a borders.xevion.dev, (docs.rs/serde) [x](foo://bar/q) localhost:8080/api 10.0.0.1/admin mailto:a@b.com\n" +
		"git@github.com:org/repo.git and [::1]:80/x"
	prev := 0
	for _, m := range Scan(in, resolve.New(testEnv())) {
		if m.Start < prev || m.End <= m.Start || m.End > len(in) {
			t.Fatalf("bad span [%d,%d) after %d for %q", m.Start, m.End, prev, m.Text)
		}
		if got := in[m.Start:m.End]; got != m.Text {
			t.Errorf("span %q != text %q", got, m.Text)
		}
		prev = m.End
	}
	if strings.Count(in, "\n") != 1 {
		t.Fatal("fixture changed")
	}
}

func TestWebURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    Match
		want string
		ok   bool
	}{
		{"url with scheme", Match{Kind: KindURL, Text: "https://x.com/a"}, "https://x.com/a", true},
		{"custom scheme", Match{Kind: KindURL, Text: "steam://run/1"}, "steam://run/1", true},
		{"mailto", Match{Kind: KindURL, Text: "mailto:a@b.com"}, "mailto:a@b.com", true},
		{"scp remote without a user has no web form", Match{Kind: KindURL, Text: "github.com:org/repo.git"}, "", false},
		{"scp remote has no web form", Match{Kind: KindURL, Text: "git@github.com:org/repo.git"}, "", false},
		{"schemeless public url", Match{Kind: KindURL, Text: "docs.rs/serde"}, "https://docs.rs/serde", true},
		{"schemeless local url", Match{Kind: KindURL, Text: "localhost:8080/api"}, "http://localhost:8080/api", true},
		{"schemeless ip url", Match{Kind: KindURL, Text: "10.0.0.1/admin"}, "http://10.0.0.1/admin", true},
		{"public domain", Match{Kind: KindHost, Text: "example.com"}, "https://example.com", true},
		{"local domain", Match{Kind: KindHost, Text: "nas.lan"}, "http://nas.lan", true},
		{"localhost", Match{Kind: KindHost, Text: "localhost"}, "http://localhost", true},
		{"domain with port", Match{Kind: KindHost, Text: "example.com:8080"}, "http://example.com:8080", true},
		{"tls port", Match{Kind: KindHost, Text: "example.com:443"}, "https://example.com:443", true},
		{"ip with port", Match{Kind: KindHost, Text: "10.0.0.1:80"}, "http://10.0.0.1:80", true},
		{"bare ip", Match{Kind: KindHost, Text: "10.0.0.1"}, "", false},
		{"cidr", Match{Kind: KindHost, Text: "10.0.0.0/8"}, "", false},
		{"bare ipv6", Match{Kind: KindHost, Text: "fe80::1"}, "", false},
		{"bracketed ipv6 with port", Match{Kind: KindHost, Text: "[::1]:3000"}, "http://[::1]:3000", true},
		{"sha", Match{Kind: KindSHA, Text: "3f2a9c1"}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := WebURL(tc.m)
			if got != tc.want || ok != tc.ok {
				t.Errorf("WebURL(%q) = (%q, %v), want (%q, %v)", tc.m.Text, got, ok, tc.want, tc.ok)
			}
		})
	}
}
