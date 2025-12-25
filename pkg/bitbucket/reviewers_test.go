package bitbucket

import (
	"testing"
)

func TestReviewersDiffEmpty(t *testing.T) {
	prev := PullRequest{}
	curr := PullRequest{}

	added, removed := ReviewersDiff(prev, curr)

	if len(added) != 0 {
		t.Errorf("ReviewersDiff() added = %v, want %v", added, []string{})
	}
	if len(removed) != 0 {
		t.Errorf("ReviewersDiff() removed = %v, want %v", removed, []string{})
	}
}

func TestReviewersDiffAdded1(t *testing.T) {
	prev := PullRequest{}
	curr := PullRequest{
		Reviewers: []Account{
			{AccountID: "AAA"},
		},
	}

	added, removed := ReviewersDiff(prev, curr)
	if len(added) != 1 || added[0] != "AAA" {
		t.Errorf("ReviewersDiff() added = %v, want %v", added, []string{"AAA"})
	}
	if len(removed) != 0 {
		t.Errorf("ReviewersDiff() removed = %v, want %v", removed, []string{})
	}
}

func TestReviewersDiffAdded3(t *testing.T) {
	prev := PullRequest{}
	curr := PullRequest{
		Reviewers: []Account{
			{AccountID: "BBB"},
			{AccountID: "AAA"},
			{AccountID: "CCC"},
		},
	}

	added, removed := ReviewersDiff(prev, curr)
	if len(added) != 3 || added[0] != "AAA" || added[1] != "BBB" || added[2] != "CCC" {
		t.Errorf("ReviewersDiff() added = %v, want %v", added, []string{"AAA", "BBB", "CCC"})
	}
	if len(removed) != 0 {
		t.Errorf("ReviewersDiff() removed = %v, want %v", removed, []string{})
	}
}

func TestReviewersDiffRemoved1(t *testing.T) {
	prev := PullRequest{
		Reviewers: []Account{
			{AccountID: "AAA"},
		},
	}
	curr := PullRequest{}

	added, removed := ReviewersDiff(prev, curr)
	if len(added) != 0 {
		t.Errorf("ReviewersDiff() added = %v, want %v", added, []string{})
	}
	if len(removed) != 1 || removed[0] != "AAA" {
		t.Errorf("ReviewersDiff() removed = %v, want %v", removed, []string{"AAA"})
	}
}

func TestReviewersDiffRemoved3(t *testing.T) {
	prev := PullRequest{
		Reviewers: []Account{
			{AccountID: "BBB"},
			{AccountID: "AAA"},
			{AccountID: "CCC"},
		},
	}
	curr := PullRequest{}

	added, removed := ReviewersDiff(prev, curr)
	if len(added) != 0 {
		t.Errorf("ReviewersDiff() added = %v, want %v", added, []string{})
	}
	if len(removed) != 3 || removed[0] != "AAA" || removed[1] != "BBB" || removed[2] != "CCC" {
		t.Errorf("ReviewersDiff() removed = %v, want %v", removed, []string{"AAA", "BBB", "CCC"})
	}
}

func TestReviewersDiffMixed(t *testing.T) {
	prev := PullRequest{
		Reviewers: []Account{
			{AccountID: "AAA"},
			{AccountID: "BBB"},
		},
	}
	curr := PullRequest{
		Reviewers: []Account{
			{AccountID: "CCC"},
			{AccountID: "DDD"},
		},
	}

	added, removed := ReviewersDiff(prev, curr)

	if len(added) != 2 || added[0] != "CCC" || added[1] != "DDD" {
		t.Errorf("ReviewersDiff() added = %v, want %v", added, []string{"CCC", "DDD"})
	}
	if len(removed) != 2 || removed[0] != "AAA" || removed[1] != "BBB" {
		t.Errorf("ReviewersDiff() removed = %v, want %v", removed, []string{"AAA", "BBB"})
	}
}
