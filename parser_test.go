package html

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// Parse the test data, render it, then compare to the expected string.
// If expected == "", compare to source instead.
func testParseRen(t *testing.T, source, expected string) {
	t.Helper()

	doc := &html.Node{Type: html.DocumentNode}
	if err := Parse(doc, []byte(source)); err != nil {
		t.Error(err)
		return
	}

	b := &bytes.Buffer{}
	html.Render(b, doc)
	rendered := b.String()

	if expected == "" {
		expected = source
	}
	if rendered != expected {
		t.Errorf("Source and rendered do not match\nExpected: %q\nRendered: %q", expected, rendered)
	}
}

func TestParseElement(t *testing.T) {
	testParseRen(t, `<a>foo</a>`, "")
	testParseRen(t, `<dIv>foo</DiV>`, `<div>foo</div>`)
	testParseRen(t, `
		<html>
			<head>
				<title>Hello, world!</title>
			</head>
			<body>
				<h1>foo bar</h1>
			</body>
		</html>
	`, "")
}

func TestSiblings(t *testing.T) {
	testParseRen(t, `1<a>2</a>3`, "")
	testParseRen(t, `1<a>2<a>3</a></a><a></a>`, "")
}

func TestAttributes(t *testing.T) {
	testParseRen(t, `<a href=foo class=bar></a>`, `<a href="foo" class="bar"></a>`)
	testParseRen(t, `<a href='foo' class='bar baz'></a>`, `<a href="foo" class="bar baz"></a>`)
	testParseRen(t, `<a href="foo" class="bar baz"></a>`, "")
}

func TestEscape(t *testing.T) {
	testParseRen(t, `& &amp; " &#34; " &quot;`, `&amp; &amp; &#34; &#34; &#34; &#34;`)
	// HTML escapes are stupid btw
	testParseRen(t, `&amp &amp; &AMP; &alpha &alpha; &ALPHA;`, `&amp; &amp; &amp; &amp;alpha α &amp;ALPHA;`)
}

func TestVoid(t *testing.T) {
	void := `<area><base><br><col><embed><hr><img><input><link><meta><param><source><track><wbr>`
	voidSC := `<area/><base/><br/><col/><embed/><hr/><img/><input/><link/><meta/><param/><source/><track/><wbr/>`
	testParseRen(t, void, voidSC)
	testParseRen(t, voidSC, "")
}
func TestRawText(t *testing.T) {
	testParseRen(t, `<script>a<B>"c&dquot;</script>`, "")
	testParseRen(t, `<style>a<B>'c&squot;</style>`, "")
}
func TestEscapableRawText(t *testing.T) {
	testParseRen(t, `<textarea>a<B>"c&quot;</textarea>`, `<textarea>a&lt;B&gt;&#34;c&#34;</textarea>`)
	testParseRen(t, `<title>a<B>'c&apos;</title>`, `<title>a&lt;B&gt;&#39;c&#39;</title>`)
}

func TestComment(t *testing.T) {
	testParseRen(t, `<!--foobar->-->`, "")
	testParseRen(t, `<!-- hello world <!-->`, "")
}

func TestMalformedComment(t *testing.T) {
	testParseRen(t, `<![CDATA[hello world]]>`, `<!--[CDATA[hello world]]-->`)
}

func TestDoctype(t *testing.T) {
	testParseRen(t, `<!DOCTYPE html>`, "")
	testParseRen(t, `<!doctype html>`, `<!DOCTYPE html>`)
	testParseRen(t, `<!doctype html "foo bar">`, `<!DOCTYPE html "foo bar">`)
}

// Benchmarks
func benchmarkParser(b *testing.B, fun func(b *testing.B, source []byte)) {
	b.Helper()
	files, err := filepath.Glob("testdata/*.html")
	if err != nil {
		b.Fatal(err)
	}

	for _, filename := range files {
		f, err := os.Open(filename)
		if err != nil {
			b.Error(err)
			continue
		}

		source, err := ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			b.Error(err)
			continue
		}

		name := strings.TrimSuffix(filepath.Base(filename), ".html")
		b.Run(name, func(b *testing.B) {
			fun(b, source)
		})
	}

}

func BenchmarkVktec(b *testing.B) {
	benchmarkParser(b, func(b *testing.B, source []byte) {
		for i := 0; i < b.N; i++ {
			node := &html.Node{Type: html.DocumentNode}
			if err := Parse(node, source); err != nil {
				b.Error(err)
			}
		}
	})
}
func BenchmarkXNet(b *testing.B) {
	benchmarkParser(b, func(b *testing.B, source []byte) {
		for i := 0; i < b.N; i++ {
			r := bytes.NewReader(source)
			if _, err := html.Parse(r); err != nil {
				b.Error(err)
			}
		}
	})
}
