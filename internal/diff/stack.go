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
	index := -1
	for idx, d := range st.diffs {
		if d.ID == diff.ID {
			index = idx
			break
		}
	}

	if index == -1 {
		return nil, fmt.Errorf("cannot find diff in stack")
	}

	return st.diffs[index+1:], nil
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

	var diffs []*Diff
	diffs = append(diffs, reversedParents...)
	diffs = append(diffs, diff)
	diffs = append(diffs, children...)

	var diffIDs []string
	for _, diff := range diffs {
		diffIDs = append(diffIDs, diff.ID)
	}
	fmt.Printf("stack: [%s]\n", strings.Join(diffIDs, ","))

	return &Stack{
		diffs: diffs,
	}, nil
}
