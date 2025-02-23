# Go Wget Clone

A wget-like utility implemented in Go that supports downloading files and mirroring websites.

## Features

- Download files from URLs (HTTP/HTTPS)
- Save files with custom names (-O flag)
- Save files to specific directories (-P flag)
- Download in background mode (-B flag)
- Limit download speed (--rate-limit flag)
- Download multiple files asynchronously (-i flag)
- Mirror websites (--mirror flag)
  - Reject specific file types (-R flag)
  - Exclude directories (-X flag)
  - Convert links for offline viewing (--convert-links flag)

## Usage

Basic download:
```bash
go run . https://example.com/file.zip
```

Download with custom name:
```bash
go run . -O=newname.zip https://example.com/file.zip
```

Download to specific directory:
```bash
go run . -P=/downloads -O=file.zip https://example.com/file.zip
```

Background download:
```bash
go run . -B https://example.com/file.zip
```

Rate limited download:
```bash
go run . --rate-limit=400k https://example.com/file.zip
```

Multiple file download:
```bash
go run . -i=downloads.txt
```

Mirror website:
```bash
go run . --mirror https://example.com
```

Mirror with filters:
```bash
go run . --mirror -R=jpg,gif -X=/assets,/css https://example.com
```
