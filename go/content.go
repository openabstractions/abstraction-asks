package asks

import (
	"github.com/openabstractions/abstraction-asks/go/typedpermission"
	"slices"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
)

// retainedAnswer grows only the versioned private Book representation. The
// published AnswerEntry remains the unchanged projection of this audit row.
type retainedAnswer struct {
	wire.AnswerEntry
	Content *wire.CatalogueContent `json:"content,omitempty"`
}

func cloneCatalogueContent(content *wire.CatalogueContent) *wire.CatalogueContent {
	if content == nil {
		return nil
	}
	out := &wire.CatalogueContent{Slots: make(map[string]string, len(content.Slots))}
	for key, value := range content.Slots {
		out.Slots[key] = value
	}
	if content.Permission != nil {
		permission := *content.Permission
		permission.Targets = slices.Clone(permission.Targets)
		out.Permission = &permission
	}
	return out
}

// ValidCatalogueContent checks retained data against the catalogue definition,
// independently of the occurrence's rendered text. Permission remains the
// separate trusted runtime target, never a value inferred from display slots.
func ValidCatalogueContent(key, about string, content wire.CatalogueContent) bool {
	return typedpermission.ValidCatalogueContent(key, about, content, permissionValidators())
}

func validRecordContent(record Record) bool {
	if record.Content == nil {
		return true
	}
	if !ValidCatalogueContent(record.Key, record.About, *record.Content) {
		return false
	}
	if record.Key == FirstUseKey {
		return record.Permission != nil && slices.Equal(record.Permission.Targets, record.Content.Permission.Targets)
	}
	return true
}
