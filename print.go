package littlealbert

import (
	"fmt"

	tp "github.com/xlab/treeprint"
)

// TreePrint renders the subtree as defined by the provided root
// Node like you would see in the tree command.
func TreePrint(root Node) string {
	tree := tp.New()

	p(root, tree)
	return tree.String()
}

func p(node Node, tree tp.Tree) {
	var label string
	switch node.(type) {
	case *conditional:
		label = "Conditional"
	case *task:
		label = "Task"
	case *sequence:
		label = "Sequence"
	case *fallback:
		label = "Fallback"
	case *parallel:
		label = "Parallel"
	case *decorator:
		v := node.(*decorator)
		if v.fn != nil {
			label = "Decorator"
		} else {
			label = "Label"
		}
	case *dynamic:
		label = "Dynamic"
	default:
		label = "Unknown Node"
	}

	if namer, ok := node.(NamedNode); ok && namer.Name() != "" {
		label += fmt.Sprintf(": %s", namer.Name())
	}

	parent, ok := node.(ParentNode)
	if ok {
		branch := tree.AddBranch(label)

		for _, child := range parent.Children() {
			p(child, branch)
		}

		return
	}

	tree.AddNode(label)
}
