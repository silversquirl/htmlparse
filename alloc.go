// This file contains a custom allocator designed to reduce GC strain when freeing node trees
// Memory is allocated as a large block of sequential nodes, reducing the number of objects that need to be scanned

package html

import "golang.org/x/net/html"

const arenaSize = 64 // How many nodes to allocate at a time

type arena []html.Node

func (a *arena) newNode() *html.Node {
	if len(*a) == 0 {
		*a = make(arena, arenaSize)
	}
	n := &(*a)[0]
	*a = (*a)[1:]
	return n
}
