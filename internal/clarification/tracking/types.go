// Package tracking implements data types and operations for clarification tracking files.
package tracking

// TrackingFile represents the root structure of a clarification tracking YAML file.
type TrackingFile struct {
	Version     int     `yaml:"version" json:"version"`
	Created     string  `yaml:"created" json:"created"`
	LastUpdated string  `yaml:"last_updated" json:"last_updated"`
	Entries     []Entry `yaml:"entries" json:"entries"`
}

// Entry represents a single clarification entry in the tracking file.
type Entry struct {
	ID                string   `yaml:"id" json:"id"`
	CanonicalQuestion string   `yaml:"canonical_question" json:"canonical_question"`
	Variants          []string `yaml:"variants,omitempty" json:"variants,omitempty"`
	CurrentAnswer     string   `yaml:"current_answer" json:"current_answer"`
	Occurrences       int      `yaml:"occurrences" json:"occurrences"`
	FirstSeen         string   `yaml:"first_seen" json:"first_seen"`
	LastSeen          string   `yaml:"last_seen" json:"last_seen"`
	SprintsSeen       []string `yaml:"sprints_seen,omitempty" json:"sprints_seen,omitempty"`
	Status            string   `yaml:"status" json:"status"`
	ContextTags       []string `yaml:"context_tags,omitempty" json:"context_tags,omitempty"`
	Confidence        string   `yaml:"confidence" json:"confidence"`
	PromotedTo        string   `yaml:"promoted_to,omitempty" json:"promoted_to,omitempty"`
	PromotedDate      string   `yaml:"promoted_date,omitempty" json:"promoted_date,omitempty"`
}

// NewTrackingFile creates a new TrackingFile with default values.
func NewTrackingFile(created string) *TrackingFile {
	return &TrackingFile{
		Version:     1,
		Created:     created,
		LastUpdated: created,
		Entries:     []Entry{},
	}
}

// NewEntry creates a new Entry with the given ID, question, and answer.
func NewEntry(id, question, answer, date string) *Entry {
	return &Entry{
		ID:                id,
		CanonicalQuestion: question,
		Variants:          []string{},
		CurrentAnswer:     answer,
		Occurrences:       1,
		FirstSeen:         date,
		LastSeen:          date,
		SprintsSeen:       []string{},
		Status:            "pending",
		ContextTags:       []string{},
		Confidence:        "medium",
	}
}
