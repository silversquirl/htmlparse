package html

import (
	"bytes"
	"fmt"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func Parse(root *html.Node, text []byte) error {
	_, err := parse(root, text, true)
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
		text = skipSpace(text[idx+1:])

		var elemS string
		var elemA atom.Atom
		switch text[0] {
		default:
			// Opening tag
			elemS, elemA, text = nextIdent(text)
			if elemS == "" {
				return nil, fmt.Errorf("Unexpected %q in opening tag", text[0])
			}

			// Construct node
			node := &html.Node{Type: html.ElementNode}
			node.Data = elemS
			node.DataAtom = elemA
			parent.AppendChild(node)

			// Attributes
			text = skipSpace(text)
			for text[0] != '>' {
				var name, val string
				// Name
				name, _, text = nextIdent(text)
				if name == "" {
					return nil, fmt.Errorf("Unexpected %q in opening %q tag", text[0], node.Data)
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
			// Skip over '>'
			text = text[1:]

			text, err = parse(node, text, false)
			if err != nil {
				return nil, err
			}

		case '/':
			// Closing tag
			text = text[1:]
			elemS, elemA, text = nextIdent(text)
			if elemS == "" {
				return nil, fmt.Errorf("Unexpected %q in closing tag", text[0])
			}
			if root || elemA != parent.DataAtom || (elemA == 0 && elemS != parent.Data) {
				return nil, fmt.Errorf("Unexpected closing %q tag", elemS)
			}

			text = skipSpace(text)
			if text[0] != '>' {
				return nil, fmt.Errorf("Unexpected %q in closing %q tag", text[0], elemS)
			}
			// Skip over '>'
			text = text[1:]

			return text, nil
		}
	}

	if !root {
		return nil, fmt.Errorf("Unclosed %q tag", parent.Data)
	}
	textNode(parent, text)
	return nil, nil
}

func textNode(root *html.Node, text []byte) {
	if len(text) > 0 {
		root.AppendChild(&html.Node{Type: html.TextNode, Data: string(text)})
	}
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
