package template

import "github.com/sizolity/worldline/world/store"

type WorldTemplate = store.WorldTemplate

var ApplyTemplate = store.ApplyTemplate

var Templates = map[string]WorldTemplate{
	"fantasy": fantasyTemplate(),
	"scifi":   scifiTemplate(),
	"modern":  modernTemplate(),
	"mystery": mysteryTemplate(),
}

func TemplateNames() []string {
	return []string{"fantasy", "modern", "mystery", "scifi"}
}
