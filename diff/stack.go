package diff

import (
	"context"
	"fmt"
	"strings"
)

type stack struct {
	diffs []*diff
}

func (st *stack) dependantDiffs(ctx context.Context, d *diff) ([]*diff, error) {
	// find index of diff
	index := st.getIndex(d)

	if index == -1 {
		return nil, fmt.Errorf("cannot find diff in stack")
	}

	return st.diffs[index+1:], nil
}

func (st *stack) size() int {
	return len(st.diffs)
}

func (st *stack) getIndex(d *diff) int {
	index := -1
	for idx, v := range st.diffs {
		if v.id == d.id {
			index = idx
			break
		}
	}

	return index
}

func (st *stack) buildTable() (string, error) {
	var sb strings.Builder

	if len(st.diffs) <= 1 {
		return "", nil
	}

	sb.WriteString("### 📚 Stack\n\n")
	sb.WriteString("| PR | Title |\n")
	sb.WriteString("| -- | -- |\n")

	for _, diff := range st.diffs {
		// TODO get subject from PR description
		if diff.commit == "" {
			sb.WriteString(fmt.Sprintf("| #%s | %s |\n", "", "TODO"))
		} else {
			sb.WriteString(fmt.Sprintf("| #%s | %s |\n", "", diff.getSubject()))
		}
	}
	return sb.String(), nil
}

func newStackFromDiff(ctx context.Context, d *diff) (*stack, error) {
	if d.isSaved() == false {
		return nil, fmt.Errorf("can't create stack: diff hasn't been saved yet")
	}

	// find all parents
	var parents []*diff
	currDiff := d
	for {
		parent, err := currDiff.parentDiff(ctx)
		if err != nil {
			return nil, err
		}

		if parent == nil {
			break
		}

		parents = append(parents, parent)

		currDiff = parent

		// start loop again
	}

	// reverse parent list
	var reversedParents []*diff
	for i := len(parents) - 1; i >= 0; i-- {
		reversedParents = append(reversedParents, parents[i])
	}

	// find children
	var children []*diff
	currDiff = d

	for {
		child, err := currDiff.childDiff(ctx)
		if err != nil {
			return nil, err
		}

		if child == nil {
			break
		}

		children = append(children, child)

		currDiff = child

		// start loop again
	}

	var diffs []*diff
	diffs = append(diffs, reversedParents...)
	diffs = append(diffs, d)
	diffs = append(diffs, children...)

	// TODO: run through all diffs to make sure stack is consistent
	// e.g. if there are diffs that are missing commits in the middle of the stack
	// that means that they were removed or combined with other diffs. In that
	// case we'll have to change where the diffs are pointing to

	return &stack{
		diffs: diffs,
	}, nil
}
