package httprouter

import (
	"bytes"
	"fmt"
	"text/tabwriter"
)

type nodeDiagLine struct {
	priority  string
	treeRepr  string
	fullPath  string
	wildcard  string
	indices   string
	nodeType  string
	maxParams string
}

func (l *nodeDiagLine) tabString(pchar byte) string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s",
		l.priority,
		l.treeRepr,
		l.fullPath,
		l.wildcard,
		l.indices,
		l.nodeType,
		l.maxParams,
	)
}

func PrettyPrint(n *node) string {
	nodes := n.prettyPrint("", 0)

	tw := new(tabwriter.Writer)

	buf := new(bytes.Buffer)

	tw.Init(buf, 0, 8, 4, '\t', 0)

	fmt.Fprintln(tw, "Priority\tNode Tree\tRoute\tWildcard\tIndices\tType\tParams")

	for _, nd := range nodes {
		fmt.Fprintln(tw, nd.tabString('\t'))
	}
	tw.Flush()

	return fmt.Sprintf("%s", buf.String())
}

func (n *node) prettyPrint(prefix string, depth int) []*nodeDiagLine {
	out := make([]*nodeDiagLine, 1)

	out[0] = &nodeDiagLine{
		priority:  fmt.Sprintf("%v", n.priority),
		wildcard:  fmt.Sprintf("%v", n.wildChild),
		indices:   n.indices,
		nodeType:  readable(n.nType),
		maxParams: fmt.Sprintf("%v", n.maxParams),
	}

	// tree representation
	if len(n.children) > 0 {
		out[0].treeRepr = fmt.Sprintf("%s\\", n.path)
	} else {
		out[0].treeRepr = n.path
	}

	// route path, if applicable
	if n.handle != nil {
		out[0].fullPath = fmt.Sprintf("[%s%s]", prefix, n.path)
	}

	var whitespace string
	for i := 1; i < len(out[0].treeRepr); i++ {
		whitespace = whitespace + " "
	}

	for i, child := range n.children {
		nodes := child.prettyPrint(prefix+n.path, depth+1)
		branchChar := "├"
		optionalPipe := "|"
		if i == (len(n.children) - 1) {
			branchChar = "└"
			optionalPipe = " "
		}

		for j, nd := range nodes {
			if j == 0 {
				nd.treeRepr = whitespace + branchChar + nd.treeRepr
			} else {
				nd.treeRepr = whitespace + optionalPipe + nd.treeRepr
			}
		}

		out = append(out, nodes...)
	}
	return out
}

func readable(nt nodeType) string {
	switch nt {
	case static:
		return "static"
	case root:
		return "root"
	case param:
		return "param"
	case catchAll:
		return "catchAll"
	default:
		return "UNKNOWN"
	}
}
