package html

import (
	"bytes"
	"errors"
	"fmt"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func Parse(parent *html.Node, text []byte) error {
	_, err := parse(parent, text, true)
	return err
}

// TODO: position information in errors
func parse(parent *html.Node, text []byte, root bool) (rest []byte, err error) {
	for {
		idx := bytes.IndexByte(text, '<')
		if idx < 0 {
			break
		}

		// Process preceding text
		textNode(parent, text[:idx])
		text = text[idx:]

		if len(text) < 2 {
			return nil, errors.New("Unexpected end of file in opening tag")
		}

		switch text[1] {
		default:
			// Opening tag
			node, selfClosing, rest, err := parseStartTag(text)
			if err != nil {
				return nil, err
			}

			text = rest
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
				text, err = parseRaw(node, text, false)
			case catEscapableRaw:
				text, err = parseRaw(node, text, true)
			case catNormal, catTemplate, catForeign:
				text, err = parse(node, text, false)
			default:
				panic("Invalid category")
			}
			if err != nil {
				return nil, err
			}

		case '/':
			// Closing tag
			start := parent
			if root {
				start = nil
			}

			ok, rest, err := parseEndTag(start, text)
			if err != nil {
				return nil, err
			}

			if ok {
				return rest, nil
			} else {
				return nil, fmt.Errorf("Unclosed %q element", parent.Data)
			}

		case '!':
			text = text[2:]
			if len(text) == 0 {
				return nil, errors.New("Unexpected end of file in comment tag")
			}
			node := &html.Node{Type: html.CommentNode}
			if bytes.HasPrefix(text, []byte("--")) {
				// Well-formed comment
				text = text[2:]
				idx = bytes.Index(text, []byte("-->"))
				node.Data, text = string(text[:idx]), text[idx+3:]
			} else {
				doctype, _, rest := nextIdent(text)
				if doctype == "doctype" {
					// DOCTYPE
					text = skipSpace(rest)
					idx = bytes.IndexByte(text, '>')
					node.Type = html.DoctypeNode
					node.Data, text = string(text[:idx]), text[idx+1:]
				} else {
					// Malformed comment
					idx = bytes.IndexByte(text, '>')
					node.Data, text = string(text[:idx]), text[idx+1:]
				}
			}
			parent.AppendChild(node)
		}
	}

	if !root {
		return nil, fmt.Errorf("Unclosed %q element", parent.Data)
	}
	textNode(parent, text)
	return nil, nil
}

func parseRaw(parent *html.Node, text []byte, escapable bool) (rest []byte, err error) {
	buf := &bytes.Buffer{}
	for {
		idx := bytes.IndexByte(text, '<')
		if idx < 0 {
			return nil, fmt.Errorf("Unclosed %q element", parent.Data)
		}

		// Process preceding text
		buf.Write(text[:idx])
		text = text[idx:]

		if len(text) < 2 {
			return nil, errors.New("Unexpected end of file in opening tag")
		}

		if text[1] == '/' {
			// Check for a closing tag
			ok, rest, err := parseEndTag(parent, text)
			if err != nil {
				return nil, err
			}

			if ok {
				if escapable {
					textNode(parent, buf.Bytes())
				} else if buf.Len() > 0 {
					parent.AppendChild(&html.Node{Type: html.TextNode, Data: buf.String()})
				}
				return rest, nil
			}
		}

		buf.Write(text[:2])
		text = text[2:]
	}
}

func textNode(parent *html.Node, text []byte) {
	if len(text) > 0 {
		// XXX: this copies the text twice, would be nice to reduce that
		// Unfortunately, fixing that would require writing an HTML unescaper, which sounds not very fun
		// Either way, it's unlikely to be a problem unless a page has megabytes of uinterrupted text
		plainText := html.UnescapeString(string(text))
		parent.AppendChild(&html.Node{Type: html.TextNode, Data: plainText})
	}
}

func parseStartTag(text []byte) (node *html.Node, selfClosing bool, rest []byte, err error) {
	text = skipSpace(text[1:])
	elemS, elemA, text := nextIdent(text)
	if elemS == "" {
		return nil, false, nil, fmt.Errorf("Unexpected %q in opening tag", text[0])
	}

	// Construct node
	node = &html.Node{Type: html.ElementNode}
	node.Data = elemS
	node.DataAtom = elemA

	// Attributes
	text = skipSpace(text)
	for text[0] != '/' && text[0] != '>' {
		var name, val string
		// Name
		name, _, text = nextIdent(text)
		if name == "" {
			return nil, false, nil, fmt.Errorf("Unexpected %q in opening %q tag", text[0], node.Data)
		}

		// Value
		text = skipSpace(text)
		if text[0] == '=' {
			text = skipSpace(text[1:])
			val, text = nextValue(text)
		}
		text = skipSpace(text)

		// Construct attribute
		node.Attr = append(node.Attr, html.Attribute{
			Key: name,
			Val: val,
		})

		text = skipSpace(text)
	}

	if text[0] == '/' {
		selfClosing = true

		text = skipSpace(text[1:])
		if text[0] != '>' {
			return nil, false, nil, fmt.Errorf("Unexpected '/' in opening %q tag", node.Data)
		}
	}
	// Skip over '>'
	text = text[1:]

	return node, selfClosing, text, nil
}

func parseEndTag(start *html.Node, text []byte) (ok bool, rest []byte, err error) {
	text = text[2:]
	elemS, elemA, text := nextIdent(text)
	if elemS == "" {
		return false, nil, fmt.Errorf("Unexpected %q in closing tag", text[0])
	}
	if start == nil || elemA != start.DataAtom || (elemA == 0 && elemS != start.Data) {
		return false, nil, nil
	}

	text = skipSpace(text)
	if text[0] != '>' {
		return false, nil, fmt.Errorf("Unexpected %q in closing %q tag", text[0], elemS)
	}
	// Skip over '>'
	text = text[1:]

	return true, text, nil
}

const (
	wspc = " \t\n\f\r" // Whitespace characters

	identInvalid  = wspc + "\000\"'>/="  // Characters that are invalid in identifiers
	unquotInvalid = wspc + "\000\"'=<>`" // Characters that are invalid in unquoted values
)

func skipSpace(text []byte) []byte {
	return bytes.TrimLeft(text, wspc)
}

func nextIdent(text []byte) (string, atom.Atom, []byte) {
	idx := bytes.IndexAny(text, identInvalid)
	identB, text := text[:idx], text[idx:]
	if len(identB) == 0 {
		return "", 0, text
	}

	identB = bytes.ToLower(identB)
	identA := atom.Lookup(identB)
	identS := identA.String()
	if identA == 0 {
		identS = string(identB)
	}
	return identS, identA, text
}

func nextValue(text []byte) (string, []byte) {
	if text[0] == '\'' || text[0] == '"' {
		delim, text := text[0], text[1:]
		idx := bytes.IndexByte(text, delim)
		return string(text[:idx]), text[idx+1:]
	} else {
		idx := bytes.IndexAny(text, unquotInvalid)
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
