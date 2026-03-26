package output

import "fmt"

// TreeNode represents a node in a displayable tree.
type TreeNode struct {
	Label    string
	Children []TreeNode
}

// PrintTree prints a tree with box-drawing connectors.
func PrintTree(root TreeNode) {
	fmt.Println(root.Label)
	printTreeNodes(root.Children, "")
}

func printTreeNodes(nodes []TreeNode, prefix string) {
	for i, n := range nodes {
		conn, next := "├── ", "│   "
		if i == len(nodes)-1 {
			conn, next = "└── ", "    "
		}
		fmt.Printf("%s%s%s\n", prefix, conn, n.Label)
		printTreeNodes(n.Children, prefix+next)
	}
}
