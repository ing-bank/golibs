package access

import (
	"fmt"
	"strings"

	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/slices"
)

// Node is a tree where all nodes have a label.
// It maps scoping labels of any depth, e.g. [CREATE, TEAM1, DEV] has a depth of 3.
// Use Insert to populate Nodes, and Find to check existence of labels. It's important
// to always perform Insert and Find operations on the root Node, and always keep the
// same order of the labels otherwise they won't match. Node is capable of understanding
// Wildcard labels.
type Node map[string]Node

func (n Node) Insert(labels []string) Node {
	if len(labels) == 0 {
		return n
	}
	label := labels[0]

	// If we already know this label, just return known Node
	if node, ok := n[label]; ok {
		return node.Insert(labels[1:])
	}

	if len(labels) == 1 { // Last label, we can safely condense wildcards
		// Node already has wildcard, matches any label
		if node, ok := n[scope.Wildcard]; ok {
			return node // We can just return this node since we have no other labels
		}
	}

	// New label, create a new Node
	node := make(Node)

	// Label is wildcard, but we already have non-wildcard registered labels
	// We can safely consolidate into wildcard if this is the last label
	if label == scope.Wildcard && len(labels) == 1 && len(n) > 0 {
		for child, _ := range n {
			delete(n, child) // Remove children to force garbage collection
		}
		n[label] = node
	}

	n[label] = node // Save this Node
	return node.Insert(labels[1:])
}

// Find tries to find the sequence of provided labels in the given node tree. Each label
// corresponds with a depth of the tree. Find is capable of understanding wildcards.
func (n Node) Find(labels []string) bool {
	if len(labels) == 0 {
		return true
	}

	// Each label represents a "depth" of the Node tree
	label := labels[0]

	// Find target label, continue searching recursively
	if node, ok := n[label]; ok && node.Find(labels[1:]) {
		return true
	}

	// We couldn't find this specific label, maybe there's a wildcard
	if node, ok := n[scope.Wildcard]; ok && node.Find(labels[1:]) { // Check for wildcard access
		return true
	}

	return false
}

// Print prints the Node. DepthOpt is used for recursive depth printing and should not be filled
// in by clients (hence why it is variadic).
func (n Node) Print(depthOpt ...int) {
	depth := opt.Opt(0, depthOpt)
	if depth == 0 {
		fmt.Println("--- Node ---")
	}

	labels, values := slices.MapItems(n)
	for i, label := range labels {
		padding := strings.Repeat("-", depth)
		fmt.Printf("%s%s\n", padding, label)
		values[i].Print(depth + 1)
	}
}
