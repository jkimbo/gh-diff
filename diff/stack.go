package diff

import (
	"context"
	"fmt"
	"strings"
)

type Stack struct {
	diffs []*Diff
}

func (st *Stack) DependantDiffs(ctx context.Context, diff *Diff) ([]*Diff, error) {
	// find index of diff
	index := st.GetIndex(diff)

	if index == -1 {
		return nil, fmt.Errorf("cannot find diff in stack")
	}

	return st.diffs[index+1:], nil
}

func (st *Stack) Size() int {
	return len(st.diffs)
}

func (st *Stack) GetIndex(diff *Diff) int {
	index := -1
	for idx, d := range st.diffs {
		if d.ID == diff.ID {
			index = idx
			break
		}
	}

	return index
}

func (st *Stack) buildTable() (string, error) {
	var sb strings.Builder

	if len(st.diffs) <= 1 {
		return "", nil
	}

	sb.WriteString("### 📚 Stack\n\n")
	sb.WriteString("| PR | Title |\n")
	sb.WriteString("| -- | -- |\n")

	for _, diff := range st.diffs {
		// TODO get subject from PR description
		sb.WriteString(fmt.Sprintf("| #%s | %s |\n", "", diff.GetSubject()))
	}
	return sb.String(), nil
}

func NewStackFromDiff(ctx context.Context, diff *Diff) (*Stack, error) {
	if diff.IsSaved() == false {
		return nil, fmt.Errorf("can't create stack: diff hasn't been saved yet")
	}

	// find the first diff in the stack
	var parents []*Diff
	parent, err := diff.StackedOn(ctx)
	if err != nil {
		return nil, err
	}

	if parent != nil {
		commit := parent.GetCommit()
		if commit == "" {
			// if we can't find the commit for a diff then that diff is no longer part
			// of the stack and we should remove it
			err := client.db.RemoveDiff(ctx, parent.ID)
			if err != nil {
				return nil, err
			}
			fmt.Printf("removed diff %s\n", parent.ID)
		} else {
			parents = append(parents, parent)

			// keep looping to find all the parents
			for {
				parent, err = parent.StackedOn(ctx)
				if err != nil {
					return nil, err
				}
				if parent == nil {
					break
				}
				parents = append(parents, parent)
			}
		}
	}

	// reverse parent list
	var reversedParents []*Diff
	for i := len(parents) - 1; i >= 0; i-- {
		reversedParents = append(reversedParents, parents[i])
	}

	// find children
	var children []*Diff
	child, err := diff.ChildDiff(ctx)
	if err != nil {
		return nil, err
	}

	if child != nil {
		commit := child.GetCommit()
		if commit == "" {
			// if we can't find the commit for a diff then that diff is no longer part
			// of the stack and we should remove it
			err := client.db.RemoveDiff(ctx, child.ID)
			if err != nil {
				return nil, err
			}
			fmt.Printf("removed diff %s\n", child.ID)
		} else {
			children = append(children, child)

			// keep looping to find all the children
			for {
				child, err = child.ChildDiff(ctx)
				if err != nil {
					return nil, err
				}
				if child == nil {
					break
				}
				children = append(children, child)
			}
		}
	}

	var diffs []*Diff
	diffs = append(diffs, reversedParents...)
	diffs = append(diffs, diff)
	diffs = append(diffs, children...)

	return &Stack{
		diffs: diffs,
	}, nil
}
