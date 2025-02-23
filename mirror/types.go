package mirror

import "sync"

// Config holds the configuration for website mirroring
type Config struct {
	URL          string   // Base URL to mirror
	RejectTypes  []string // File extensions to reject (-R flag)
	ExcludePaths []string // Paths to exclude (-X flag)
	ConvertLinks bool     // Whether to convert links for offline viewing
	OutputDir    string   // Directory to save mirrored content
}

// Resource represents a web resource to be downloaded
type Resource struct {
	URL         string
	LocalPath   string
	ContentType string
	IsHTML      bool
}

// Queue represents a download queue for resources
type Queue struct {
	Resources   chan Resource
	Processed   map[string]bool
	ProcessLock sync.RWMutex
}

// NewQueue creates a new download queue
func NewQueue() *Queue {
	return &Queue{
		Resources:   make(chan Resource, 1000),
		Processed:   make(map[string]bool),
		ProcessLock: sync.RWMutex{},
	}
}
