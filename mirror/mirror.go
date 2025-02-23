package mirror

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
)

// Mirror handles the website mirroring process
type Mirror struct {
	config     *Config
	parser     *Parser
	downloader *Downloader
	converter  *Converter
	queue      *Queue
}

// New creates a new Mirror instance
func New(config *Config) (*Mirror, error) {
	// Parse the base URL
	baseURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, err
	}

	// Create output directory if it doesn't exist
	if config.OutputDir == "" {
		config.OutputDir = baseURL.Host
	}
	
	// Create queue
	queue := NewQueue()

	// Create components
	parser, err := NewParser(config.URL, config, queue)
	if err != nil {
		return nil, err
	}

	downloader := NewDownloader(config)
	
	converter, err := NewConverter(config.URL, config)
	if err != nil {
		return nil, err
	}

	return &Mirror{
		config:     config,
		parser:     parser,
		downloader: downloader,
		converter:  converter,
		queue:      queue,
	}, nil
}

// Start begins the mirroring process
func (m *Mirror) Start() error {
	// Create initial resource
	initialResource := Resource{
		URL:       m.config.URL,
		LocalPath: path.Join(m.config.OutputDir, path.Base(m.config.URL)),
		IsHTML:    true,
	}

	// Add to queue
	m.queue.Resources <- initialResource
	m.queue.Processed[m.config.URL] = true

	// Start download workers
	var wg sync.WaitGroup
	wg.Add(1)
	
	// Process queue
	go func() {
		defer wg.Done()
		defer close(m.queue.Resources)

		for resource := range m.queue.Resources {
			// Download the resource
			if err := m.downloader.downloadResource(resource); err != nil {
				fmt.Printf("Error downloading %s: %v\n", resource.URL, err)
				continue
			}

			// If it's HTML, parse it for more links
			if resource.IsHTML {
				f, err := os.Open(resource.LocalPath)
				if err != nil {
					fmt.Printf("Error opening %s: %v\n", resource.LocalPath, err)
					continue
				}

				if err := m.parser.Parse(f); err != nil {
					fmt.Printf("Error parsing %s: %v\n", resource.LocalPath, err)
				}
				f.Close()

				// Convert links if needed
				if m.config.ConvertLinks {
					if err := m.converter.ConvertLinks(resource.LocalPath); err != nil {
						fmt.Printf("Error converting links in %s: %v\n", resource.LocalPath, err)
					}
				}
			}
		}
	}()

	// Wait for completion
	wg.Wait()

	return nil
}

// processURL normalizes and validates a URL
func (m *Mirror) processURL(rawURL string) (*url.URL, error) {
	if rawURL == "" || strings.HasPrefix(rawURL, "#") {
		return nil, fmt.Errorf("invalid URL")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(m.config.URL)
	if err != nil {
		return nil, err
	}

	if !u.IsAbs() {
		u = baseURL.ResolveReference(u)
	}

	return u, nil
}
