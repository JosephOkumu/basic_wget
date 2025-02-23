package mirror

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// Converter handles link conversion for offline viewing
type Converter struct {
	baseURL *url.URL
	config  *Config
}

// NewConverter creates a new Converter instance
func NewConverter(baseURL string, config *Config) (*Converter, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &Converter{
		baseURL: parsedURL,
		config:  config,
	}, nil
}

// ConvertLinks converts links in HTML files for offline viewing
func (c *Converter) ConvertLinks(filePath string) error {
	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Parse HTML
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return err
	}

	// Convert links
	c.convertNode(doc, filepath.Dir(filePath))

	// Write back to file
	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return err
	}

	return os.WriteFile(filePath, buf.Bytes(), 0644)
}

// convertNode recursively processes HTML nodes and converts links
func (c *Converter) convertNode(n *html.Node, basePath string) {
	if n.Type == html.ElementNode {
		var attr string
		switch n.Data {
		case "a", "link":
			attr = "href"
		case "img", "script":
			attr = "src"
		}

		if attr != "" {
			for i, a := range n.Attr {
				if a.Key == attr {
					if newPath := c.convertPath(a.Val, basePath); newPath != "" {
						n.Attr[i].Val = newPath
					}
				}
			}
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.convertNode(child, basePath)
	}
}

// convertPath converts a URL to a relative path for offline viewing
func (c *Converter) convertPath(rawURL string, basePath string) string {
	// Skip empty URLs, anchors, and absolute URLs to other domains
	if rawURL == "" || strings.HasPrefix(rawURL, "#") {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Handle absolute URLs
	if u.IsAbs() {
		if u.Host != c.baseURL.Host {
			return ""
		}
		// Convert to path relative to the base directory
		return filepath.Join(c.config.OutputDir, u.Host, u.Path)
	}

	// Handle relative URLs
	return filepath.Join(basePath, u.Path)
}
