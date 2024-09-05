package charxml

import "fmt"

type GMLevel uint32

const (
	GMCivilian = GMLevel(iota)
	GMForumModerator
	GMJuniorModerator
	GMModerator
	GMSeniorModerator
	GMLeadModerator
	GMJuniorDeveloper
	GMInactiveDeveloper
	GMDeveloper
	GMOperator
)

func (gmLevel GMLevel) String() string {
	switch gmLevel {
	case GMCivilian:
		return "Civilian"
	case GMForumModerator:
		return "Forum Moderator"
	case GMJuniorModerator:
		return "Junior Moderator"
	case GMModerator:
		return "Moderator"
	case GMSeniorModerator:
		return "Senior Moderator"
	case GMLeadModerator:
		return "Lead Moderator"
	case GMJuniorDeveloper:
		return "Junior Developer"
	case GMInactiveDeveloper:
		return "Inactive Developer"
	case GMDeveloper:
		return "Developer"
	case GMOperator:
		return "Operator"
	default:
		return fmt.Sprintf("GMLevel(%d)", gmLevel)
	}
}
