package htmlparse

import (
	"bytes"
	"errors"
	"fmt"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func Parse(parent *html.Node, text []byte) error {
	p := &parser{text: text}
	return p.parse(parent, true)
}

type parser struct {
	arena
	text     []byte
	lowerBuf []byte
}

// TODO: position information in errors
func (p *parser) parse(parent *html.Node, root bool) error {
	for {
		idx := bytes.IndexByte(p.text, '<')
		if idx < 0 {
			break
		}

		// Process preceding p.text
		p.textNode(parent, p.text[:idx])
		p.text = p.text[idx:]

		if len(p.text) < 2 {
			return errors.New("Unexpected end of file in opening tag")
		}

		switch p.text[1] {
		default:
			// Opening tag
			node, selfClosing, err := p.parseStartTag()
			if err != nil {
				return err
			}

			parent.AppendChild(node)

			if selfClosing {
				// Allow self-closing for any node type
				// This is not spec-compliant, but is normally fine and means we can mostly not worry about foreign nodes
				break
			}

			switch categorize(node.DataAtom) {
			case catVoid:
				// Do nothing
			case catRaw:
				err = p.parseRaw(node, false)
			case catEscapableRaw:
				err = p.parseRaw(node, true)
			case catNormal, catTemplate, catForeign:
				err = p.parse(node, false)
			default:
				panic("Invalid category")
			}
			if err != nil {
				return err
			}

		case '/':
			// Closing tag
			start := parent
			if root {
				start = nil
			}

			ok, err := p.parseEndTag(start)
			if err != nil {
				return err
			}

			if ok {
				return nil
			} else {
				return fmt.Errorf("Unclosed %q element", parent.Data)
			}

		case '!':
			p.text = p.text[2:]
			if len(p.text) == 0 {
				return errors.New("Unexpected end of file in comment tag")
			}
			node := p.newNode()
			node.Type = html.CommentNode
			if bytes.HasPrefix(p.text, []byte("--")) {
				// Well-formed comment
				p.text = p.text[2:]
				idx = bytes.Index(p.text, []byte("-->"))
				node.Data, p.text = string(p.text[:idx]), p.text[idx+3:]
			} else {
				doctype, _, rest := p.nextIdent(p.text)
				if doctype == "doctype" {
					// DOCTYPE
					p.text = skipSpace(rest)
					idx = bytes.IndexByte(p.text, '>')
					node.Type = html.DoctypeNode
					node.Data, p.text = string(p.text[:idx]), p.text[idx+1:]
				} else {
					// Malformed comment
					idx = bytes.IndexByte(p.text, '>')
					node.Data, p.text = string(p.text[:idx]), p.text[idx+1:]
				}
			}
			parent.AppendChild(node)
		}
	}

	if !root {
		return fmt.Errorf("Unclosed %q element", parent.Data)
	}
	p.textNode(parent, p.text)
	return nil
}

func (p *parser) parseRaw(parent *html.Node, escapable bool) error {
	buf := &bytes.Buffer{}
	for {
		idx := bytes.IndexByte(p.text, '<')
		if idx < 0 {
			return fmt.Errorf("Unclosed %q element", parent.Data)
		}

		// Process preceding p.text
		buf.Write(p.text[:idx])
		p.text = p.text[idx:]

		if len(p.text) < 2 {
			return errors.New("Unexpected end of file in opening tag")
		}

		if p.text[1] == '/' {
			// Check for a closing tag
			oldText := p.text
			ok, err := p.parseEndTag(parent)
			if err != nil {
				return err
			}

			if ok {
				if escapable {
					p.textNode(parent, buf.Bytes())
				} else if buf.Len() > 0 {
					node := p.newNode()
					node.Type = html.TextNode
					node.Data = buf.String()
					parent.AppendChild(node)
				}
				return nil
			} else {
				// Reset the text
				p.text = oldText
			}
		}

		buf.Write(p.text[:2])
		p.text = p.text[2:]
	}
}

func (p *parser) textNode(parent *html.Node, text []byte) {
	if len(text) > 0 {
		node := p.newNode()
		node.Type = html.TextNode
		node.Data = html.UnescapeString(string(text))
		parent.AppendChild(node)
	}
}

func (p *parser) parseStartTag() (node *html.Node, selfClosing bool, err error) {
	p.text = skipSpace(p.text[1:])
	elemS, elemA, rest := p.nextIdent(p.text)
	p.text = rest
	if elemS == "" {
		return nil, false, fmt.Errorf("Unexpected %q in opening tag", p.text[0])
	}

	// Construct node
	node = p.newNode()
	node.Type = html.ElementNode
	node.Data = elemS
	node.DataAtom = elemA

	// Attributes
	p.text = skipSpace(p.text)
	for p.text[0] != '/' && p.text[0] != '>' {
		var name, val string
		// Name
		name, _, p.text = p.nextIdent(p.text)
		if name == "" {
			return nil, false, fmt.Errorf("Unexpected %q in opening %q tag", p.text[0], node.Data)
		}

		// Value
		p.text = skipSpace(p.text)
		if p.text[0] == '=' {
			p.text = skipSpace(p.text[1:])
			val, p.text = p.nextValue(p.text)
		}
		p.text = skipSpace(p.text)

		// Construct attribute
		node.Attr = append(node.Attr, html.Attribute{
			Key: name,
			Val: val,
		})

		p.text = skipSpace(p.text)
	}

	if p.text[0] == '/' {
		selfClosing = true

		p.text = skipSpace(p.text[1:])
		if p.text[0] != '>' {
			return nil, false, fmt.Errorf("Unexpected '/' in opening %q tag", node.Data)
		}
	}
	// Skip over '>'
	p.text = p.text[1:]

	return node, selfClosing, nil
}

func (p *parser) parseEndTag(start *html.Node) (ok bool, err error) {
	p.text = p.text[2:]
	elemS, elemA, rest := p.nextIdent(p.text)
	p.text = rest
	if elemS == "" {
		return false, fmt.Errorf("Unexpected %q in closing tag", p.text[0])
	}
	if start == nil || elemA != start.DataAtom || (elemA == 0 && elemS != start.Data) {
		return false, nil
	}

	p.text = skipSpace(p.text)
	if p.text[0] != '>' {
		return false, fmt.Errorf("Unexpected %q in closing %q tag", p.text[0], elemS)
	}
	// Skip over '>'
	p.text = p.text[1:]

	return true, nil
}

func whitespaceF(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\f' || r == '\r'
}
func identInvalidF(r rune) bool {
	return whitespaceF(r) || r == 0 || r == '"' || r == '\'' || r == '=' || r == '/' || r == '>'
}
func unquotInvalidF(r rune) bool {
	return whitespaceF(r) || r == 0 || r == '"' || r == '\'' || r == '=' || r == '<' || r == '>'
}

func skipSpace(text []byte) []byte {
	return bytes.TrimLeftFunc(text, whitespaceF)
}

// asciiLower returns a copy of text with all uppercase ascii letters converted to lowercase.
// The returned slice is only valid until the next call to asciiLower.
func (p *parser) asciiLower(text []byte) []byte {
	if cap(p.lowerBuf) < len(text) {
		n := 2 * cap(p.lowerBuf)
		if n < len(text) {
			n = len(text)
		}
		p.lowerBuf = make([]byte, n)
	}
	p.lowerBuf = p.lowerBuf[:len(text)]

	for i, ch := range text {
		if 'A' <= ch && ch <= 'Z' {
			p.lowerBuf[i] = ch | 0x20
		} else {
			p.lowerBuf[i] = ch
		}
	}
	return p.lowerBuf
}

func (p *parser) nextIdent(text []byte) (string, atom.Atom, []byte) {
	idx := bytes.IndexFunc(text, identInvalidF)
	identB, text := text[:idx], text[idx:]
	if len(identB) == 0 {
		return "", 0, text
	}

	identB = p.asciiLower(identB)
	identA := atom.Lookup(identB)
	identS := identA.String()
	if identA == 0 {
		identS = string(identB)
	}
	return identS, identA, text
}

func (p *parser) nextValue(text []byte) (string, []byte) {
	if text[0] == '\'' || text[0] == '"' {
		delim, text := text[0], text[1:]
		idx := bytes.IndexByte(text, delim)
		return string(text[:idx]), text[idx+1:]
	} else {
		idx := bytes.IndexFunc(text, unquotInvalidF)
		return string(text[:idx]), text[idx:]
	}
}

type category int

const (
	catVoid category = iota
	catTemplate
	catRaw
	catEscapableRaw
	catForeign
	catNormal
)

func categorize(a atom.Atom) category {
	switch a {
	case atom.Area, atom.Base, atom.Br, atom.Col, atom.Embed, atom.Hr, atom.Img, atom.Input,
		atom.Link, atom.Meta, atom.Param, atom.Source, atom.Track, atom.Wbr:
		return catVoid
	case atom.Template:
		return catTemplate
	case atom.Script, atom.Style:
		return catRaw
	case atom.Textarea, atom.Title:
		return catEscapableRaw
	default:
		return catNormal
	}
}
