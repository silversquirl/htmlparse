package html

import "golang.org/x/net/html"

func NewNode(ty html.NodeType) *html.Node {
	return &html.Node{Type: ty}
}

const (
	ErrorNode    = html.ErrorNode
	TextNode     = html.TextNode
	DocumentNode = html.DocumentNode
	ElementNode  = html.ElementNode
	CommentNode  = html.CommentNode
	DoctypeNode  = html.DoctypeNode
	RawNode      = html.RawNode
)
