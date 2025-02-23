package mirror

import (
	"golang.org/x/net/html"
	"io"
	"net/url"
	"path"
	"strings"
)

// Parser handles HTML parsing and link extraction
type Parser struct {
	baseURL      *url.URL
	config       *Config
	queue        *Queue
}

// NewParser creates a new Parser instance
func NewParser(baseURL string, config *Config, queue *Queue) (*Parser, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &Parser{
		baseURL: parsedURL,
		config:  config,
		queue:   queue,
	}, nil
}

// Parse processes an HTML document and extracts links
func (p *Parser) Parse(r io.Reader) error {
	doc, err := html.Parse(r)
	if err != nil {
		return err
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var attr string
			switch n.Data {
			case "a", "link":
				attr = "href"
			case "img", "script":
				attr = "src"
			}

			if attr != "" {
				for _, a := range n.Attr {
					if a.Key == attr {
						p.processURL(a.Val)
						break
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return nil
}

// processURL handles a discovered URL
func (p *Parser) processURL(rawURL string) {
	// Skip empty URLs and anchors
	if rawURL == "" || strings.HasPrefix(rawURL, "#") {
		return
	}

	// Parse the URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}

	// Make absolute URL if relative
	if !u.IsAbs() {
		u = p.baseURL.ResolveReference(u)
	}

	// Skip if different host
	if u.Host != p.baseURL.Host {
		return
	}

	// Check excluded paths
	for _, exclude := range p.config.ExcludePaths {
		if strings.HasPrefix(u.Path, exclude) {
			return
		}
	}

	// Check rejected file types
	ext := strings.ToLower(path.Ext(u.Path))
	if ext != "" {
		ext = ext[1:] // remove dot
		for _, reject := range p.config.RejectTypes {
			if ext == reject {
				return
			}
		}
	}

	// Add to queue if not processed
	p.queue.ProcessLock.RLock()
	if !p.queue.Processed[u.String()] {
		p.queue.ProcessLock.RUnlock()
		p.queue.ProcessLock.Lock()
		if !p.queue.Processed[u.String()] {
			p.queue.Processed[u.String()] = true
			p.queue.Resources <- Resource{
				URL:       u.String(),
				LocalPath: path.Join(p.config.OutputDir, u.Host, u.Path),
				IsHTML:    ext == "html" || ext == "htm",
			}
		}
		p.queue.ProcessLock.Unlock()
	} else {
		p.queue.ProcessLock.RUnlock()
	}
}
