package slack

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/pkg/data"
	"github.com/tzrikka/revchat/pkg/users"
)

func explainCodeOwners(paths []string, owners, groups map[string][]string, approvers map[string]bool) string {
	var msg strings.Builder
	msg.WriteString(":mag_right: Code owners per file in this PR:")

	for _, p := range paths {
		msg.WriteString(fmt.Sprintf("\n\n  •  `%s`", p))
		fileOwners := owners[p]
		if len(fileOwners) == 0 {
			msg.WriteString("\n          ◦   (No code owners found)")
			continue
		}

		// First set of bullets: direct owners.
		var nestedOwners []string
		for _, owner := range fileOwners {
			msg.WriteString("\n          ◦   " + ownerMention(owner, approvers))
			if strings.HasPrefix(owner, "@") {
				msg.WriteString(" - ")
				for i, member := range groups[owner] {
					if i > 0 {
						msg.WriteString(", ")
					}
					msg.WriteString(ownerMention(member, approvers))
					if strings.HasPrefix(member, "@") && !slices.Contains(nestedOwners, member) {
						nestedOwners = append(nestedOwners, member)
					}
				}
			}
		}

		// Expand every group found above, until only individual users remain.
		for i := 0; i < len(nestedOwners); i++ {
			group := nestedOwners[i]
			for _, member := range groups[group] {
				if strings.HasPrefix(member, "@") && !slices.Contains(nestedOwners, member) {
					nestedOwners = append(nestedOwners, member)
				}
			}
		}

		// Second set of bullets: nested groups expanded.
		for _, group := range nestedOwners {
			msg.WriteString(fmt.Sprintf("\n          ◦   %s - ", strings.TrimPrefix(group, "@")))
			for i, member := range groups[group] {
				if i > 0 {
					msg.WriteString(", ")
				}
				msg.WriteString(ownerMention(member, approvers))
			}
		}
	}

	// Last (optional) bullet: other approvers who are not code owners.
	for a, marked := range approvers {
		if marked {
			delete(approvers, a)
		}
	}
	if len(approvers) == 0 {
		return msg.String()
	}

	msg.WriteString("\n\n  •  Other approvers who are not code owners")
	msg.WriteString("\n          ◦   ")

	mentions := slices.Collect(maps.Keys(approvers))
	slices.Sort(mentions)
	for i, m := range mentions {
		if i > 0 {
			msg.WriteString(", ")
		}
		msg.WriteString(m + " :+1:")
	}

	return msg.String()
}

func ownerMention(owner string, approvers map[string]bool) string {
	if name, isGroup := strings.CutPrefix(owner, "@"); isGroup {
		return name
	}

	plusOne := ""
	if _, approved := approvers[owner]; approved {
		approvers[owner] = true
		plusOne = " :+1:"
	}

	user, err := data.SelectUserByRealName(owner)
	if err != nil || user.SlackID == "" {
		return owner + plusOne
	}

	mention := fmt.Sprintf("<@%s>", user.SlackID)
	if _, approved := approvers[mention]; approved {
		approvers[mention] = true
		plusOne = " :+1:"
	}

	return mention + plusOne
}

func approversForExplain(ctx workflow.Context, pr map[string]any) map[string]bool {
	participants, ok := pr["participants"].([]any)
	if !ok {
		return nil
	}

	mentions := map[string]bool{}
	for _, p := range participants {
		participant, ok := p.(map[string]any)
		if !ok {
			continue
		}
		approved, ok := participant["approved"].(bool)
		if !ok || !approved {
			continue
		}

		user, ok := participant["user"].(map[string]any)
		if !ok {
			continue
		}
		accountID, ok := user["account_id"].(string)
		if !ok {
			continue
		}

		mentions[users.BitbucketToSlackRef(ctx, accountID, "")] = false
	}

	return mentions
}
