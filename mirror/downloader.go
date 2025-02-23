package mirror

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Downloader handles the downloading of resources
type Downloader struct {
	config *Config
	client *http.Client
}

// NewDownloader creates a new Downloader instance
func NewDownloader(config *Config) *Downloader {
	return &Downloader{
		config: config,
		client: &http.Client{},
	}
}

// Download downloads resources from the queue
func (d *Downloader) Download(queue *Queue, workers int) error {
	var wg sync.WaitGroup
	errors := make(chan error, workers)

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for resource := range queue.Resources {
				if err := d.downloadResource(resource); err != nil {
					errors <- fmt.Errorf("error downloading %s: %v", resource.URL, err)
					return
				}
			}
		}()
	}

	// Wait for all downloads to complete
	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

// downloadResource downloads a single resource
func (d *Downloader) downloadResource(resource Resource) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(resource.LocalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Download the file
	resp, err := d.client.Get(resource.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received status code %d", resp.StatusCode)
	}

	// Create the file
	f, err := os.Create(resource.LocalPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Copy the content
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
