package output

import "testing"

func TestPrintTree(t *testing.T) {
	root := TreeNode{
		Label: "Root",
		Children: []TreeNode{
			{Label: "Child 1", Children: []TreeNode{
				{Label: "Grandchild"},
			}},
			{Label: "Child 2"},
		},
	}
	// Verify it doesn't panic; output goes to stdout.
	DefaultPrinter().PrintTree(root)
}

func TestPrintTreeSingleNode(t *testing.T) {
	DefaultPrinter().PrintTree(TreeNode{Label: "Only root"})
}
