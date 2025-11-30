package files

import (
	"reflect"
	"testing"
)

func TestParseCodeOwnersFile(t *testing.T) {
	tests := []struct {
		name string
		file string
		want *CodeOwners
	}{
		{
			name: "empty_file",
		},
		{
			name: "comment",
			file: `# Ignore me`,
			want: &CodeOwners{
				Groups: map[string][]string{},
				Paths:  map[string][]string{},
				Users:  map[string]bool{},
			},
		},
		{
			name: "check",
			file: `Check(...)`,
			want: &CodeOwners{
				Groups: map[string][]string{},
				Paths:  map[string][]string{},
				Users:  map[string]bool{},
			},
		},
		{
			name: "flat_group",
			file: `
			@@@GroupA @"User 1" @"User 2"
			* @@GroupA
			`,
			want: &CodeOwners{
				PathList: []string{"**/*"},
				Groups:   map[string][]string{"@GroupA": {"User 1", "User 2"}},
				Paths:    map[string][]string{"**/*": {"User 1", "User 2"}},
				Users:    map[string]bool{"User 1": true, "User 2": true},
			},
		},
		{
			name: "nested_groups",
			file: `
			@@@GroupA @"User 1" @@GroupB
			@@@GroupB @@GroupC @"User 2" @@GroupD
			@@@GroupC @"User 3" @@GroupD
			@@@GroupD @"User 4" @"User 5"
			* @@GroupA
			`,
			want: &CodeOwners{
				PathList: []string{"**/*"},
				Groups: map[string][]string{
					"@GroupA": {"User 1", "User 2", "User 3", "User 4", "User 5"},
					"@GroupB": {"User 2", "User 3", "User 4", "User 5"},
					"@GroupC": {"User 3", "User 4", "User 5"},
					"@GroupD": {"User 4", "User 5"},
				},
				Paths: map[string][]string{"**/*": {"User 1", "User 2", "User 3", "User 4", "User 5"}},
				Users: map[string]bool{"User 1": true, "User 2": true, "User 3": true, "User 4": true, "User 5": true},
			},
		},
		{
			name: "ignore_paths",
			file: `
			!/ignored/path/file
			!ignored/path/{dir1,dir2}/
			!**/ignored/tree/**
			`,
			want: &CodeOwners{
				IgnoreList: []string{"/ignored/path/file", "**/ignored/path/{dir1,dir2}/**/*", "**/ignored/tree/**/*"},
				Groups:     map[string][]string{},
				Paths:      map[string][]string{},
				Users:      map[string]bool{},
			},
		},
		{
			name: "path_order",
			file: `
			/first/path/*  @"User 1"
			/second/path/* @"User 2"
			/third/path/*  @"User 3"
			`,
			want: &CodeOwners{
				PathList: []string{"/third/path/*", "/second/path/*", "/first/path/*"},
				Paths: map[string][]string{
					"/first/path/*":  {"User 1"},
					"/second/path/*": {"User 2"},
					"/third/path/*":  {"User 3"},
				},
				Groups: map[string][]string{},
				Users:  map[string]bool{"User 1": true, "User 2": true, "User 3": true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCodeOwnersFile(nil, tt.file, true)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCodeOwnersFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodeOwnersAllApproved(t *testing.T) {
	tests := []struct {
		name      string
		co        *CodeOwners
		approvers []string
		owners    []string
		want      bool
	}{
		{
			name:      "zero_owners_nonzero_approvers",
			co:        &CodeOwners{},
			approvers: []string{"user1", "user2"},
			owners:    []string{},
			want:      true,
		},
		{
			name:      "single_owner_approved_individual",
			co:        &CodeOwners{},
			approvers: []string{"user1", "user2"},
			owners:    []string{"user1"},
			want:      true,
		},
		{
			name: "single_owner_approved_partial_group",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
			}},
			approvers: []string{"user1"},
			owners:    []string{"@GroupA"},
			want:      true,
		},
		{
			name: "single_owner_approved_entire_group",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
			}},
			approvers: []string{"user1", "user2"},
			owners:    []string{"@GroupA"},
			want:      true,
		},
		{
			name:      "single_owner_not_approved_individual",
			co:        &CodeOwners{},
			approvers: []string{"user1", "user2"},
			owners:    []string{"user3"},
			want:      false,
		},
		{
			name: "single_owner_not_approved_entire_group",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
			}},
			approvers: []string{"user3", "user4"},
			owners:    []string{"@GroupA"},
			want:      false,
		},
		{
			name:      "multiple_owners_all_approved_individuals",
			co:        &CodeOwners{},
			approvers: []string{"user1", "user2", "user3"},
			owners:    []string{"user1", "user2"},
			want:      true,
		},
		{
			name: "multiple_owners_all_approved_partial_groups_1",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
				"@GroupB": {"user2", "user3"},
			}},
			approvers: []string{"user1", "user2"},
			owners:    []string{"@GroupA", "@GroupB"},
			want:      true,
		},
		{
			name: "multiple_owners_all_approved_partial_groups_2",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
				"@GroupB": {"user3", "user4"},
			}},
			approvers: []string{"user1", "user2", "user3"},
			owners:    []string{"@GroupA", "@GroupB"},
			want:      true,
		},
		{
			name: "multiple_owners_all_approved_entire_groups",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
				"@GroupB": {"user3", "user4"},
			}},
			approvers: []string{"user1", "user2", "user3", "user4"},
			owners:    []string{"@GroupA", "@GroupB"},
			want:      true,
		},
		{
			name:      "multiple_owners_some_approved_individuals",
			co:        &CodeOwners{},
			approvers: []string{"user1", "user2"},
			owners:    []string{"user2", "user3"},
			want:      false,
		},
		{
			name: "multiple_owners_some_approved_partial_groups_1",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
				"@GroupB": {"user3", "user4"},
			}},
			approvers: []string{"user1", "user3"},
			owners:    []string{"@GroupA", "@GroupB"},
			want:      true,
		},
		{
			name: "multiple_owners_some_approved_partial_groups_2",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
				"@GroupB": {"user3", "user4"},
			}},
			approvers: []string{"user1", "user2", "user3"},
			owners:    []string{"@GroupA", "@GroupB"},
			want:      true,
		},
		{
			name: "multiple_owners_some_approved_partial_groups_3",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
				"@GroupB": {"user3", "user4"},
			}},
			approvers: []string{"user1", "user2"},
			owners:    []string{"@GroupA", "@GroupB"},
			want:      false,
		},
		{
			name:      "multiple_owners_none_approved_individuals",
			co:        &CodeOwners{},
			approvers: []string{"user1", "user2"},
			owners:    []string{"user3", "user4"},
			want:      false,
		},
		{
			name: "multiple_owners_none_approved_groups",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA": {"user1", "user2"},
				"@GroupB": {"user3", "user4"},
			}},
			approvers: []string{"user5", "user6"},
			owners:    []string{"@GroupA", "@GroupB"},
			want:      false,
		},
		{
			name: "single_top_level_approval",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA":         {"user1", "user2"},
				"@GroupB":         {"user3", "user4"},
				"@FallbackOwners": {"user5", "user6"},
			}},
			approvers: []string{"user5"},
			owners:    []string{"@GroupA", "@GroupB", "user7", "user8"},
			want:      true,
		},
		{
			name: "all_top_level_approval",
			co: &CodeOwners{Groups: map[string][]string{
				"@GroupA":         {"user1", "user2"},
				"@GroupB":         {"user3", "user4"},
				"@FallbackOwners": {"user5", "user6"},
			}},
			approvers: []string{"user5", "user6"},
			owners:    []string{"@GroupA", "@GroupB", "user7", "user8"},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.co.allApproved(nil, tt.approvers, tt.owners, true)
			if got != tt.want {
				t.Errorf("allApproved() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodeOwnersGetOwners(t *testing.T) {
	tests := []struct {
		name    string
		co      *CodeOwners
		path    string
		want    []string
		wantErr bool
	}{
		{
			name:    "no_match",
			co:      &CodeOwners{},
			path:    "nonexistent/file.txt",
			want:    nil,
			wantErr: false,
		},
		{
			name: "single_absolute_match_basic",
			co: &CodeOwners{
				PathList: []string{
					"/aaa/bbb/ccc/ddd.txt",
				},
				Paths: map[string][]string{
					"/aaa/bbb/ccc/ddd.txt": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd.txt",
			want: []string{"user"},
		},
		{
			name: "single_absolute_match_with_dir_stars",
			co: &CodeOwners{
				PathList: []string{
					"/*/*/c*/ddd.txt",
				},
				Paths: map[string][]string{
					"/*/*/c*/ddd.txt": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd.txt",
			want: []string{"user"},
		},
		{
			name: "single_absolute_match_with_file_stars_1",
			co: &CodeOwners{
				PathList: []string{
					"/aaa/bbb/ccc/*",
				},
				Paths: map[string][]string{
					"/aaa/bbb/ccc/*": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd.txt",
			want: []string{"user"},
		},
		{
			name: "single_absolute_match_with_file_stars_2",
			co: &CodeOwners{
				PathList: []string{
					"/aaa/bbb/ccc/*.txt",
				},
				Paths: map[string][]string{
					"/aaa/bbb/ccc/*.txt": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd.txt",
			want: []string{"user"},
		},
		{
			name: "single_absolute_prefix_1",
			co: &CodeOwners{
				PathList: []string{
					"/aaa/bbb/ccc/**/*",
				},
				Paths: map[string][]string{
					"/aaa/bbb/ccc/**/*": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd.txt",
			want: []string{"user"},
		},
		{
			name: "single_absolute_prefix_2",
			co: &CodeOwners{
				PathList: []string{
					"/aaa/**/*",
				},
				Paths: map[string][]string{
					"/aaa/**/*": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd.txt",
			want: []string{"user"},
		},
		{
			name: "single_relative_match_in_root_1",
			co: &CodeOwners{
				PathList: []string{
					"**/aaa/bbb/ccc/ddd.txt",
				},
				Paths: map[string][]string{
					"**/aaa/bbb/ccc/ddd.txt": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd.txt",
			want: []string{"user"},
		},
		{
			name: "single_relative_match_in_root_2",
			co: &CodeOwners{
				PathList: []string{
					"**/ddd.txt",
				},
				Paths: map[string][]string{
					"**/ddd.txt": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd.txt",
			want: []string{"user"},
		},
		{
			name: "single_relative_match_doublestar_in_subdirs",
			co: &CodeOwners{
				PathList: []string{
					"**/ccc/**/*f*.txt",
				},
				Paths: map[string][]string{
					"**/ccc/**/*f*.txt": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd/eee/fff.txt",
			want: []string{"user"},
		},
		{
			name: "single_relative_match_with_options_1",
			co: &CodeOwners{
				PathList: []string{
					"**/ccc/{ddd,eee}/**/*",
				},
				Paths: map[string][]string{
					"**/ccc/{ddd,eee}/**/*": {"user"},
				},
			},
			path: "aaa/bbb/ccc/ddd/eee/fff.txt",
			want: []string{"user"},
		},
		{
			name: "single_relative_match_with_options_2",
			co: &CodeOwners{
				PathList: []string{
					"**/ccc/{ddd,eee}/**/*",
				},
				Paths: map[string][]string{
					"**/ccc/{ddd,eee}/**/*": {"user"},
				},
			},
			path: "aaa/bbb/ccc/eee/fff/ggg.txt",
			want: []string{"user"},
		},
		{
			name: "single_relative_match_with_options_3_no_match",
			co: &CodeOwners{
				PathList: []string{
					"**/ccc/{ddd,eee}/**/*",
				},
				Paths: map[string][]string{
					"**/ccc/{ddd,eee}/**/*": {"user"},
				},
			},
			path: "aaa/bbb/ccc/fff/ggg.txt",
			want: nil,
		},
		{
			name: "multiple_matches",
			co: &CodeOwners{
				PathList: []string{
					"**/aaa/**/*",
					"**/bbb.txt",
				},
				Paths: map[string][]string{
					"**/aaa/**/*": {"user1"},
					"**/bbb.txt":  {"user2"},
				},
			},
			path: "aaa/bbb.txt",
			want: []string{"user1"},
		},
		{
			name: "ignore_match_1",
			co: &CodeOwners{
				PathList: []string{
					"**/aaa/**/*",
				},
				IgnoreList: []string{
					"**/bbb/**/*",
				},
				Paths: map[string][]string{
					"**/aaa/**/*": {"user"},
				},
			},
			path: "aaa/bbb/ccc.txt",
			want: nil,
		},
		{
			name: "ignore_match_2",
			co: &CodeOwners{
				PathList: []string{
					"**/bbb/*",
				},
				IgnoreList: []string{
					"/{aaa,ddd}/**/*",
				},
				Paths: map[string][]string{
					"**/bbb/*": {"user"},
				},
			},
			path: "aaa/bbb/ccc.txt",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := tt.co.getOwners(tt.path)
			if (gotErr != nil) != tt.wantErr {
				t.Fatalf("getOwners() error: %v, wantErr %v", gotErr, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getOwners() = %q, want %q", got, tt.want)
			}
		})
	}
}
