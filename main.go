package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	
	"wget/mirror"
)

type Config struct {
	outputFile    string
	outputDir     string
	background    bool
	rateLimit     string
	rateBytes     int64 // bytes per second after parsing rateLimit
	inputFile     string
	mirror        bool
	reject        string
	exclude       string
	convertLinks  bool
}

type DownloadProgress struct {
	total     int64
	current   int64
	startTime time.Time
}

func (dp *DownloadProgress) Write(p []byte) (int, error) {
	n := len(p)
	dp.current += int64(n)
	dp.printProgress()
	return n, nil
}

func (dp *DownloadProgress) printProgress() {
	elapsed := time.Since(dp.startTime)
	speed := float64(dp.current) / elapsed.Seconds() / 1024 // KB/s
	
	if dp.total <= 0 {
		// Unknown total size
		fmt.Printf("\r %.2f KiB transferred at %.2f KiB/s",
			float64(dp.current)/1024,
			speed)
		return
	}
	
	percent := float64(dp.current) * 100 / float64(dp.total)
	
	// Create progress bar
	width := 50
	completed := int(float64(width) * float64(dp.current) / float64(dp.total))
	bar := strings.Repeat("=", completed) + strings.Repeat(" ", width-completed)
	
	// Calculate remaining time
	remainingBytes := dp.total - dp.current
	remainingTime := time.Duration(float64(remainingBytes) / (float64(dp.current) / elapsed.Seconds()) * float64(time.Second))
	if dp.current == dp.total {
		remainingTime = 0
	}
	
	fmt.Printf("\r %.2f KiB / %.2f KiB [%s] %.2f%% %.2f KiB/s %v",
		float64(dp.current)/1024,
		float64(dp.total)/1024,
		bar,
		percent,
		speed,
		remainingTime.Round(time.Second))
	
	if dp.current == dp.total {
		fmt.Println()
	}
}

func parseRateLimit(rateLimit string) (int64, error) {
	if rateLimit == "" {
		return 0, nil
	}
	
	rateLimit = strings.ToLower(rateLimit)
	multiplier := int64(1)
	
	if strings.HasSuffix(rateLimit, "k") {
		multiplier = 1024
		rateLimit = rateLimit[:len(rateLimit)-1]
	} else if strings.HasSuffix(rateLimit, "m") {
		multiplier = 1024 * 1024
		rateLimit = rateLimit[:len(rateLimit)-1]
	}
	
	rate, err := strconv.ParseInt(rateLimit, 10, 64)
	if err != nil {
		return 0, err
	}
	
	return rate * multiplier, nil
}

type rateLimitedReader struct {
	r        io.Reader
	rateBytes int64 // bytes per second
	lastRead time.Time
	bytesRead int64
}

func newRateLimitedReader(r io.Reader, rateBytes int64) io.Reader {
	return &rateLimitedReader{
		r:        r,
		rateBytes: rateBytes,
		lastRead: time.Now(),
	}
}

func (r *rateLimitedReader) Read(p []byte) (n int, err error) {
	if r.rateBytes <= 0 {
		return r.r.Read(p)
	}
	
	now := time.Now()
	expectedDuration := time.Duration(float64(r.bytesRead) / float64(r.rateBytes) * float64(time.Second))
	actualDuration := now.Sub(r.lastRead)
	
	if actualDuration < expectedDuration {
		time.Sleep(expectedDuration - actualDuration)
	}
	
	n, err = r.r.Read(p)
	r.bytesRead += int64(n)
	return
}

func downloadFile(url string, config Config) error {
	startTime := time.Now()
	fmt.Printf("start at %s\n", startTime.Format("2006-01-02 15:04:05"))

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("sending request, awaiting response... status %s\n", resp.Status)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	contentLength := resp.ContentLength
	fmt.Printf("content size: %d [~%.2fMB]\n", contentLength, float64(contentLength)/(1024*1024))

	fileName := config.outputFile
	if fileName == "" {
		fileName = filepath.Base(url)
	}
	
	if config.outputDir != "" {
		err = os.MkdirAll(config.outputDir, 0755)
		if err != nil {
			return err
		}
		fileName = filepath.Join(config.outputDir, fileName)
	}

	fmt.Printf("saving file to: %s\n", fileName)

	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer out.Close()

	progress := &DownloadProgress{
		total:     contentLength,
		startTime: time.Now(),
	}

	reader := io.TeeReader(resp.Body, progress)
	if config.rateBytes > 0 {
		reader = newRateLimitedReader(reader, config.rateBytes)
	}

	_, err = io.Copy(out, reader)
	if err != nil {
		return err
	}

	fmt.Printf("\nDownloaded [%s]\n", url)
	fmt.Printf("finished at %s\n", time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

func downloadMultipleFiles(inputFile string, config Config) error {
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urls = append(urls, scanner.Text())
	}

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			if err := downloadFile(url, config); err != nil {
				log.Printf("Error downloading %s: %v\n", url, err)
			}
		}(url)
	}
	wg.Wait()
	return nil
}

func main() {
	config := Config{}
	
	flag.StringVar(&config.outputFile, "O", "", "Output file name")
	flag.StringVar(&config.outputDir, "P", "", "Output directory")
	flag.BoolVar(&config.background, "B", false, "Download in background")
	flag.StringVar(&config.rateLimit, "rate-limit", "", "Rate limit (e.g., 400k)")
	flag.StringVar(&config.inputFile, "i", "", "Input file containing URLs")
	flag.BoolVar(&config.mirror, "mirror", false, "Mirror website")
	flag.StringVar(&config.reject, "R", "", "Reject file types")
	flag.StringVar(&config.exclude, "X", "", "Exclude directories")
	flag.BoolVar(&config.convertLinks, "convert-links", false, "Convert links for offline viewing")
	
	flag.Parse()
	
	// Parse rate limit
	if config.rateLimit != "" {
		rateBytes, err := parseRateLimit(config.rateLimit)
		if err != nil {
			fmt.Printf("Error parsing rate limit: %v\n", err)
			os.Exit(1)
		}
		config.rateBytes = rateBytes
	}

	if config.background {
		logFile, err := os.Create("wget-log")
		if err != nil {
			log.Fatal(err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
		fmt.Println("Output will be written to \"wget-log\".")
	}

	args := flag.Args()
	if len(args) == 0 && config.inputFile == "" {
		fmt.Println("Please provide a URL or use -i flag with an input file")
		os.Exit(1)
	}

	if config.mirror {
		// Convert reject and exclude flags to slices
		rejectTypes := []string{}
		if config.reject != "" {
			rejectTypes = strings.Split(config.reject, ",")
		}
		
		excludePaths := []string{}
		if config.exclude != "" {
			excludePaths = strings.Split(config.exclude, ",")
		}
		
		// Create mirror config
		mirrorConfig := &mirror.Config{
			URL:          args[0],
			RejectTypes:  rejectTypes,
			ExcludePaths: excludePaths,
			ConvertLinks: config.convertLinks,
			OutputDir:    config.outputDir,
		}
		
		// Create mirror instance
		m, err := mirror.New(mirrorConfig)
		if err != nil {
			log.Fatal(err)
		}
		
		// Start mirroring
		if err := m.Start(); err != nil {
			log.Fatal(err)
		}
		
		return
	}

	if config.inputFile != "" {
		if err := downloadMultipleFiles(config.inputFile, config); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := downloadFile(args[0], config); err != nil {
		log.Fatal(err)
	}
}
